package robots_test

import (
	"testing"
	"time"

	"github.com/KillianMeersman/wander"
	"github.com/KillianMeersman/wander/limits"
	"github.com/KillianMeersman/wander/limits/robots"
)

func TestSitemapParsing(t *testing.T) {
	spider, err := wander.NewSpider(wander.Throttle(limits.NewDefaultThrottle(1*time.Second)), wander.AllowedDomains("localhost:8080"))
	if err != nil {
		t.Fatal(err)
	}

	robots, err := robots.NewRobotFileFromURL("https://bol.com/robots.txt", spider)
	if err != nil {
		t.Fatal(err)
	}

	sitemap, err := robots.Sitemap("wander", spider)
	if err != nil {
		t.Fatal(err)
	}

	locations, err := sitemap.GetURLs(spider, 50000)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(len(locations))
}
