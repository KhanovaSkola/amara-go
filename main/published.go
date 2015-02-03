package main

import (
    "./app"
    "./db"
    "./out"
    "./remote"
    "./structs"

	"container/list"
	"fmt"
	_ "github.com/lib/pq"
	"regexp"
	"time"
)

var reRevision *regexp.Regexp
var reNextPage *regexp.Regexp

func processRevision(revision structs.Revision, c chan int) {
    out.Debugln("\tProcessing", revision)
    out.Verbose(".")

	requests := 0

	url, err := remote.RedirectUrl(fmt.Sprintf(app.Urls["amaraHtml"], revision.Amara_id, revision.Language))
	requests++
	if err != nil {
        out.Verbose("F")
        out.Debugln("Failed to get redirect url of ", revision.Amara_id)

		c <- requests
		return
	}

	page := "1" // intentionally string, regex result below assigned
	for {
		html, err := remote.Fetch(app.Client, fmt.Sprintf(app.Urls["_published"], url, page))
		requests++
		if err != nil {
            out.Verbose("F")
            out.Debugln("Failed to fetch revisions html page")

			c <- requests
			return
		}

		found := reRevision.FindAllStringSubmatch(html, -1)
		for i := range found {
			date := fmt.Sprintf("%v-%v-%v", found[i][4], found[i][2], found[i][3])

            out.Debugln("Updating revision", found[i][1], date)

            db.UpdateRevision(date, revision.Video_id, revision.Language, found[i][1])
		}

		matches := reNextPage.FindStringSubmatch(html)
		if len(matches) != 0 {
			page = matches[1]
			continue
		}
		break
	}

	c <- requests
}

func main() {
	app.Init()
    defer db.Close()

    reRevision = regexp.MustCompile("Revision (\\d+) - (\\d+)/(\\d+)/(\\d+)")
    reNextPage = regexp.MustCompile(`href="\?page=(\d+)&amp;tab=revisions" rel="next"`)

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
			go processRevision(revision.(structs.Revision), c)
		}

		requests += <-c
		count--
	}
}
