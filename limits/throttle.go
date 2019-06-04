package limits

import (
	"time"

	"github.com/KillianMeersman/wander/request"
)

// Throttle specifies an interface all throttles must comply with.
type Throttle interface {
	Wait(*request.Request)
}

type specificThrottle interface {
	wait()
	delay() time.Duration
}

// ThrottleCollection combines throttles to use the most approriate one for the request
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

func (t *ThrottleCollection) getThrottle(req *request.Request) specificThrottle {
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
		throttle.wait()
	}
}

// SetDomainThrottle sets a domain throttle.
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
func (t *DefaultThrottle) applies(path string) bool {
	return true
}

// Wait for the throttle
func (t *DefaultThrottle) wait() {
	<-t.ticker.C
}

func (t *DefaultThrottle) delay() time.Duration {
	return t.interval
}

// DomainThrottle will only throttle certain domains
type DomainThrottle struct {
	DefaultThrottle
	domain string
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DomainThrottle) applies(req *request.Request) bool {
	return t.domain == req.Host
}

// NewDomainThrottle will only throttle certain domains
func NewDomainThrottle(domain string, delay time.Duration) *DomainThrottle {
	throttle := NewDefaultThrottle(delay)

	return &DomainThrottle{
		*throttle,
		domain,
	}
}
