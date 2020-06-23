package request

import (
	"fmt"
	"io"
	"sync"
)

type QueueResult struct {
	Error   error
	Request *Request
}

// Queue is a prioritized FIFO queue for requests
type Queue interface {
	io.Closer
	// Enqueue adds the request to the queue, returns an error if no more space is available.
	Enqueue(req *Request, priority int) error
	// Dequeue pops the highest priority request from the queue.
	Dequeue() <-chan QueueResult
	// Count returns the amount of queued requests.
	Count() (int, error)
	Clear()
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

// RequestHeapQueue is a heap implementation for request.Queue.
type RequestHeapQueue struct {
	data           []heapNode
	count          int
	maxSize        int
	insertionCount int
	lock           *sync.Mutex
	waitCondition  *sync.Cond
	waitGroup      *sync.WaitGroup
	isDone         bool
}

// NewRequestHeap returns a request heap (priority queue).
func NewRequestHeap(maxSize int) *RequestHeapQueue {
	lock := &sync.Mutex{}
	heap := &RequestHeapQueue{
		data:          make([]heapNode, maxSize/10),
		maxSize:       maxSize,
		lock:          lock,
		waitCondition: sync.NewCond(lock),
		waitGroup:     &sync.WaitGroup{},
		isDone:        false,
	}
	return heap
}

// BuildHeap builds a request heap from existing data.
func BuildHeap(data []heapNode, maxSize int) *RequestHeapQueue {
	heap := NewRequestHeap(maxSize)

	for i := len(data) / 2; i >= 0; i-- {
		heap.maxHeapify(i)
	}

	return heap
}

// Enqueue a request with the given priority.
func (r *RequestHeapQueue) Enqueue(req *Request, priority int) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.insert(req, priority)
}

func (r *RequestHeapQueue) Dequeue() <-chan QueueResult {
	outlet := make(chan QueueResult)
	go func() {
		r.waitGroup.Add(1)
		r.waitCondition.L.Lock()

		// wait untl an item is available or Close is called
		for r.count < 1 && !r.isDone {
			r.waitCondition.Wait()
		}

		if r.isDone {
			r.waitCondition.L.Unlock()
		} else {
			req := r.extract()
			r.waitCondition.L.Unlock()
			outlet <- QueueResult{
				Request: req,
			}

		}

		r.waitGroup.Done()
	}()

	return outlet
}

func (r *RequestHeapQueue) Close() error {
	r.isDone = true
	r.waitCondition.Broadcast()
	r.waitGroup.Wait()
	return nil
}

func (r *RequestHeapQueue) Clear() {
	for i := range r.data {
		r.data[i] = heapNode{}
	}
}

// Count returns the amount of requests in the queue.
func (r *RequestHeapQueue) Count() (int, error) {
	return r.count, nil
}

// insert a request.
func (r *RequestHeapQueue) insert(req *Request, priority int) error {
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
	r.waitCondition.Signal()

	return nil
}

// extract the root node and replace it with the last element, then sift down.
func (r *RequestHeapQueue) extract() *Request {
	req := r.data[0].request
	r.count--
	r.data[0] = r.data[r.count]
	r.maxHeapify(0)
	return req
}

// Sort the heap so that the highest priority request is the root node
// Starts from i (array index) and sifts down, swapping nodes as nescesary along the way
func (r *RequestHeapQueue) maxHeapify(i int) {
	max := i
	for {
		// get the children and set the current max value to the starting node
		left := leftChildIndex(i)
		right := rightChildIndex(i)

		// if left child is not the last node and is less than the parent node, set max to this node index
		if left < r.count && less(r.data[max], r.data[left]) {
			max = left
		}
		// same thing, but with right child
		if right < r.count && less(r.data[max], r.data[right]) {
			max = right
		}

		// stop sifting if no swap occured, the heap is sorted
		if max == i {
			return
		}

		// if a swap occured, swap the actual data and continue sifting into the next node
		r.data[i], r.data[max] = r.data[max], r.data[i]
		i = max
	}
}

// get the index of the left child node
func leftChildIndex(i int) int {
	return (i * 2) + 1
}

// get the index of the right child node
func rightChildIndex(i int) int {
	return (i * 2) + 2
}

// get the index of the parent node
func parentIndex(i int) int {
	parent := ((i + 1) / 2) - 1
	if parent < 0 {
		return 0
	}
	return parent
}
