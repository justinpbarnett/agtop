package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/justinpbarnett/agtop/internal/run"
)

func TestAppInitialRenderSnapshot(t *testing.T) {
	adapter := newTestAppAdapter(t)

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))

	// Send WindowSizeMsg to trigger ready state
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	waitForContains(t, tm, "Runs")

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(adapter.View()))
}

func TestAppHelpModalFlow(t *testing.T) {
	adapter := newTestAppAdapter(t)

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	waitForContains(t, tm, "Runs")

	// Open help with ?
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	waitForContains(t, tm, "Keybinds")

	// Snapshot with help overlay open
	if adapter.app.helpOverlay == nil {
		t.Error("expected helpOverlay to be open")
	}

	// Close with Esc — this sends the key to the overlay, which returns CloseModalMsg.
	// We need to pump the command through.
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(200 * time.Millisecond)

	// The CloseModalMsg should have been processed
	if adapter.app.helpOverlay != nil {
		t.Error("expected helpOverlay to be closed after Esc")
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestAppNewRunModalFlow(t *testing.T) {
	adapter := newTestAppAdapter(t)

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	waitForContains(t, tm, "Runs")

	// Open new run modal with n
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	waitForContains(t, tm, "New Run")

	if adapter.app.newRunModal == nil {
		t.Error("expected newRunModal to be open")
	}

	// Dismiss with Esc
	tm.Send(tea.KeyMsg{Type: tea.KeyEsc})
	time.Sleep(200 * time.Millisecond)

	if adapter.app.newRunModal != nil {
		t.Error("expected newRunModal to be closed after Esc")
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestAppFocusCycleVisual(t *testing.T) {
	adapter := newTestAppAdapter(t)

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	waitForContains(t, tm, "Runs")

	// Initial focus is on panel 0 (run list)
	if adapter.app.focusedPanel != panelRunList {
		t.Errorf("expected initial focus on panelRunList, got %d", adapter.app.focusedPanel)
	}

	// Tab to logview
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	time.Sleep(100 * time.Millisecond)

	if adapter.app.focusedPanel != panelLogView {
		t.Errorf("expected focus on panelLogView after tab, got %d", adapter.app.focusedPanel)
	}

	// Tab to detail
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	time.Sleep(100 * time.Millisecond)

	if adapter.app.focusedPanel != panelDetail {
		t.Errorf("expected focus on panelDetail after second tab, got %d", adapter.app.focusedPanel)
	}

	// Tab wraps back to run list
	tm.Send(tea.KeyMsg{Type: tea.KeyTab})
	time.Sleep(100 * time.Millisecond)

	if adapter.app.focusedPanel != panelRunList {
		t.Errorf("expected focus wrapped to panelRunList, got %d", adapter.app.focusedPanel)
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestAppNavigationWithRuns(t *testing.T) {
	adapter := newTestAppAdapter(t)

	// Add some runs to the store
	adapter.app.store.Add(&run.Run{
		Branch:   "feat/first",
		Workflow: "build",
		State:    run.StateRunning,
		Tokens:   5000,
		Cost:     0.15,
	})
	adapter.app.store.Add(&run.Run{
		Branch:   "feat/second",
		Workflow: "sdlc",
		State:    run.StateCompleted,
		Tokens:   20000,
		Cost:     0.80,
	})

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	waitForContains(t, tm, "Runs")

	// Notify the UI that store changed
	tm.Send(RunStoreUpdatedMsg{})
	time.Sleep(100 * time.Millisecond)

	// The newest run (feat/second) should be selected first (run list shows newest first)
	selected := adapter.app.runList.SelectedRun()
	if selected == nil {
		t.Fatal("expected a selected run")
	}

	// Navigate down
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	time.Sleep(100 * time.Millisecond)

	// Detail panel should reflect the newly selected run
	newSelected := adapter.app.runList.SelectedRun()
	if newSelected == nil {
		t.Fatal("expected a selected run after j")
	}
	if selected.ID == newSelected.ID {
		t.Error("expected different run selected after j")
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestAppStoreUpdatePropagation(t *testing.T) {
	adapter := newTestAppAdapter(t)

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	tm.Send(tea.WindowSizeMsg{Width: 120, Height: 40})
	waitForContains(t, tm, "Runs")

	// Add a run
	adapter.app.store.Add(&run.Run{
		Branch:       "feat/new-feature",
		Workflow:     "build",
		State:        run.StateRunning,
		CurrentSkill: "build",
		Tokens:       1000,
		Cost:         0.05,
	})

	// Send store update
	tm.Send(RunStoreUpdatedMsg{})
	time.Sleep(200 * time.Millisecond)

	// Verify the run list picked it up
	selected := adapter.app.runList.SelectedRun()
	if selected == nil {
		t.Fatal("expected a selected run after store update")
	}
	if selected.Branch != "feat/new-feature" {
		t.Errorf("expected branch 'feat/new-feature', got %q", selected.Branch)
	}

	// Verify detail panel reflects the run by checking its view.
	// The branch line has 13 chars of prefix ("  Branch   : ") + the branch value.
	// At LeftColWidth=30, innerWidth=28, so only 15 chars of the branch value are
	// visible — check a prefix that fits within the panel width.
	detailView := adapter.app.detail.View()
	if !containsStr(detailView, "feat/new-featur") {
		t.Error("expected detail panel to show the new run's branch")
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}
