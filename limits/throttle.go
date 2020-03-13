package limits

import (
	"time"

	"github.com/KillianMeersman/wander/request"
)

// Throttle is used to limit the rate of requests.
type Throttle interface {
	Wait(*request.Request)
	Applies(*request.Request) bool
}

// ThrottleCollection combines a default and domain specific throttles.
type ThrottleCollection struct {
	defaultThrottle *DefaultThrottle
	domainThrottles map[string]*DomainThrottle
}

func NewThrottleCollection(defaultThrottle *DefaultThrottle, domainThrottles ...*DomainThrottle) ThrottleCollection {
	col := ThrottleCollection{
		defaultThrottle: defaultThrottle,
		domainThrottles: make(map[string]*DomainThrottle),
	}

	for _, domainThrottle := range domainThrottles {
		col.domainThrottles[domainThrottle.domain] = domainThrottle
	}

	return col
}

func (t *ThrottleCollection) getThrottle(req *request.Request) Throttle {
	throttle, ok := t.domainThrottles[req.Host]
	if ok {
		return throttle
	}
	if t.defaultThrottle != nil {
		return t.defaultThrottle
	}
	return nil
}

// Wait blocks until the most approprate timer has ticked over.
func (t *ThrottleCollection) Wait(req *request.Request) {
	throttle := t.getThrottle(req)
	if throttle != nil {
		throttle.Wait(req)
	}
}

// SetDomainThrottle sets a domain throttle.
// Will overwrite existing domain throttle if it already exists.
func (t *ThrottleCollection) SetDomainThrottle(throttle *DomainThrottle) {
	t.domainThrottles[throttle.domain] = throttle
}

// DefaultThrottle will throttle all domains
type DefaultThrottle struct {
	interval time.Duration
	ticker   *time.Ticker
}

// NewDefaultThrottle will throttle all domains
func NewDefaultThrottle(delay time.Duration) *DefaultThrottle {
	return &DefaultThrottle{
		delay,
		time.NewTicker(delay),
	}
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DefaultThrottle) Applies(_ *request.Request) bool {
	return true
}

// Wait for the throttle
func (t *DefaultThrottle) Wait(_ *request.Request) {
	<-t.ticker.C
}

// DomainThrottle will throttle a specific domain.
type DomainThrottle struct {
	DefaultThrottle
	domain string
}

// NewDomainThrottle instantiates a new domain throttle.
func NewDomainThrottle(domain string, delay time.Duration) *DomainThrottle {
	throttle := NewDefaultThrottle(delay)

	return &DomainThrottle{
		*throttle,
		domain,
	}
}

// Applies returns true if the path matches the Throttle domain.
func (t *DomainThrottle) Applies(req *request.Request) bool {
	return t.domain == req.Host
}
