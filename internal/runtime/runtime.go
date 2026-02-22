package runtime

import "context"

type Runtime interface {
	Start(ctx context.Context, prompt string, opts RunOptions) (*Process, error)
	Stop(proc *Process) error
}

type RunOptions struct {
	Model        string
	WorkDir      string
	AllowedTools []string
	MaxTurns     int
}

type Process struct {
	PID    int
	Events chan Event
	Done   chan error
}

type Event struct {
	Type string
	Data string
}
