package out

import (
    "../app"
    "fmt"
)

func Debugln(args ...interface{}) {
    if app.Debug {
        fmt.Println(args...)
    }
}

func Debug(args ...interface{}) {
    if app.Debug {
        fmt.Print(args...)
    }
}

func Verboseln(args ...interface{}) {
    if app.Verbose {
        fmt.Println(args...)
    }
}

func Verbose(args ...interface{}) {
    if app.Verbose {
        fmt.Print(args...)
    }
}
