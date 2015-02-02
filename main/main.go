package main

import (
	"./db"
	"./remote"
	"./structs"

	"container/list"
	"database/sql"
    "encoding/json"
	"flag"
	"fmt"
	"github.com/lib/pq/hstore"
	"log"
	"net/http"
    "strconv"
	"time"
)

var verbose bool
var debug bool
var urls map[string]string
var client http.Client

func processVideo(row structs.Video, c chan int) {

	if debug {
		fmt.Println("\tProcessing #", row.Id, " ", row.Youtube_id)
	} else if verbose {
		fmt.Print(".")
	}
	requests := 0

	// get amara id

	var amara_id string
	if row.Amara_id.Valid {
		amara_id = row.Amara_id.String
	} else {
		// fmt.Println("\tupdating amara_id")

		raw, err := remote.Fetch(client, fmt.Sprintf(urls["amaraId"], row.Youtube_id))
		requests++
		if err != nil {
			fmt.Println("\tfailed to fetch, will retry next bach")

			c <- requests
			return
		}

		var result structs.AmaraResult
		err = json.Unmarshal([]byte(raw), &result)
		if err != nil {
			log.Fatal("Failed to parse json:", err)
		}

		if len(result.Objects) == 0 {
			if debug {
				fmt.Println("\tamara_id not found")
			} else if verbose {
				fmt.Print("A")
			}
            err = db.SkipVideo(row.Id)
			if err != nil {
				log.Fatal("Failed to skip video:", err)
			}

			c <- requests
			return
		}

		amara_id = result.Objects[0].Id
        err = db.UpdateVideo(row.Id, amara_id)
		if err != nil {
			log.Fatal("Failed to set amara_id:", err)
		}
	}

	// get revisions

	// fmt.Println("\tgetting revisions")
	raw, err := remote.Fetch(client, fmt.Sprintf(urls["amaraRevisions"], amara_id))
    requests++
    if err != nil {
        c <- requests
        return
    }

	var result structs.AmaraRevisionsResult
	err = json.Unmarshal([]byte(raw), &result)
	if err != nil {
		if debug {
			fmt.Println("Failed to parse revisions json, will retry next batch")
		} else if verbose {
			fmt.Print("F")
		}

		c <- requests
		return
	}

	count := 0
	csubs := make(chan int)
	for _, wrapper := range result.Objects {
		// fmt.Println(wrapper.Language_code)
		countStr := row.Revisions.Map[wrapper.Language_code]
		var lastRevision = 0
		if countStr.Valid {
			lastRevision, err = strconv.Atoi(countStr.String)
			if err != nil {
				lastRevision = 0
			}
		}

		if row.Revisions.Map == nil {
			row.Revisions.Map = make(map[string]sql.NullString)
		}
		row.Revisions.Map[wrapper.Language_code] = sql.NullString{strconv.Itoa(len(wrapper.Versions)), true}

		for _, revision := range wrapper.Versions {
			if revision.Version_no <= lastRevision {
				// fmt.Println("\tskip", wrapper.Language_code, revision.Version_no)
				continue
			}
			// fmt.Println("\tdownload", wrapper.Language_code, revision.Version_no)

			// download subs for this version

			count++
			go saveSubtitles(row.Id, amara_id, wrapper, revision, csubs)
		}
	}

	for i := 0; i < count; i++ {
		// block until all subs are downloaded
		requests += <-csubs
	}

	err = db.UpdateVideoRevisions(row.Id, row.Revisions)
	if err != nil {
		log.Fatal("Failed to update video:", err)
	}

	c <- requests
}

func saveSubtitles(id int, amara_id string, wrapper structs.AmaraRevisionWrapper, revision structs.AmaraRevision, csubs chan int) {
	srt, err := remote.Fetch(client, fmt.Sprintf(urls["amaraSrt"], amara_id, wrapper.Language_code, revision.Version_no))
	if err != nil {
		log.Fatal(err)
	}

	var content hstore.Hstore
	content.Map = make(map[string]sql.NullString)

	content.Map["title"] = sql.NullString{wrapper.Title, true}
	content.Map["description"] = sql.NullString{wrapper.Description, true}
	content.Map["srt"] = sql.NullString{srt, true}

	err = db.AddRevision(id, wrapper.Language_code, revision.Version_no, revision.Author, content)
	if err != nil {
		log.Fatal("Failed to save revision:", err)
	}

    csubs <- 1
}

func main() {
	flag.BoolVar(&verbose, "v", false, "verbose mode")
	flag.BoolVar(&debug, "vv", false, "debug mode")
	flag.Parse()

	db.Init("mikulas", "mikulas", "report", 5432)
	defer db.Close()

	client = http.Client{
		Timeout: time.Duration(60 * time.Second),
	}

	urls = map[string]string{
		"amaraId":        "https://www.amara.org/api2/partners/videos/?format=json&video_url=http%%3A%%2F%%2Fwww.youtube.com%%2Fwatch%%3Fv%%3D%v",
		"amaraRevisions": "http://www.amara.org/api2/partners/videos/%v/languages/?limit=120&format=json",
		"amaraSrt":       "http://www.amara.org/api2/partners/videos/%v/languages/%v/subtitles?format=srt&version=%v",
	}

	videos := list.New()
	c := make(chan int)
	concurrency := 100
	count := 0
	start := time.Now()
	requests := 1
	var elapsed time.Duration

	for {
		for count < concurrency {
			if videos.Len() == 0 {
				if count != 0 {
					elapsed = time.Now().Sub(start)
					fmt.Printf("\nrequests %v, per request %v, elapsed %v\n", requests, time.Duration(int(elapsed)/requests), elapsed)
				}

				db.FetchVideos(func(video structs.Video) {
					videos.PushBack(video)
				}, 10000)
			}

			video := videos.Remove(videos.Front())
			count++
            go processVideo(video.(structs.Video), c)
		}

		requests += <-c
		count--
	}
}
