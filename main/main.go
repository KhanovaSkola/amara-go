package main

import (
    "./app"
	"./db"
    "./out"
	"./remote"
	"./structs"

	"container/list"
	"database/sql"
    "encoding/json"
	"fmt"
	"github.com/lib/pq/hstore"
	"log"
    "strconv"
	"time"
)

func processVideo(row structs.Video, c chan int) {
    out.Debugln("\tProcessing #", row.Id, " ", row.Youtube_id)
    out.Verbose(".")

    requests := 0

	// get amara id

	var amara_id string
	if row.Amara_id.Valid {
		amara_id = row.Amara_id.String
	} else {
		// fmt.Println("\tupdating amara_id")

		raw, err := remote.Fetch(app.Client, fmt.Sprintf(app.Urls["amaraId"], row.Youtube_id))
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
            out.Debugln("\tamara_id not found")
            out.Verbose("A")

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
	raw, err := remote.Fetch(app.Client, fmt.Sprintf(app.Urls["amaraRevisions"], amara_id))
    requests++
    if err != nil {
        c <- requests
        return
    }

	var result structs.AmaraRevisionsResult
	err = json.Unmarshal([]byte(raw), &result)
	if err != nil {
        out.Debugln("Failed to parse revisions json, will retry next batch")
        out.Verbose("F")

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
	srt, err := remote.Fetch(app.Client, fmt.Sprintf(app.Urls["amaraSrt"], amara_id, wrapper.Language_code, revision.Version_no))
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
	app.Init()
    defer db.Close()

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
