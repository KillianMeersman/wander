package request

import (
	"sync"
)

// Cache holds visited urls to prevent revisitation
type Cache interface {
	AddRequest(req *Request)
	VisitedURL(req *Request) bool
}

// LocalCache holds urls in maps. Safe for use by multiple goroutines.
type LocalCache struct {
	requests map[string]struct{}
	lock     sync.RWMutex
}

func NewCache() *LocalCache {
	return &LocalCache{
		requests: make(map[string]struct{}),
		lock:     sync.RWMutex{},
	}
}

// AddRequest adds a request url to the cache.
func (c *LocalCache) AddRequest(req *Request) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.requests[req.URL.String()] = struct{}{}
}

// VisitedURL returns true if the request url has been visited before.
func (c *LocalCache) VisitedURL(req *Request) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	_, ok := c.requests[req.URL.String()]
	return ok
}
