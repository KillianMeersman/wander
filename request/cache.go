package request

import (
	"net/url"
	"sync"
)

// Cache holds visited urls to prevent revisitation
type Cache interface {
	AddRequest(req *Request)
	VisitedURL(req *Request) bool
	VisitedDomain(req *Request) bool
}

// LocalCache holds the visited urls in maps. Safe for use by multiple goroutines.
type LocalCache struct {
	requests map[*url.URL]struct{}
	domains  map[string]struct{}
	lock     sync.RWMutex
}

func NewCache() *LocalCache {
	return &LocalCache{
		requests: make(map[*url.URL]struct{}),
		domains:  make(map[string]struct{}),
		lock:     sync.RWMutex{},
	}
}

// AddRequest adds a request url to the cache.
func (c *LocalCache) AddRequest(req *Request) {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.requests[req.URL] = struct{}{}
}

// VisitedURL returns true if the request url has been visited before.
func (c *LocalCache) VisitedURL(req *Request) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	_, ok := c.requests[req.URL]
	return ok
}

// VisitedDomain returns true if the request domain has been visited before.
func (c *LocalCache) VisitedDomain(req *Request) bool {
	c.lock.RLock()
	defer c.lock.RUnlock()

	_, ok := c.domains[req.Host]
	return ok
}
