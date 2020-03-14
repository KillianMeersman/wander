package wander

import (
	"net/http"
	"net/url"
	"regexp"
	"sync"

	"github.com/KillianMeersman/wander/limits"
	"github.com/KillianMeersman/wander/limits/robots"
	"github.com/KillianMeersman/wander/request"
	"github.com/PuerkitoBio/goquery"
)

// NewSpider instantiates a new spider.
func NewSpider(options ...SpiderConstructorOption) (*Spider, error) {
	lock := &sync.Mutex{}
	cond := sync.NewCond(lock)
	spider := &Spider{
		SpiderState:    SpiderState{},
		allowedDomains: make([]*regexp.Regexp, 0),
		limits:         make(map[string]limits.RequestFilter),

		pipelineN: 1,
		ingestorN: 1,

		client:                 &http.Client{},
		UserAgent:              "WanderBot",
		RobotExclusionFunction: FollowRobotRules,

		ingestorWg:  &sync.WaitGroup{},
		pipelineWg:  &sync.WaitGroup{},
		reqc:        make(chan *request.Request),
		resc:        make(chan *request.Response),
		errc:        make(chan error),
		lock:        lock,
		runningCond: cond,

		requestFunc:      func(req *request.Request) {},
		responseFunc:     func(res *request.Response) {},
		errorFunc:        func(err error) {},
		selectors:        make(map[string]func(*request.Response, *goquery.Selection)),
		pipelineDoneFunc: func() {},
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
		s.AddLimits(limits.NewMaxDepthFilter(max))
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
func RobotLimits(limits *robots.RobotRules) SpiderConstructorOption {
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
