# Wander
## Overview
Easy-to-use scraping library for Gophers.

Based on Colly and Scrapy, Wander aims to provide a convenient API that supports advanced use cases.

## Features
* Prioritized request queueing
* Easy parallelization of crawlers and pipelines.
* Stop, save and resume crawls.
* Global and per-domain throttling.
* Proxies
* Support for robots.txt and custom filter functions (e.a. ignore certain rules)

## Installation
`go get -u github.com/KillianMeersman/wander`

## Examples
```go
package main

import (
	"context"
	"log"
	"time"

	"github.com/KillianMeersman/wander/limits"

	"github.com/PuerkitoBio/goquery"

	"github.com/KillianMeersman/wander"
	"github.com/KillianMeersman/wander/request"
)

func main() {
	spid, err := wander.NewSpider(
		wander.AllowedDomains("github.com", "www.github.com"),
		wander.MaxDepth(10),
		wander.IgnoreRobots(),
	)
	if err != nil {
		log.Fatal(err)
	}
	spid.SetThrottles(limits.NewDefaultThrottle(time.Second))

	spid.OnRequest(func(req *request.Request) {
		log.Printf("visiting %s", req)
	})

	spid.OnResponse(func(res *request.Response) {
		log.Printf("response from %s", res.Request)
		res.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
			link, ok := sel.Attr("href")
			if ok {
				spid.Follow(link, res, 10-res.Request.Depth())
			}
		})
	})

	spid.OnError(func(err error) {
		log.Fatal(err)
	})

	ctx := context.Background()
	spid.Visit("https://github.com")
	spid.Start(ctx)
}
```