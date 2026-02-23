package runtime

import (
	"context"
	"io"
	"os"
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
	Agent          string
	StdoutFile     *os.File // If set, redirect process stdout to this file instead of a pipe
	StderrFile     *os.File // If set, redirect process stderr to this file instead of a pipe
}

type Process struct {
	PID        int
	Cmd        *exec.Cmd
	Stdout     io.ReadCloser
	Stderr     io.ReadCloser
	Done       <-chan error
	StdoutPath string // Log file path (set when using file-based output)
	StderrPath string // Log file path (set when using file-based output)
}
