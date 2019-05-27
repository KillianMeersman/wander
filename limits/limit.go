package limits

import (
	"fmt"

	"github.com/KillianMeersman/wander/request"
)

// Limit interface filters requests when they are enqueued
type Limit interface {
	// FilterRequest filters a request before it is enqueued
	FilterRequest(req *request.Request) error
}

// MaxDepthLimit will filter a request if it's depth > max
type MaxDepthLimit struct {
	Max int
}

// MaxDepth will filter a request if it's depth > max
func MaxDepth(max int) *MaxDepthLimit {
	return &MaxDepthLimit{
		max,
	}
}

func (m *MaxDepthLimit) FilterRequest(req *request.Request) error {
	if req.Depth() > m.Max {
		return fmt.Errorf("Maximum depth reached (%d)", m.Max)
	}
	return nil
}

func (m *MaxDepthLimit) ID() string {
	return fmt.Sprintf("MaxDepthLimit-%d", m.Max)
}
