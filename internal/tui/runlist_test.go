package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/run"
)

func testStore() *run.Store {
	s := run.NewStore()
	s.Add(&run.Run{Branch: "feat/add-auth", Workflow: "sdlc", State: run.StateRunning, SkillIndex: 3, SkillTotal: 7, Tokens: 12400, Cost: 0.42})
	s.Add(&run.Run{Branch: "fix/nav-bug", Workflow: "quick-fix", State: run.StatePaused, SkillIndex: 1, SkillTotal: 3, Tokens: 3100, Cost: 0.08})
	s.Add(&run.Run{Branch: "feat/dashboard", Workflow: "plan-build", State: run.StateReviewing, SkillIndex: 3, SkillTotal: 3, Tokens: 45200, Cost: 1.23})
	s.Add(&run.Run{Branch: "fix/css-overflow", Workflow: "build", State: run.StateFailed, SkillIndex: 2, SkillTotal: 3, Tokens: 8700, Cost: 0.31})
	return s
}

func keyMsg(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestRunListNavigation(t *testing.T) {
	rl := NewRunList(testStore())

	if rl.selected != 0 {
		t.Errorf("expected initial selection 0, got %d", rl.selected)
	}

	rl, _ = rl.Update(keyMsg("j"))
	if rl.selected != 1 {
		t.Errorf("expected selection 1 after j, got %d", rl.selected)
	}

	rl, _ = rl.Update(keyMsg("j"))
	if rl.selected != 2 {
		t.Errorf("expected selection 2 after second j, got %d", rl.selected)
	}

	rl, _ = rl.Update(keyMsg("k"))
	if rl.selected != 1 {
		t.Errorf("expected selection 1 after k, got %d", rl.selected)
	}
}

func TestRunListBounds(t *testing.T) {
	rl := NewRunList(testStore())

	// Can't go below 0
	rl, _ = rl.Update(keyMsg("k"))
	if rl.selected != 0 {
		t.Errorf("expected selection clamped at 0, got %d", rl.selected)
	}

	// Can't go above len-1
	for i := 0; i < 10; i++ {
		rl, _ = rl.Update(keyMsg("j"))
	}
	if rl.selected != len(rl.filtered)-1 {
		t.Errorf("expected selection clamped at %d, got %d", len(rl.filtered)-1, rl.selected)
	}
}

func TestRunListJumpBottom(t *testing.T) {
	rl := NewRunList(testStore())

	rl, _ = rl.Update(keyMsg("G"))
	if rl.selected != len(rl.filtered)-1 {
		t.Errorf("expected selection at last, got %d", rl.selected)
	}
}

func TestRunListJumpTop(t *testing.T) {
	rl := NewRunList(testStore())

	// Go to bottom first
	rl, _ = rl.Update(keyMsg("G"))

	// Double-g for top
	rl, _ = rl.Update(keyMsg("g"))
	rl, _ = rl.Update(keyMsg("g"))
	if rl.selected != 0 {
		t.Errorf("expected selection at 0 after gg, got %d", rl.selected)
	}
}

func TestRunListView(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(80, 10)
	view := rl.View()

	if !strings.Contains(view, "#001") {
		t.Error("expected view to contain run #001")
	}
	if !strings.Contains(view, "feat/add-auth") {
		t.Error("expected view to contain branch name")
	}
	if !strings.Contains(view, "sdlc") {
		t.Error("expected view to contain workflow name")
	}
}

func TestRunListSelectedRun(t *testing.T) {
	rl := NewRunList(testStore())

	r := rl.SelectedRun()
	if r == nil {
		t.Fatal("expected non-nil selected run")
	}
	// Store prepends, so newest is first. Run #004 was added last.
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
	rl.SetSize(80, 20)

	// Add a new run
	s.Add(&run.Run{Branch: "feat/new-run", Workflow: "build", State: run.StateRunning})

	// Simulate the RunStoreUpdatedMsg
	rl, _ = rl.Update(RunStoreUpdatedMsg{})

	view := rl.View()
	if !strings.Contains(view, "feat/new-run") {
		t.Error("expected new run to appear in view after store update")
	}
	if len(rl.filtered) != 5 {
		t.Errorf("expected 5 filtered runs, got %d", len(rl.filtered))
	}
}

func TestRunListFilter(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(80, 20)

	// Activate filter
	rl, _ = rl.Update(keyMsg("/"))
	if !rl.filterActive {
		t.Fatal("expected filter to be active")
	}

	// Type "feat"
	for _, c := range "feat" {
		rl, _ = rl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
	}

	// Should only show runs with "feat" in branch
	count := 0
	for _, r := range rl.filtered {
		if strings.Contains(strings.ToLower(r.Branch), "feat") {
			count++
		}
	}
	if count != len(rl.filtered) {
		t.Errorf("expected all filtered runs to contain 'feat', got %d/%d", count, len(rl.filtered))
	}
	if len(rl.filtered) != 2 {
		t.Errorf("expected 2 runs matching 'feat', got %d", len(rl.filtered))
	}
}

func TestRunListFilterClear(t *testing.T) {
	s := testStore()
	rl := NewRunList(s)
	rl.SetSize(80, 20)
	totalRuns := len(rl.filtered)

	// Activate filter and type
	rl, _ = rl.Update(keyMsg("/"))
	for _, c := range "feat" {
		rl, _ = rl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
	}

	if len(rl.filtered) == totalRuns {
		t.Error("expected filter to reduce results")
	}

	// Press Esc to clear filter
	rl, _ = rl.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if rl.filterActive {
		t.Error("expected filter to be deactivated after Esc")
	}
	if len(rl.filtered) != totalRuns {
		t.Errorf("expected all %d runs after Esc, got %d", totalRuns, len(rl.filtered))
	}
}

func TestRunListScrolling(t *testing.T) {
	s := run.NewStore()
	for i := 0; i < 20; i++ {
		s.Add(&run.Run{Branch: "branch", Workflow: "build", State: run.StateRunning})
	}
	rl := NewRunList(s)
	rl.SetSize(80, 5)

	// Navigate down past visible area
	for i := 0; i < 10; i++ {
		rl, _ = rl.Update(keyMsg("j"))
	}

	if rl.selected != 10 {
		t.Errorf("expected selection 10, got %d", rl.selected)
	}
	// Offset should have adjusted to keep selection visible
	if rl.offset == 0 {
		t.Error("expected offset to be non-zero after scrolling down")
	}
}

func TestRunListScrollIndicators(t *testing.T) {
	s := run.NewStore()
	for i := 0; i < 20; i++ {
		s.Add(&run.Run{Branch: "branch", Workflow: "build", State: run.StateRunning})
	}
	rl := NewRunList(s)
	rl.SetSize(80, 5)

	// At top, should see bottom indicator
	view := rl.View()
	if !strings.Contains(view, "▼") {
		t.Error("expected ▼ indicator when more content below")
	}

	// Scroll to middle
	for i := 0; i < 10; i++ {
		rl, _ = rl.Update(keyMsg("j"))
	}
	view = rl.View()
	if !strings.Contains(view, "▲") {
		t.Error("expected ▲ indicator when content above")
	}
	if !strings.Contains(view, "▼") {
		t.Error("expected ▼ indicator when content below")
	}
}

func TestRunListFilterNoMatch(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(80, 20)

	rl, _ = rl.Update(keyMsg("/"))
	for _, c := range "zzzzz" {
		rl, _ = rl.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
	}

	view := rl.View()
	if !strings.Contains(view, "No matching runs") {
		t.Error("expected 'No matching runs' message")
	}
	if rl.SelectedRun() != nil {
		t.Error("expected nil SelectedRun when filter matches nothing")
	}
}
