package runtime

import (
	"context"
	"io"
	"os/exec"
)

type Runtime interface {
	Start(ctx context.Context, prompt string, opts RunOptions) (*Process, error)
	Stop(proc *Process) error
	Pause(proc *Process) error
	Resume(proc *Process) error
}

type RunOptions struct {
	Model          string
	WorkDir        string
	AllowedTools   []string
	MaxTurns       int
	PermissionMode string
}

type Process struct {
	PID    int
	Cmd    *exec.Cmd
	Stdout io.ReadCloser
	Stderr io.ReadCloser
	Done   <-chan error
}
