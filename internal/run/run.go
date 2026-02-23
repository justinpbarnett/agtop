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
	StateMerging   State = "merging"
)

type Run struct {
	ID           string        `json:"id"`
	Prompt       string        `json:"prompt"`
	TaskID       string        `json:"task_id,omitempty"`
	Branch       string        `json:"branch"`
	Worktree     string        `json:"worktree"`
	Workflow     string        `json:"workflow"`
	State        State         `json:"state"`
	SkillIndex   int           `json:"skill_index"`
	SkillTotal   int           `json:"skill_total"`
	Tokens       int           `json:"tokens"`
	TokensIn     int           `json:"tokens_in"`
	TokensOut    int           `json:"tokens_out"`
	Cost         float64       `json:"cost"`
	CreatedAt    time.Time     `json:"created_at"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  time.Time     `json:"completed_at"`
	CurrentSkill string        `json:"current_skill"`
	Model        string        `json:"model"`
	Command      string        `json:"command"`
	Error        string        `json:"error"`
	PID          int           `json:"pid"`
	SkillCosts   []cost.SkillCost `json:"skill_costs"`
	DevServerPort int          `json:"dev_server_port"`
	DevServerURL  string       `json:"dev_server_url"`
	MergeStatus   string       `json:"merge_status,omitempty"`
	PRURL         string       `json:"pr_url,omitempty"`
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
	case StateMerging:
		return "⟳"
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
	if !r.CompletedAt.IsZero() {
		return r.CompletedAt.Sub(r.StartedAt)
	}
	return time.Since(r.StartedAt)
}
