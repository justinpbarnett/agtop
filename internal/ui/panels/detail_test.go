package panels

import (
	"strings"
	"testing"

	"github.com/justinpbarnett/agtop/internal/run"
)

func TestDetailNoRun(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)
	view := d.View()

	if !strings.Contains(view, "No run selected") {
		t.Error("expected 'No run selected' when no run set")
	}
}

func TestDetailSetRun(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)

	r := &run.Run{
		ID:           "042",
		Branch:       "feat/test",
		Workflow:     "sdlc",
		State:        run.StateRunning,
		Tokens:       5000,
		TokensIn:     3200,
		TokensOut:    1800,
		Cost:         0.15,
		CurrentSkill: "build",
		Model:        "claude-sonnet-4-5",
	}
	d.SetRun(r)

	view := d.View()
	if !strings.Contains(view, "feat/test") {
		t.Error("expected details to show branch")
	}
	if !strings.Contains(view, "build") {
		t.Error("expected details to show skill name")
	}
	if !strings.Contains(view, "$0.15") {
		t.Error("expected details to show cost")
	}
	if !strings.Contains(view, "claude-sonnet-4-5") {
		t.Error("expected details to show model name")
	}
	if !strings.Contains(view, "3.2k in") {
		t.Error("expected details to show token in/out format")
	}
}

func TestDetailBorder(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)
	view := d.View()

	if !strings.Contains(view, "Details") {
		t.Error("expected 'Details' title in border")
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
		t.Error("expected border characters")
	}
}

func TestDetailCostColoring(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)

	r := &run.Run{
		ID:     "001",
		Branch: "test",
		State:  run.StateRunning,
		Cost:   5.50,
	}
	d.SetRun(r)
	view := d.View()

	if !strings.Contains(view, "$5.50") {
		t.Error("expected cost to be displayed")
	}
}

func TestDetailNilRunHandling(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)
	d.SetRun(nil)

	view := d.View()
	if !strings.Contains(view, "No run selected") {
		t.Error("expected nil run handling")
	}
}
