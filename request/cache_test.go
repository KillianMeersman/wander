package request_test

import (
	"testing"

	"github.com/KillianMeersman/wander/request"
)

func TestLocalRequestCache(t *testing.T) {
	cache := request.NewRequestCache()

	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	for _, req := range requests {
		cache.AddRequest(req)
	}

	for _, req := range requests {
		if !cache.VisitedURL(req) {
			t.Fatal("request not in cache")
		}
	}

	req, err := randomRequests(1)
	if err != nil {
		t.Fatal(err)
	}
	if cache.VisitedURL(req[0]) {
		t.Fatal("request in cache when it shouldn't be")
	}

}
