package cost

import (
	"sync"
	"time"
)

// SkillCost records the token usage and cost for a single skill execution.
type SkillCost struct {
	SkillName    string    `json:"skill_name"`
	InputTokens  int       `json:"input_tokens"`
	OutputTokens int       `json:"output_tokens"`
	TotalTokens  int       `json:"total_tokens"`
	CostUSD      float64   `json:"cost_usd"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  time.Time `json:"completed_at"`
}

// Tracker maintains per-run skill cost ledgers and session-wide aggregates.
type Tracker struct {
	mu            sync.RWMutex
	runs          map[string][]SkillCost
	sessionTokens int
	sessionCost   float64
}

func NewTracker() *Tracker {
	return &Tracker{
		runs: make(map[string][]SkillCost),
	}
}

// Record appends a skill cost entry and updates session totals.
func (t *Tracker) Record(runID string, sc SkillCost) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runs[runID] = append(t.runs[runID], sc)
	t.sessionTokens += sc.TotalTokens
	t.sessionCost += sc.CostUSD
}

// RunCosts returns the per-skill cost ledger for a run.
func (t *Tracker) RunCosts(runID string) []SkillCost {
	t.mu.RLock()
	defer t.mu.RUnlock()
	entries := t.runs[runID]
	if entries == nil {
		return nil
	}
	out := make([]SkillCost, len(entries))
	copy(out, entries)
	return out
}

// RunTotal sums a single run's tokens and cost.
func (t *Tracker) RunTotal(runID string) (tokens int, cost float64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	for _, sc := range t.runs[runID] {
		tokens += sc.TotalTokens
		cost += sc.CostUSD
	}
	return
}

// SessionTotal returns session-wide aggregate tokens and cost.
func (t *Tracker) SessionTotal() (tokens int, cost float64) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.sessionTokens, t.sessionCost
}

// Remove cleans up the ledger for a removed run and adjusts session totals.
func (t *Tracker) Remove(runID string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, sc := range t.runs[runID] {
		t.sessionTokens -= sc.TotalTokens
		t.sessionCost -= sc.CostUSD
	}
	delete(t.runs, runID)
}
