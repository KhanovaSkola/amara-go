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
    "net/url")

var reJoined *regexp.Regexp

func processAuthor(author structs.Author, c chan int) {
    out.Debugln("\tProcessing", author)
    out.Verbose(".")

    requests := 0

    raw, err := remote.Fetch(app.Client, fmt.Sprintf(app.Urls["amaraAuthor"], author.Username))
    fmt.Println("raw", raw)
    requests++
    if err != nil {
        out.Debugln("Failed to fetch author api page", author)
        out.Verbose("F")

        c <- requests
        return
    }

    var result structs.AmaraAuthor
    err = json.Unmarshal([]byte(raw), &result)
    if err != nil {
        out.Debugln("Failed to decode author json", author)
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
        out.Debugln("Failed to fetch author html page", author)
        out.Verbose("F")

        c <- requests
        return
    }

    matches := reJoined.FindStringSubmatch(html)
    if len(matches) != 0 {
        fmt.Println(matches)
    }

    c <- requests
}

func main() {
    app.Init()
    defer db.Close()

    reJoined = regexp.MustCompile("joined Amara on (.*?)\\s*</")

    authors := list.New()
    c := make(chan int)
    concurrency := 1
    count := 0
    start := time.Now()
    requests := 1
    var elapsed time.Duration

    for {
        for count < concurrency {
            if authors.Len() == 0 {
                elapsed = time.Now().Sub(start)
                fmt.Printf("\nrequests %v, per request %v, elapsed %v\n", requests, time.Duration(int(elapsed)/requests), elapsed)

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
