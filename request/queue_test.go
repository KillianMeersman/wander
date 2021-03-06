package request_test

import (
	"net/url"
	"testing"

	"github.com/KillianMeersman/wander/request"
	"github.com/KillianMeersman/wander/util"
)

func randomRequests(n int) ([]*request.Request, error) {
	requests := make([]*request.Request, n)
	var parent *request.Request = nil
	for i := 0; i < n; i++ {
		url := &url.URL{
			Scheme: "http",
			Host:   util.RandomString(50),
		}
		req, err := request.NewRequest(url, parent)
		if err != nil {
			return nil, err
		}
		requests[i] = req
		if i%100 != 0 {
			parent = requests[i]
		} else {
			parent = nil
		}
	}

	return requests, nil
}

func BenchmarkRequestHeap(b *testing.B) {
	n := b.N

	requests, err := randomRequests(n)
	if err != nil {
		b.Fatal(err)
	}

	heap := request.NewRequestHeap(n)
	defer heap.Close()
	if err != nil {
		b.Fatal(err)
	}

	for i, req := range requests {
		err := heap.Enqueue(req, n-i)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestRequestHeapEqualPriority(t *testing.T) {
	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	heap := request.NewRequestHeap(10000)
	defer heap.Close()
	if err != nil {
		t.Fatal(err)
	}

	for _, req := range requests {
		err := heap.Enqueue(req, 1)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _, a := range requests {
		b := <-heap.Dequeue()
		if a != b.Request {
			t.Fatal("requests dequeued in incorrect order")
		}
	}
}

func TestRequestHeapDifferentPriority(t *testing.T) {
	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	heap := request.NewRequestHeap(1001)
	defer heap.Close()
	if err != nil {
		t.Fatal(err)
	}

	for i, req := range requests {
		err := heap.Enqueue(req, i)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 999; i >= 0; i-- {
		req := <-heap.Dequeue()
		if req.Request != requests[i] {
			t.Fatal("requests dequeued in incorrect order")
		}
	}
}

func TestRequestRedisEqualPriority(t *testing.T) {
	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	queue, err := request.NewRedisQueue("localhost", 6379, "", "requests", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Clear()
	defer queue.Close()
	if err != nil {
		t.Fatal(err)
	}

	for _, req := range requests {
		err := queue.Enqueue(req, 1)
		if err != nil {
			t.Fatal(err)
		}
	}

	for _ = range requests {
		b := <-queue.Dequeue()
		if b.Error != nil {
			t.Fatal(b.Error)
		}
	}
}

func TestRequestRedisDifferentPriority(t *testing.T) {
	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	queue, err := request.NewRedisQueue("localhost", 6379, "", "requests", 0)
	if err != nil {
		t.Fatal(err)
	}
	defer queue.Clear()
	defer queue.Close()
	if err != nil {
		t.Fatal(err)
	}

	for i, req := range requests {
		err := queue.Enqueue(req, i)
		if err != nil {
			t.Fatal(err)
		}
	}

	for i := 999; i >= 0; i-- {
		req := <-queue.Dequeue()
		if req.Error != nil {
			t.Fatal(req.Error)
		}
		if *req.Request.URL != *requests[i].URL {
			t.Fatal("requests dequeued in incorrect order")
		}
	}
}
