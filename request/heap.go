package request

import (
	"context"
	"fmt"
	"sync"
)

// RequestQueue is a priorized FIFO queue for Requests
type RequestQueue interface {
	Enqueue(req *Request, priority int) error
	Dequeue(ctx context.Context) <-chan *Request
	Count() int
}

type RequestQueueMaxSize struct {
	size int
}

func (r *RequestQueueMaxSize) Error() string {
	return fmt.Sprintf("Request queue has reached maximum size of %d", r.size)
}

type requestHeapNode struct {
	priority int
	request  *Request
}

type RequestHeap struct {
	data        []requestHeapNode
	count       int
	maxSize     int
	lock        *sync.Mutex
	available   *sync.Cond
	outc        chan *Request
	inintalized bool
}

func NewRequestHeap(maxSize int) *RequestHeap {
	data := make([]requestHeapNode, maxSize/10)
	return BuildRequestHeap(data, maxSize)
}

func BuildRequestHeap(data []requestHeapNode, maxSize int) *RequestHeap {
	lock := &sync.Mutex{}
	heap := &RequestHeap{
		data:      data,
		maxSize:   maxSize,
		lock:      lock,
		available: sync.NewCond(lock),
		outc:      make(chan *Request),
	}

	for i := len(data) / 2; i >= 0; i-- {
		heap.maxHeapify(i)
	}

	return heap
}

func (r *RequestHeap) Enqueue(req *Request, priority int) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	err := r.insert(req, priority)
	if err != nil {
		return err
	}
	r.available.Broadcast()
	return nil
}

func (r *RequestHeap) Dequeue(ctx context.Context) <-chan *Request {
	if !r.inintalized {
		go func() {
			for {
				r.lock.Lock()
				for r.count < 1 {
					r.available.Wait()
				}
				req := r.extract()
				r.lock.Unlock()

				select {
				case r.outc <- req:
				case <-ctx.Done():
					return
				}

			}
		}()
		r.inintalized = true
	}

	return r.outc
}

func (r *RequestHeap) Count() int {
	return r.count
}

func (r *RequestHeap) insert(req *Request, priority int) error {
	node := requestHeapNode{
		priority: priority,
		request:  req,
	}

	if r.count >= len(r.data) {
		newSize := (len(r.data) * 2) + 1
		if newSize > r.maxSize {
			if r.count == r.maxSize {
				return &RequestQueueMaxSize{size: r.maxSize}
			}
			newSize = r.maxSize
		}
		data := make([]requestHeapNode, newSize)
		copy(data, r.data)
		r.data = data
	}

	i := r.count
	parent := parentIndex(i)
	r.data[i] = node

	for i > 0 && r.data[i].priority > r.data[parent].priority {
		r.data[i], r.data[parent] = r.data[parent], r.data[i]
		i = parentIndex(i)
		parent = parentIndex(i)
	}

	r.count++
	return nil
}

func (r *RequestHeap) extract() *Request {
	req := r.data[0].request
	r.count--
	r.data[0] = r.data[r.count-1]

	r.maxHeapify(0)

	return req
}

func (r *RequestHeap) maxHeapify(i int) {
	left := leftChildIndex(i)
	right := rightChildIndex(i)
	max := i

	if left < r.count && r.data[left].priority > r.data[max].priority {
		max = left
	}
	if right < r.count && r.data[right].priority > r.data[max].priority {
		max = right
	}
	if max != i {
		r.data[i], r.data[max] = r.data[max], r.data[i]
		r.maxHeapify(max)
	}
}

func leftChildIndex(i int) int {
	return (i * 2) + 1
}

func rightChildIndex(i int) int {
	return (i * 2) + 2
}

func parentIndex(i int) int {
	return i / 2
}

func (r *RequestHeap) leftChild(i int) requestHeapNode {
	return r.data[leftChildIndex(i)]
}

func (r *RequestHeap) rightChild(i int) requestHeapNode {
	return r.data[rightChildIndex(i)]
}

func (r *RequestHeap) parent(i int) requestHeapNode {
	return r.data[parentIndex(i)]
}
