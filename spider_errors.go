package wander

import (
	"fmt"
	"net/url"
)

// InvalidRobots is thrown when the spider encounters an invalid robots.txt file.
type InvalidRobots struct {
	Domain string
	Err    string
}

func (e InvalidRobots) Error() string {
	return fmt.Sprintf("robots.txt for %s invalid: %s", e.Domain, e.Err)
}

// RobotDenied is thrown when a request was denied by a site's robots.txt file.
type RobotDenied struct {
	URL *url.URL
}

func (e RobotDenied) Error() string {
	return fmt.Sprintf("request for %s denied by robots.txt", e.URL.String())
}

// ForbiddenDomain is thrown when the request's URL points to a domain not in the spider's allowed domains.
type ForbiddenDomain struct {
	URL *url.URL
}

func (e ForbiddenDomain) Error() string {
	return fmt.Sprintf("request to %s filtered, not in allowed domains", e.URL.String())
}

// AlreadyVisited is thrown when a request's URL has been visited before by the spider.
type AlreadyVisited struct {
	URL *url.URL
}

func (e AlreadyVisited) Error() string {
	return fmt.Sprintf("request to %s filtered, already visited", e.URL.String())
}
