package panels

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
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
	if !strings.Contains(view, "claude-sonnet-4-5") {
		t.Error("expected details to show model name")
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

func TestDetailPromptScrollable(t *testing.T) {
	d := NewDetail()
	d.SetSize(40, 10)
	d.SetFocused(true)

	longPrompt := strings.Repeat("word ", 50) // 250 chars, will wrap to many lines
	r := &run.Run{
		ID:     "020",
		Branch: "feat/scroll",
		State:  run.StateRunning,
		Prompt: longPrompt,
		Cost:   0.10,
	}
	d.SetRun(r)

	// The initial view shows the beginning of the prompt
	view := d.View()
	if !strings.Contains(view, "Prompt") {
		t.Error("expected Prompt field to be visible at top")
	}

	// Scroll down enough to see fields below the prompt
	for i := 0; i < 20; i++ {
		d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	scrolledView := d.View()
	// After scrolling, Status or Cost should be visible
	if !strings.Contains(scrolledView, "Status") && !strings.Contains(scrolledView, "Cost") && !strings.Contains(scrolledView, "Branch") {
		t.Error("expected fields below prompt to be visible after scrolling down")
	}
}

func TestDetailScrollKeys(t *testing.T) {
	d := NewDetail()
	d.SetSize(40, 10)
	d.SetFocused(true)

	// Create a run with a long prompt that requires scrolling
	longPrompt := strings.Repeat("word ", 60)
	r := &run.Run{
		ID:     "021",
		Branch: "feat/scrollkeys",
		State:  run.StateRunning,
		Prompt: longPrompt,
		Cost:   0.10,
	}
	d.SetRun(r)

	// Scroll down with j
	initialOffset := d.viewport.YOffset
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if d.viewport.YOffset <= initialOffset {
		t.Error("expected j to scroll down")
	}

	// Scroll back up with k
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if d.viewport.YOffset != initialOffset {
		t.Errorf("expected k to scroll back to initial offset %d, got %d", initialOffset, d.viewport.YOffset)
	}

	// G should jump to bottom
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	if !d.viewport.AtBottom() {
		t.Error("expected G to jump to bottom")
	}

	// gg should jump to top
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	if d.viewport.YOffset != 0 {
		t.Errorf("expected gg to jump to top (offset 0), got %d", d.viewport.YOffset)
	}
}

func TestDetailScrollResetOnRunChange(t *testing.T) {
	d := NewDetail()
	d.SetSize(40, 10)
	d.SetFocused(true)

	longPrompt := strings.Repeat("word ", 60)
	r1 := &run.Run{
		ID:     "030",
		Branch: "feat/first",
		State:  run.StateRunning,
		Prompt: longPrompt,
	}
	d.SetRun(r1)

	// Scroll down
	for i := 0; i < 10; i++ {
		d, _ = d.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	if d.viewport.YOffset == 0 {
		t.Error("expected non-zero scroll after j key presses")
	}

	// Switch to a different run — scroll should reset to top
	r2 := &run.Run{
		ID:     "031",
		Branch: "feat/second",
		State:  run.StateRunning,
		Prompt: longPrompt,
	}
	d.SetRun(r2)

	if d.viewport.YOffset != 0 {
		t.Errorf("expected scroll to reset to 0 on run change, got %d", d.viewport.YOffset)
	}
}
