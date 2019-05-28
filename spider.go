package wander

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/KillianMeersman/wander/limits"

	"github.com/KillianMeersman/wander/request"
)

// RequestFunc callback for requests
type RequestFunc func(req *request.Request)

// ResponseFunc callback for responses
type ResponseFunc func(res *request.Response)

// ErrorFunc callback for errors
type ErrorFunc func(err error)

//SpiderState holds a spider state after a crawl, allows a spider to resume a stopped crawl
type SpiderState struct {
	queue request.RequestQueue
	cache request.RequestCache
}

// Spider is the high level crawler
type Spider struct {
	SpiderState
	allowedDomains []*regexp.Regexp
	limits         map[string]limits.Limit
	throttle       limits.ThrottleCollection

	// parallelism
	ingestorN int
	pipelineN int

	// http
	client *http.Client

	// callbacks
	requestFunc  RequestFunc
	responseFunc ResponseFunc
	errFunc      ErrorFunc
}

// NewSpider return a new spider
func NewSpider(options ...func(*Spider) error) (*Spider, error) {
	spider := &Spider{
		SpiderState:    SpiderState{},
		allowedDomains: make([]*regexp.Regexp, 0),
		limits:         make(map[string]limits.Limit),
		pipelineN:      1,
		ingestorN:      1,
		client:         &http.Client{},
		responseFunc:   func(res *request.Response) {},
		requestFunc:    func(req *request.Request) {},
		errFunc:        func(err error) {},
	}

	for _, option := range options {
		err := option(spider)
		if err != nil {
			return nil, err
		}
	}
	spider.init()

	return spider, nil
}

/*
	Constructor options
*/

// AllowedDomains sets allowed domains, utility funtion for SetAllowedDomains
func AllowedDomains(domains ...string) func(s *Spider) error {
	return func(s *Spider) error {
		return s.SetAllowedDomains(domains...)
	}
}

// Ingestors sets the amount of goroutines for ingestors
func Ingestors(n int) func(s *Spider) error {
	return func(s *Spider) error {
		s.ingestorN = n
		return nil
	}
}

// Pipelines sets the amount of goroutines for callback functions
func Pipelines(n int) func(s *Spider) error {
	return func(s *Spider) error {
		s.pipelineN = n
		return nil
	}
}

// Threads sets the amount of ingestors and pipelines to n, spawning a total of n*2 goroutines.
func Threads(n int) func(s *Spider) error {
	return func(s *Spider) error {
		s.ingestorN = n
		s.pipelineN = n
		return nil
	}
}

// ProxyFunc sets the proxy function, utility function for SetProxyFunc
func ProxyFunc(f func(r *http.Request) (*url.URL, error)) func(s *Spider) error {
	return func(s *Spider) error {
		s.SetProxyFunc(f)
		return nil
	}
}

// MaxDepth sets the maximum spider depth
func MaxDepth(max int) func(s *Spider) error {
	return func(s *Spider) error {
		s.AddLimits(limits.MaxDepth(max))
		return nil
	}
}

// Queue sets the RequestQueue
func Queue(queue request.RequestQueue) func(s *Spider) error {
	return func(s *Spider) error {
		s.queue = queue
		return nil
	}
}

// Cache sets the RequestCache
func Cache(cache request.RequestCache) func(s *Spider) error {
	return func(s *Spider) error {
		s.cache = cache
		return nil
	}
}

// AddLimits adds limits to the spider, deduplicates limits.
func (s *Spider) AddLimits(limits ...limits.Limit) {
	for _, limit := range limits {
		contents, _ := json.Marshal(limit)
		s.limits[string(contents)] = limit
	}
}

// RemoveLimits removes the given limits
func (s *Spider) RemoveLimits(limits ...limits.Limit) {
	for _, limit := range limits {
		contents, _ := json.Marshal(limit)
		delete(s.limits, string(contents))
	}
}

