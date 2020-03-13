// Package wander is a scraping library for Go.
// It aims to provide an easy to use API while also exposing tools for advanced use cases.
package wander

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/KillianMeersman/wander/limits/robots"
	"github.com/KillianMeersman/wander/util"

	"github.com/PuerkitoBio/goquery"

	"github.com/KillianMeersman/wander/limits"

	"github.com/KillianMeersman/wander/request"
)

// SpiderConstructorOption is used for chaining constructor options.
type SpiderConstructorOption func(s *Spider) error

// RobotLimitFunction determines how a spider acts upon robot.txt limitations.
// The default is FollowRobotRules, IgnoreRobotRules is also provided.
// It's possible to define your own RobotLimitFunction in order to e.a. ignore only certain limitations.
type RobotLimitFunction func(spid *Spider, req *request.Request) error

// handleRequest waits for any throttling, sends the request down the request channel and gets the response.
func (s *Spider) handleRequest(req *request.Request, reqChannel chan *request.Request) (*request.Response, error) {
	s.throttle.Wait(req)

	select {
	case s.reqc <- req:
	default:
	}

	return s.getResponse(req)
}

// spawnIngestors spawns a new ingestor goroutine.
// Ingestors make requests and do not handle pipelines as these often have blocking code (such as db calls).
func (s *Spider) spawnIngestors(n int) {
	s.ingestorWg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			for {
				select {
				case <-s.stopIngestors:
					s.ingestorWg.Done()
					return
				default:
				}

				req, ok := s.queue.Dequeue()
				if ok {
					res, err := s.handleRequest(req, s.reqc)
					if err != nil {
						select {
						case s.errc <- err:
						default:
						}

						continue
					}

					select {
					case s.resc <- res:
					default:
					}

				}
			}
		}()
	}
}

// spawnPipelines spawns a new pipeline goroutine.
// Pipelines handle document parsing and callbacks.
func (s *Spider) spawnPipelines(n int) {
	s.pipelineWg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			for {
				select {
				case <-s.stopPipelines:
					s.pipelineWg.Done()
					return

				case req := <-s.reqc:
					s.requestFunc(req)

				case res := <-s.resc:
					s.responseFunc(res)
					for selector, pipeline := range s.selectors {
						res.Find(selector).Each(func(i int, el *goquery.Selection) {
							pipeline(res, el)
						})
					}
					s.pipelineDoneFunc()

				case err := <-s.errc:
					s.errorFunc(err)

				}
			}
		}()
	}
}

// SpiderState holds a spider's state, such as the request queue and cache.
// It is returned by the Start and Resume methods, allowing the Resume method to resume a previously stopped crawl.
type SpiderState struct {
	queue request.Queue
	cache request.Cache
}

// Spider provides a parallelized scraper.
type Spider struct {
	SpiderState
	allowedDomains []*regexp.Regexp
	RobotLimits    *robots.Cache
	limits         map[string]limits.RequestFilter
	throttle       limits.ThrottleCollection

	// parallelism
	ingestorN int
	pipelineN int

	stopIngestors chan struct{}
	stopPipelines chan struct{}
	ingestorWg    *sync.WaitGroup
	pipelineWg    *sync.WaitGroup

	reqc        chan *request.Request
	resc        chan *request.Response
	errc        chan error
	lock        *sync.Mutex
	running     bool
	runningCond *sync.Cond

	// callbacks
	requestFunc      func(*request.Request)
	responseFunc     func(*request.Response)
	errorFunc        func(error)
	selectors        map[string]func(*request.Response, *goquery.Selection)
	pipelineDoneFunc func()

	// http
	client *http.Client

	// options
	UserAgent              string
	RobotExclusionFunction RobotLimitFunction
}

/*
	Getters/setters
*/
// AddLimits adds limits to the spider, it will not add duplicate limits.
func (s *Spider) AddLimits(limits ...limits.RequestFilter) {
	for _, limit := range limits {
		contents, _ := json.Marshal(limit)
		s.limits[string(contents)] = limit
	}
}

