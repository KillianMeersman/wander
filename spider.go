// Package wander is a scraping library for Go.
// It aims to provide an easy to use API while also exposing tools for advanced use cases.
package wander

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

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

// UserAgentFunction determines what User-Agent the spider will use.
type UserAgentFunction func(req *request.Request) string

// SpiderState holds a spider's state, such as the request queue and cache.
// It is returned by the Start and Resume methods, allowing the Resume method to resume a previously stopped crawl.
type SpiderState struct {
	Queue request.Queue
	Cache request.Cache
}

// SpiderParameters crawling parameters for a spider
type SpiderParameters struct {
	UserAgent              UserAgentFunction
	RobotExclusionFunction RobotLimitFunction
	// DefaultWaitTime for 429 & 503 responses without a Retry-After header
	DefaultWaitTime time.Duration
	// MaxWaitTime for 429 & 503 responses with a Retry-After header
	MaxWaitTime time.Duration
	// IgnoreTimeouts if true, the bot will ignore 429 response timeouts.
	// Defaults to false.
	IgnoreTimeouts bool
}

// Spider provides a parallelized scraper.
type Spider struct {
	SpiderState
	SpiderParameters
	RobotLimits    *robots.RobotRules
	AllowedDomains []string
	limits         map[string]limits.RequestFilter
	throttle       limits.ThrottleCollection

	// parallelism
	ingestorN int

	done       chan struct{}
	ingestorWg *sync.WaitGroup

	lock      *sync.Mutex
	isRunning bool

	// callbacks
	requestFunc      func(*request.Request) *request.Request
	responseFunc     func(*request.Response)
	errorFunc        func(error)
	selectors        map[string]func(*request.Response, *goquery.Selection)
	pipelineDoneFunc func()

	// http
	client *http.Client
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

// SetAllowedDomains sets the allowed domains.
func (s *Spider) SetAllowedDomains(paths ...string) error {
	s.AllowedDomains = paths
	return nil
}

/*
	Pipeline functions
*/

// OnRequest is called when a request is about to be made.
// This function should return a request, allowing the callback to mutate the request.
// If null is returned, no requests are made.
// This will overwrite any previous callbacks set by this method.
func (s *Spider) OnRequest(f func(req *request.Request) *request.Request) {
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
func (s *Spider) Visit(url *url.URL) error {
	req, err := request.NewRequest(url, nil)
	if err != nil {
		return err
	}

	return s.addRequest(req, util.MaxInt)
}

// VisitNow visits the given url without adding it to the queue.
// It will still wait for any throttling.
func (s *Spider) VisitNow(url *url.URL) (*request.Response, error) {
	req, err := request.NewRequest(url, nil)
	if err != nil {
		return nil, err
	}

	return s.getResponse(req)
}

// Follow a link by adding the path to the queue, blocks when the queue is full until there is free space.
// Unlike Visit, this method also accepts a response, allowing the url parser to convert relative urls into absolute ones and keep track of depth.
func (s *Spider) Follow(url *url.URL, res *request.Response, priority int) error {
	req, err := request.NewRequest(url, res.Request)
	if err != nil {
		return err
	}

	return s.addRequest(req, priority)
}

// start the spider by spawning all required ingestors/pipelines
// This method is idempotent and will return without doing anything if the spider is already isRunning.
func (s *Spider) start() {
	if s.isRunning {
		return
	}
	s.isRunning = true

	s.done = make(chan struct{})
	s.spawn(s.ingestorN)
}

// Start the spider.
// This method is idempotent and will return without doing anything if the spider is already isRunning.
func (s *Spider) Start() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.start()
}

// Resume from spider state.
// This method is idempotent and will return without doing anything if the spider is already isRunning.
func (s *Spider) Resume(ctx context.Context, state *SpiderState) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.SpiderState = *state
	s.start()
}

// Stop the spider if it is currently isRunning, returns a SpiderState to allow a later call to Resume.
// Accepts a context and will forcibly stop the spider if cancelled, regardless of status.
// This method is idempotent and will return without doing anything if the spider is not isRunning.
func (s *Spider) Stop(ctx context.Context) *SpiderState {
	s.lock.Lock()
	defer s.lock.Unlock()

	if !s.isRunning {
		return &s.SpiderState
	}
	s.isRunning = false

	close(s.done)
	done := make(chan struct{})
	go func() {
		s.ingestorWg.Wait()
		close(done)
	}()
	s.Queue.Close()

	// Wait for the ingestors to stop or the context to cancel.
	select {
	case <-done:
	case <-ctx.Done():
	}
	return &s.SpiderState
}

// Wait blocks until the spider has been stopped.
func (s *Spider) Wait() {
	if !s.isRunning {
		return
	}
	<-s.done
}

/*
	Private methods
*/

// filterRequestDomain returns true if the spider is allowed to visit the domain.
func (s *Spider) filterRequestDomain(request *request.Request) bool {
	for _, domain := range s.AllowedDomains {
		if robots.MatchURLRule(domain, request.URL.Host) {
			return true
		}
	}
	return false
}

