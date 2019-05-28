package request_test

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/KillianMeersman/wander/request"
)

var letterRunes = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

func init() {
	rand.Seed(time.Now().UnixNano())
}

func randomString(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[rand.Intn(len(letterRunes))]
	}
	return string(b)
}

func randomRequests(n int) ([]*request.Request, error) {
	requests := make([]*request.Request, n)
	var parent *request.Request = nil
	for i := 0; i < n; i++ {
		req, err := request.NewRequest(fmt.Sprintf("http://%s", randomString(50)), parent)
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
	requests, err := randomRequests(1000)
	if err != nil {
		b.Fatal(err)
	}
	heap := request.NewRequestHeap(10000)
	if err != nil {
		b.Fatal(err)
	}

	done := false
	go func() {
		for _, req := range requests {
			heap.Enqueue(req, 100-req.Depth())
			if err != nil {
				b.Fatal(err)
			}
		}
		done = true
	}()

	ctx := context.Background()
	c := make(chan struct{})
	time.Sleep(100 * time.Millisecond)
	go func() {
		select {
		case <-heap.Dequeue(ctx):
		default:
			if heap.Count() < 1 && done {
				c <- struct{}{}
			}
		}
	}()
	<-c
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
