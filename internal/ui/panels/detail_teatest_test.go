package panels

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/justinpbarnett/agtop/internal/run"
)

func TestDetailWithRunSnapshot(t *testing.T) {
	d := NewDetail()
	d.SetSize(40, 20)
	d.SetFocused(true)
	d.SetRun(&run.Run{
		ID:           "001",
		Branch:       "feat/add-auth",
		Workflow:     "sdlc",
		State:        run.StateRunning,
		CurrentSkill: "build",
		Tokens:       12400,
		TokensIn:     8200,
		TokensOut:    4200,
		Cost:         0.42,
		Model:        "sonnet",
		Prompt:       "Add user authentication",
	})

	tm := teatest.NewTestModel(t, wrapDetail(&d), teatest.WithInitialTermSize(40, 20))
	waitForContains(t, tm, "Details")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(d.View()))
}

func TestDetailNoRunSnapshot(t *testing.T) {
	d := NewDetail()
	d.SetSize(40, 20)
	d.SetFocused(true)

	tm := teatest.NewTestModel(t, wrapDetail(&d), teatest.WithInitialTermSize(40, 20))
	waitForContains(t, tm, "No run selected")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(d.View()))
}

func TestDetailStateVariations(t *testing.T) {
	states := []run.State{
		run.StateRunning,
		run.StatePaused,
		run.StateReviewing,
		run.StateCompleted,
		run.StateFailed,
		run.StateRejected,
	}

	for _, state := range states {
		t.Run(string(state), func(t *testing.T) {
			d := NewDetail()
			d.SetSize(40, 20)
			d.SetFocused(true)
			r := &run.Run{
				ID:           "001",
				Branch:       "feat/test",
				Workflow:     "build",
				State:        state,
				CurrentSkill: "build",
				Tokens:       5000,
				Cost:         0.15,
			}
			if state == run.StateFailed {
				r.Error = "build skill timed out"
			}
			d.SetRun(r)

			tm := teatest.NewTestModel(t, wrapDetail(&d), teatest.WithInitialTermSize(40, 20))
			waitForContains(t, tm, "Details")
			tm.Send(tea.QuitMsg{})
			tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
			teatest.RequireEqualOutput(t, []byte(d.View()))
		})
	}
}
