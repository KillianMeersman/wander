package request

import (
	"context"
	"fmt"
	"sync"
)

// Queue is a priorized FIFO queue for requests
type Queue interface {
	Start(ctx context.Context)
	Enqueue(req *Request, priority int) error
	Dequeue() <-chan *Request
	Count() int
}

// QueueMaxSize signals the Queue has reached its maximum size.
type QueueMaxSize struct {
	size int
}

func (r QueueMaxSize) Error() string {
	return fmt.Sprintf("Request queue has reached maximum size of %d", r.size)
}

type heapNode struct {
	priority       int
	insertionCount int
	request        *Request
}

func less(a, b heapNode) bool {
	if a.priority < b.priority {
		return true
	}
	if a.priority == b.priority {
		if a.insertionCount > b.insertionCount {
			return true
		}
	}
	return false
}

// Heap is a heap implementation for request.Queue.
type Heap struct {
	data           []heapNode
	count          int
	maxSize        int
	lock           *sync.Mutex
	available      *sync.Cond
	outc           chan *Request
	inintalized    bool
	insertionCount int
}

func NewHeap(maxSize int) *Heap {
	lock := &sync.Mutex{}
	heap := &Heap{
		data:      make([]heapNode, maxSize/10),
		maxSize:   maxSize,
		lock:      lock,
		available: sync.NewCond(lock),
		outc:      make(chan *Request),
	}
	return heap
}

// BuildHeap builds a request heap from existing data.
func BuildHeap(data []heapNode, maxSize int) *Heap {
	lock := &sync.Mutex{}
	heap := &Heap{
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

// Start starts the consumer goroutine. This method should be called before any calls to Dequeue.
func (r *Heap) Start(ctx context.Context) {
	if !r.inintalized {
		r.outc = make(chan *Request)
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
					r.inintalized = false
					return
				}

			}
		}()
		r.inintalized = true
	}
}

// Enqueue a request with the given priority.
func (r *Heap) Enqueue(req *Request, priority int) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	err := r.insert(req, priority)
	if err != nil {
		return err
	}
	r.available.Broadcast()
	return nil
}

// Dequeue a request. The Dequeue channel is not closed when the context supplied to Start is cancelled.
func (r *Heap) Dequeue() <-chan *Request {
	return r.outc
}

// Count returns the amount of requests in the queue.
func (r *Heap) Count() int {
	return r.count
}

// insert a request.
func (r *Heap) insert(req *Request, priority int) error {
	node := heapNode{
		priority:       priority,
		request:        req,
		insertionCount: r.insertionCount + 1,
	}

	if r.count >= len(r.data) {
		newSize := (len(r.data) * 2) + 1
		if newSize > r.maxSize {
			if r.count == r.maxSize {
				return &QueueMaxSize{size: r.maxSize}
			}
			newSize = r.maxSize
		}
		data := make([]heapNode, newSize)
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
	r.insertionCount++
	return nil
}

// extract the root node and replace it with the last element, then sift down.
func (r *Heap) extract() *Request {
	req := r.data[0].request
	r.count--
	r.data[0] = r.data[r.count]
	r.maxHeapify(0)
	return req
}

func (r *Heap) maxHeapify(i int) {
	left := leftChildIndex(i)
	right := rightChildIndex(i)
	max := i

	if left < r.count && less(r.data[max], r.data[left]) {
		max = left
	}
	if right < r.count && less(r.data[max], r.data[right]) {
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
	parent := ((i + 1) / 2) - 1
	if parent < 0 {
		return 0
	}
	return parent
}
