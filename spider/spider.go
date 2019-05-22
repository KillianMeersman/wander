package spider

import (
	"context"
	"net/http"
	"regexp"
	"sync"
)

type ParseFunc func(response *Response)
type RequestFunc func(path string)
type ErrorFunc func(err error)

// Spider with callbacks
type Spider struct {
	cache          sync.Map
	queue          chan string
	allowedDomains []regexp.Regexp

	threadn     int
	parseFunc   ParseFunc
	requestFunc RequestFunc
	errFunc     ErrorFunc
}

func NewSpider() *Spider {
	return &Spider{
		cache:       sync.Map{},
		queue:       make(chan string, 100),
		threadn:     4,
		parseFunc:   func(response *Response) {},
		requestFunc: func(path string) {},
		errFunc:     func(err error) {},
	}
}

// Parse a page
func (s *Spider) Parse(pfunc ParseFunc) {
	s.parseFunc = pfunc
}

// OnError is called on errors
func (s *Spider) OnError(efunc ErrorFunc) {
	s.errFunc = efunc
}

// OnRequest is called when a request is about to be made
func (s *Spider) OnRequest(vfunc RequestFunc) {
	s.requestFunc = vfunc
}

// Visit a page by adding the path to the queue, blocks when the queue is full until there is free space
func (s *Spider) Visit(path string) {
	// check if the
	if _, ok := s.cache.Load(path); !ok {
		s.queue <- path
	}
}

// Run the spider, blocks while the spider is running
func (s *Spider) Run(ctx context.Context) {
	wg := sync.WaitGroup{}
	wg.Add(s.threadn)
	for i := 0; i < s.threadn; i++ {
		go func() {
			for {
				select {
				case path := <-s.queue:
					go s.requestFunc(path)
					res, err := http.Get(path)
					if err != nil {
						go s.errFunc(err)
						continue
					}
					doc, err := NewResponse(res)
					if err != nil {
						go s.errFunc(err)
					}
					go s.parseFunc(doc)
				case <-ctx.Done():
					wg.Done()
					return
				}
			}
		}()
	}
	wg.Wait()
}
