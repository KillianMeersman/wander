package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/KillianMeersman/wander/spider"
	"github.com/PuerkitoBio/goquery"
)

func main() {
	spid := spider.NewSpider()
	ctx, cancel := context.WithCancel(context.Background())

	spid.Parse(func(res *spider.Response) {
		res.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
			link, ok := sel.Attr("href")
			if ok {
				spid.Visit(link)
			}
		})

		fmt.Printf("Received response from %s\n", res.Request.URL)
	})

	spid.OnError(func(err error) {
		fmt.Println(err)
	})

	spid.OnRequest(func(path string) {
		fmt.Printf("Visiting %s\n", path)
	})

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
	<-ctx.Done()
}
