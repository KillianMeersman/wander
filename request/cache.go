package request

import (
	"net/url"
	"sync"
)

type RequestCache interface {
	AddRequest(req *Request)
	Visited(req *Request) bool
}

type LocalRequestCache struct {
	requests map[*url.URL]*Request
	lock     sync.RWMutex
}

func NewRequestCache() *LocalRequestCache {
	return &LocalRequestCache{
		requests: make(map[*url.URL]*Request),
		lock:     sync.RWMutex{},
	}
}

func (c *LocalRequestCache) AddRequest(req *Request) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.requests[req.URL] = req
}

func (c *LocalRequestCache) Visited(req *Request) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	_, ok := c.requests[req.URL]
	return ok
}
