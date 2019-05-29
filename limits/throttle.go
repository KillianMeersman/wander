package limits

import (
	"time"

	"github.com/KillianMeersman/wander/request"
)

// Throttle specifies an interface all throttles must comply with.
type Throttle interface {
	Applies(req *request.Request) bool
	Wait()
	Ticker() <-chan time.Time
}

// ThrottleCollection combines throttles to use the most approriate one for the request
type ThrottleCollection struct {
	defaultThrottle *DefaultThrottle
	domainThrottles map[string]*DomainThrottle
}

func NewThrottleCollection(defaultCollector *DefaultThrottle, domainThrottles ...*DomainThrottle) ThrottleCollection {

	return ThrottleCollection{
		defaultThrottle: defaultCollector,
		domainThrottles: make(map[string]*DomainThrottle),
	}
}

// Wait blocks until the most approprate timer has ticked over.
func (t *ThrottleCollection) Wait(req *request.Request) {
	// check domain specific throttles
	throttle, ok := t.domainThrottles[req.String()]
	if ok {
		throttle.Wait()
		return
	}

	// check default throttle
	if t.defaultThrottle != nil {
		t.defaultThrottle.Wait()
	}
}

// SetDomainThrottle sets a domain throttle.
func (t *ThrottleCollection) SetDomainThrottle(throttle *DomainThrottle) {
	t.domainThrottles[throttle.domain] = throttle
}

// DefaultThrottle will throttle all domains
type DefaultThrottle struct {
	delay  time.Duration
	ticker *time.Ticker
}

// NewDefaultThrottle will throttle all domains
func NewDefaultThrottle(delay time.Duration) *DefaultThrottle {
	return &DefaultThrottle{
		delay,
		time.NewTicker(delay),
	}
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DefaultThrottle) Applies(path string) bool {
	return true
}

// Wait for the throttle
func (t *DefaultThrottle) Wait() {
	<-t.Ticker()
}

// Ticker channel
func (t *DefaultThrottle) Ticker() <-chan time.Time {
	return t.ticker.C
}

// DomainThrottle will only throttle certain domains
type DomainThrottle struct {
	DefaultThrottle
	domain string
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DomainThrottle) Applies(req *request.Request) bool {
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
