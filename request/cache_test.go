package request_test

import (
	"testing"

	"github.com/KillianMeersman/wander/request"
)

func TestLocalRequestCache(t *testing.T) {
	cache := request.NewCache()

	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	for _, req := range requests {
		cache.AddRequest(req)
	}

	for _, req := range requests {
		visited, err := cache.VisitedURL(req)
		if err != nil {
			t.Fatal(err)
		}
		if !visited {
			t.Fatal("request not in cache")
		}
	}

	req, err := randomRequests(1)
	if err != nil {
		t.Fatal(err)
	}

	visited, err := cache.VisitedURL(req[0])
	if err != nil {
		t.Fatal(err)
	}
	if visited {
		t.Fatal("request in cache when it shouldn't be")
	}

}

func TestRedisRequestCache(t *testing.T) {
	cache, err := request.NewRedisCache("localhost", 6379, "", "wander_request_cache", 1)
	if err != nil {
		t.Fatal(err)
	}

	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	for _, req := range requests {
		err := cache.AddRequest(req)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, req := range requests {
		visited, err := cache.VisitedURL(req)
		if err != nil {
			t.Fatal(err)
		}
		if !visited {
			t.Fatal("request not in cache")
		}
	}

	req, err := randomRequests(1)
	if err != nil {
		t.Fatal(err)
	}

	visited, err := cache.VisitedURL(req[0])
	if err != nil {
		t.Fatal(err)
	}
	if visited {
		t.Fatal("request in cache when it shouldn't be")
	}
}
