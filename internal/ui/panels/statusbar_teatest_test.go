package panels

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/justinpbarnett/agtop/internal/run"
)

func TestStatusBarSnapshot(t *testing.T) {
	s := testStore() // 4 runs with various states and costs
	sb := NewStatusBar(s)
	sb.SetSize(120)

	tm := teatest.NewTestModel(t, wrapStatusBar(&sb), teatest.WithInitialTermSize(120, 1))
	waitForContains(t, tm, "agtop")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(sb.View()))
}

func TestStatusBarFlashSnapshot(t *testing.T) {
	s := run.NewStore()
	sb := NewStatusBar(s)
	sb.SetSize(120)
	sb.SetFlash("Operation completed successfully")

	tm := teatest.NewTestModel(t, wrapStatusBar(&sb), teatest.WithInitialTermSize(120, 1))
	waitForContains(t, tm, "Operation completed successfully")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(sb.View()))
}
