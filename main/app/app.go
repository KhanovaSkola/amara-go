package app

import (
	"../db"
	"code.google.com/p/gcfg"
	"flag"
	"log"
	"net/http"
	"time"
)

var Verbose bool
var Debug bool

var Urls map[string]string
var Client http.Client

type Config struct {
	Database struct {
		User, Password, Dbname string
		Port                   int
	}
}

func Init() {
	Urls = map[string]string{
		"amaraId":        "https://www.amara.org/api2/partners/videos/?format=json&video_url=http%%3A%%2F%%2Fwww.youtube.com%%2Fwatch%%3Fv%%3D%v",
		"amaraRevisions": "http://www.amara.org/api2/partners/videos/%v/languages/?limit=120&format=json",
		"amaraSrt":       "http://www.amara.org/api2/partners/videos/%v/languages/%v/subtitles?format=srt&version=%v",
		"amaraHtml":      "http://www.amara.org/en/videos/%v/%v/",
		"_published":     "%v?tab=revisions&page=%v",
	}

	flag.BoolVar(&Verbose, "v", false, "verbose mode")
	flag.BoolVar(&Debug, "vv", false, "debug mode")
	flag.Parse()

	var cfg Config
	err := gcfg.ReadFileInto(&cfg, "config.gcfg")
	if err != nil {
		log.Fatal(err)
	}
	db.Init(cfg.Database.User, cfg.Database.Password, cfg.Database.Dbname, cfg.Database.Port)

	Client = http.Client{
		Timeout: time.Duration(60 * time.Second),
	}
}
