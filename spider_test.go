package wander_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/KillianMeersman/wander"
	"github.com/KillianMeersman/wander/limits"
	"github.com/KillianMeersman/wander/request"
	"github.com/KillianMeersman/wander/util"
	"github.com/PuerkitoBio/goquery"
)

type route struct {
	pattern *regexp.Regexp
	handler http.Handler
}

type regexpHandler struct {
	routes []*route
}

func (h *regexpHandler) Handler(pattern *regexp.Regexp, handler http.Handler) {
	h.routes = append(h.routes, &route{pattern, handler})
}

func (h *regexpHandler) HandleFunc(pattern *regexp.Regexp, handler func(http.ResponseWriter, *http.Request)) {
	h.routes = append(h.routes, &route{pattern, http.HandlerFunc(handler)})
}

func (h *regexpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, route := range h.routes {
		if route.pattern.MatchString(r.URL.Path) {
			route.handler.ServeHTTP(w, r)
			return
		}
	}
	// no pattern matched; send 404 response
	http.NotFound(w, r)
}

func randomLinkServer() *http.Server {

	randomLinks := func(w http.ResponseWriter, r *http.Request) {
		msg := []byte(fmt.Sprintf(`
		<html>
		<head>
		</head>
		<body>
		<a href="/test/%s">test1</a>
		<a href="/test/%s">test2</a>
		<a href="/test/%s">test3</a>
		</body>
		</html>`, util.RandomString(20), util.RandomString(20), util.RandomString(20)))

		w.Write(msg)

	}

	robots := func(w http.ResponseWriter, r *http.Request) {
		msg := []byte(`
		User-agent: *
		Disallow:

		# too many repeated hits, too quick
		User-agent: Wander/0.1
		Disallow: /test1

		# Yahoo. too many repeated hits, too quick
		User-agent: Slurp
		Disallow: /
		Allow: /test

		# too many repeated hits, too quick
		User-agent: Baidu
		Disallow: /
		`)

		w.Write(msg)
	}

	handler := &regexpHandler{}
	handler.HandleFunc(regexp.MustCompile(`(?m)^\/robots\.txt$`), robots)
	handler.HandleFunc(regexp.MustCompile(`(?m)^\/test.*`), randomLinks)

	serv := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: handler,
	}
	return serv
}

func TestMain(m *testing.M) {
	ctx := context.Background()
	serv := randomLinkServer()
	go serv.ListenAndServe()
	defer serv.Shutdown(ctx)
	m.Run()
}

func TestSyncVisit(t *testing.T) {
	queue := request.NewHeap(10)
	spid, err := wander.NewSpider(
		wander.AllowedDomains("127.0.0.1", "localhost"),
		wander.Threads(6),
		wander.Queue(queue),
		wander.Throttle(limits.NewDefaultThrottle(time.Second)),
	)
	if err != nil {
		t.Fatal(err)
	}

	url := &url.URL{
		Scheme: "http",
		Host:   "localhost:8080",
		Path:   "/test",
	}
	_, err = spid.VisitNow(url)
	if err != nil {
		t.Fatal(err)
	}
	_, err = spid.VisitNow(url)
	if err != nil {
		t.Fatal(err)
	}
}

func BenchmarkSpiderWithHeapQueue(b *testing.B) {
	queue := request.NewHeap(10000)
	benchmarkSpider(b, queue)
}

func BenchmarkSpiderWithRedisQueue(b *testing.B) {
	queue, err := request.NewRedisQueue("localhost", 6379, "", "requests", 0)
	if err != nil {
		b.Fatal(err)
	}
	benchmarkSpider(b, queue)
}

func benchmarkSpider(b *testing.B, queue request.Queue) {
	defer queue.Clear()
	defer queue.Close()

	spid, err := wander.NewSpider(
		wander.AllowedDomains("127.0.0.1", "localhost"),
		wander.Threads(6),
		wander.Queue(queue),
	)
	if err != nil {
		b.Fatal(err)
	}

	reqn := 0
	resn := 0
	resLock := sync.Mutex{}

	spid.OnResponse(func(res *request.Response) {
		resLock.Lock()
		resn++
		if resn >= b.N {
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			spid.Stop(ctx)
		}
		resLock.Unlock()

		if res.StatusCode != 200 {
			b.Fatalf("server returned %d", res.StatusCode)
		}
	})

	spid.OnHTML("a[href]", func(res *request.Response, el *goquery.Selection) {
		link, ok := el.Attr("href")
		url, err := url.Parse(link)
		if err != nil {
			return
		}

		if ok {
			err := spid.Follow(url, res, 10-res.Request.Depth)
			if err != nil {
				switch err.(type) {
				case *request.QueueMaxSize:
				default:
					log.Fatal(err)
				}
			}
		}
	})

	spid.OnError(func(err error) {
		b.Fatal(err)
	})

	spid.OnRequest(func(req *request.Request) {
		reqn++
	})

	b.ResetTimer()

	url := &url.URL{
		Scheme: "http",
		Host:   "localhost:8080",
		Path:   "/test",
	}
	err = spid.Visit(url)
	if err != nil {
		log.Fatal(err)
	}
	spid.Start()
	spid.Wait()

	count, err := queue.Count()
	if err != nil {
		log.Fatal(err)
	}
	b.Logf("Visited %d, received %d responses. Queue size is %d", reqn, resn, count)
}
