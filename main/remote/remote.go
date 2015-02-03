package remote

import (
	"io/ioutil"
	"net/http"
	"net/url"
)

func Fetch(client http.Client, url string) (string, error) {
//    url = "http://requestb.in/zhi8ggzh"
    req, err := http.NewRequest("GET", url, nil)
    req.Header.Set("X-api-username", "CzechBot")
    req.Header.Set("X-apikey", "97f712b4716b30f7d567fe0a866f2874dda24d32")
    resp, err := client.Do(req)

	if err != nil {
		return "", err
	}
	body, err := ioutil.ReadAll(resp.Body)
	return string(body), err
}

func RedirectUrl(url string) (*url.URL, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	return resp.Location()
}
