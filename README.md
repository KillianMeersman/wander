# Wander
## Overview
Convenient scraping library for Gophers.

Based on Colly and Scrapy, Wander aims to provide an easy-to-use API while also exposing the tools for advanced use cases.

## Features
* Prioritized request queueing.
* Easy parallelization of crawlers and pipelines.
* Stop, save and resume crawls.
* Global and per-domain throttling.
* Proxy switching.
* Support for robots.txt, including non-standard directives and custom filter functions (e.a. ignore certain rules).

## Roadmap
* Support for Redis, allowing distributed scraping.
* Sitemap support.

## Example
```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/PuerkitoBio/goquery"

	"github.com/KillianMeersman/wander"
	"github.com/KillianMeersman/wander/limits"
	"github.com/KillianMeersman/wander/request"
)

func main() {
	spid, _ := wander.NewSpider(
		wander.AllowedDomains("wikipedia.org", "en.wikipedia.org"),
		wander.MaxDepth(10),
		wander.Throttle(limits.NewDefaultThrottle(200*time.Millisecond)),
		wander.Threads(2),
	)

	spid.OnRequest(func(req *request.Request) {
		log.Printf("visiting %s", req)
	})

	spid.OnResponse(func(res *request.Response) {
		log.Printf("response from %s", res.Request)
	})

	spid.OnError(func(err error) {
		log.Fatal(err)
	})

	spid.OnHTML("a[href]", func(res *request.Response, el *goquery.Selection) {
		link, _ := el.Attr("href")

		if err := spid.Follow(link, res, 10-res.Request.Depth()); err != nil {
			log.Printf(err.Error())
		}
	})

	spid.Visit("https://en.wikipedia.org")

	go func() {
		<-time.After(5 * time.Second)
		spid.Stop(context.Background())
	}()

	spid.Start()
	spid.Wait()
}
```

## Installation
`go get -u github.com/KillianMeersman/wander`