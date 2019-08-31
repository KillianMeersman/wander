package robots

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/KillianMeersman/wander/request"
)

// RobotLimitCache holds the robot exclusions for multiple domains.
type Cache struct {
	limits map[string]*Limits
	lock   sync.RWMutex
}

func NewCache() *Cache {
	return &Cache{
		limits: make(map[string]*Limits),
		lock:   sync.RWMutex{},
	}
}

// Allowed returns true if the userAgent is allowed to access the given path on the given domain.
// Returns error if no robot file is cached for the given domain.
func (c *Cache) Allowed(userAgent string, req *request.Request) (bool, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	limits, ok := c.limits[req.Host]
	if !ok {
		return false, fmt.Errorf("No limits found for domain %s", req.Host)
	}
	return limits.Allowed(userAgent, req.Path), nil
}

// GetLimits gets the limits for a host. Returns an error when no limits are cached for the given host.
func (c *Cache) GetLimits(host string) (*Limits, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	limits, ok := c.limits[host]
	if !ok {
		return nil, fmt.Errorf("No limits found for domain %s", host)
	}
	return limits, nil
}

// AddLimits adds or replaces the limits for a host. Returns an error if the limits are invalid.
func (c *Cache) AddLimits(in io.Reader, host string) (*Limits, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	limits, err := FromReader(in)
	if err != nil {
		return nil, err
	}

	c.limits[host] = limits
	return limits, nil
}

// Limits holds all the limits imposed by a robots exclusion file.
type Limits struct {
	defaultLimits *Group
	groups        map[string]*Group
	sitemap       *url.URL
}

func newLimits() *Limits {
	return &Limits{
		groups: make(map[string]*Group, 0),
	}
}

func FromResponse(res *request.Response) (*Limits, error) {
	return nil, nil
}

// FromReader will parse a robot exclusion file, returns a normal error if it encounters an invalid directive.
func FromReader(in io.Reader) (*Limits, error) {
	scanner := bufio.NewScanner(in)
	limits := newLimits()

	// current host specification
	group := newGroup("*")
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " \t")

		// ignore empty lines
		if len(line) < 1 {
			continue
		}
		// skip if line is comment
		if line[0] == '#' {
			continue
		}

		// separate directive and parameter
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 1 {
			return nil, fmt.Errorf("Invalid directive %s", line)
		}
		directive := parts[0]
		parameter := ""
		if len(parts) > 1 {
			parameter = strings.TrimPrefix(parts[1], " ")
		}

		switch strings.ToLower(directive) {
		case "user-agent":
			if parameter == "" {
				return nil, fmt.Errorf("Invalid User-agent directive %s", line)
			}
			limits.addGroup(group)
			group = newGroup(parameter)

		case "disallow":
			if parameter == "" {
				group.disallowed = make([]string, 0)
			} else {
				group.disallowed = append(group.disallowed, parameter)
			}

		case "allow":
			if parameter == "" {
				group.allowed = make([]string, 0)
			} else {
				group.allowed = append(group.allowed, parameter)
			}

		case "crawl-delay":
			dur, err := time.ParseDuration(fmt.Sprintf("%ss", parameter))
			if err != nil {
				return nil, err
			}
			if dur.Seconds() < 0 {
				return nil, fmt.Errorf("negative crawl-delay not allowed")
			}
			group.delay = dur

		case "sitemap":
			url, err := url.Parse(parameter)
			if err != nil {
				return nil, err
			}
			limits.sitemap = url

		default:
			continue
		}
	}
	limits.addGroup(group)
	return limits, nil
}

func (l *Limits) addGroup(g *Group) {
	if g.userAgent == "*" {
		l.defaultLimits = g
		return
	}
	l.groups[g.userAgent] = g
}

// Allowed returns true if the user agent is allowed to access the given url.
func (l *Limits) Allowed(userAgent, url string) bool {
	group, ok := l.groups[userAgent]
	if ok {
		return group.Allowed(url)
	}
	return l.defaultLimits.Allowed(url)
}

// GetLimits gets the Group for the userAgent, returns the default (*) group if it was present and no other groups apply.
// Returns nil if no groups apply and no default group was supplied.
func (l *Limits) GetLimits(userAgent string) *Group {
	for _, group := range l.groups {
		if group.userAgent == userAgent {
			return group
		}
	}
	return l.defaultLimits
}

// Delay returns the User-agent specific crawl-delay if it exists, otherwise the catch-all delay.
// Returns def if neither a specific or global crawl-delay exist.
func (l *Limits) Delay(userAgent string, def time.Duration) time.Duration {
	return l.GetLimits(userAgent).Delay(def)
}

// Sitemap returns the URL to the sitemap for the given User-agent.
// Returns the default sitemap if not User-agent specific sitemap was specified, otherwise nil.
func (l *Limits) Sitemap(userAgent string) *url.URL {
	return l.sitemap
}

// Group holds the limits for a single user agent
type Group struct {
	userAgent  string
	allowed    []string
	disallowed []string
	delay      time.Duration
}

func newGroup(userAgent string) *Group {
	return &Group{
		userAgent:  userAgent,
		allowed:    make([]string, 0),
		disallowed: make([]string, 0),
		delay:      -1,
	}
}

// Applies returns true if the group applies to the given userAgent
func (g *Group) Applies(userAgent string) bool {
	return g.userAgent == userAgent
}

// Allowed returns true if the url is allowed by the group rules. Check if the group applies to the user agent first by using Applies.
func (g *Group) Allowed(url string) bool {
	for _, rule := range g.allowed {
		if MatchURLRule(rule, url) {
			return true
		}
	}
	for _, rule := range g.disallowed {
		if MatchURLRule(rule, url) {
			return false

		}
	}
	return true
}

// Delay returns the Crawl-delay. Returns defs if no crawl delay was specified.
func (g *Group) Delay(def time.Duration) time.Duration {
	if g.delay == -1 {
		return def
	}
	return g.delay
}

func matchWildcard(seekChar byte, value string) (int, bool) {
	for i := 0; i < len(value); i++ {
		if value[i] == seekChar {
			return i, true
		}
	}
	// return false if seekChar was not ofund
	return 0, false
}

// MatchURLRule will return true if the given robot exclusion rule matches the given URL.
// Supports wildcards ('*') and end of line ('$').
func MatchURLRule(rule, url string) bool {
	// if the rule is longer than the url, return false
	if len(rule) > len(url) {
		return false
	}

	// j is the current character in url
	j := 0
	for i := 0; i < len(rule); i++ {
		switch rule[i] {
		// wildcard: loop until next rule characer is found
		case '*':
			// return true if last rule character is *
			if i+1 == len(rule) {
				return true
			}

			// loop until next rule character is found in url, return false if not found
			seekChar := rule[i+1]
			add, match := matchWildcard(seekChar, url[j:len(url)])
			j += add
			if !match {
				return false
			}

		// end of line, return true if we have actually reached the end of url
		case '$':
			return j == len(url)

		// check if url and rule matches on indexes j, i
		default:
			if j >= len(url) || rule[i] != url[j] {
				return false
			}
			j++
		}

	}
	return true
}
