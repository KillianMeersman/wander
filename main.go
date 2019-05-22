package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/KillianMeersman/wander/spider"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	spid, err := spider.NewSpider([]string{"bol\\.com"}, 5)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	spid.Parse(func(res *spider.Response) {
		log.Printf("Received response from %s\n", res.Request.URL)
		res.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
			link, ok := sel.Attr("href")
			if ok {
				spid.Follow(link, res)
			}
		})
	})

	spid.OnError(func(err error) {
		log.Printf("Error: %s\n", err)
	})

	spid.OnRequest(func(path string) {
		log.Printf("Visiting %s\n", path)
	})

	throttle, err := spider.NewThrottle(".*", 100*time.Millisecond)
	if err != nil {
		log.Fatal(err)
	}
	spid.Throttle(throttle)

	spid.Visit("http://bol.com")
	spid.Visit("http://reddit.com")
	spid.Visit("http://2dehands.be")
	spid.Visit("http://hln.be")

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt)
	go func() {
		<-sigintc
		cancel()
	}()

	spid.Run(ctx)
}
