package proxy

import (
	"fmt"
	"net/http"
	"net/url"
)

func NewProxyURL(username, password, scheme, path string) (*url.URL, error) {
	completePath := fmt.Sprintf("%s://%s:%s@%s", scheme, username, password, path)
	return url.Parse(completePath)
}

func RoundRobinProxy(urls ...*url.URL) func(r *http.Request) (*url.URL, error) {
	if len(urls) < 1 {
		return nil
	}
	i := 0

	return func(r *http.Request) (*url.URL, error) {
		i = (i + 1) % len(urls)
		return urls[i], nil
	}
}
