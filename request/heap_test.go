package request_test

import (
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

	heap := request.NewHeap(10000)
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
		b, _ := heap.Dequeue()
		if a != b {
			t.Fatal("requests dequeued in incorrect order")
		}
	}
}

func TestRequestHeapDifferentPriority(t *testing.T) {
	requests, err := randomRequests(1000)
	if err != nil {
		t.Fatal(err)
	}

	heap := request.NewHeap(1001)
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
		req, _ := heap.Dequeue()
		if req != requests[i] {
			t.Fatal("requests dequeued in incorrect order")
		}
	}
}
