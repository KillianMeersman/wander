package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/KillianMeersman/wander"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	spid, err := wander.NewSpider([]string{"bol\\.com"}, 4)
	if err != nil {
		log.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())

	spid.OnResponse(func(res *wander.Response) {
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

	spid.OnRequest(func(req *wander.Request) {
		log.Printf("Visiting %s\n", req.String())
	})

	globalThrottle, err := wander.NewThrottle(3 * time.Second)
	bolThrottle, err := wander.NewDomainThrottle("bol\\.com", 1*time.Second)
	if err != nil {
		log.Fatal(err)
	}
	spid.Throttle(globalThrottle)
	spid.ThrottleDomains(bolThrottle)

	spid.Visit("http://bol.com")

	sigintc := make(chan os.Signal, 1)
	signal.Notify(sigintc, os.Interrupt)
	go func() {
		<-sigintc
		log.Print("cancelling...")
		cancel()
	}()

	spid.Run(ctx)
}
