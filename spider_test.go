package wander_test

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"testing"

	"github.com/KillianMeersman/wander"
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

func BenchmarkSpider(b *testing.B) {
	queue := request.NewHeap(b.N)
	spid, err := wander.NewSpider(
		wander.AllowedDomains("127.0.0.1", "localhost"),
		wander.Threads(2),
		wander.Queue(queue),
	)
	if err != nil {
		b.Fatal(err)
	}

	reqn := 0
	resn := 0
	resLock := sync.Mutex{}
	reqLock := sync.Mutex{}
	ctx, stop := context.WithCancel(context.Background())

	spid.OnResponse(func(res *request.Response) {
		resLock.Lock()
		resn++
		if resn >= b.N {
			stop()
		}
		resLock.Unlock()

		if res.StatusCode != 200 {
			b.Fatalf("server returned %d", res.StatusCode)
		}

		res.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
			link, ok := sel.Attr("href")
			if ok {
				err := spid.Follow(link, res, 10-res.Request.Depth())
				if err != nil {
					switch err.(type) {
					case *request.QueueMaxSize:
					default:
						log.Fatal(err)
					}
				}
			}
		})
	})

	spid.OnError(func(err error) {
		b.Fatal(err)
	})

	spid.OnRequest(func(req *request.Request) {
		reqLock.Lock()
		reqn++
		reqLock.Unlock()
	})

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt)
	go func() {
		<-sigintc
		stop()
	}()

	b.ResetTimer()
	err = spid.Visit("http://localhost:8080/test/")
	if err != nil {
		log.Fatal(err)
	}
	spid.Start(ctx)

	b.Logf("Visited %d, received %d responses. Queue size is %d", reqn, resn, queue.Count())
}
