package wander

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/PuerkitoBio/goquery"

	"github.com/KillianMeersman/wander/limits"

	"github.com/KillianMeersman/wander/request"
)

type RequestPipeline func(req *request.Request)
type ResponsePipeline func(res *request.Response)
type HTMLPipeline func(el *goquery.Selection)
type ErrorPipeline func(err error)
type SpiderConstructorOption func(s *Spider) error
type RobotLimitFunction func(spid *Spider, req *request.Request) error

// SpiderState holds a spider's state, such as the request queue and cache.
// It is returned by the Start and Resume methods, allows the Resume method to resume a previously stopped crawl.
type SpiderState struct {
	queue request.Queue
	cache request.Cache
}

// Spider provides a parallelized scraper.
type Spider struct {
	SpiderState
	allowedDomains []*regexp.Regexp
	RobotLimits    *limits.RobotLimitCache
	limits         map[string]limits.Limit
	throttle       limits.ThrottleCollection
	selectors      map[string]HTMLPipeline

	// parallelism
	ingestorN int
	pipelineN int
	ctx       context.Context
	cancel    func()

	// http
	client *http.Client

	// options
	UserAgent              string
	RobotExclusionFunction RobotLimitFunction

	// callbacks
	requestPipeline  RequestPipeline
	responsePipeline ResponsePipeline
	errorPipeline    ErrorPipeline
}

