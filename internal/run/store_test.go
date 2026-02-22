package run

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStoreAdd(t *testing.T) {
	s := NewStore()
	id := s.Add(&Run{Branch: "feat/test"})

	if id != "001" {
		t.Errorf("expected id 001, got %s", id)
	}

	r, ok := s.Get(id)
	if !ok {
		t.Fatal("expected run to be found")
	}
	if r.Branch != "feat/test" {
		t.Errorf("expected branch feat/test, got %s", r.Branch)
	}
}

func TestStoreAddExplicitID(t *testing.T) {
	s := NewStore()
	id := s.Add(&Run{ID: "custom", Branch: "feat/custom"})

	if id != "custom" {
		t.Errorf("expected id custom, got %s", id)
	}

	r, ok := s.Get("custom")
	if !ok {
		t.Fatal("expected run to be found")
	}
	if r.Branch != "feat/custom" {
		t.Errorf("expected branch feat/custom, got %s", r.Branch)
	}
}

func TestStoreUpdate(t *testing.T) {
	s := NewStore()
	id := s.Add(&Run{Branch: "feat/test", State: StateQueued})

	s.Update(id, func(r *Run) {
		r.State = StateRunning
		r.Tokens = 1000
	})

	r, _ := s.Get(id)
	if r.State != StateRunning {
		t.Errorf("expected state running, got %s", r.State)
	}
	if r.Tokens != 1000 {
		t.Errorf("expected 1000 tokens, got %d", r.Tokens)
	}
}

func TestStoreUpdateMissing(t *testing.T) {
	s := NewStore()
	// Should not panic
	s.Update("nonexistent", func(r *Run) {
		r.State = StateRunning
	})
}

func TestStoreGetReturnsCopy(t *testing.T) {
	s := NewStore()
	id := s.Add(&Run{Branch: "feat/test", Tokens: 100})

	r, _ := s.Get(id)
	r.Tokens = 999

	r2, _ := s.Get(id)
	if r2.Tokens != 100 {
		t.Errorf("expected original tokens 100, got %d (Get did not return a copy)", r2.Tokens)
	}
}

func TestStoreList(t *testing.T) {
	s := NewStore()
	s.Add(&Run{Branch: "first"})
	s.Add(&Run{Branch: "second"})
	s.Add(&Run{Branch: "third"})

	list := s.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(list))
	}

	// Newest first
	if list[0].Branch != "third" {
		t.Errorf("expected first item to be 'third' (newest), got %s", list[0].Branch)
	}
	if list[1].Branch != "second" {
		t.Errorf("expected second item to be 'second', got %s", list[1].Branch)
	}
	if list[2].Branch != "first" {
		t.Errorf("expected third item to be 'first' (oldest), got %s", list[2].Branch)
	}
}

func TestStoreRemove(t *testing.T) {
	s := NewStore()
	id := s.Add(&Run{Branch: "feat/test"})

	s.Remove(id)

	_, ok := s.Get(id)
	if ok {
		t.Error("expected run to be removed")
	}
	if s.Count() != 0 {
		t.Errorf("expected count 0, got %d", s.Count())
	}
}

func TestStoreRemoveMissing(t *testing.T) {
	s := NewStore()
	// Should not panic
	s.Remove("nonexistent")
}

func TestStoreCount(t *testing.T) {
	s := NewStore()
	if s.Count() != 0 {
		t.Errorf("expected count 0, got %d", s.Count())
	}

	s.Add(&Run{Branch: "a"})
	s.Add(&Run{Branch: "b"})
	if s.Count() != 2 {
		t.Errorf("expected count 2, got %d", s.Count())
	}
}

func TestStoreAggregates(t *testing.T) {
	s := NewStore()
	s.Add(&Run{State: StateRunning, Tokens: 1000, Cost: 0.50})
	s.Add(&Run{State: StatePaused, Tokens: 2000, Cost: 1.00})
	s.Add(&Run{State: StateCompleted, Tokens: 3000, Cost: 1.50})
	s.Add(&Run{State: StateFailed, Tokens: 500, Cost: 0.25})
	s.Add(&Run{State: StateQueued, Tokens: 0, Cost: 0.00})

	if s.ActiveRuns() != 3 {
		t.Errorf("expected 3 active runs (running+paused+queued), got %d", s.ActiveRuns())
	}
	if s.TotalTokens() != 6500 {
		t.Errorf("expected 6500 total tokens, got %d", s.TotalTokens())
	}
	expected := 3.25
	if s.TotalCost() != expected {
		t.Errorf("expected total cost %.2f, got %.2f", expected, s.TotalCost())
	}
}

func TestStoreSubscriber(t *testing.T) {
	s := NewStore()
	called := 0
	s.Subscribe(func() {
		called++
	})

	s.Add(&Run{Branch: "a"})
	if called != 1 {
		t.Errorf("expected subscriber called 1 time after add, got %d", called)
	}

	s.Update("001", func(r *Run) { r.Tokens = 100 })
	if called != 2 {
		t.Errorf("expected subscriber called 2 times after update, got %d", called)
	}

	s.Remove("001")
	if called != 3 {
		t.Errorf("expected subscriber called 3 times after remove, got %d", called)
	}
}

func TestStoreChangesChannel(t *testing.T) {
	s := NewStore()

	s.Add(&Run{Branch: "a"})

	select {
	case <-s.Changes():
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Error("expected change notification on channel")
	}
}

func TestStoreChangesChannelCoalesces(t *testing.T) {
	s := NewStore()

	// Rapid mutations
	s.Add(&Run{Branch: "a"})
	s.Add(&Run{Branch: "b"})
	s.Add(&Run{Branch: "c"})

	// Should get at least one notification
	select {
	case <-s.Changes():
		// ok
	case <-time.After(100 * time.Millisecond):
		t.Error("expected at least one change notification")
	}
}

func TestStoreConcurrency(t *testing.T) {
	s := NewStore()
	var wg sync.WaitGroup
	n := 50

	// Concurrent adds
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Add(&Run{Branch: fmt.Sprintf("branch-%d", i)})
		}(i)
	}
	wg.Wait()

	if s.Count() != n {
		t.Errorf("expected %d runs, got %d", n, s.Count())
	}

	// Concurrent updates
	list := s.List()
	for _, r := range list {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			s.Update(id, func(r *Run) {
				r.Tokens += 100
			})
		}(r.ID)
	}
	wg.Wait()

	// Concurrent reads
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = s.List()
			_ = s.Count()
			_ = s.TotalTokens()
			_ = s.ActiveRuns()
		}()
	}
	wg.Wait()
}