// RoundTrip implements the http.RoundTripper interface.
// It will wait for any throttles before making requests.
func (s *Spider) RoundTrip(req *http.Request) (*http.Response, error) {
	s.throttle.Wait(req)
	return s.client.Get(req.URL.String())
}

// getResponse waits for throttles and makes a GET request.
func (s *Spider) getResponse(req *request.Request) (*request.Response, error) {
	if req == nil {
		panic("Wander request is nil")
	}

	res, err := s.RoundTrip(&req.Request)
	if err != nil {
		return nil, err
	}

	doc := request.NewResponse(req, *res)
	if err != nil {
		return nil, err
	}
	return doc, nil
}

// addRequest adds a request to the queue.
func (s *Spider) addRequest(req *request.Request, priority int) error {
	if !s.filterRequestDomain(req) {
		return limits.ForbiddenDomain{*req.URL}
	}

	for _, limit := range s.limits {
		err := limit.FilterRequest(req)
		if err != nil {
			return err
		}
	}

	// check cache to prevent URL revisit
	visited, err := s.Cache.VisitedURL(req)
	if err != nil {
		return err
	}
	if visited {
		return AlreadyVisited{*req.URL}
	}
	s.Cache.AddRequest(req)

	// check robots.txt
	err = s.RobotExclusionFunction(s, req)
	if err != nil {
		return err
	}

	err = s.Queue.Enqueue(req, priority)
	if err != nil {
		return err
	}
	return nil
}

// spawn spawns a new ingestor goroutine.
// Ingestors make requests and handle callbacks.
func (s *Spider) spawn(n int) {
	s.ingestorWg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			for {
				select {
				case <-s.done:
					s.ingestorWg.Done()
					return
				case req := <-s.Queue.Dequeue():
					if req.Error != nil {
						s.errorFunc(req.Error)
						return
					}

					// Run the request callback and execute the request.
					newRequest := s.requestFunc(req.Request)
					if newRequest == nil {
						continue
					}
					res, err := s.getResponse(newRequest)
					if err != nil {
						s.errorFunc(err)
						return
					}

					s.CheckResponseStatus(res)
					s.responseFunc(res)

					// If there are selectors, parse the document and run the selector callbacks.
					if len(s.selectors) > 0 {
						_, err := res.Parse()
						if err != nil {
							s.errorFunc(err)
							continue
						}
						for selector, pipeline := range s.selectors {
							res.Document.Find(selector).Each(func(i int, el *goquery.Selection) {
								pipeline(res, el)
							})
						}
					}

					s.pipelineDoneFunc()
				}

			}
		}()
	}
}

// DownloadRobotLimits downloads and parses the robots.txt file for a domain.
// Respects the spider throttles.
func (s *Spider) DownloadRobotLimits(req *request.Request) (*robots.RobotFile, error) {
	url := &url.URL{
		Scheme: req.URL.Scheme,
		Host:   req.URL.Host,
		Path:   "/robots.txt",
	}
	robotFile, err := robots.NewRobotFileFromURL(url, s)
	if err != nil {
		return nil, err
	}
	s.RobotLimits.AddLimits(robotFile, req.URL.Host)
	return robotFile, nil
}

// CheckResponseStatus checks the response for any non-standard status codes.
// It will apply additional throttling when it encounters a 429 or 503 status code, according to the spider parameters.
func (s *Spider) CheckResponseStatus(res *request.Response) {
	if !s.IgnoreTimeouts && (res.StatusCode == 429 || res.StatusCode == 503) {
		retryAfter := res.Header.Get("Retry-After")
		if len(retryAfter) > 0 {
			// Parse Retry-Duration as RFC1123 timestamp
			retryTime, err := time.Parse(time.RFC1123, retryAfter)
			if err != nil {
				// Parse Retry-Duration as seconds
				retryAfterDuration, err := time.ParseDuration(fmt.Sprintf("%ss", retryAfter))
				if err != nil {

				}
				retryTime = time.Now().Add(retryAfterDuration)
			}
			waitDuration := retryTime.Sub(time.Now())
			s.throttle.SetWaitTime(waitDuration)
		} else {
			// No Retry-After header, use the default wait time
			s.throttle.SetWaitTime(s.DefaultWaitTime)
		}
	}
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
	rules, err := s.RobotLimits.GetRulesForHost(req.URL.Host)
	if err != nil {
		rules, err = s.DownloadRobotLimits(req)
		if err != nil {
			return err
		}
	}

	// check if the rules allow this request
	if !rules.Allowed(s.UserAgent(req), req.URL.Path) {
		return robots.RobotDenied{*req.URL}
	}

	// check crawl-delay
	delay := rules.GetDelay(s.UserAgent(req), -1)
	if delay > -1 {
		// override spider throttle for this domain with the given crawl delay
		s.throttle.SetDomainThrottle(limits.NewDomainThrottle(req.URL.Host, delay))
	}
	return nil
}
