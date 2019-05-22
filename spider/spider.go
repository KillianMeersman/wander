package spider

import (
	"context"
	"errors"
	"fmt"
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
	queue          chan *Request
	allowedDomains []*regexp.Regexp
	throttles      []*Throttle

	threadn     int
	parseFunc   ParseFunc
	requestFunc RequestFunc
	errFunc     ErrorFunc
}

func NewSpider(allowedDomains []string, threadn int) (*Spider, error) {
	allowedRegexs := make([]*regexp.Regexp, len(allowedDomains))
	for i, path := range allowedDomains {
		allowed, err := regexp.Compile(path)
		if err != nil {
			return nil, err
		}
		allowedRegexs[i] = allowed
	}

	return &Spider{
		cache:          sync.Map{},
		queue:          make(chan *Request, 100),
		threadn:        threadn,
		allowedDomains: allowedRegexs,
		throttles:      make([]*Throttle, 0),

		parseFunc:   func(response *Response) {},
		requestFunc: func(path string) {},
		errFunc:     func(err error) {},
	}, nil
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
	request, err := NewRequest(path, nil)
	if err != nil {
		s.errFunc(err)
		return
	}

	if _, ok := s.cache.Load(path); !ok {
		s.cache.Store(path, struct{}{})
		s.queue <- request
	}
}

// Follow a link by adding the path to the queue, blocks when the queue is full until there is free space.
// Unlike Visit, this method also accepts a response, allowing the url parser to convert relative urls into absolute ones and keep track of depth.
func (s *Spider) Follow(path string, res *Response) {
	request, err := NewRequest(path, res.Request)
	if err != nil {
		s.errFunc(err)
		return
	}

	if _, ok := s.cache.Load(path); !ok {
		s.cache.Store(path, struct{}{})
		s.queue <- request
	}
}

func (s *Spider) Throttle(throttles ...*Throttle) {
	for _, throttle := range throttles {
		s.throttles = append(s.throttles, throttle)
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
				case request := <-s.queue:
					for _, domain := range s.allowedDomains {
						if domain.MatchString(request.Hostname()) {
							goto allowed
						}
					}
					s.errFunc(errors.New(fmt.Sprintf("domain %s filtered", request.String())))
					continue

				allowed:
					for _, throttle := range s.throttles {
						if throttle.Applies(request.String()) {
							throttle.Wait()
						}
					}

					go s.requestFunc(request.String())
					res, err := http.Get(request.String())
					if err != nil {
						go s.errFunc(err)
						continue
					}
					doc, err := NewResponse(request, res)
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
