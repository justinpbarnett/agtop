package run

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
	ID       string
	Branch   string
	Worktree string
	Workflow string
	State    State
	Tokens   int
	Cost     float64
}
