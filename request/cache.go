package request

import (
	"net/url"
	"sync"
)

type RequestCache interface {
	AddRequest(req *Request)
	VisitedURL(req *Request) bool
	VisitedDomain(req *Request) bool
}

type LocalRequestCache struct {
	requests map[*url.URL]struct{}
	domains  map[string]struct{}
	lock     sync.RWMutex
}

func NewRequestCache() *LocalRequestCache {
	return &LocalRequestCache{
		requests: make(map[*url.URL]struct{}),
		domains:  make(map[string]struct{}),
		lock:     sync.RWMutex{},
	}
}

func (c *LocalRequestCache) AddRequest(req *Request) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.requests[req.URL] = struct{}{}
}

func (c *LocalRequestCache) VisitedURL(req *Request) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	_, ok := c.requests[req.URL]
	return ok
}

func (c *LocalRequestCache) VisitedDomain(req *Request) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	_, ok := c.domains[req.Hostname()]
	return ok
}
