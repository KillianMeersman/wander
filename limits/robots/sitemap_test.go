package robots_test

import (
	"os"
	"testing"

	"github.com/KillianMeersman/wander/limits/robots"
)

func TestSitemapParsing(t *testing.T) {
	indexFile, err := os.Open("index.xml")
	if err != nil {
		t.Fatal(err)
	}
	index, err := robots.NewSitemapFromReader(indexFile)

	if len(index.Index) < 1 {
		t.Fatal("Index empty")
	}

	urlsetFile, err := os.Open("urlset.xml")
	if err != nil {
		t.Fatal(err)
	}

	index, err = robots.NewSitemapFromReader(urlsetFile)

	if len(index.URLSet) < 1 {
		t.Fatal("URLSet empty")
	}
}
