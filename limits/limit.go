package limits

import (
	"fmt"

	"github.com/KillianMeersman/wander/request"
)

type Limit interface {
	Check(req *request.Request) error
	NewRequest(req *request.Request) error
}

type MaxDepthLimit struct {
	max int
}

func MaxDepth(max int) *MaxDepthLimit {
	return &MaxDepthLimit{
		max,
	}
}

func (m *MaxDepthLimit) Check(req *request.Request) error {
	return nil
}

func (m *MaxDepthLimit) NewRequest(req *request.Request) error {
	if req.Depth() > m.max {
		return fmt.Errorf("Maximum depth reached (%d)", m.max)
	}
	return nil
}
