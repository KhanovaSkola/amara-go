package db

import (
    "database/sql"
    _ "github.com/lib/pq"
    "github.com/lib/pq/hstore"
    "fmt"
    "log"
    "../structs"
)

const schema = `"1"`
var Conn *sql.DB

func Init(user, pwd, dbname string, port int) {
    var err error
    Conn, err = sql.Open("postgres", fmt.Sprintf("user=%v password=%v dbname=%v port=%v sslmode=disable", user, pwd, dbname, port))
    if err != nil {
        log.Fatal(err)
    }

    Conn.SetMaxOpenConns(500)
}

func Close() {
    Conn.Close()
}

func query(format string, args ...interface {}) (*sql.Rows, error) {
    query := fmt.Sprintf(format, schema)
//    fmt.Println("Query:", query)
    return Conn.Query(query, args...)
}

type FetchVideo func(structs.Video)

func FetchVideos(fn FetchVideo, limit int) {
    rows, err := query(`
        SELECT id, youtube_id, amara_id, revisions
        FROM %v.video
        WHERE skip = 'f'
        ORDER BY last_checked ASC NULLS FIRST
        LIMIT $1`, limit)
    if err != nil {
        log.Fatal("Failed to fetch videos: ", err)
    }

    for rows.Next() {
        var video structs.Video
        err = rows.Scan(&video.Id, &video.Youtube_id, &video.Amara_id, &video.Revisions)
        if err != nil {
            log.Fatal("Failed to fetch row:", err)
        }
        fn(video)
    }
    err = rows.Close()
    if err != nil {
        fmt.Println(err)
    }
}

func SkipVideo(rowId int) error {
    res, err := query(`
        UPDATE %v.video
        SET last_checked = Now(), skip = 't'
        WHERE id = $1`, rowId)
    res.Close()
    return err
}

func UpdateVideo(rowId int, amaraId string) error {
    res, err := query(`
        UPDATE %v.video
        SET amara_id = $1
        WHERE id = $2`, amaraId, rowId)
    res.Close()
    return err
}

func UpdateVideoRevisions(rowId int, revisions hstore.Hstore) error {
    res, err := query(`
        UPDATE %v.video
        SET last_checked = Now(), revisions = $1
        WHERE id = $2`, revisions, rowId)
    res.Close()
    return err
}

func AddRevision(rowId int, lang string, revision int, author string, content hstore.Hstore) error {
    res, err := query(`
        INSERT INTO %v.revision
        (video_id, language, revision, author, content) VALUES
        ($1, $2, $3, $4, $5, $6)`,
        rowId, lang, revision, author, content)
    res.Close()
    return err
}

type FetchRevision func(structs.Revision)

func FetchRevisions(fn FetchRevision, limit int) {
    rows, err := query(`
        SELECT r.language, r.video_id, v.amara_id
		FROM %v.revision r
		LEFT JOIN %v.video v ON v.id = r.video_id
		WHERE published_at IS NULL
		GROUP BY r.video_id, v.amara_id, r.language
		LIMIT $1`, limit)
    if err != nil {
        log.Fatal("Failed to fetch videos: ", err)
    }

    for rows.Next() {
        var revision structs.Revision
        err = rows.Scan(&revision.Language, &revision.Video_id, &revision.Amara_id)
        if err != nil {
            log.Fatal("Failed to fetch row:", err)
        }
        fn(revision)
    }
    err = rows.Close()
    if err != nil {
        fmt.Println(err)
    }
}
