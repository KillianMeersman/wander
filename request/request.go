package request

import (
	"net/url"
)

// Request contains the to-be-visited URL as well as the origin domain.
type Request struct {
	url.URL
	SourceHost string
	Depth      int
}

// NewRequest will return a Request with absolute URL, converting relative URL's to absolute ones as needed.
// Returns an error if the URL could not be parsed.
func NewRequest(url *url.URL, parent *Request) (*Request, error) {
	hostname := ""
	depth := 0
	if parent != nil {
		if !url.IsAbs() {
			url.Scheme = parent.Scheme
			url.Host = parent.Host
		}

		hostname = parent.Host
		depth = parent.Depth + 1
	}

	return &Request{
		URL:        *url,
		SourceHost: hostname,
		Depth:      depth,
	}, nil
}
