package limits

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/KillianMeersman/wander/request"
)

// RobotParsingError signals the spider encountered an invalid robots.txt file.
type RobotParsingError struct {
	Domain string
	Err    string
}

func (e *RobotParsingError) Error() string {
	return fmt.Sprintf("Robots.txt for %s invalid: %s", e.Domain, e.Err)
}

// RobotLimitCache holds the robot exclusions for multiple domains.
type RobotLimitCache struct {
	limits map[string]*RobotLimits
}

func NewRobotLimitCache() *RobotLimitCache {
	return &RobotLimitCache{
		limits: make(map[string]*RobotLimits),
	}
}

// Allowed returns true if the userAgent is allowed to access the given path on the given domain.
// Returns error if no robot file is cached for the given domain.
func (c *RobotLimitCache) Allowed(userAgent string, req *request.Request) (bool, error) {
	limits, ok := c.limits[req.Host]
	if !ok {
		return false, fmt.Errorf("No limits found for domain %s", req.Host)
	}
	return limits.Allowed(userAgent, req.Path), nil
}

// GetLimits gets the limits for a host. Returns an error when no limits are cached for the given host.
func (c *RobotLimitCache) GetLimits(host string) (*RobotLimits, error) {
	limits, ok := c.limits[host]
	if !ok {
		return nil, fmt.Errorf("No limits found for domain %s", host)
	}
	return limits, nil
}

// AddLimits adds or replaces the limits for a host. Returns an error if the limits are invalid.
func (c *RobotLimitCache) AddLimits(in io.Reader, host string) (*RobotLimits, error) {
	limits, err := ParseRobotLimits(in)
	if err != nil {
		return nil, err
	}

	c.limits[host] = limits
	return limits, nil
}

// RobotLimits holds all the limits imposed by a robots exclusion file.
type RobotLimits struct {
	defaultLimits *RobotLimitGroup
	groups        map[string]*RobotLimitGroup
}

func newRobotLimits() *RobotLimits {
	return &RobotLimits{
		groups: make(map[string]*RobotLimitGroup, 0),
	}
}

// ParseRobotLimits will parse a robot exclusion file, returns a normal error if it encounters an invalid directive.
func ParseRobotLimits(in io.Reader) (*RobotLimits, error) {
	scanner := bufio.NewScanner(in)
	limits := newRobotLimits()

	// current host specification
	var group *RobotLimitGroup
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
		parts := strings.Split(line, ":")
		if len(parts) < 1 {
			return nil, fmt.Errorf("Invalid directive %s", line)
		}
		directive := parts[0]
		parameter := ""
		if len(parts) > 1 {
			parameter = strings.TrimPrefix(parts[1], " ")
		}

		switch directive {
		case "User-agent":
			if parameter == "" {
				return nil, fmt.Errorf("Invalid User-agent directive %s", line)
			}
			if group != nil {
				limits.addLimitGroup(group)
			}
			group = newRobotLimitGroup(parameter)

		case "Disallow":
			if group == nil {
				return nil, errors.New("Disallow directive without User-agent")
			}
			if parameter == "" {
				group.disallowed = make([]string, 0)
			} else {
				group.disallowed = append(group.disallowed, parameter)
			}

		case "Allow":
			if group == nil {
				return nil, errors.New("Allow directive without User-agent")
			}
			if parameter == "" {
				group.allowed = make([]string, 0)
			} else {
				group.allowed = append(group.allowed, parameter)
			}

		default:
			return nil, fmt.Errorf("Unknown directive %s", line)
		}
	}

	limits.addLimitGroup(group)
	return limits, nil
}

func (l *RobotLimits) addLimitGroup(g *RobotLimitGroup) {
	if g.host == "*" {
		l.defaultLimits = g
		return
	}
	l.groups[g.host] = g
}

// Allowed returns true if the user agent is allowed to access the given url.
func (l *RobotLimits) Allowed(userAgent, url string) bool {
	group, ok := l.groups[userAgent]
	if ok {
		return group.Allowed(url)
	}
	if l.defaultLimits != nil {
		return l.defaultLimits.Allowed(url)
	}
	return false
}

// GetLimits gets the RobotLimitGroup for the userAgent, returns the default (*) group if it was present and no other groups apply.
func (l *RobotLimits) GetLimits(userAgent string) *RobotLimitGroup {
	for _, group := range l.groups {
		if group.host == userAgent {
			return group
		}
	}
	return l.defaultLimits
}

// RobotLimitGroup holds the limits for a single user agent
type RobotLimitGroup struct {
	host       string
	allowed    []string
	disallowed []string
}

func newRobotLimitGroup(host string) *RobotLimitGroup {
	return &RobotLimitGroup{
		host: host,
	}
}

// Applies returns true if the group applies to the given userAgent
func (g *RobotLimitGroup) Applies(userAgent string) bool {
	return g.host == userAgent
}

// Allowed returns true if the url is allowed by the group rules. Check if the group applies to the user agent first by using Applies.
func (g *RobotLimitGroup) Allowed(url string) bool {
	for _, allowed := range g.allowed {
		if strings.HasPrefix(url, allowed) {
			return true
		}
	}
	for _, disallowed := range g.disallowed {
		if strings.HasPrefix(url, disallowed) {
			return false
		}
	}
	return true
}