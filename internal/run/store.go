package run

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
)

type Store struct {
	mu          sync.RWMutex
	runs        map[string]*Run
	order       []string
	subscribers []func()
	changeCh    chan struct{}
}

func NewStore() *Store {
	return &Store{
		runs:     make(map[string]*Run),
		changeCh: make(chan struct{}, 1),
	}
}

func (s *Store) Add(r *Run) string {
	s.mu.Lock()
	if r.ID == "" {
		r.ID = generateID()
	}
	id := r.ID
	s.runs[id] = r
	s.order = append([]string{id}, s.order...)
	s.mu.Unlock()
	s.notify()
	return id
}

func generateID() string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)[:7]
}

func (s *Store) Update(id string, fn func(*Run)) {
	s.mu.Lock()
	r, ok := s.runs[id]
	if ok {
		fn(r)
	}
	s.mu.Unlock()
	if ok {
		s.notify()
	}
}

func (s *Store) Get(id string) (Run, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[id]
	if !ok {
		return Run{}, false
	}
	return *r, true
}

func (s *Store) List() []Run {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]Run, 0, len(s.order))
	for _, id := range s.order {
		if r, ok := s.runs[id]; ok {
			result = append(result, *r)
		}
	}
	return result
}

func (s *Store) Remove(id string) {
	s.mu.Lock()
	_, ok := s.runs[id]
	if ok {
		delete(s.runs, id)
		for i, oid := range s.order {
			if oid == id {
				s.order = append(s.order[:i], s.order[i+1:]...)
				break
			}
		}
	}
	s.mu.Unlock()
	if ok {
		s.notify()
	}
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runs)
}

func (s *Store) ActiveRuns() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, r := range s.runs {
		switch r.State {
		case StateRunning, StateRouting, StateQueued, StatePaused:
			count++
		}
	}
	return count
}

func (s *Store) TotalTokens() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, r := range s.runs {
		total += r.Tokens
	}
	return total
}

func (s *Store) TotalCost() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0.0
	for _, r := range s.runs {
		total += r.Cost
	}
	return total
}

func (s *Store) Subscribe(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers = append(s.subscribers, fn)
}

func (s *Store) Changes() <-chan struct{} {
	return s.changeCh
}

func (s *Store) notify() {
	s.mu.RLock()
	subs := make([]func(), len(s.subscribers))
	copy(subs, s.subscribers)
	s.mu.RUnlock()

	for _, fn := range subs {
		fn()
	}

	select {
	case s.changeCh <- struct{}{}:
	default:
	}
}
