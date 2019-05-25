package wander

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/KillianMeersman/wander/limits"

	"github.com/KillianMeersman/wander/request"
)

type RequestFunc func(req *request.Request)
type ResponseFunc func(res *request.Response)
type ErrorFunc func(err error)

type SpiderState struct {
	requests request.RequestQueue
}

// Spider with callbacks
type Spider struct {
	cache          sync.Map
	queue          request.RequestQueue
	allowedDomains []*regexp.Regexp
	limits         []limits.Limit

	// parallelism
	threadn int

	// http
	client *http.Client

	// control channels
	pause  chan struct{}
	resume chan struct{}

	requestFunc  RequestFunc
	responseFunc ResponseFunc
	errFunc      ErrorFunc
}

func NewSpider(allowedDomains []string, threadn int) (*Spider, error) {
	spider := &Spider{
		cache:   sync.Map{},
		queue:   request.NewRequestHeap(10000),
		threadn: threadn,
		limits:  make([]limits.Limit, 0),
		client:  &http.Client{},

		responseFunc: func(res *request.Response) {},
		requestFunc:  func(req *request.Request) {},
		errFunc:      func(err error) {},
	}
	err := spider.AllowedDomains(allowedDomains...)
	if err != nil {
		return nil, err
	}

	return spider, nil
}

func (s *Spider) AddLimits(limits ...limits.Limit) {
	s.limits = append(s.limits, limits...)
}

func (s *Spider) SetProxyFunc(proxyFunc func(r *http.Request) (*url.URL, error)) {
	s.client.Transport = &http.Transport{
		Proxy: proxyFunc,
	}
}

// AllowedDomains sets the allowed domains for this spider
func (s *Spider) AllowedDomains(paths ...string) error {
	regexs := make([]*regexp.Regexp, len(paths))
	for i, path := range paths {
		regex, err := regexp.Compile(path)
		if err != nil {
			return err
		}
		regexs[i] = regex
	}
	s.allowedDomains = regexs
	return nil
}

// OnResponse is called when a response has been received and tokenized
func (s *Spider) OnResponse(pfunc ResponseFunc) {
	s.responseFunc = pfunc
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
	req, err := request.NewRequest(path, nil)
	if err != nil {
		s.err(err)
		return
	}

	s.addRequest(req, 100000000000000000)
}

// Follow a link by adding the path to the queue, blocks when the queue is full until there is free space.
// Unlike Visit, this method also accepts a response, allowing the url parser to convert relative urls into absolute ones and keep track of depth.
func (s *Spider) Follow(path string, res *request.Response, priority int) {
	req, err := request.NewRequest(path, res.Request)
	if err != nil {
		s.err(err)
		return
	}

	s.addRequest(req, priority)
}

// Start the spider, blocks while the spider is running. Returns the spider state after context is cancelled.
func (s *Spider) Start(ctx context.Context) *SpiderState {
	wg := sync.WaitGroup{}
	wg.Add(s.threadn)
	for i := 0; i < s.threadn; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					wg.Done()
					return

				case req := <-s.queue.Dequeue(ctx):
					if s.filterDomains(req) {
						for _, limit := range s.limits {
							err := limit.Check(req)
							if err != nil {
								s.err(err)
								continue
							}
						}
						s.getResponse(req)
					} else {
						s.err(fmt.Errorf("domain %s filtered", req.String()))
					}
				}
			}
		}()
	}
	wg.Wait()
	return &SpiderState{
		requests: s.queue,
	}
}

// Resume from spider state
func (s *Spider) Resume(ctx context.Context, state *SpiderState) *SpiderState {
	s.queue = state.requests
	return s.Start(ctx)
}

func (s *Spider) filterDomains(request *request.Request) bool {
	for _, domain := range s.allowedDomains {
		if domain.MatchString(request.Hostname()) {
			return true
		}
	}
	return false
}

func (s *Spider) getResponse(req *request.Request) {
	s.request(req)
	res, err := s.client.Get(req.String())
	if err != nil {
		s.err(err)
		return
	}
	doc, err := request.NewResponse(req, res)
	if err != nil {
		s.err(err)
		return
	}
	s.response(doc)
}

func (s *Spider) addRequest(req *request.Request, priority int) {
	for _, limit := range s.limits {
		err := limit.NewRequest(req)
		if err != nil {
			s.err(err)
			return
		}
	}
	if _, ok := s.cache.Load(req.URL.String()); !ok {
		s.cache.Store(req.URL.String(), struct{}{})
		err := s.queue.Enqueue(req, priority)
		if err != nil {
			s.err(err)
		}
	}
}

func (s *Spider) request(req *request.Request) {
	go s.requestFunc(req)
}

func (s *Spider) response(res *request.Response) {
	go s.responseFunc(res)
}

func (s *Spider) err(err error) {
	go s.errFunc(err)
}