// RemoveLimits removes the given limits (if present).
func (s *Spider) RemoveLimits(limits ...limits.RequestFilter) {
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
// This will overwrite any previous callbacks set by this method.
func (s *Spider) OnRequest(f func(req *request.Request)) {
	s.requestFunc = f
}

// OnResponse is called when a response has been received and tokenized.
// This will overwrite any previous callbacks set by this method.
func (s *Spider) OnResponse(f func(res *request.Response)) {
	s.responseFunc = f
}

// OnHTML is called for each element matching the selector in a response body
func (s *Spider) OnHTML(selector string, f func(res *request.Response, el *goquery.Selection)) {
	s.selectors[selector] = f
}

// OnError is called when an error is encountered.
// This will overwrite any previous callbacks set by this method.
func (s *Spider) OnError(f func(err error)) {
	s.errorFunc = f
}

// OnPipelineFinished is called when a pipeline (all callbacks and selectors) finishes.
// This will overwrite any previous callbacks set by this method.
func (s *Spider) OnPipelineFinished(f func()) {
	s.pipelineDoneFunc = f
}

/*
	Control/navigation functions
*/
// Visit adds a request with the given path to the queue with maximum priority. Blocks when the queue is full until there is free space.
// This method is meant to be used solely for setting the starting points of crawls before calling Start.
func (s *Spider) Visit(path string) error {
	req, err := request.NewRequest(path, nil)
	if err != nil {
		return err
	}

	return s.addRequest(req, util.MaxInt)
}

// VisitNow visits the given url without adding it to the queue.
// It will still wait for any throttling.
func (s *Spider) VisitNow(path string) (*request.Response, error) {
	req, err := request.NewRequest(path, nil)
	if err != nil {
		return nil, err
	}

	return s.handleRequest(req, s.reqc)
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

// start the spider by spawning all required ingestors/pipelines
// This method is idempotent and will return without doing anything if the spider is already running.
func (s *Spider) start() {
	if s.running {
		return
	}
	s.running = true

	s.stopIngestors = make(chan struct{})
	s.stopPipelines = make(chan struct{})
	s.spawnIngestors(s.ingestorN)
	s.spawnPipelines(s.pipelineN)
}

// Start the spider.
// This method is idempotent and will return without doing anything if the spider is already running.
func (s *Spider) Start() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.start()
}

// Resume from spider state.
// This method is idempotent and will return without doing anything if the spider is already running.
func (s *Spider) Resume(ctx context.Context, state *SpiderState) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.SpiderState = *state
	s.start()
}

// Stop the spider if it is currently running, returns a SpiderState to allow a later call to Resume.
// Accepts a context and will forcibly stop the spider if cancelled, regardless of status.
// This method is idempotent and will return without doing anything if the spider is not running.
func (s *Spider) Stop(ctx context.Context) *SpiderState {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.running {
		return &s.SpiderState
	}
	s.running = false

	ctx, cancel := context.WithCancel(ctx)

	go func(cancel context.CancelFunc) {
		close(s.stopIngestors)
		s.ingestorWg.Wait()
		close(s.stopPipelines)
		s.pipelineWg.Wait()
		cancel()
	}(cancel)

	<-ctx.Done()
	s.runningCond.Broadcast()
	return &s.SpiderState
}

// Wait blocks until the spider has been stopped.
func (s *Spider) Wait() {
	s.lock.Lock()
	for s.running {
		s.runningCond.Wait()
	}
	s.lock.Unlock()
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
		s.RobotLimits = robots.NewCache()
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

	s.throttle.Wait(req)

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
func (s *Spider) GetRobotLimits(req *request.Request) (*robots.Limits, error) {
	s.throttle.Wait(req)
	res, err := s.client.Get(fmt.Sprintf("%s://%s/robots.txt", req.Scheme, req.Host))
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	domainLimits, err := s.RobotLimits.AddLimits(res.Body, req.Host)
	if err != nil {
		return nil, robots.InvalidRobots{
			Domain: req.Host,
			Err:    err.Error(),
		}
	}
	return domainLimits, nil
}

// addRequest adds a request to the queue.
func (s *Spider) addRequest(req *request.Request, priority int) error {
	if !s.filterDomains(req) {
		return limits.ForbiddenDomain{req.URL}
	}

	for _, limit := range s.limits {
		err := limit.FilterRequest(req)
		if err != nil {
			return err
		}
	}

	// check cache to prevent URL revisit
	if s.cache.VisitedURL(req) {
		return AlreadyVisited{req.URL}
	}
	s.cache.AddRequest(req)

	// check robots.txt
	err := s.RobotExclusionFunction(s, req)
	if err != nil {
		return err
	}

	err = s.queue.Enqueue(req, priority)
	if err != nil {
		return err
	}
	return nil
}

/*
	Robots.txt interpretation functions
*/

// IgnoreRobotRules ignores the robots.txt file.
// Implementation of RobotLimitFunction.
func IgnoreRobotRules(s *Spider, req *request.Request) error {
	return nil
}

// FollowRobotRules fetches and follows the limitations imposed by the robots.txt file.
// Implementation of RobotLimitFunction.
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
		return robots.RobotDenied{req.URL}
	}

	// check crawl-delay
	delay := rules.Delay(s.UserAgent, -1)
	if delay > -1 {
		// override spider throttle for this domain with the given crawl delay
		s.throttle.SetDomainThrottle(limits.NewDomainThrottle(req.Host, delay))
	}
	return nil
}
