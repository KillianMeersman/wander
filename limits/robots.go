package limits

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

// RobotParsingError signals the spider encountered an invalid robots.txt file.
type RobotParsingError struct {
	Domain string
	Err    string
}

func (e *RobotParsingError) Error() string {
	return fmt.Sprintf("Robots.txt for %s invalid: %s", e.Domain, e.Err)
}

type RobotLimits struct {
	defaultLimits *RobotLimitGroup
	groups        map[string]*RobotLimitGroup
}

func newRobotLimits() *RobotLimits {
	return &RobotLimits{
		groups: make(map[string]*RobotLimitGroup, 0),
	}
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

func ParseRobotLimits(in io.Reader) (*RobotLimits, error) {
	scanner := bufio.NewScanner(in)

	var group *RobotLimitGroup
	limits := newRobotLimits()

	for scanner.Scan() {
		line := scanner.Text()

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
		directive := strings.Trim(parts[0], " \t")
		parameter := ""
		if len(parts) > 1 {
			parameter = strings.Trim(parts[1], " \t")
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

		case "":
			continue

		default:
			return nil, fmt.Errorf("Unknown directive %s", line)
		}
	}

	limits.addLimitGroup(group)
	return limits, nil
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
