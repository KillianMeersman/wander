// Package limits provides request filters and throttles.
package limits

import (
	"github.com/KillianMeersman/wander/request"
)

// RequestFilter is used to filter request before they are enqueued.
type RequestFilter interface {
	// FilterRequest filters a request before it is enqueued
	FilterRequest(req *request.Request) error
}

// MaxDepthFilter will filter a request if it's depth is larger than the maximum.
type MaxDepthFilter struct {
	MaxDepth int
}

// NewMaxDepthFilter instantiates a new max depth filter.
func NewMaxDepthFilter(maxDepth int) *MaxDepthFilter {
	return &MaxDepthFilter{
		maxDepth,
	}
}

// FilterRequest returns an
func (m *MaxDepthFilter) FilterRequest(req *request.Request) error {
	if req.Depth() > m.MaxDepth {
		return &MaxDepthReached{Depth: m.MaxDepth, Request: req}
	}
	return nil
}
