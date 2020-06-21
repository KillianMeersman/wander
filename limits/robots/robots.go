package robots

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// RobotRules holds the robot exclusions for multiple domains.
type RobotRules struct {
	hosts map[string]*RobotFile
	lock  sync.RWMutex
}

// NewRobotRules instantiates a new robot limit cache.
func NewRobotRules() *RobotRules {
	return &RobotRules{
		hosts: make(map[string]*RobotFile),
		lock:  sync.RWMutex{},
	}
}

// Allowed returns true if the userAgent is allowed to access the given path on the given domain.
// Returns error if no robot file is cached for the given domain.
func (c *RobotRules) Allowed(userAgent string, url *url.URL) (bool, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	limits, ok := c.hosts[url.Host]
	if !ok {
		return false, fmt.Errorf("No limits found for domain %s", url.Host)
	}
	return limits.Allowed(userAgent, url.Path), nil
}

// GetRulesForHost gets the rules for a host. Returns an error when no limits are cached for the given host.
func (c *RobotRules) GetRulesForHost(host string) (*RobotFile, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	limits, ok := c.hosts[host]
	if !ok {
		return nil, fmt.Errorf("No limits found for domain %s", host)
	}
	return limits, nil
}

// AddLimits adds or replaces the limits for a host. Returns an error if the limits are invalid.
func (c *RobotRules) AddLimits(in io.Reader, host string) (*RobotFile, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	limits, err := NewRobotFileFromReader(in)
	if err != nil {
		return nil, err
	}

	c.hosts[host] = limits
	return limits, nil
}

// RobotFile holds all the information in a robots exclusion file.
type RobotFile struct {
	defaultLimits *UserAgentRules
	groups        map[string]*UserAgentRules
	sitemap       *url.URL
}

func newRobotFile() *RobotFile {
	return &RobotFile{
		groups: make(map[string]*UserAgentRules, 0),
	}
}

func NewRobotFileFromURL(url *url.URL, client http.RoundTripper) (*RobotFile, error) {
	request, err := http.NewRequest("GET", url.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := client.RoundTrip(request)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return NewRobotFileFromReader(res.Body)
}

// RobotFileFromReader will parse a robot exclusion file from an io.Reader.
// Returns a default error if it encounters an invalid directive.
func NewRobotFileFromReader(in io.Reader) (*RobotFile, error) {
	scanner := bufio.NewScanner(in)
	limits := newRobotFile()

	// current host specification
	rules := newUserAgentRules("*")
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " \t")

		// ignore empty lines or comments
		if len(line) < 1 || line[0] == '#' {
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
			// Trim parameter of leading spaces
			parameter = strings.TrimPrefix(parts[1], " ")
		}

		// Match directive
		switch strings.ToLower(directive) {
		case "user-agent":
			if parameter == "" {
				return nil, fmt.Errorf("Invalid User-agent directive %s", line)
			}
			// Add the current user-agent rules to the RobotFile and begin a new UserAgentRules.
			limits.addUserAgentRules(rules)
			rules = newUserAgentRules(parameter)

		case "disallow":
			if parameter == "" {
				// Reset the disallowed on empty string
				rules.disallowed = make([]string, 0)
			} else {
				rules.disallowed = append(rules.disallowed, parameter)
			}

		case "allow":
			if parameter == "" {
				// Reset the allowed on empty string
				rules.allowed = make([]string, 0)
			} else {
				rules.allowed = append(rules.allowed, parameter)
			}

		case "crawl-delay":
			dur, err := time.ParseDuration(fmt.Sprintf("%ss", parameter))
			if err != nil {
				return nil, err
			}
			if dur.Seconds() < 0 {
				return nil, fmt.Errorf("Negative crawl-delay not allowed")
			}
			rules.delay = dur

		case "sitemap":
			url, err := url.Parse(parameter)
			if err != nil {
				return nil, err
			}
			limits.sitemap = url

		default:
			// Unknown directive, ignore
			continue
		}
	}
	limits.addUserAgentRules(rules)
	return limits, nil
}

func (l *RobotFile) addUserAgentRules(g *UserAgentRules) {
	if g.userAgent == "*" {
		l.defaultLimits = g
		return
	}
	l.groups[g.userAgent] = g
}

// Allowed returns true if the user agent is allowed to access the given url.
func (l *RobotFile) Allowed(userAgent, url string) bool {
	group, ok := l.groups[userAgent]
	if ok {
		return group.Allowed(url)
	}
	return l.defaultLimits.Allowed(url)
}

// GetUserAgentRules gets the rules for the userAgent, returns the default (*) group if it was present and no other groups apply.
// Returns nil if no groups apply and no default group was supplied.
func (l *RobotFile) GetUserAgentRules(userAgent string) *UserAgentRules {
	for _, group := range l.groups {
		if group.userAgent == userAgent {
			return group
		}
	}
	return l.defaultLimits
}

// GetDelay returns the User-agent specific crawl-delay if it exists, otherwise the catch-all delay.
// Returns def if neither a specific or global crawl-delay exist.
func (l *RobotFile) GetDelay(userAgent string, defaultDelay time.Duration) time.Duration {
	return l.GetUserAgentRules(userAgent).GetDelay(defaultDelay)
}

// Sitemap returns the URL to the sitemap for the given User-agent.
// Returns the default sitemap if no User-agent specific sitemap was specified, otherwise nil.
func (l *RobotFile) GetSitemap(userAgent string, client http.RoundTripper) (*Sitemap, error) {
	if l.sitemap == nil {
		return nil, errors.New("No sitemap in robots.txt")
	}

	return NewSitemapFromURL(l.sitemap.String(), client)
}

// UserAgentRules holds limits for a single user agent.
type UserAgentRules struct {
	userAgent  string
	allowed    []string
	disallowed []string
	delay      time.Duration
}

func newUserAgentRules(userAgent string) *UserAgentRules {
	return &UserAgentRules{
		userAgent:  userAgent,
		allowed:    make([]string, 0),
		disallowed: make([]string, 0),
		delay:      -1,
	}
}

// Applies returns true if the group applies to the given userAgent
func (g *UserAgentRules) Applies(userAgent string) bool {
	return g.userAgent == userAgent
}

// Allowed returns true if the url is allowed by the group rules. Check if the group applies to the user agent first by using Applies.
func (g *UserAgentRules) Allowed(url string) bool {
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

// GetDelay returns the Crawl-delay.
// Returns defaultDelay if no crawl delay was specified.
func (g *UserAgentRules) GetDelay(defaultDelay time.Duration) time.Duration {
	if g.delay == -1 {
		return defaultDelay
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
			add, match := matchWildcard(seekChar, url[j:])
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