func NewSpider(options ...SpiderConstructorOption) (*Spider, error) {
	spider := &Spider{
		SpiderState:    SpiderState{},
		allowedDomains: make([]*regexp.Regexp, 0),
		limits:         make(map[string]limits.Limit),
		selectors:      make(map[string]HTMLPipeline),

		pipelineN: 1,
		ingestorN: 1,

		client:                 &http.Client{},
		UserAgent:              "Wander/0.1",
		RobotExclusionFunction: FollowRobotRules,

		responsePipeline: func(res *request.Response) {},
		requestPipeline:  func(req *request.Request) {},
		errorPipeline:    func(err error) {},
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

// AllowedDomains sets allowed domains, utility funtion for SetAllowedDomains.
func AllowedDomains(domains ...string) SpiderConstructorOption {
	return func(s *Spider) error {
		return s.SetAllowedDomains(domains...)
	}
}

// Ingestors sets the amount of goroutines for ingestors.
func Ingestors(n int) SpiderConstructorOption {
	return func(s *Spider) error {
		s.ingestorN = n
		return nil
	}
}

// Pipelines sets the amount of goroutines for callback functions.
func Pipelines(n int) SpiderConstructorOption {
	return func(s *Spider) error {
		s.pipelineN = n
		return nil
	}
}

// Threads sets the amount of ingestors and pipelines to n, spawning a total of n*2 goroutines.
func Threads(n int) SpiderConstructorOption {
	return func(s *Spider) error {
		s.ingestorN = n
		s.pipelineN = n
		return nil
	}
}

// ProxyFunc sets the proxy function, utility function for SetProxyFunc.
func ProxyFunc(f func(r *http.Request) (*url.URL, error)) SpiderConstructorOption {
	return func(s *Spider) error {
		s.SetProxyFunc(f)
		return nil
	}
}

// MaxDepth sets the maximum request depth.
func MaxDepth(max int) SpiderConstructorOption {
	return func(s *Spider) error {
		s.AddLimits(limits.MaxDepth(max))
		return nil
	}
}

// Queue sets the RequestQueue.
// Allows request queues to be shared between spiders.
func Queue(queue request.Queue) SpiderConstructorOption {
	return func(s *Spider) error {
		s.queue = queue
		return nil
	}
}

// Cache sets the RequestCache.
// Allows request caches to be shared between spiders.
func Cache(cache request.Cache) SpiderConstructorOption {
	return func(s *Spider) error {
		s.cache = cache
		return nil
	}
}

// RobotLimits sets the robot exclusion cache.
func RobotLimits(limits *limits.RobotLimitCache) SpiderConstructorOption {
	return func(s *Spider) error {
		s.RobotLimits = limits
		return nil
	}
}

// IgnoreRobots sets the spider's RobotExclusionFunction to IgnoreRobotRules, ignoring robots.txt.
func IgnoreRobots() SpiderConstructorOption {
	return func(s *Spider) error {
		s.RobotExclusionFunction = IgnoreRobotRules
		return nil
	}
}

// UserAgent set the spider User-agent.
func UserAgent(agent string) SpiderConstructorOption {
	return func(s *Spider) error {
		s.UserAgent = agent
		return nil
	}
}

// Throttle is a constructor function for SetThrottles.
func Throttle(defaultThrottle *limits.DefaultThrottle, domainThrottles ...*limits.DomainThrottle) SpiderConstructorOption {
	return func(s *Spider) error {
		s.SetThrottles(defaultThrottle, domainThrottles...)
		return nil
	}
}

/*
	Getters/setters
*/

// AddLimits adds limits to the spider, it will not add duplicate limits.
func (s *Spider) AddLimits(limits ...limits.Limit) {
	for _, limit := range limits {
		contents, _ := json.Marshal(limit)
		s.limits[string(contents)] = limit
	}
}

// RemoveLimits removes the given limits (if present).
func (s *Spider) RemoveLimits(limits ...limits.Limit) {
	for _, limit := range limits {
		contents, _ := json.Marshal(limit)
		delete(s.limits, string(contents))
	}
}

// SetThrottles sets or replaces the default and custom throttles for the spider.
func (s *Spider) SetThrottles(def *limits.DefaultThrottle, domainThrottles ...*limits.DomainThrottle) {
	s.throttle = limits.NewThrottleCollection(def, domainThrottles...)
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

/*
	Pipeline functions
*/

// OnRequest is called when a request is about to be made.
func (s *Spider) OnRequest(f RequestPipeline) {
	s.requestPipeline = f
}

// OnResponse is called when a response has been received and tokenized.
func (s *Spider) OnResponse(f ResponsePipeline) {
	s.responsePipeline = f
}

// OnHTML is called for each element matching the selector in a response body
func (s *Spider) OnHTML(selector string, f HTMLPipeline) {
	s.selectors[selector] = f
}

// OnError is called when an error is encountered.
func (s *Spider) OnError(f ErrorPipeline) {
	s.errorPipeline = f
}

/*
	Control/navigation functions
*/

// Visit a page by adding the path to the queue, blocks when the queue is full until there is free space.
// This method is meant to be used solely for setting the starting points of crawls before calling Start.
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

// Start the spider, blocks while the spider is running. Returns the a cancel function similar to context that returns the Spider state.
// The spider state can be usedwith the Resume function to resume a crawl.
func (s *Spider) Start(ctx context.Context) func() *SpiderState {
	s.ctx, s.cancel = context.WithCancel(ctx)

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
				case <-s.ctx.Done():
					ingestors.Done()
					return
				default:
				}

				req, ok := s.queue.Dequeue()
				if ok {
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
						s.errorPipeline(fmt.Errorf("domain %s filtered", req.String()))
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
					s.requestPipeline(req)
				case res := <-resc:
					s.responsePipeline(res)
					for selector, pipeline := range s.selectors {
						res.Find(selector).Each(func(i int, el *goquery.Selection) {
							pipeline(el)
						})
					}
				case err := <-errc:
					s.errorPipeline(err)
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
	return func() *SpiderState {
		s.cancel()
		return &s.SpiderState
	}
}

// Resume from spider state, blocks while the spider is running. Returns the spider state after context is cancelled.
func (s *Spider) Resume(ctx context.Context, state *SpiderState) func() *SpiderState {
	s.SpiderState = *state
	return s.Start(ctx)
}

/*
	Private methods
*/

// init sets some spider fields to default values if none were supplied.
func (s *Spider) init() {
	if s.queue == nil {
		s.queue = request.NewHeap(10000)
	}
	if s.cache == nil {
		s.cache = request.NewCache()
	}
	if s.RobotLimits == nil {
		s.RobotLimits = limits.NewRobotLimitCache()
	}
}

// filterDomains returns true if the spider is allowed to visit the domain.
func (s *Spider) filterDomains(request *request.Request) bool {
	for _, domain := range s.allowedDomains {
		if domain.MatchString(request.Host) {
			return true
		}
	}
	return false
}

// getResponse waits for throttles and makes a GET request.
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

// GetRobotLimits downloads and parses the robots.txt file for a domain.
// Respects the spider throttles.
func (s *Spider) GetRobotLimits(req *request.Request) (*limits.RobotLimits, error) {
	s.throttle.Wait(req)
	res, err := s.client.Get(fmt.Sprintf("%s://%s/robots.txt", req.Scheme, req.Host))
	if err != nil {
		return nil, err
	}

	domainLimits, err := s.RobotLimits.AddLimits(res.Body, req.Host)
	defer res.Body.Close()
	if err != nil {
		return nil, &limits.RobotParsingError{
			Domain: req.Host,
			Err:    err.Error(),
		}
	}
	return domainLimits, nil
}

// addRequest adds a request to the queue.
func (s *Spider) addRequest(req *request.Request, priority int) error {
	for _, limit := range s.limits {
		err := limit.FilterRequest(req)
		if err != nil {
			return err
		}
	}

	// check robots.txt
	err := s.RobotExclusionFunction(s, req)
	if err != nil {
		return err
	}

	// check cache to prevent URL revisit
	if !s.cache.VisitedURL(req) {
		s.cache.AddRequest(req)
		err := s.queue.Enqueue(req, priority)
		if err != nil {
			return err
		}
	}
	return nil
}

/*
	Robots.txt interpretation functions
*/

// IgnoreRobotRules ignores the robots.txt file.
func IgnoreRobotRules(s *Spider, req *request.Request) error {
	return nil
}

// FollowRobotRules fetches and follows the limitations imposed by the robots.txt file.
func FollowRobotRules(s *Spider, req *request.Request) error {
	rules, err := s.RobotLimits.GetLimits(req.Host)
	if err != nil {
		rules, err = s.GetRobotLimits(req)
		if err != nil {
			return err
		}
	}

	// check if the rules allow this request
	if !rules.Allowed(s.UserAgent, req.Path) {
		return fmt.Errorf("request for %s denied by robots.txt", req.String())
	}

	// check crawl-delay
	delay := rules.Delay(s.UserAgent, -1)
	if delay > -1 {
		// override spider throttle for this domain with the given crawl delay
		s.throttle.SetDomainThrottle(limits.NewDomainThrottle(req.Host, delay))
	}
	return nil
}
