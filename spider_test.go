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

type RegexpHandler struct {
	routes []*route
}

func (h *RegexpHandler) Handler(pattern *regexp.Regexp, handler http.Handler) {
	h.routes = append(h.routes, &route{pattern, handler})
}

func (h *RegexpHandler) HandleFunc(pattern *regexp.Regexp, handler func(http.ResponseWriter, *http.Request)) {
	h.routes = append(h.routes, &route{pattern, http.HandlerFunc(handler)})
}

func (h *RegexpHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	handleFunc := func(w http.ResponseWriter, r *http.Request) {
		msg := []byte(fmt.Sprintf(`<html><head>
		</head>
		<body>
		<a href="/%s">test</a>
		<a href="/%s">test</a>
		<a href="/%s">test</a>
		</body>
		</html>`, util.RandomString(20), util.RandomString(20), util.RandomString(20)))

		w.Write(msg)
	}

	handler := &RegexpHandler{}
	handler.HandleFunc(regexp.MustCompile(".*"), handleFunc)
	serv := &http.Server{
		Addr:    "127.0.0.1:8080",
		Handler: handler,
	}
	go serv.ListenAndServe()
	return serv
}

func BenchmarkSpider(b *testing.B) {
	queue := request.NewRequestHeap(10000)
	spid, err := wander.NewSpider(
		wander.AllowedDomains("127.0.0.1"),
		wander.Threads(1),
		wander.Queue(queue),
	)
	if err != nil {
		b.Fatal(err)
	}

	ctx, stop := context.WithCancel(context.Background())
	reqn := 0
	resn := 0
	resLock := sync.Mutex{}
	reqLock := sync.Mutex{}

	spid.OnResponse(func(res *request.Response) {
		resLock.Lock()
		resn++
		if resn >= 1000 {
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
					case *request.RequestQueueMaxSize:
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

	err = spid.Visit("http://127.0.0.1:8080/")
	if err != nil {
		log.Fatal(err)
	}

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt)
	go func() {
		<-sigintc
		stop()
	}()

	serv := randomLinkServer()
	spid.Start(ctx)
	serv.Shutdown(ctx)

	b.Logf("Visited %d, received %d responses. Queue size is %d", reqn, resn, queue.Count())
}
