package request

import (
	"encoding/json"
	"net/http"
	"net/url"
)

// Request contains the to-be-visited URL as well as the origin domain.
type Request struct {
	http.Request
	Depth int
}

func (r *Request) MarshalJSON() ([]byte, error) {
	data := struct {
		Depth  int
		Method string
		URL    *url.URL
	}{
		r.Depth,
		r.Method,
		r.URL,
	}

	return json.Marshal(data)
}

// NewRequest will return a Request with absolute URL, converting relative URL's to absolute ones as needed.
// Returns an error if the URL could not be parsed.
func NewRequest(url *url.URL, parent *Request) (*Request, error) {
	depth := 0
	if parent != nil {
		if !url.IsAbs() {
			url.Scheme = parent.URL.Scheme
			url.Host = parent.URL.Host
		}

		depth = parent.Depth + 1
	}

	req := http.Request{
		Method: "GET",
		URL:    url,
	}

	return &Request{
		Request: req,
		Depth:   depth,
	}, nil
}
