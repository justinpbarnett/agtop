package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type ClaudeRuntime struct {
	claudePath string
}

func NewClaudeRuntime() (*ClaudeRuntime, error) {
	path, err := exec.LookPath("claude")
	if err != nil {
		return nil, fmt.Errorf("claude binary not found in PATH — install from https://docs.anthropic.com/en/docs/claude-code")
	}
	return &ClaudeRuntime{claudePath: path}, nil
}

func (c *ClaudeRuntime) BuildArgs(prompt string, opts RunOptions) []string {
	args := []string{
		"-p", prompt,
		"--output-format", "stream-json",
		"--verbose",
	}
	if opts.Model != "" {
		args = append(args, "--model", opts.Model)
	}
	if opts.MaxTurns > 0 {
		args = append(args, "--max-turns", strconv.Itoa(opts.MaxTurns))
	}
	if len(opts.AllowedTools) > 0 {
		args = append(args, "--allowedTools", strings.Join(opts.AllowedTools, ","))
	}
	if opts.PermissionMode != "" {
		args = append(args, "--permission-mode", opts.PermissionMode)
	}
	return args
}

func (c *ClaudeRuntime) Start(ctx context.Context, prompt string, opts RunOptions) (*Process, error) {
	args := c.BuildArgs(prompt, opts)
	cmd := exec.CommandContext(ctx, c.claudePath, args...)
	if opts.WorkDir != "" {
		cmd.Dir = opts.WorkDir
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	doneCh := make(chan error, 1)
	go func() {
		doneCh <- cmd.Wait()
	}()

	return &Process{
		PID:    cmd.Process.Pid,
		Cmd:    cmd,
		Stdout: stdout,
		Stderr: stderr,
		Done:   doneCh,
	}, nil
}

func (c *ClaudeRuntime) Stop(proc *Process) error {
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

func (c *ClaudeRuntime) Pause(proc *Process) error {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return nil
	}
	return proc.Cmd.Process.Signal(syscall.SIGSTOP)
}

func (c *ClaudeRuntime) Resume(proc *Process) error {
	if proc.Cmd == nil || proc.Cmd.Process == nil {
		return nil
	}
	return proc.Cmd.Process.Signal(syscall.SIGCONT)
}
