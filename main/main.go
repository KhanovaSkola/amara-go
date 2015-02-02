package main

import (
    "../remote"

	"container/list"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
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

func processVideo(row Video, db *sql.DB, c chan int) {
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

		var result AmaraResult
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
			res, err := db.Query(`UPDATE "1".video SET last_checked = Now(), skip = 't' WHERE id = $1`, row.Id)
			if err != nil {
				log.Fatal("Failed to skip video:", err)
			}
			err = res.Close()
			if err != nil {
				fmt.Println(err)
			}

			c <- requests
			return
		}

		amara_id = result.Objects[0].Id
		res, err := db.Query(`UPDATE "1".video SET amara_id = $1 WHERE id = $2`, amara_id, row.Id)
		if err != nil {
			log.Fatal("Failed to set amara_id:", err)
		}
		err = res.Close()
		if err != nil {
			fmt.Println(err)
		}

		// fmt.Println("\tamara_id found", amara_id)
	}

	// get revisions

	// fmt.Println("\tgetting revisions")
	raw, err := remote.Fetch(client, fmt.Sprintf(urls["amaraRevisions"], amara_id))
    requests++
    if err != nil {
        c <- requests
        return
    }

	var result AmaraRevisionsResult
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
			go saveSubtitles(row.Id, amara_id, wrapper, revision, db, csubs)
		}
	}

	for i := 0; i < count; i++ {
		// block until all subs are downloaded
		requests += <-csubs
	}

	res, err := db.Query(`UPDATE "1".video SET last_checked = Now(), revisions = $1 WHERE id = $2`, row.Revisions, row.Id)
	if err != nil {
		log.Fatal("Failed to update video:", err)
	}
	err = res.Close()
	if err != nil {
		fmt.Println(err)
	}

	// fmt.Println(" - v done")

	c <- requests
}

func saveSubtitles(id int, amara_id string, wrapper AmaraRevisionWrapper, revision AmaraRevision, db *sql.DB, csubs chan int) {
	srt, err := remote.Fetch(client, fmt.Sprintf(urls["amaraSrt"], amara_id, wrapper.Language_code, revision.Version_no))
    if err != nil {
        log.Fatal(err)
    }

	var content hstore.Hstore
	content.Map = make(map[string]sql.NullString)

	content.Map["title"] = sql.NullString{wrapper.Title, true}
	content.Map["description"] = sql.NullString{wrapper.Description, true}
	content.Map["srt"] = sql.NullString{srt, true}

	res, err := db.Query(`INSERT INTO "1".revision (video_id, language, revision, author, published_at, content) VALUES ($1, $2, $3, $4, $5, $6)`, id, wrapper.Language_code, revision.Version_no, revision.Author, nil, content)
	if err != nil {
		log.Fatal("Failed to save revision:", err)
	}
	err = res.Close()
	if err != nil {
		fmt.Println(err)
	}

	csubs <- 1
}

func fetchVideos(videos *list.List, db *sql.DB, limit int) {
	fmt.Println("Fetch videos", limit)

	rows, err := db.Query(`SELECT id, youtube_id, amara_id, revisions FROM "1".video WHERE skip = 'f' ORDER BY last_checked ASC NULLS FIRST LIMIT $1`, limit)
	if err != nil {
		log.Fatal("Failed to fetch videos:", err)
	}

	for rows.Next() {
		var row Video
		err = rows.Scan(&row.Id, &row.Youtube_id, &row.Amara_id, &row.Revisions)
		if err != nil {
			log.Fatal("Failed to fetch row:", err)
		}
		videos.PushBack(row)
	}
	err = rows.Close()
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	flag.BoolVar(&verbose, "v", false, "verbose mode")
	flag.BoolVar(&debug, "vv", false, "debug mode")
	flag.Parse()

	db, err := sql.Open("postgres", "user=mikulas dbname=report sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to postgres:", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(500)

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

				fetchVideos(videos, db, 100000)
			}

			video := videos.Remove(videos.Front())
			count++
			go processVideo(video.(Video), db, c)
		}

		requests += <-c
		count--
	}
}
