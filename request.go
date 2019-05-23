package wander

import (
	"net/url"
	"strings"
)

// Request contains the to-be-visited URL as well as the origin domain, it will automatically convert relative URL's to absolute ones
type Request struct {
	*url.URL
	sourceHost string
	depth      int
}

func NewRequest(path string, parent *Request) (*Request, error) {
	path = strings.ReplaceAll(path, "\n", "")
	newURL, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	if !newURL.IsAbs() {
		newURL.Scheme = parent.Scheme
		newURL.Host = parent.Host
	}

	hostname := ""
	depth := 0
	if parent != nil {
		hostname = parent.Hostname()
		depth = parent.depth
	}

	return &Request{
		newURL,
		hostname,
		depth + 1,
	}, nil
}
