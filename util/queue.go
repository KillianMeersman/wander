package util

import (
	"errors"
	"sync"
)

// NoData is returned when no data is availabe to dequeue
type NoData struct{}

func (n NoData) Error() string {
	return "No data"
}

// StringQueue is a FIFO queue
type StringQueue interface {
	Enqueue(value string) error
	Dequeue() (string, bool)
	Count() int
}

// CircularStringBuffer is a FIFO buffer that automatically expands as needed (up to maxSize)
type CircularStringBuffer struct {
	data       []string
	head, tail int
	count      int
	maxSize    int
	lock       *sync.Mutex
	available  *sync.Cond
}

func NewCircularStringBuffer(capacity int, maxSize int) *CircularStringBuffer {
	lock := &sync.Mutex{}

	return &CircularStringBuffer{
		data:      make([]string, capacity),
		maxSize:   maxSize,
		lock:      lock,
		available: sync.NewCond(lock),
	}
}

func (q *CircularStringBuffer) Enqueue(value string) error {
	q.lock.Lock()
	defer q.lock.Unlock()

	// check if the buffer slice needs to be reallocated
	if q.head == q.tail && q.count > 0 {
		newSize := len(q.data) * 2
		// will doubling the size go over max size?
		if newSize > q.maxSize {
			// if already at max size, return error
			if newSize == q.maxSize {
				return errors.New("Circular string buffer at max size")
			}
			// set size to max size
			newSize = q.maxSize
		}
		data := make([]string, newSize)
		copy(data, q.data[q.head:])
		copy(data[len(q.data)-q.head:], q.data[:q.head])
		q.head = 0
		q.tail = len(q.data)
		q.data = data
	}

	q.data[q.tail] = value
	q.tail = (q.tail + 1) % len(q.data)
	q.count++

	if q.count > 1 {
		q.available.Signal()
	}

	return nil
}

func (q *CircularStringBuffer) Dequeue() (string, bool) {
	q.lock.Lock()
	for q.count < 1 {
		q.available.Wait()
	}
	defer q.lock.Unlock()

	if q.count < 1 {
		return "", false
	}
	value := q.data[q.head]

	q.head = (q.head + 1) % len(q.data)
	q.count--
	return value, true
}

func (q *CircularStringBuffer) Count() int {
	return q.count
}
