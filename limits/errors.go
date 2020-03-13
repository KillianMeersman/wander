package limits

import (
	"fmt"
	"net/url"

	"github.com/KillianMeersman/wander/request"
)

// MaxDepthReached indicates a request's depth went beyond the maximum and was filtered.
type MaxDepthReached struct {
	Depth   int
	Request *request.Request
}

func (m *MaxDepthReached) Error() string {
	return fmt.Sprintf("Maximum depth reached (%d)", m.Depth)
}

// ForbiddenDomain indicates a request's URL points to a domain not in the spider's allowed domains.
type ForbiddenDomain struct {
	URL *url.URL
}

func (e ForbiddenDomain) Error() string {
	return fmt.Sprintf("request to %s filtered, not in allowed domains", e.URL.String())
}
