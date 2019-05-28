package limits

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"
)

type RobotLimits struct {
	defaultLimits *RobotLimitGroup
	groups        []*RobotLimitGroup
}

func newRobotLimits() *RobotLimits {
	return &RobotLimits{
		groups: make([]*RobotLimitGroup, 0),
	}
}

func (l *RobotLimits) addLimitGroup(g *RobotLimitGroup) {
	if g.host == "*" {
		l.defaultLimits = g
		return
	}
	l.groups = append(l.groups, g)
}

func (l *RobotLimits) Allowed(userAgent, url string) bool {
	for _, group := range l.groups {
		if group.Applies(userAgent) {
			return group.Allowed(url)
		}
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

		parts := strings.Split(line, ":")
		if len(parts) < 1 {
			return nil, fmt.Errorf("Invalid directive %s", line)
		}
		directive := parts[0]
		parameter := ""
		if len(parts) > 1 {
			parameter = parts[1]
			if strings.HasPrefix(parameter, " ") {
				parameter = strings.Replace(parameter, " ", "", 1)
			}
		}

		switch directive {
		case "User-agent":
			if len(parts) < 2 {
				return nil, fmt.Errorf("Invalid directive %s", line)
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

func (g *RobotLimitGroup) Applies(host string) bool {
	return g.host == host
}

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
