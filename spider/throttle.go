package spider

import (
	"regexp"
	"time"
)

type Throttle struct {
	domain *regexp.Regexp
	delay  time.Duration
	ticker *time.Ticker
}

func NewThrottle(domain string, delay time.Duration) (*Throttle, error) {
	regex, err := regexp.Compile(domain)
	if err != nil {
		return nil, err
	}

	return &Throttle{
		regex,
		delay,
		time.NewTicker(delay),
	}, nil
}

// Applies returns true if the path matches the Throttle domain regex
func (t *Throttle) Applies(path string) bool {
	return t.domain.MatchString(path)
}

// Wait until the throttle delay has passed
func (t *Throttle) Wait() {
	<-t.ticker.C
}
