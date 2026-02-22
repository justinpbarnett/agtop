package cost

import (
	"sync"
	"testing"
	"time"
)

func TestRecordAndRunCosts(t *testing.T) {
	tr := NewTracker()

	sc1 := SkillCost{SkillName: "build", TotalTokens: 1000, CostUSD: 0.05, CompletedAt: time.Now()}
	sc2 := SkillCost{SkillName: "test", TotalTokens: 500, CostUSD: 0.02, CompletedAt: time.Now()}

	tr.Record("run-1", sc1)
	tr.Record("run-1", sc2)

	costs := tr.RunCosts("run-1")
	if len(costs) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(costs))
	}
	if costs[0].SkillName != "build" {
		t.Errorf("expected first skill 'build', got %q", costs[0].SkillName)
	}
	if costs[1].SkillName != "test" {
		t.Errorf("expected second skill 'test', got %q", costs[1].SkillName)
	}
}

func TestRunCostsReturnsNilForUnknownRun(t *testing.T) {
	tr := NewTracker()
	costs := tr.RunCosts("nonexistent")
	if costs != nil {
		t.Errorf("expected nil for unknown run, got %v", costs)
	}
}

func TestRunCostsReturnsCopy(t *testing.T) {
	tr := NewTracker()
	tr.Record("run-1", SkillCost{SkillName: "build", TotalTokens: 100})

	costs := tr.RunCosts("run-1")
	costs[0].SkillName = "modified"

	original := tr.RunCosts("run-1")
	if original[0].SkillName != "build" {
		t.Error("RunCosts should return a copy, not a reference")
	}
}

func TestRunTotal(t *testing.T) {
	tr := NewTracker()
	tr.Record("run-1", SkillCost{TotalTokens: 1000, CostUSD: 0.05})
	tr.Record("run-1", SkillCost{TotalTokens: 2000, CostUSD: 0.10})
	tr.Record("run-1", SkillCost{TotalTokens: 500, CostUSD: 0.02})

	tokens, cost := tr.RunTotal("run-1")
	if tokens != 3500 {
		t.Errorf("expected 3500 tokens, got %d", tokens)
	}
	if cost != 0.17 {
		t.Errorf("expected 0.17 cost, got %f", cost)
	}
}

func TestRunTotalUnknownRun(t *testing.T) {
	tr := NewTracker()
	tokens, cost := tr.RunTotal("nonexistent")
	if tokens != 0 || cost != 0 {
		t.Errorf("expected 0/0 for unknown run, got %d/%f", tokens, cost)
	}
}

func TestSessionTotal(t *testing.T) {
	tr := NewTracker()
	tr.Record("run-1", SkillCost{TotalTokens: 1000, CostUSD: 0.05})
	tr.Record("run-2", SkillCost{TotalTokens: 2000, CostUSD: 0.10})
	tr.Record("run-1", SkillCost{TotalTokens: 500, CostUSD: 0.02})

	tokens, cost := tr.SessionTotal()
	if tokens != 3500 {
		t.Errorf("expected 3500 session tokens, got %d", tokens)
	}
	if cost != 0.17 {
		t.Errorf("expected 0.17 session cost, got %f", cost)
	}
}

func TestRemove(t *testing.T) {
	tr := NewTracker()
	tr.Record("run-1", SkillCost{TotalTokens: 1000, CostUSD: 0.05})
	tr.Record("run-2", SkillCost{TotalTokens: 2000, CostUSD: 0.10})

	tr.Remove("run-1")

	costs := tr.RunCosts("run-1")
	if costs != nil {
		t.Errorf("expected nil after remove, got %v", costs)
	}

	tokens, sessionCost := tr.SessionTotal()
	if tokens != 2000 {
		t.Errorf("expected 2000 session tokens after remove, got %d", tokens)
	}
	if sessionCost < 0.099 || sessionCost > 0.101 {
		t.Errorf("expected ~0.10 session cost after remove, got %f", sessionCost)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	tr := NewTracker()
	tr.Remove("nonexistent") // should not panic
}

func TestConcurrentRecords(t *testing.T) {
	tr := NewTracker()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			runID := "run-1"
			if n%2 == 0 {
				runID = "run-2"
			}
			tr.Record(runID, SkillCost{TotalTokens: 10, CostUSD: 0.001})
		}(i)
	}

	wg.Wait()

	tokens, cost := tr.SessionTotal()
	if tokens != 1000 {
		t.Errorf("expected 1000 total tokens, got %d", tokens)
	}
	expectedCost := 0.1
	if cost < expectedCost-0.001 || cost > expectedCost+0.001 {
		t.Errorf("expected ~0.1 total cost, got %f", cost)
	}
}
