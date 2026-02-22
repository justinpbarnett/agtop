package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/run"
)

func TestDetailTabCycle(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 30)

	if d.activeTab != 0 {
		t.Errorf("expected initial tab 0, got %d", d.activeTab)
	}

	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if d.activeTab != 1 {
		t.Errorf("expected tab 1 after l, got %d", d.activeTab)
	}

	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if d.activeTab != 2 {
		t.Errorf("expected tab 2 after second l, got %d", d.activeTab)
	}

	// Wrap around forward
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	if d.activeTab != 0 {
		t.Errorf("expected tab 0 after wrap, got %d", d.activeTab)
	}

	// Wrap around backward
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if d.activeTab != 2 {
		t.Errorf("expected tab 2 after backward wrap, got %d", d.activeTab)
	}
}

func TestDetailNoRun(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 30)
	view := d.View()

	if !strings.Contains(view, "No run selected") {
		t.Error("expected 'No run selected' when no run set")
	}
}

func TestDetailSetRun(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 30)

	r := &run.Run{
		ID:       "042",
		Branch:   "feat/test",
		Workflow: "sdlc",
		State:    run.StateRunning,
		Tokens:   5000,
		Cost:     0.15,
	}
	d.SetRun(r)

	view := d.View()
	if !strings.Contains(view, "#042") {
		t.Error("expected details to show run ID")
	}
	if !strings.Contains(view, "feat/test") {
		t.Error("expected details to show branch")
	}
	if !strings.Contains(view, "sdlc") {
		t.Error("expected details to show workflow")
	}
}

func TestDetailTabContent(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 30)
	r := &run.Run{ID: "001", State: run.StateRunning}
	d.SetRun(r)

	// Tab 0 = Details
	view := d.View()
	if !strings.Contains(view, "#001") {
		t.Error("expected details tab to show run info")
	}

	// Tab 1 = Logs
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	view = d.View()
	if !strings.Contains(view, "build") {
		t.Error("expected logs tab to show log content")
	}

	// Tab 2 = Diff
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	view = d.View()
	if !strings.Contains(view, "diff") {
		t.Error("expected diff tab to show diff content")
	}
}
