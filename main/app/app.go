package app

import (
    "../db"
    "net/http"
    "flag"
    "time"
)

var Verbose bool
var Debug bool

var Urls map[string]string
var Client http.Client

func Init() {
    Urls = map[string]string{
        "amaraId":        "https://www.amara.org/api2/partners/videos/?format=json&video_url=http%%3A%%2F%%2Fwww.youtube.com%%2Fwatch%%3Fv%%3D%v",
        "amaraRevisions": "http://www.amara.org/api2/partners/videos/%v/languages/?limit=120&format=json",
        "amaraSrt":       "http://www.amara.org/api2/partners/videos/%v/languages/%v/subtitles?format=srt&version=%v",
    }

    flag.BoolVar(&Verbose, "v", false, "verbose mode")
    flag.BoolVar(&Debug, "vv", false, "debug mode")
    flag.Parse()

    db.Init("mikulas", "mikulas", "report", 5432)

    Client = http.Client{
        Timeout: time.Duration(60 * time.Second),
    }
}
