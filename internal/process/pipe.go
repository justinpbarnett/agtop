package process

import "sync"

type RingBuffer struct {
	mu           sync.RWMutex
	lines        []string
	capacity     int
	head         int
	count        int
	totalWritten int
}

func NewRingBuffer(capacity int) *RingBuffer {
	if capacity <= 0 {
		capacity = 10000
	}
	return &RingBuffer{
		lines:    make([]string, capacity),
		capacity: capacity,
	}
}

func (rb *RingBuffer) Append(line string) {
	rb.mu.Lock()
	rb.lines[rb.head] = line
	rb.head = (rb.head + 1) % rb.capacity
	if rb.count < rb.capacity {
		rb.count++
	}
	rb.totalWritten++
	rb.mu.Unlock()
}

func (rb *RingBuffer) Lines() []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return nil
	}

	result := make([]string, rb.count)
	if rb.count < rb.capacity {
		copy(result, rb.lines[:rb.count])
	} else {
		// Buffer has wrapped: oldest is at head, newest is at head-1
		n := copy(result, rb.lines[rb.head:])
		copy(result[n:], rb.lines[:rb.head])
	}
	return result
}

func (rb *RingBuffer) Tail(n int) []string {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if n <= 0 || rb.count == 0 {
		return nil
	}
	if n > rb.count {
		n = rb.count
	}

	result := make([]string, n)
	// The newest line is at (head-1+capacity)%capacity
	// We want the last n lines
	start := (rb.head - n + rb.capacity) % rb.capacity
	if start+n <= rb.capacity {
		copy(result, rb.lines[start:start+n])
	} else {
		first := rb.capacity - start
		copy(result, rb.lines[start:])
		copy(result[first:], rb.lines[:n-first])
	}
	return result
}

func (rb *RingBuffer) Len() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.count
}

func (rb *RingBuffer) TotalWritten() int {
	rb.mu.RLock()
	defer rb.mu.RUnlock()
	return rb.totalWritten
}

func (rb *RingBuffer) Reset() {
	rb.mu.Lock()
	rb.head = 0
	rb.count = 0
	rb.totalWritten = 0
	rb.mu.Unlock()
}
