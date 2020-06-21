# Wander

## Overview

Convenient scraping library for Gophers.

Based on Colly and Scrapy, Wander aims to provide an easy-to-use API while also exposing the tools for advanced use cases.

## Features

- Prioritized request queueing.
- Redis support for distributed scraping.
- Easy parallelization of crawlers and pipelines.
- Stop, save and resume crawls.
- Global and per-domain throttling.
- Proxy switching.
- Support for robots.txt, including non-standard directives and custom filter functions (e.a. ignore certain rules).
- Sitemap support

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
		wander.AllowedDomains("localhost:8080"),
		wander.MaxDepth(10),
		wander.Throttle(limits.NewDefaultThrottle(200*time.Millisecond)),
		wander.Threads(2),
	)

	spid.OnRequest(func(req *request.Request) *request.Request {
		log.Printf("visiting %s", req.URL.String())
		return req
	})

	spid.OnResponse(func(res *request.Response) {
		log.Printf("response from %s", res.Request.URL.String())
	})

	spid.OnError(func(err error) {
		log.Fatal(err)
	})

	spid.OnHTML("a[href]", func(res *request.Response, el *goquery.Selection) {
		link, _ := el.Attr("href")
		url, err := url.Parse(link)
		if err != nil {
			log.Fatal(err)
		}

		if err := spid.Follow(url, res, 10-res.Request.Depth); err != nil {
			log.Printf(err.Error())
		}
	})

	root := &url.URL{
		Scheme: "http",
		Host:   "localhost:8080",
	}
	spid.Visit(root)

	// Get sitemap
	robotsURL := root
	robotsURL.Path = "/robots.txt"
	robots, err := robots.NewRobotFileFromURL(robotsURL, spid)
	if err != nil {
		log.Fatal(err)
	}

	sitemap, err := robots.GetSitemap("wander", spid)
	if err != nil {
		log.Fatal(err)
	}

	locations, err := sitemap.GetLocations(spid, 10000)
	if err != nil {
		log.Fatal(err)
	}
	for _, location := range locations {
		url, err := url.Parse(location.Loc)
		if err != nil {
			log.Fatal(err)
		}
		spid.Visit(url)
	}

	go func() {
		<-time.After(5 * time.Second)
		spid.Stop(context.Background())
	}()

	spid.Start()
	spid.Wait()
```

## Installation

`go get -u github.com/KillianMeersman/wander`
