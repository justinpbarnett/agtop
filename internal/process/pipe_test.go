package process

import (
	"fmt"
	"sync"
	"testing"
)

func TestRingBufferAppend(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Append("line 1")
	rb.Append("line 2")
	rb.Append("line 3")

	lines := rb.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(lines))
	}
	if lines[0] != "line 1" {
		t.Errorf("expected 'line 1', got %q", lines[0])
	}
	if lines[1] != "line 2" {
		t.Errorf("expected 'line 2', got %q", lines[1])
	}
	if lines[2] != "line 3" {
		t.Errorf("expected 'line 3', got %q", lines[2])
	}
}

func TestRingBufferOverflow(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Append("a")
	rb.Append("b")
	rb.Append("c")
	rb.Append("d")
	rb.Append("e")

	lines := rb.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines (capacity), got %d", len(lines))
	}
	// Oldest lines dropped, should have c, d, e
	if lines[0] != "c" {
		t.Errorf("expected 'c' at index 0, got %q", lines[0])
	}
	if lines[1] != "d" {
		t.Errorf("expected 'd' at index 1, got %q", lines[1])
	}
	if lines[2] != "e" {
		t.Errorf("expected 'e' at index 2, got %q", lines[2])
	}
}

func TestRingBufferTail(t *testing.T) {
	rb := NewRingBuffer(10)

	for i := 0; i < 7; i++ {
		rb.Append(fmt.Sprintf("line %d", i))
	}

	tail := rb.Tail(3)
	if len(tail) != 3 {
		t.Fatalf("expected 3 tail lines, got %d", len(tail))
	}
	if tail[0] != "line 4" {
		t.Errorf("expected 'line 4', got %q", tail[0])
	}
	if tail[1] != "line 5" {
		t.Errorf("expected 'line 5', got %q", tail[1])
	}
	if tail[2] != "line 6" {
		t.Errorf("expected 'line 6', got %q", tail[2])
	}
}

func TestRingBufferTailMoreThanCount(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Append("only")

	tail := rb.Tail(5)
	if len(tail) != 1 {
		t.Fatalf("expected 1 tail line, got %d", len(tail))
	}
	if tail[0] != "only" {
		t.Errorf("expected 'only', got %q", tail[0])
	}
}

func TestRingBufferTailWithOverflow(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Append("a")
	rb.Append("b")
	rb.Append("c")
	rb.Append("d")
	rb.Append("e")

	tail := rb.Tail(2)
	if len(tail) != 2 {
		t.Fatalf("expected 2 tail lines, got %d", len(tail))
	}
	if tail[0] != "d" {
		t.Errorf("expected 'd', got %q", tail[0])
	}
	if tail[1] != "e" {
		t.Errorf("expected 'e', got %q", tail[1])
	}
}

func TestRingBufferTailZero(t *testing.T) {
	rb := NewRingBuffer(10)
	rb.Append("a")

	tail := rb.Tail(0)
	if tail != nil {
		t.Errorf("expected nil for Tail(0), got %v", tail)
	}
}

func TestRingBufferLen(t *testing.T) {
	rb := NewRingBuffer(5)

	if rb.Len() != 0 {
		t.Errorf("expected len 0, got %d", rb.Len())
	}

	rb.Append("a")
	rb.Append("b")
	if rb.Len() != 2 {
		t.Errorf("expected len 2, got %d", rb.Len())
	}

	// Fill and overflow
	for i := 0; i < 10; i++ {
		rb.Append("x")
	}
	if rb.Len() != 5 {
		t.Errorf("expected len 5 (capped at capacity), got %d", rb.Len())
	}
}

func TestRingBufferTotalWritten(t *testing.T) {
	rb := NewRingBuffer(3)

	if rb.TotalWritten() != 0 {
		t.Errorf("expected 0 total written, got %d", rb.TotalWritten())
	}

	rb.Append("a")
	rb.Append("b")
	rb.Append("c")
	rb.Append("d")

	if rb.TotalWritten() != 4 {
		t.Errorf("expected 4 total written, got %d", rb.TotalWritten())
	}
}

func TestRingBufferReset(t *testing.T) {
	rb := NewRingBuffer(10)

	rb.Append("a")
	rb.Append("b")
	rb.Reset()

	if rb.Len() != 0 {
		t.Errorf("expected len 0 after reset, got %d", rb.Len())
	}
	if rb.TotalWritten() != 0 {
		t.Errorf("expected 0 total written after reset, got %d", rb.TotalWritten())
	}
	lines := rb.Lines()
	if lines != nil {
		t.Errorf("expected nil lines after reset, got %v", lines)
	}
}

func TestRingBufferEmpty(t *testing.T) {
	rb := NewRingBuffer(10)

	lines := rb.Lines()
	if lines != nil {
		t.Errorf("expected nil for empty buffer, got %v", lines)
	}

	tail := rb.Tail(5)
	if tail != nil {
		t.Errorf("expected nil tail for empty buffer, got %v", tail)
	}
}

func TestRingBufferExactCapacity(t *testing.T) {
	rb := NewRingBuffer(3)

	rb.Append("a")
	rb.Append("b")
	rb.Append("c")

	lines := rb.Lines()
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines at exact capacity, got %d", len(lines))
	}
	if lines[0] != "a" || lines[1] != "b" || lines[2] != "c" {
		t.Errorf("expected [a, b, c], got %v", lines)
	}

	// One more should wrap
	rb.Append("d")
	lines = rb.Lines()
	if lines[0] != "b" || lines[1] != "c" || lines[2] != "d" {
		t.Errorf("expected [b, c, d] after wrap, got %v", lines)
	}
}

func TestRingBufferConcurrent(t *testing.T) {
	rb := NewRingBuffer(100)

	var wg sync.WaitGroup
	// 10 writers
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				rb.Append(fmt.Sprintf("writer %d line %d", id, j))
			}
		}(i)
	}
	// 5 readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = rb.Lines()
				_ = rb.Tail(10)
				_ = rb.Len()
				_ = rb.TotalWritten()
			}
		}()
	}

	wg.Wait()

	// After all writes, buffer should be at capacity
	if rb.Len() != 100 {
		t.Errorf("expected len 100, got %d", rb.Len())
	}
	if rb.TotalWritten() != 1000 {
		t.Errorf("expected 1000 total written, got %d", rb.TotalWritten())
	}
}

func TestRingBufferDefaultCapacity(t *testing.T) {
	rb := NewRingBuffer(0)
	if rb.capacity != 10000 {
		t.Errorf("expected default capacity 10000, got %d", rb.capacity)
	}

	rb2 := NewRingBuffer(-5)
	if rb2.capacity != 10000 {
		t.Errorf("expected default capacity 10000 for negative, got %d", rb2.capacity)
	}
}
