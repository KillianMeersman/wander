package robots

import (
	"fmt"
	"net/url"
)

// InvalidRobots indicates an invalid robots.txt file.
type InvalidRobots struct {
	Domain string
	Err    string
}

func (e InvalidRobots) Error() string {
	return fmt.Sprintf("robots.txt for %s invalid: %s", e.Domain, e.Err)
}

// RobotDenied indicates a request was denied by a site's robots.txt file.
type RobotDenied struct {
	URL url.URL
}

func (e RobotDenied) Error() string {
	return fmt.Sprintf("request for %s denied by robots.txt", e.URL.String())
}
