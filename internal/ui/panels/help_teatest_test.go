package panels

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func TestHelpOverlaySnapshot(t *testing.T) {
	h := NewHelpOverlay()

	tm := teatest.NewTestModel(t, wrapHelpOverlay(h), teatest.WithInitialTermSize(50, 30))
	waitForContains(t, tm, "Keybinds")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(h.View()))
}
