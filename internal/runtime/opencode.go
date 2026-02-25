package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

type OpenCodeRuntime struct {
	opencodePath string
}

func NewOpenCodeRuntime() (*OpenCodeRuntime, error) {
	path, err := exec.LookPath("opencode")
	if err != nil {
		return nil, fmt.Errorf("opencode binary not found in PATH — install from https://opencode.ai")
	}
	return &OpenCodeRuntime{opencodePath: path}, nil
}

func (o *OpenCodeRuntime) BuildArgs(prompt string, opts RunOptions) []string {
	args := []string{
		"run", prompt,
		"--format", "json",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.Agent != "" {
		args = append(args, "--agent", opts.Agent)
	}
	return args
}

func (o *OpenCodeRuntime) Start(_ context.Context, prompt string, opts RunOptions) (*Process, error) {
	args := o.BuildArgs(prompt, opts)
	cmd := exec.Command(o.opencodePath, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	proc := &Process{Cmd: cmd}

	if opts.StdoutFile != nil {
		cmd.Stdout = opts.StdoutFile
		proc.StdoutPath = opts.StdoutFile.Name()
	} else {
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			return nil, fmt.Errorf("stdout pipe: %w", err)
		}
		proc.Stdout = stdout
	}

	if opts.StderrFile != nil {
		cmd.Stderr = opts.StderrFile
		proc.StderrPath = opts.StderrFile.Name()
	} else {
		stderr, err := cmd.StderrPipe()
		if err != nil {
			return nil, fmt.Errorf("stderr pipe: %w", err)
		}
		proc.Stderr = stderr
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	proc.PID = cmd.Process.Pid

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Wait()
	}()
	proc.Done = doneCh

	return proc, nil
}

func (o *OpenCodeRuntime) Stop(proc *Process) error {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return nil
	}
	_ = proc.Cmd.Process.Signal(syscall.SIGTERM)
	go func() {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-proc.Done:
		case <-timer.C:
			_ = proc.Cmd.Process.Signal(syscall.SIGKILL)
		}
	}()
	return nil
}

func (o *OpenCodeRuntime) Pause(proc *Process) error {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return nil
	}
	return proc.Cmd.Process.Signal(syscall.SIGSTOP)
}

func (o *OpenCodeRuntime) Resume(proc *Process) error {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return nil
	}
	return proc.Cmd.Process.Signal(syscall.SIGCONT)
}
