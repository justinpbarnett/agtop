package panels

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/run"
)

func testStore() *run.Store {
	now := time.Now()
	s := run.NewStore()
	s.Add(&run.Run{Branch: "feat/add-auth", Workflow: "sdlc", State: run.StateRunning, SkillIndex: 3, SkillTotal: 7, Tokens: 12400, Cost: 0.42, CurrentSkill: "build", StartedAt: now.Add(-3 * time.Minute)})
	s.Add(&run.Run{Branch: "fix/nav-bug", Workflow: "quick-fix", State: run.StatePaused, SkillIndex: 1, SkillTotal: 3, Tokens: 3100, Cost: 0.08, CurrentSkill: "build", StartedAt: now.Add(-7 * time.Minute)})
	s.Add(&run.Run{Branch: "feat/dashboard", Workflow: "plan-build", State: run.StateReviewing, SkillIndex: 3, SkillTotal: 3, Tokens: 45200, Cost: 1.23, StartedAt: now.Add(-12 * time.Minute), CompletedAt: now.Add(-2 * time.Minute)})
	s.Add(&run.Run{Branch: "fix/css-overflow", Workflow: "build", State: run.StateFailed, SkillIndex: 2, SkillTotal: 3, Tokens: 8700, Cost: 0.31, Error: "build skill timed out", StartedAt: now.Add(-5 * time.Minute), CompletedAt: now.Add(-1 * time.Minute)})
	return s
}

func keyMsg(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestRunListNavigation(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)

	if rl.selected != 0 {
		t.Errorf("expected initial selection 0, got %d", rl.selected)
	}

	rl, _ = rl.Update(keyMsg("j"))
	if rl.selected != 1 {
		t.Errorf("expected selection 1 after j, got %d", rl.selected)
	}

	rl, _ = rl.Update(keyMsg("k"))
	if rl.selected != 0 {
		t.Errorf("expected selection 0 after k, got %d", rl.selected)
	}
}

func TestRunListBounds(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)

	rl, _ = rl.Update(keyMsg("k"))
	if rl.selected != 0 {
		t.Errorf("expected selection clamped at 0, got %d", rl.selected)
	}

	for i := 0; i < 10; i++ {
		rl, _ = rl.Update(keyMsg("j"))
	}
	if rl.selected != len(rl.filtered)-1 {
		t.Errorf("expected selection clamped at %d, got %d", len(rl.filtered)-1, rl.selected)
	}
}

func TestRunListJumpBottom(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)

	rl, _ = rl.Update(keyMsg("G"))
	if rl.selected != len(rl.filtered)-1 {
		t.Errorf("expected selection at last, got %d", rl.selected)
	}
}

func TestRunListJumpTop(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)

	rl, _ = rl.Update(keyMsg("G"))
	rl, _ = rl.Update(keyMsg("g"))
	rl, _ = rl.Update(keyMsg("g"))
	if rl.selected != 0 {
		t.Errorf("expected selection at 0 after gg, got %d", rl.selected)
	}
}

func TestRunListView(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)
	view := rl.View()

	if !strings.Contains(view, "Runs") {
		t.Error("expected view to contain 'Runs' title")
	}
	if !strings.Contains(view, "ID") || !strings.Contains(view, "STATE") {
		t.Error("expected view to contain column headers")
	}
	if !strings.Contains(view, "running") || !strings.Contains(view, "paused") {
		t.Error("expected view to contain state labels")
	}
}

func TestRunListSelectedRun(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)

	r := rl.SelectedRun()
	if r == nil {
		t.Fatal("expected non-nil selected run")
	}
	if r.ID != "004" {
		t.Errorf("expected run 004 (newest), got %s", r.ID)
	}

	rl, _ = rl.Update(keyMsg("j"))
	r = rl.SelectedRun()
	if r.ID != "003" {
		t.Errorf("expected run 003, got %s", r.ID)
	}
}

func TestRunListEmpty(t *testing.T) {
	s := run.NewStore()
	rl := NewRunList(s)
	rl.SetSize(60, 20)
	view := rl.View()
	if !strings.Contains(view, "No runs") {
		t.Error("expected empty list message")
	}
	if rl.SelectedRun() != nil {
		t.Error("expected nil for empty list SelectedRun")
	}
}

func TestRunListStoreUpdate(t *testing.T) {
	s := testStore()
	rl := NewRunList(s)
	rl.SetSize(60, 20)

	s.Add(&run.Run{Branch: "feat/new-run", Workflow: "build", State: run.StateRunning})
	rl, _ = rl.Update(RunStoreUpdatedMsg{})

	if len(rl.filtered) != 5 {
		t.Errorf("expected 5 filtered runs, got %d", len(rl.filtered))
	}
}

func TestRunListFilter(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)

	rl, _ = rl.Update(keyMsg("/"))
	if !rl.filterActive {
		t.Fatal("expected filter to be active")
	}

	for _, c := range "feat" {
		rl, _ = rl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
	}

	if len(rl.filtered) != 2 {
		t.Errorf("expected 2 runs matching 'feat', got %d", len(rl.filtered))
	}
}

func TestRunListFilterClear(t *testing.T) {
	s := testStore()
	rl := NewRunList(s)
	rl.SetSize(60, 20)
	totalRuns := len(rl.filtered)

	rl, _ = rl.Update(keyMsg("/"))
	for _, c := range "feat" {
		rl, _ = rl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
	}

	rl, _ = rl.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if rl.filterActive {
		t.Error("expected filter deactivated after Esc")
	}
	if len(rl.filtered) != totalRuns {
		t.Errorf("expected all %d runs after Esc, got %d", totalRuns, len(rl.filtered))
	}
}

func TestRunListIconRendering(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)
	view := rl.View()

	// Running run should have ● icon
	if !strings.Contains(view, "●") {
		t.Error("expected ● icon for running run")
	}
	// Failed run should have ✗ icon
	if !strings.Contains(view, "✗") {
		t.Error("expected ✗ icon for failed run")
	}
}

func TestRunListScrolling(t *testing.T) {
	s := run.NewStore()
	for i := 0; i < 20; i++ {
		s.Add(&run.Run{Branch: "branch", Workflow: "build", State: run.StateRunning})
	}
	rl := NewRunList(s)
	rl.SetSize(60, 8) // Small height to force scrolling

	for i := 0; i < 10; i++ {
		rl, _ = rl.Update(keyMsg("j"))
	}

	if rl.selected != 10 {
		t.Errorf("expected selection 10, got %d", rl.selected)
	}
	if rl.offset == 0 {
		t.Error("expected offset to be non-zero after scrolling down")
	}
}
