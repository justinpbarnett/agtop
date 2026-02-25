package process

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/cost"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
)

// makeSkillMockRuntime creates a mock runtime that writes predetermined
// JSON events to stdout, including a result event with the given text.
func makeSkillMockRuntime(resultText string, doneCh chan error) *mockRuntime {
	pr, pw := io.Pipe()

	go func() {
		pw.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Working on task..."}]}}` + "\n"))
		pw.Write([]byte(`{"type":"result","result":"` + resultText + `","usage":{"input_tokens":100,"output_tokens":50},"total_cost_usd":0.01}` + "\n"))
		pw.Close()
	}()

	return &mockRuntime{
		startFn: func(_ context.Context, _ string, _ runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    12345,
				Stdout: pr,
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}
}

func TestStartSkillReturnsChannel(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := makeSkillMockRuntime("task completed", doneCh)
	mgr, store := testManager(rt)

	runID := store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "build"})

	ch, err := mgr.StartSkill(runID, "test prompt", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}

	// Signal process exit so consumeSkillEvents completes
	doneCh <- nil

	select {
	case result, ok := <-ch:
		if !ok {
			t.Fatal("channel closed without sending result")
		}
		if result.Err != nil {
			t.Errorf("expected no error, got %v", result.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result on channel")
	}
}

func TestStartSkillDoesNotSetTerminalState(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := makeSkillMockRuntime("done", doneCh)
	mgr, store := testManager(rt)

	runID := store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "build"})

	ch, err := mgr.StartSkill(runID, "test prompt", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start skill: %v", err)
	}

	// Signal clean exit
	doneCh <- nil

	// Wait for result
	select {
	case <-ch:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}

	// Give a moment for any async state updates
	time.Sleep(100 * time.Millisecond)

	r, ok := store.Get(runID)
	if !ok {
		t.Fatal("run not found in store")
	}
	if r.State == run.StateCompleted || r.State == run.StateFailed {
		t.Errorf("StartSkill should NOT set terminal state, but state is %s", r.State)
	}
}

func TestStartSkillDisconnectPreservesPID(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := makeSkillMockRuntime("done", doneCh)
	mgr, store := testManager(rt)

	runID := store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "build"})

	ch, err := mgr.StartSkill(runID, "test prompt", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start skill: %v", err)
	}

	// Mark manager as disconnecting BEFORE process exits
	mgr.SetDisconnecting()

	// Signal process exit
	doneCh <- nil

	select {
	case result := <-ch:
		if result.Err != ErrDisconnected {
			t.Errorf("expected ErrDisconnected, got %v", result.Err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}

	time.Sleep(100 * time.Millisecond)

	r, ok := store.Get(runID)
	if !ok {
		t.Fatal("run not found in store")
	}
	// PID should NOT be zeroed when disconnecting
	if r.PID == 0 {
		t.Error("PID should NOT be zeroed when disconnecting")
	}
	// State should still be running (not failed/completed)
	if r.State != run.StateRunning {
		t.Errorf("expected state Running (preserved), got %s", r.State)
	}
}

func TestStartSkillCapturesResultText(t *testing.T) {
	doneCh := make(chan error, 1)
	expectedText := "The feature has been implemented successfully"
	rt := makeSkillMockRuntime(expectedText, doneCh)
	mgr, store := testManager(rt)

	runID := store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "build"})

	ch, err := mgr.StartSkill(runID, "test prompt", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start skill: %v", err)
	}

	// Signal process exit
	doneCh <- nil

	select {
	case result := <-ch:
		if result.ResultText != expectedText {
			t.Errorf("expected result text %q, got %q", expectedText, result.ResultText)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for result")
	}
}

// TestStartSkillWithLogFilesCompletes verifies that StartSkill completes when
// using log files (FollowReader) instead of pipes. Before the fix, the
// FollowReader would poll on EOF forever after the process exited, causing
// a deadlock in consumeSkillEvents.
func TestStartSkillWithLogFilesCompletes(t *testing.T) {
	sessionsDir := t.TempDir()

	resultText := "build"
	doneCh := make(chan error, 1)

	// Mock runtime that writes events to the StdoutFile (log file) when provided.
	rt := &mockRuntime{
		startFn: func(_ context.Context, _ string, opts runtime.RunOptions) (*runtime.Process, error) {
			var stdout io.ReadCloser
			if opts.StdoutFile != nil {
				// Write events to the log file, as the real runtime would
				opts.StdoutFile.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Routing..."}]}}` + "\n"))
				opts.StdoutFile.Write([]byte(`{"type":"result","result":"` + resultText + `","usage":{"input_tokens":42,"output_tokens":100},"total_cost_usd":0.03}` + "\n"))
				opts.StdoutFile.Sync()
				// No pipe stdout — FollowReader will read the file
				stdout = io.NopCloser(strings.NewReader(""))
			} else {
				t.Fatal("expected StdoutFile to be set when sessionsDir is configured")
				return nil, nil
			}

			var stderr io.ReadCloser
			if opts.StderrFile != nil {
				stderr = io.NopCloser(strings.NewReader(""))
			} else {
				stderr = io.NopCloser(strings.NewReader(""))
			}

			return &runtime.Process{
				PID:    99999,
				Stdout: stdout,
				Stderr: stderr,
				Done:   doneCh,
			}, nil
		},
	}

	store := run.NewStore()
	cfg := &config.LimitsConfig{MaxConcurrentRuns: 5}
	tracker := cost.NewTracker()
	limiter := &cost.LimitChecker{}
	mgr := NewManager(store, rt, "claude", sessionsDir, cfg, tracker, limiter, nil)

	runID := store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "route"})

	ch, err := mgr.StartSkill(runID, "test prompt", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start skill: %v", err)
	}

	// Simulate process exit shortly after writing output
	go func() {
		time.Sleep(100 * time.Millisecond)
		doneCh <- nil
	}()

	select {
	case result := <-ch:
		if result.Err != nil {
			t.Errorf("expected no error, got %v", result.Err)
		}
		if result.ResultText != resultText {
			t.Errorf("expected result text %q, got %q", resultText, result.ResultText)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for result — FollowReader deadlock not fixed")
	}

	// Verify cleanup
	time.Sleep(100 * time.Millisecond)
	if mgr.ActiveCount() != 0 {
		t.Errorf("expected 0 active processes after completion, got %d", mgr.ActiveCount())
	}

	// Clean up log files
	os.RemoveAll(sessionsDir)
}
