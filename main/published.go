package main

import (
    "./db"
    "./remote"
    "./structs"

	"container/list"
	// "encoding/json"
	"flag"
	"fmt"
	_ "github.com/lib/pq"
	// "github.com/lib/pq/hstore"
	"github.com/mikulas/crawler"
	//	"io/ioutil"
	"log"
	"net/http"
	//	"net/url"
	"regexp"
	"time"
	// "strconv"
)

func processRevision(revision structs.Revision, c chan int) {
	if debug {
		fmt.Println("\tProcessing", revision)
	} else if verbose {
		fmt.Print(".")
	}
	requests := 0

	url, err := crawler.RedirectUrl(fmt.Sprintf(urls["amaraHtml"], revision.Amara_id, revision.Language))
	requests++
	if err != nil {
		if debug || verbose {
			fmt.Print("F")
		}
		c <- requests
		return
	}

	page := "1" // intentionally string, regex result below assigned
	for {
		html, err := crawler.Fetch(client, fmt.Sprintf(urls["_published"], url, page))
		requests++
		if err != nil {
			if debug || verbose {
				fmt.Print("F")
			}
			c <- requests
			return
		}

		// TODO compile just once
		re := regexp.MustCompile("Revision (\\d+) - (\\d+)/(\\d+)/(\\d+)")
		found := re.FindAllStringSubmatch(html, -1)
		for i := range found {
			date := fmt.Sprintf("%v-%v-%v", found[i][4], found[i][2], found[i][3])

			if debug {
				fmt.Println("Updating revision", found[i][1], date)
			}
			res, err := db.Query(`
                UPDATE "1".revision
                SET published_at=$1
                WHERE video_id=$2 AND language=$3 AND revision=$4
            `, date, revision.Video_id, revision.Language, found[i][1])
			if err != nil {
				log.Fatal("Failed to save revision updated_at:", err)
			}
			res.Close()
		}

		re = regexp.MustCompile(`href="\?page=(\d+)&amp;tab=revisions" rel="next"`)
		matches := re.FindStringSubmatch(html)
		if len(matches) != 0 {
			page = matches[1]
			continue
		}
		break
	}

	c <- requests
}

func fetchRevisions(revisions *list.List, db *sql.DB, limit int) {
	fmt.Println("Fetch revisions", limit)

	rows, err := db.Query(`SELECT r.language, r.video_id, v.amara_id
		FROM "1".revision r
		LEFT JOIN "1".video v ON v.id = r.video_id
		WHERE published_at IS NULL
		GROUP BY r.video_id, v.amara_id, r.language
		LIMIT $1`, limit)
	if err != nil {
		log.Fatal("Failed to fetch revisions:", err)
	}

	for rows.Next() {
		var row Revision
		err = rows.Scan(&row.Language, &row.Video_id, &row.Amara_id)
		if err != nil {
			log.Fatal("Failed to fetch row:", err)
		}
		revisions.PushBack(row)
	}
	err = rows.Close()
	if err != nil {
		fmt.Println(err)
	}
}

func main() {
	db, err := sql.Open("postgres", "port=5432 user=mikulas dbname=report sslmode=disable")
	if err != nil {
		log.Fatal("Failed to connect to postgres:", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(500)

	client = http.Client{
		Timeout: time.Duration(60 * time.Second),
	}

	urls = map[string]string{
		"amaraHtml":  "http://www.amara.org/en/videos/%v/%v/",
		"_published": "%v?tab=revisions&page=%v",
	}

	revisions := list.New()
	c := make(chan int)
	concurrency := 100
	count := 0
	start := time.Now()
	requests := 1
	var elapsed time.Duration

	for {
		for count < concurrency {
			if revisions.Len() == 0 {
				elapsed = time.Now().Sub(start)
				fmt.Printf("\nrequests %v, per request %v, elapsed %v\n", requests, time.Duration(int(elapsed)/requests), elapsed)

                db.FetchRevisions(func(revision structs.Revision) {
                    revisions.PushBack(revision)
                }, 10000)
			}

			revision := revisions.Remove(revisions.Front())
			count++
			go processRevision(revision.(structs.Revision), db, c)
		}

		requests += <-c
		count--
	}
}
