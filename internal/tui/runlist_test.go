package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func keyMsg(s string) tea.Msg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func TestRunListNavigation(t *testing.T) {
	rl := NewRunList()

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
	rl := NewRunList()

	// Can't go below 0
	rl, _ = rl.Update(keyMsg("k"))
	if rl.selected != 0 {
		t.Errorf("expected selection clamped at 0, got %d", rl.selected)
	}

	// Can't go above len-1
	for i := 0; i < 10; i++ {
		rl, _ = rl.Update(keyMsg("j"))
	}
	if rl.selected != len(rl.runs)-1 {
		t.Errorf("expected selection clamped at %d, got %d", len(rl.runs)-1, rl.selected)
	}
}

func TestRunListJumpBottom(t *testing.T) {
	rl := NewRunList()

	rl, _ = rl.Update(keyMsg("G"))
	if rl.selected != len(rl.runs)-1 {
		t.Errorf("expected selection at last, got %d", rl.selected)
	}
}

func TestRunListJumpTop(t *testing.T) {
	rl := NewRunList()

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
	rl := NewRunList()
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
	rl := NewRunList()

	r := rl.SelectedRun()
	if r == nil {
		t.Fatal("expected non-nil selected run")
	}
	if r.ID != "001" {
		t.Errorf("expected run 001, got %s", r.ID)
	}

	rl, _ = rl.Update(keyMsg("j"))
	r = rl.SelectedRun()
	if r.ID != "002" {
		t.Errorf("expected run 002, got %s", r.ID)
	}
}

func TestRunListEmpty(t *testing.T) {
	rl := RunList{}
	view := rl.View()
	if !strings.Contains(view, "No runs") {
		t.Error("expected empty list message")
	}
	if rl.SelectedRun() != nil {
		t.Error("expected nil for empty list SelectedRun")
	}
}
