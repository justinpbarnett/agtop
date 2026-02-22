package run

import (
	"time"

	"github.com/justinpbarnett/agtop/internal/cost"
)

type State string

const (
	StateQueued    State = "queued"
	StateRouting   State = "routing"
	StateRunning   State = "running"
	StatePaused    State = "paused"
	StateCompleted State = "completed"
	StateReviewing State = "reviewing"
	StateAccepted  State = "accepted"
	StateRejected  State = "rejected"
	StateFailed    State = "failed"
)

type Run struct {
	ID           string
	Branch       string
	Worktree     string
	Workflow     string
	State        State
	SkillIndex   int
	SkillTotal   int
	Tokens       int
	TokensIn     int
	TokensOut    int
	Cost         float64
	CreatedAt    time.Time
	StartedAt    time.Time
	CurrentSkill string
	Model        string
	Command      string
	Error        string
	PID          int
	SkillCosts   []cost.SkillCost
}

func (r *Run) StatusIcon() string {
	switch r.State {
	case StateRunning, StateRouting:
		return "●"
	case StatePaused:
		return "◐"
	case StateCompleted, StateAccepted:
		return "✓"
	case StateFailed, StateRejected:
		return "✗"
	case StateReviewing:
		return "◉"
	case StateQueued:
		return "◌"
	default:
		return "·"
	}
}

func (r *Run) IsTerminal() bool {
	switch r.State {
	case StateCompleted, StateAccepted, StateRejected, StateFailed:
		return true
	}
	return false
}

func (r *Run) ElapsedTime() time.Duration {
	if r.StartedAt.IsZero() {
		return 0
	}
	return time.Since(r.StartedAt)
}
