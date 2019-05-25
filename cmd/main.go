package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/KillianMeersman/wander/limits"

	"github.com/KillianMeersman/wander"
	"github.com/KillianMeersman/wander/request"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	spid, err := wander.NewSpider([]string{"bol\\.com"}, 4)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())

	spid.OnResponse(func(res *request.Response) {
		log.Printf("Received response from %s\n", res.Request.URL)
		res.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
			link, ok := sel.Attr("href")
			if ok {
				spid.Follow(link, res, 10-res.Request.Depth())
			}
		})
	})

	spid.OnError(func(err error) {
		//log.Printf("Error: %s\n", err)
	})

	spid.OnRequest(func(req *request.Request) {
		log.Printf("Visiting %s\n", req.String())
	})

	if err != nil {
		log.Fatal(err)
	}
	spid.AddLimits(
		limits.NewThrottleCollection(
			limits.ThrottleDefault(3*time.Second),
			limits.ThrottleDomain("bol\\.com", 100*time.Millisecond),
		),
		limits.MaxDepth(10),
	)

	spid.Visit("http://bol.com")

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt)
	go func() {
		<-sigintc
		log.Print("STOPPING...")
		stop()
	}()

	spid.Start(ctx)
}
