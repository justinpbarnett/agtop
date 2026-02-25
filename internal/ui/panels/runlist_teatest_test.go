package panels

import (
	"fmt"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/justinpbarnett/agtop/internal/run"
)

func TestRunListSnapshot(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)
	rl.SetFocused(true)

	tm := teatest.NewTestModel(t, wrapRunList(&rl), teatest.WithInitialTermSize(60, 20))
	waitForContains(t, tm, "Runs")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(rl.View()))
}

func TestRunListEmptySnapshot(t *testing.T) {
	s := run.NewStore()
	rl := NewRunList(s)
	rl.SetSize(60, 20)
	rl.SetFocused(true)

	tm := teatest.NewTestModel(t, wrapRunList(&rl), teatest.WithInitialTermSize(60, 20))
	waitForContains(t, tm, "No runs")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(rl.View()))
}

func TestRunListNavigationFlow(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)
	rl.SetFocused(true)

	tm := teatest.NewTestModel(t, wrapRunList(&rl), teatest.WithInitialTermSize(60, 20))
	waitForContains(t, tm, "Runs")

	// Navigate down twice then back up
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})

	// Give time for updates to process
	time.Sleep(100 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	fm := tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))

	// The adapter holds a closure over rl, verify via rl
	_ = fm
	if rl.selected != 1 {
		t.Errorf("expected selection 1 after j/j/k, got %d", rl.selected)
	}
}

func TestRunListFilterFlow(t *testing.T) {
	rl := NewRunList(testStore())
	rl.SetSize(60, 20)
	rl.SetFocused(true)

	tm := teatest.NewTestModel(t, wrapRunList(&rl), teatest.WithInitialTermSize(60, 20))
	waitForContains(t, tm, "Runs")

	// Activate filter
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	time.Sleep(50 * time.Millisecond)

	// Type "feat"
	for _, c := range "feat" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
	}
	time.Sleep(100 * time.Millisecond)

	if len(rl.filtered) != 2 {
		t.Errorf("expected 2 filtered runs for 'feat', got %d", len(rl.filtered))
	}

	// Dismiss filter with Esc — should restore all runs
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(100 * time.Millisecond)

	if rl.filterActive {
		t.Error("expected filter deactivated after Esc")
	}
	if len(rl.filtered) != 4 {
		t.Errorf("expected 4 runs after Esc, got %d", len(rl.filtered))
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestRunListScrollSnapshot(t *testing.T) {
	s := run.NewStore()
	for i := 0; i < 20; i++ {
		s.Add(&run.Run{ID: fmt.Sprintf("scroll%02d", i), Branch: "branch", Workflow: "build", State: run.StateRunning})
	}
	rl := NewRunList(s)
	rl.SetSize(60, 12)
	rl.SetFocused(true)

	tm := teatest.NewTestModel(t, wrapRunList(&rl), teatest.WithInitialTermSize(60, 12))
	waitForContains(t, tm, "Runs")

	// Navigate down 10 times to trigger scrolling
	for i := 0; i < 10; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	time.Sleep(100 * time.Millisecond)

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(rl.View()))
}
