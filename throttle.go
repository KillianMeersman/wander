package wander

import (
	"regexp"
	"time"
)

type DomainThrottle struct {
	*Throttle
	domain *regexp.Regexp
}

// Applies returns true if the path matches the Throttle domain regex
func (t *DomainThrottle) Applies(path string) bool {
	return t.domain.MatchString(path)
}

func NewDomainThrottle(domain string, delay time.Duration) (*DomainThrottle, error) {
	regex, err := regexp.Compile(domain)
	if err != nil {
		return nil, err
	}
	throttle, err := NewThrottle(delay)
	if err != nil {
		return nil, err
	}

	return &DomainThrottle{
		throttle,
		regex,
	}, nil
}

type Throttle struct {
	delay  time.Duration
	ticker *time.Ticker
}

func NewThrottle(delay time.Duration) (*Throttle, error) {
	return &Throttle{
		delay,
		time.NewTicker(delay),
	}, nil
}

// Wait until the throttle delay has passed
func (t *Throttle) Wait() {
	<-t.ticker.C
}

// Ticker channel
func (t *Throttle) Ticker() <-chan time.Time {
	return t.ticker.C
}
