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
    "encoding/json"
    "net/url"
)

var reJoined *regexp.Regexp

func processAuthor(author structs.Author, c chan int) {
    out.Debugln("\tProcessing", author.Username)
    out.Verbose(".")

    requests := 0

    raw, err := remote.Fetch(app.Client, fmt.Sprintf(app.Urls["amaraAuthor"], author.Username))
    requests++
    if err != nil {
        out.Debugln("Failed to fetch author api page", author.Username)
        out.Verbose("F")

        c <- requests
        return
    }

    var result structs.AmaraAuthor
    err = json.Unmarshal([]byte(raw), &result)
    if err != nil {
        out.Debugln("Failed to decode author json", author.Username)
        out.Verbose("F")

        c <- requests
        return
    }

    author.Avatar = result.Avatar
    author.FirstName = result.First_name
    author.LastName = result.Last_name

    html, err := remote.Fetch(app.Client, fmt.Sprintf(app.Urls["amaraAuthorHtml"], url.QueryEscape(author.Username)))
    requests++
    requests++
    if err != nil {
        out.Debugln("Failed to fetch author html page", author.Username)
        out.Verbose("F")

        c <- requests
        return
    }

    matches := reJoined.FindStringSubmatch(html)
    author.JoinedAt.Valid = false
    if len(matches) != 0 {
        dateText := fmt.Sprintf("%v-%v-%v", matches[3], matches[1][0:3], matches[2])
        date, err := time.Parse("2006-Jan-2", dateText)
        if err != nil {
            out.Debugln(err)
            out.Verbose("F")
        }
        author.JoinedAt.Valid = true
        author.JoinedAt.String = date.Format("2006-Jan-02")
    }

    db.AddAuthor(author)

    c <- requests
}

func main() {
    app.Init()
    defer db.Close()

    reJoined = regexp.MustCompile("joined Amara on\\s+(\\w*?)\\.\\s+(\\d+),\\s+(\\d+)\\s*</")

    authors := list.New()
    c := make(chan int)
    concurrency := 100
    count := 0
    start := time.Now()
    requests := 1
    var elapsed time.Duration

    for {
        for count < concurrency {
            if authors.Len() == 0 {
                if count != 0 {
                    elapsed = time.Now().Sub(start)
                    fmt.Printf("\nrequests %v, per request %v, elapsed %v\n", requests, time.Duration(int(elapsed)/requests), elapsed)
                }

                db.FetchAuthors(func(author structs.Author) {
                    authors.PushBack(author)
                }, 10000)
            }

            author := authors.Remove(authors.Front())
            count++
            go processAuthor(author.(structs.Author), c)
        }

        requests += <-c
        count--
    }
}
