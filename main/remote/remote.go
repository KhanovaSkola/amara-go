package remote

import (
    "net/http"
    "io/ioutil"
)

func Fetch(client http.Client, url string) (string, error) {
    resp, err := client.Get(url)
    if err != nil {
        return "", err
    }
    body, err := ioutil.ReadAll(resp.Body)
    return string(body), err
}
