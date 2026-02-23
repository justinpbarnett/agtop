package panels

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// panelAdapter wraps panel types that use typed Update signatures into
// a proper tea.Model so they can be used with teatest.
type panelAdapter struct {
	view     func() string
	updateFn func(tea.Msg) tea.Cmd
}

func (a panelAdapter) Init() tea.Cmd                           { return nil }
func (a panelAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) { return a, a.updateFn(msg) }
func (a panelAdapter) View() string                            { return a.view() }

// wrapRunList creates a tea.Model adapter around a RunList for teatest use.
func wrapRunList(rl *RunList) tea.Model {
	return panelAdapter{
		view: func() string { return rl.View() },
		updateFn: func(msg tea.Msg) tea.Cmd {
			newRL, cmd := rl.Update(msg)
			*rl = newRL
			return cmd
		},
	}
}

// wrapDetail creates a tea.Model adapter around a Detail for teatest use.
func wrapDetail(d *Detail) tea.Model {
	return panelAdapter{
		view: func() string { return d.View() },
		updateFn: func(msg tea.Msg) tea.Cmd {
			newD, cmd := d.Update(msg)
			*d = newD
			return cmd
		},
	}
}

// wrapLogView creates a tea.Model adapter around a LogView for teatest use.
func wrapLogView(lv *LogView) tea.Model {
	return panelAdapter{
		view: func() string { return lv.View() },
		updateFn: func(msg tea.Msg) tea.Cmd {
			newLV, cmd := lv.Update(msg)
			*lv = newLV
			return cmd
		},
	}
}

// wrapDiffView creates a tea.Model adapter around a DiffView for teatest use.
func wrapDiffView(dv *DiffView) tea.Model {
	return panelAdapter{
		view: func() string { return dv.Content() },
		updateFn: func(msg tea.Msg) tea.Cmd {
			newDV, cmd := dv.Update(msg)
			*dv = newDV
			return cmd
		},
	}
}

// wrapStatusBar creates a tea.Model adapter around a StatusBar for teatest use.
// StatusBar has no Update method, so the adapter uses a no-op.
func wrapStatusBar(sb *StatusBar) tea.Model {
	return panelAdapter{
		view:     func() string { return sb.View() },
		updateFn: func(tea.Msg) tea.Cmd { return nil },
	}
}

// wrapHelpOverlay creates a tea.Model adapter around a HelpOverlay for teatest use.
func wrapHelpOverlay(h *HelpOverlay) tea.Model {
	return panelAdapter{
		view: func() string { return h.View() },
		updateFn: func(msg tea.Msg) tea.Cmd {
			newH, cmd := h.Update(msg)
			*h = newH
			return cmd
		},
	}
}

// waitDuration is the standard timeout for WaitFor calls in tests.
const waitDuration = 3 * time.Second

// waitForContains waits until the output contains the given substring.
func waitForContains(tb testing.TB, tm *teatest.TestModel, substr string) {
	tb.Helper()
	teatest.WaitFor(
		tb,
		tm.Output(),
		func(bts []byte) bool { return contains(bts, substr) },
		teatest.WithDuration(waitDuration),
	)
}

func contains(bts []byte, s string) bool {
	return len(s) > 0 && len(bts) >= len(s) && bytesContains(bts, []byte(s))
}

func bytesContains(haystack, needle []byte) bool {
	for i := 0; i <= len(haystack)-len(needle); i++ {
		found := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}
