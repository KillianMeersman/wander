package wander

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/KillianMeersman/wander/limits"

	"github.com/KillianMeersman/wander/request"
)

// RequestFunc Callback for requests
type RequestFunc func(req *request.Request)

// ResponseFunc callback for responses
type ResponseFunc func(res *request.Response)

// ErrorFunc callback for errors
type ErrorFunc func(err error)

//SpiderState holds a spider state after a crawl, allows a spider to resume a stopped crawl
type SpiderState struct {
	requests request.RequestQueue
}

type Spider struct {
	cache          sync.Map
	queue          request.RequestQueue
	allowedDomains []*regexp.Regexp
	limits         []limits.Limit

	// parallelism
	threadn int

	// http
	client *http.Client

	// callbacks
	requestFunc  RequestFunc
	responseFunc ResponseFunc
	errFunc      ErrorFunc
}

func NewSpider(options ...func(*Spider) error) (*Spider, error) {
	spider := &Spider{
		cache:          sync.Map{},
		queue:          request.NewRequestHeap(10000),
		allowedDomains: make([]*regexp.Regexp, 0),
		limits:         make([]limits.Limit, 0),
		threadn:        1,
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

	return spider, nil
}

// Constructor options
func AllowedDomains(domains ...string) func(s *Spider) error {
	return func(s *Spider) error {
		return s.AllowedDomains(domains...)
	}
}

func Threads(n int) func(s *Spider) error {
	return func(s *Spider) error {
		s.threadn = n
		return nil
	}
}

func ProxyFunc(f func(r *http.Request) (*url.URL, error)) func(s *Spider) error {
	return func(s *Spider) error {
		s.SetProxyFunc(f)
		return nil
	}
}

func MaxDepth(max int) func(s *Spider) error {
	return func(s *Spider) error {
		s.AddLimits(limits.MaxDepth(max))
		return nil
	}
}

func (s *Spider) AddLimits(limits ...limits.Limit) {
	s.limits = append(s.limits, limits...)
}

func (s *Spider) Throttle(def *limits.DefaultThrottle, throttles ...limits.Throttle) {
	group := limits.NewThrottleCollection(def, throttles...)
	s.AddLimits(group)
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
func (s *Spider) Visit(path string) error {
	req, err := request.NewRequest(path, nil)
	if err != nil {
		return err
	}

	return s.addRequest(req, 100000000000000000)
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
	ingestors.Add(s.threadn)

	extractors := sync.WaitGroup{}
	extractors.Add(s.threadn)
	extractorCtx, stopExtractors := context.WithCancel(context.Background())

	reqc := make(chan *request.Request)
	resc := make(chan *request.Response)
	errc := make(chan error)

	for i := 0; i < s.threadn; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					ingestors.Done()
					log.Println("ingestor done")
					return

				case req := <-s.queue.Dequeue(ctx):
					if s.filterDomains(req) {
						for _, limit := range s.limits {
							err := limit.Check(req)
							if err != nil {
								errc <- err
								continue
							}
						}
						reqc <- req
						res, err := s.getResponse(req)
						if err != nil {
							errc <- err
							continue
						}
						resc <- res
					} else {
						errc <- fmt.Errorf("domain %s filtered", req.String())
					}
				}
			}
		}()

		go func() {
			for {
				select {
				case <-extractorCtx.Done():
					extractors.Done()
					log.Println("extractor done")
					return

				case req := <-reqc:
					s.requestFunc(req)
				case res := <-resc:
					s.responseFunc(res)
				case err := <-errc:
					s.errFunc(err)
				}
			}
		}()
	}

	ingestors.Wait()
	stopExtractors()
	extractors.Wait()
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
		err := limit.NewRequest(req)
		if err != nil {
			return err
		}
	}
	if _, ok := s.cache.Load(req.URL.String()); !ok {
		s.cache.Store(req.URL.String(), struct{}{})
		err := s.queue.Enqueue(req, priority)
		if err != nil {
			return err
		}
	}
	return nil
}
