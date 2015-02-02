package main

import (
    "database/sql"
    "github.com/lib/pq/hstore"
)

type Video struct {
    Id         int
    Youtube_id string
    Amara_id   sql.NullString
    Revisions  hstore.Hstore
}

type AmaraMeta struct {
    Limit, Offset, Total_count int
}

type AmaraResult struct {
    Meta    AmaraMeta
    Objects []AmaraVideo
}

type AmaraVideo struct {
    Id string
}

type AmaraRevisionsResult struct {
    Meta    AmaraMeta
    Objects []AmaraRevisionWrapper
}

type AmaraRevisionWrapper struct {
    Language_code, Title, Description string
    Versions                          []AmaraRevision
}

type AmaraRevision struct {
    Author     string
    Version_no int
}
