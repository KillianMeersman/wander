package limits

import (
	"net/http"
	"time"
)

// Throttle is used to limit the rate of requests.
type Throttle interface {
	// Wait for the throttle.
	Wait(*http.Request)
	// Applies returns true if the throttle applies to a request.
	Applies(*http.Request) bool
	// SetWaitTime add a wait time and return the total wait time.
	SetWaitTime(time.Duration)
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

func (t *ThrottleCollection) getThrottle(req *http.Request) Throttle {
	throttle, ok := t.domainThrottles[req.URL.Host]
	if ok {
		return throttle
	}
	if t.defaultThrottle != nil {
		return t.defaultThrottle
	}
	return nil
}

// Wait blocks until the most approprate timer has ticked over.
func (t *ThrottleCollection) Wait(req *http.Request) {
	throttle := t.getThrottle(req)
	if throttle != nil {
		throttle.Wait(req)
	}
}

// Applies returns true if the path matches the Throttle domain regex
func (t *ThrottleCollection) Applies(_ *http.Request) bool {
	return true
}

// SetWaitTime make all throttles block for a duration.
func (t *ThrottleCollection) SetWaitTime(waitTime time.Duration) {
	if t.defaultThrottle != nil {
		t.defaultThrottle.SetWaitTime(waitTime)
	}
	for _, domainThrottle := range t.domainThrottles {
		domainThrottle.SetWaitTime(waitTime)
	}
}

// SetDomainThrottle sets a domain throttle.
// Will overwrite existing domain throttle if it already exists.
func (t *ThrottleCollection) SetDomainThrottle(throttle *DomainThrottle) {
	t.domainThrottles[throttle.domain] = throttle
}

// DefaultThrottle will throttle all domains
type DefaultThrottle struct {
	interval    time.Duration
	ticker      *time.Ticker
	waitChannel <-chan time.Time
}

// NewDefaultThrottle will throttle all domains
func NewDefaultThrottle(delay time.Duration) *DefaultThrottle {
	return &DefaultThrottle{
		delay,
		time.NewTicker(delay),
		nil,
	}
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DefaultThrottle) Applies(_ *http.Request) bool {
	return true
}

// Wait for the throttle
func (t *DefaultThrottle) Wait(_ *http.Request) {
	if t.waitChannel != nil {
		<-t.waitChannel
		t.waitChannel = nil
	}
	<-t.ticker.C
}

func (t *DefaultThrottle) SetWaitTime(waitTime time.Duration) {
	t.waitChannel = time.After(waitTime)
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
func (t *DomainThrottle) Applies(req *http.Request) bool {
	return t.domain == req.URL.Host
}

func (t *DomainThrottle) SetWaitTime(waitTime time.Duration) {
	t.waitChannel = time.After(waitTime)
}
