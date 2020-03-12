package request

import (
	"fmt"
	"sync"
)

// Queue is a prioritized FIFO queue for requests
type Queue interface {
	Enqueue(req *Request, priority int) error
	Dequeue() (*Request, bool)
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
	insertionCount int
	lock           *sync.Mutex
}

// NewHeap returns a request heap (priority queue).
func NewHeap(maxSize int) *Heap {
	lock := &sync.Mutex{}
	heap := &Heap{
		data:    make([]heapNode, maxSize/10),
		maxSize: maxSize,
		lock:    lock,
	}
	return heap
}

// BuildHeap builds a request heap from existing data.
func BuildHeap(data []heapNode, maxSize int) *Heap {
	heap := NewHeap(maxSize)

	for i := len(data) / 2; i >= 0; i-- {
		heap.maxHeapify(i)
	}

	return heap
}

// Enqueue a request with the given priority.
func (r *Heap) Enqueue(req *Request, priority int) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	err := r.insert(req, priority)
	if err != nil {
		return err
	}

	return nil
}

// Dequeue a request.
func (r *Heap) Dequeue() (*Request, bool) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if r.count > 0 {
		return r.extract(), true
	}
	return nil, false
}

// Peek returns the next request without removing it from the queue.
func (r *Heap) Peek() *Request {
	return r.data[0].request
}

// Count returns the amount of requests in the queue.
// Returns nil when no requests are in the heap.
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

// Sort the heap so that the highest priority request is the root node
// Starts from i (array index) and sifts down, swapping nodes as nescesary along the way
func (r *Heap) maxHeapify(i int) {
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
