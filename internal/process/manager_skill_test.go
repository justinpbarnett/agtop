package process

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

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

	store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "build"})
	runID := "001"

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

	store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "build"})
	runID := "001"

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

func TestStartSkillCapturesResultText(t *testing.T) {
	doneCh := make(chan error, 1)
	expectedText := "The feature has been implemented successfully"
	rt := makeSkillMockRuntime(expectedText, doneCh)
	mgr, store := testManager(rt)

	store.Add(&run.Run{State: run.StateRunning, CurrentSkill: "build"})
	runID := "001"

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