// SetThrottles sets or replaces the default and custom throttles for the spider
func (s *Spider) SetThrottles(def *limits.DefaultThrottle, throttles ...limits.Throttle) {
	s.throttle = limits.NewThrottleCollection(def, throttles...)
}

// SetProxyFunc sets the proxy function to be used
func (s *Spider) SetProxyFunc(proxyFunc func(r *http.Request) (*url.URL, error)) {
	s.client.Transport = &http.Transport{
		Proxy: proxyFunc,
	}
}

// SetAllowedDomains sets the allowed domain regexs.
func (s *Spider) SetAllowedDomains(paths ...string) error {
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
func (s *Spider) Visit(path string) error {
	req, err := request.NewRequest(path, nil)
	if err != nil {
		return err
	}

	return s.addRequest(req, int(^uint(0)>>1))
}

// Follow a link by adding the path to the queue, blocks when the queue is full until there is free space.
// Unlike Visit, this method also accepts a response, allowing the url parser to convert relative urls into absolute ones and keep track of depth.
func (s *Spider) Follow(path string, res *request.Response, priority int) error {
	req, err := request.NewRequest(path, res.Request)
	if err != nil {
		return err
	}

	return s.addRequest(req, priority)
}

// Start the spider, blocks while the spider is running. Returns the spider state after context is cancelled.
func (s *Spider) Start(ctx context.Context) *SpiderState {
	ingestors := sync.WaitGroup{}
	ingestors.Add(s.ingestorN)
	pipelines := sync.WaitGroup{}
	pipelines.Add(s.pipelineN)
	pipelinesCtx, stopPipelines := context.WithCancel(context.Background())

	reqc := make(chan *request.Request)
	resc := make(chan *request.Response)
	errc := make(chan error)

	for i := 0; i < s.ingestorN; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					ingestors.Done()
					return

				case req := <-s.queue.Dequeue(ctx):
					if s.filterDomains(req) {
						s.throttle.Wait(req)
						reqc <- req

						res, err := s.getResponse(req)
						if err != nil {
							errc <- err
							continue
						}
						resc <- res

					} else {
						s.errFunc(fmt.Errorf("domain %s filtered", req.String()))
					}
				}
			}
		}()
	}

	for i := 0; i < s.pipelineN; i++ {
		go func() {
			for {
				select {
				case req := <-reqc:
					s.requestFunc(req)
				case res := <-resc:
					s.responseFunc(res)
				case err := <-errc:
					s.errFunc(err)
				case <-pipelinesCtx.Done():
					pipelines.Done()
					return
				}
			}
		}()
	}

	ingestors.Wait()
	stopPipelines()
	pipelines.Wait()
	return &s.SpiderState
}

// Resume from spider state
func (s *Spider) Resume(ctx context.Context, state *SpiderState) *SpiderState {
	s.SpiderState = *state
	return s.Start(ctx)
}

func (s *Spider) init() {
	if s.queue == nil {
		s.queue = request.NewRequestHeap(10000)
	}
	if s.cache == nil {
		s.cache = request.NewRequestCache()
	}
}

func (s *Spider) filterDomains(request *request.Request) bool {
	for _, domain := range s.allowedDomains {
		if domain.MatchString(request.Hostname()) {
			return true
		}
	}
	return false
}

func (s *Spider) getResponse(req *request.Request) (*request.Response, error) {
	res, err := s.client.Get(req.String())
	if err != nil {
		return nil, err
	}

	doc, err := request.NewResponse(req, res)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

func (s *Spider) addRequest(req *request.Request, priority int) error {
	for _, limit := range s.limits {
		err := limit.FilterRequest(req)
		if err != nil {
			return err
		}
	}

	if !s.cache.Visited(req) {
		s.cache.AddRequest(req)
		err := s.queue.Enqueue(req, priority)
		if err != nil {
			return err
		}
	}
	return nil
}
