package request_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/KillianMeersman/wander/request"
	"github.com/KillianMeersman/wander/util"
)

func randomRequests(n int) ([]*request.Request, error) {
	requests := make([]*request.Request, n)
	var parent *request.Request = nil
	for i := 0; i < n; i++ {
		req, err := request.NewRequest(fmt.Sprintf("http://%s", util.RandomString(50)), parent)
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

func TestRequestHeapEqualPriority(t *testing.T) {
	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	heap := request.NewRequestHeap(10000)
	if err != nil {
		t.Fatal(err)
	}

	for _, req := range requests {
		err := heap.Enqueue(req, 1)
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	i := 0
	for req := range heap.Dequeue(ctx) {
		if req != requests[i] {
			t.Fatal("requests dequeued in incorrect order")
		}
		i++
		if heap.Count() < 1 {
			break
		}
	}
}

func TestRequestHeapDifferentPriority(t *testing.T) {
	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	heap := request.NewRequestHeap(10000)
	if err != nil {
		t.Fatal(err)
	}

	for i, req := range requests {
		err := heap.Enqueue(req, i)
		if err != nil {
			t.Fatal(err)
		}
	}

	ctx := context.Background()
	i := 999
	for req := range heap.Dequeue(ctx) {
		if req != requests[i] {
			t.Fatal("requests dequeued in incorrect order")
		}
		i--
		if heap.Count() < 1 {
			break
		}
	}
}
