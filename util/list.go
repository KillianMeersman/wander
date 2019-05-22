package util

import "sync"

type stringLinkedListNode struct {
	value    string
	next     *stringLinkedListNode
	previous *stringLinkedListNode
}

type StringLinkedList struct {
	first, last *stringLinkedListNode
	count       int
	lock        sync.RWMutex
}

func NewStringLinkedList() *StringLinkedList {
	return &StringLinkedList{
		lock: sync.RWMutex{},
	}
}

// Add a value to the end of the list
func (l *StringLinkedList) Add(value string) {
	l.lock.Lock()
	defer l.lock.Unlock()

	newNode := &stringLinkedListNode{
		value:    value,
		previous: l.last,
		next:     nil,
	}

	if l.first == nil {
		l.first = newNode
		l.last = newNode
	} else {
		l.last.next = newNode
		l.last = newNode
	}
	l.count++
}

// Pop the last value off the list
func (l *StringLinkedList) Pop() string {
	l.lock.Lock()
	defer l.lock.Unlock()

	node := l.last

	if node == l.first {
		l.last = nil
		l.first = nil
	} else {
		l.last = l.last.previous
		l.last.next = nil
	}
	l.count--

	return node.value
}

func (l *StringLinkedList) Count() int {
	return l.count
}
