package panels

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// newRunAdapter wraps the pointer-receiver NewRunModal into a tea.Model.
// NewRunModal.Update returns (*NewRunModal, tea.Cmd) where nil means dismiss.
type newRunAdapter struct {
	modal     *NewRunModal
	dismissed bool
	lastCmd   tea.Cmd
}

func (a *newRunAdapter) Init() tea.Cmd {
	if a.modal != nil {
		return a.modal.Init()
	}
	return nil
}

func (a *newRunAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.modal == nil {
		return a, nil
	}
	newModal, cmd := a.modal.Update(msg)
	if newModal == nil {
		a.dismissed = true
		a.lastCmd = cmd
		a.modal = nil
	} else {
		a.modal = newModal
	}
	return a, cmd
}

func (a *newRunAdapter) View() string {
	if a.modal == nil {
		return "(dismissed)"
	}
	return a.modal.View()
}

func TestNewRunModalDefaultSnapshot(t *testing.T) {
	modal := NewNewRunModal(120, 40)
	adapter := &newRunAdapter{modal: modal}

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	waitForContains(t, tm, "New Run")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(modal.View()))
}

func TestNewRunModalWorkflowSwitching(t *testing.T) {
	modal := NewNewRunModal(120, 40)
	adapter := &newRunAdapter{modal: modal}

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	waitForContains(t, tm, "New Run")

	// Default should be "auto"
	if modal.Workflow() != "auto" {
		t.Errorf("expected default workflow 'auto', got %q", modal.Workflow())
	}

	// Switch to build
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlB})
	time.Sleep(100 * time.Millisecond)

	if adapter.modal != nil && adapter.modal.Workflow() != "build" {
		t.Errorf("expected workflow 'build' after Ctrl+B, got %q", adapter.modal.Workflow())
	}

	// Switch to plan-build
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlP})
	time.Sleep(100 * time.Millisecond)

	if adapter.modal != nil && adapter.modal.Workflow() != "plan-build" {
		t.Errorf("expected workflow 'plan-build' after Ctrl+P, got %q", adapter.modal.Workflow())
	}

	// Switch to sdlc
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlL})
	time.Sleep(100 * time.Millisecond)

	if adapter.modal != nil && adapter.modal.Workflow() != "sdlc" {
		t.Errorf("expected workflow 'sdlc' after Ctrl+L, got %q", adapter.modal.Workflow())
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestNewRunModalSubmitFlow(t *testing.T) {
	modal := NewNewRunModal(120, 40)
	adapter := &newRunAdapter{modal: modal}

	tm := teatest.NewTestModel(t, adapter, teatest.WithInitialTermSize(120, 40))
	waitForContains(t, tm, "New Run")

	// Type a prompt
	tm.Type("Implement user authentication")
	time.Sleep(100 * time.Millisecond)

	// Submit with Ctrl+S
	tm.Send(tea.KeyMsg{Type: tea.KeyCtrlS})
	time.Sleep(100 * time.Millisecond)

	if !adapter.dismissed {
		t.Error("expected modal to be dismissed after Ctrl+S submit")
	}
	if adapter.lastCmd == nil {
		t.Fatal("expected a command from submit")
	}
	msg := adapter.lastCmd()
	if submit, ok := msg.(SubmitNewRunMsg); ok {
		if submit.Prompt != "Implement user authentication" {
			t.Errorf("expected prompt 'Implement user authentication', got %q", submit.Prompt)
		}
		if submit.Workflow != "auto" {
			t.Errorf("expected workflow 'auto', got %q", submit.Workflow)
		}
	} else {
		t.Errorf("expected SubmitNewRunMsg, got %T", msg)
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}
