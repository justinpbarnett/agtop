package panels

import (
	"strings"
	"testing"
	"time"

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
		SkillIndex:   3,
		SkillTotal:   6,
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
	if !strings.Contains(view, "sdlc") {
		t.Error("expected details to show workflow name")
	}
	if !strings.Contains(view, "3/6") {
		t.Error("expected details to show step progress")
	}
	if !strings.Contains(view, "$0.15") {
		t.Error("expected details to show cost")
	}
	if !strings.Contains(view, "claude-sonnet-4-5") {
		t.Error("expected details to show model name")
	}
	if !strings.Contains(view, "5.0k") {
		t.Error("expected details to show total tokens")
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

func TestDetailTerminalElapsedTime(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)

	now := time.Now()
	r := &run.Run{
		ID:          "003",
		Branch:      "test",
		State:       run.StateCompleted,
		StartedAt:   now.Add(-5 * time.Minute),
		CompletedAt: now.Add(-2 * time.Minute),
	}
	d.SetRun(r)
	view := d.View()

	if !strings.Contains(view, "3m") {
		t.Error("expected completed run to show elapsed duration")
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

func TestDetailMergeStatus(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)

	r := &run.Run{
		ID:          "010",
		Branch:      "feat/merge",
		State:       run.StateMerging,
		MergeStatus: "rebasing",
	}
	d.SetRun(r)
	view := d.View()

	if !strings.Contains(view, "rebasing") {
		t.Error("expected merge status 'rebasing' to be displayed")
	}
}

func TestDetailMergeStatusMerged(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)

	r := &run.Run{
		ID:          "011",
		Branch:      "feat/merged",
		State:       run.StateAccepted,
		MergeStatus: "merged",
	}
	d.SetRun(r)
	view := d.View()

	if !strings.Contains(view, "merged") {
		t.Error("expected merge status 'merged' to be displayed")
	}
}

func TestDetailPRURL(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)

	r := &run.Run{
		ID:     "012",
		Branch: "feat/pr",
		State:  run.StateRunning,
		PRURL:  "https://github.com/org/repo/pull/42",
	}
	d.SetRun(r)
	view := d.View()

	if !strings.Contains(view, "https://github.com/org/repo/pull/42") {
		t.Error("expected PR URL to be displayed")
	}
}

func TestDetailNoMergeStatusWhenEmpty(t *testing.T) {
	d := NewDetail()
	d.SetSize(80, 15)

	r := &run.Run{
		ID:          "013",
		Branch:      "feat/nomerge",
		State:       run.StateRunning,
		MergeStatus: "",
	}
	d.SetRun(r)
	view := d.View()

	if strings.Contains(view, "Merge") {
		t.Error("expected no Merge row when MergeStatus is empty")
	}
}
