package request

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

	hostname := ""
	depth := 0
	if parent != nil {
		if !newURL.IsAbs() {
			newURL.Scheme = parent.Scheme
			newURL.Host = parent.Host
		}

		hostname = parent.Host
		depth = parent.depth + 1
	}

	return &Request{
		newURL,
		hostname,
		depth,
	}, nil
}

func (r *Request) Depth() int {
	return r.depth
}
