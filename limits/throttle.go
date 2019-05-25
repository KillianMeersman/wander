package limits

import (
	"regexp"
	"time"

	"github.com/KillianMeersman/wander/request"
)

type Throttle interface {
	Applies(req *request.Request) bool
	Wait()
	Ticker() <-chan time.Time
}

// Throttles combines throttles to use the most specific one for the request
type ThrottleCollection struct {
	defaultThrottle *DefaultThrottle
	throttles       []Throttle
}

func NewThrottleCollection(defaultCollector *DefaultThrottle, throttles ...Throttle) *ThrottleCollection {
	return &ThrottleCollection{
		defaultThrottle: defaultCollector,
		throttles:       throttles,
	}
}

func (t *ThrottleCollection) Check(req *request.Request) error {
	for _, throttle := range t.throttles {
		if throttle.Applies(req) {
			throttle.Wait()
			return nil
		}
	}
	if t.defaultThrottle != nil {
		t.defaultThrottle.Wait()
	}
	return nil
}

func (t *ThrottleCollection) NewRequest(req *request.Request) error {
	return nil
}

type DefaultThrottle struct {
	delay  time.Duration
	ticker *time.Ticker
}

func ThrottleDefault(delay time.Duration) *DefaultThrottle {
	return &DefaultThrottle{
		delay,
		time.NewTicker(delay),
	}
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DefaultThrottle) Applies(path string) bool {
	return true
}

func (t *DefaultThrottle) Wait() {
	<-t.Ticker()
}

// Ticker channel
func (t *DefaultThrottle) Ticker() <-chan time.Time {
	return t.ticker.C
}

type DomainThrottle struct {
	DefaultThrottle
	domain *regexp.Regexp
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DomainThrottle) Applies(req *request.Request) bool {
	return t.domain.MatchString(req.URL.String())
}

func ThrottleDomain(domain string, delay time.Duration) *DomainThrottle {
	regex, err := regexp.Compile(domain)
	if err != nil {
		return nil
	}
	throttle := ThrottleDefault(delay)

	return &DomainThrottle{
		*throttle,
		regex,
	}
}
