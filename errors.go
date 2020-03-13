package wander

import (
	"fmt"
	"net/url"
)

// AlreadyVisited is thrown when a request's URL has been visited before by the spider.
type AlreadyVisited struct {
	URL *url.URL
}

func (e AlreadyVisited) Error() string {
	return fmt.Sprintf("request to %s filtered, already visited", e.URL.String())
}
