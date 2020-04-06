package request

import (
	"net/url"
	"strings"
)

// Request contains the to-be-visited URL as well as the origin domain.
type Request struct {
	url.URL
	SourceHost string
	Depth      int
}

// NewRequest will return a Request with absolute URL, converting relative URL's to absolute ones as needed.
// Returns an error if the URL could not be parsed.
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
		depth = parent.Depth + 1
	}

	return &Request{
		URL:        *newURL,
		SourceHost: hostname,
		Depth:      depth,
	}, nil
}
