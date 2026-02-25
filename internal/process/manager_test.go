package process

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/cost"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
)

type mockRuntime struct {
	startFn  func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error)
	stopFn   func(proc *runtime.Process) error
	pauseFn  func(proc *runtime.Process) error
	resumeFn func(proc *runtime.Process) error
}

func (m *mockRuntime) Start(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
	if m.startFn != nil {
		return m.startFn(ctx, prompt, opts)
	}
	return nil, nil
}

func (m *mockRuntime) Stop(proc *runtime.Process) error {
	if m.stopFn != nil {
		return m.stopFn(proc)
	}
	return nil
}

func (m *mockRuntime) Pause(proc *runtime.Process) error {
	if m.pauseFn != nil {
		return m.pauseFn(proc)
	}
	return nil
}

func (m *mockRuntime) Resume(proc *runtime.Process) error {
	if m.resumeFn != nil {
		return m.resumeFn(proc)
	}
	return nil
}

func newMockProcess(events <-chan StreamEvent, done <-chan error) *runtime.Process {
	return &runtime.Process{
		PID:    12345,
		Cmd:    &exec.Cmd{},
		Stdout: io.NopCloser(strings.NewReader("")),
		Stderr: io.NopCloser(strings.NewReader("")),
		Done:   done,
	}
}

func makeMockRuntime(eventsCh chan StreamEvent, doneCh chan error) *mockRuntime {
	// Create a pipe that the stream parser will read from
	pr, pw := io.Pipe()

	// Write events as JSON lines to the pipe in a goroutine
	go func() {
		for event := range eventsCh {
			var line string
			switch event.Type {
			case EventText:
				line = `{"type":"assistant","message":{"content":[{"type":"text","text":"` + event.Text + `"}]}}`
			case EventResult:
				if event.Usage != nil {
					line = `{"type":"result","result":"done","usage":{"input_tokens":` +
						itoa(event.Usage.InputTokens) + `,"output_tokens":` +
						itoa(event.Usage.OutputTokens) + `},"total_cost_usd":` +
						ftoa(event.Usage.CostUSD) + `}`
				}
			default:
				line = event.Text
			}
			if line != "" {
				pw.Write([]byte(line + "\n"))
			}
		}
		pw.Close()
	}()

	return &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    12345,
				Cmd:    &exec.Cmd{},
				Stdout: pr,
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}
}

func itoa(n int) string {
	return fmt.Sprintf("%d", n)
}

func ftoa(f float64) string {
	return fmt.Sprintf("%g", f)
}

func testManager(rt runtime.Runtime) (*Manager, *run.Store) {
	store := run.NewStore()
	cfg := &config.LimitsConfig{
		MaxConcurrentRuns: 5,
		MaxTokensPerRun:   500000,
		MaxCostPerRun:     5.00,
	}
	tracker := cost.NewTracker()
	limiter := &cost.LimitChecker{
		MaxTokensPerRun: cfg.MaxTokensPerRun,
		MaxCostPerRun:   cfg.MaxCostPerRun,
	}
	mgr := NewManager(store, rt, "claude", "", cfg, tracker, limiter, nil)
	return mgr, store
}

func TestManagerStart(t *testing.T) {
	doneCh := make(chan error, 1)
	eventsCh := make(chan StreamEvent)
	rt := makeMockRuntime(eventsCh, doneCh)
	mgr, store := testManager(rt)

	runID := store.Add(&run.Run{State: run.StateQueued})

	err := mgr.Start(runID, "test prompt", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Check run state updated
	r, ok := store.Get(runID)
	if !ok {
		t.Fatal("run not found in store")
	}
	if r.State != run.StateRunning {
		t.Errorf("expected state Running, got %s", r.State)
	}
	if r.PID != 12345 {
		t.Errorf("expected PID 12345, got %d", r.PID)
	}

	// Cleanup
	close(eventsCh)
	doneCh <- nil
	time.Sleep(100 * time.Millisecond)
}

func TestManagerConcurrencyLimit(t *testing.T) {
	store := run.NewStore()
	cfg := &config.LimitsConfig{MaxConcurrentRuns: 2}

	doneCh1 := make(chan error, 1)
	doneCh2 := make(chan error, 1)

	callCount := 0
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			callCount++
			done := doneCh1
			if callCount == 2 {
				done = doneCh2
			}
			return &runtime.Process{
				PID:    callCount,
				Cmd:    &exec.Cmd{},
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   done,
			}, nil
		},
	}

	mgr := NewManager(store, rt, "claude", "", cfg, cost.NewTracker(), &cost.LimitChecker{}, nil)

	id1 := store.Add(&run.Run{State: run.StateQueued})
	id2 := store.Add(&run.Run{State: run.StateQueued})
	id3 := store.Add(&run.Run{State: run.StateQueued})

	err := mgr.Start(id1, "prompt 1", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start 1: %v", err)
	}

	err = mgr.Start(id2, "prompt 2", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start 2: %v", err)
	}

	err = mgr.Start(id3, "prompt 3", runtime.RunOptions{})
	if err == nil {
		t.Error("expected concurrency limit error for 3rd start")
	}
	if !strings.Contains(err.Error(), "concurrency limit") {
		t.Errorf("expected 'concurrency limit' in error, got %q", err.Error())
	}

	// Cleanup
	doneCh1 <- nil
	doneCh2 <- nil
	time.Sleep(100 * time.Millisecond)
}

func TestManagerPause(t *testing.T) {
	paused := false
	doneCh := make(chan error, 1)
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    1,
				Cmd:    &exec.Cmd{},
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
		pauseFn: func(proc *runtime.Process) error {
			paused = true
			return nil
		},
	}

	mgr, store := testManager(rt)
	runID := store.Add(&run.Run{State: run.StateQueued})

	mgr.Start(runID, "test", runtime.RunOptions{})
	time.Sleep(50 * time.Millisecond)

	err := mgr.Pause(runID)
	if err != nil {
		t.Fatalf("pause error: %v", err)
	}

	if !paused {
		t.Error("expected runtime.Pause to be called")
	}

	r, _ := store.Get(runID)
	if r.State != run.StatePaused {
		t.Errorf("expected state Paused, got %s", r.State)
	}

	// Cleanup
	doneCh <- nil
	time.Sleep(100 * time.Millisecond)
}

func TestManagerResume(t *testing.T) {
	resumed := false
	doneCh := make(chan error, 1)
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    1,
				Cmd:    &exec.Cmd{},
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
		pauseFn: func(proc *runtime.Process) error {
			return nil
		},
		resumeFn: func(proc *runtime.Process) error {
			resumed = true
			return nil
		},
	}

	mgr, store := testManager(rt)
	runID := store.Add(&run.Run{State: run.StateQueued})

	mgr.Start(runID, "test", runtime.RunOptions{})
	time.Sleep(50 * time.Millisecond)

	mgr.Pause(runID)
	err := mgr.Resume(runID)
	if err != nil {
		t.Fatalf("resume error: %v", err)
	}

	if !resumed {
		t.Error("expected runtime.Resume to be called")
	}

	r, _ := store.Get(runID)
	if r.State != run.StateRunning {
		t.Errorf("expected state Running after resume, got %s", r.State)
	}

	// Cleanup
	doneCh <- nil
	time.Sleep(100 * time.Millisecond)
}

func TestManagerProcessExit(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    1,
				Cmd:    &exec.Cmd{},
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}

	mgr, store := testManager(rt)
	runID := store.Add(&run.Run{State: run.StateQueued})

	mgr.Start(runID, "test", runtime.RunOptions{})
	time.Sleep(50 * time.Millisecond)

	// Signal clean exit
	doneCh <- nil
	time.Sleep(200 * time.Millisecond)

	r, _ := store.Get(runID)
	if r.State != run.StateCompleted {
		t.Errorf("expected state Completed after clean exit, got %s", r.State)
	}
	if r.PID != 0 {
		t.Errorf("expected PID 0 after exit, got %d", r.PID)
	}

	if mgr.ActiveCount() != 0 {
		t.Errorf("expected 0 active processes, got %d", mgr.ActiveCount())
	}
}

func TestManagerProcessExitError(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    1,
				Cmd:    &exec.Cmd{},
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}

	mgr, store := testManager(rt)
	runID := store.Add(&run.Run{State: run.StateQueued})

	mgr.Start(runID, "test", runtime.RunOptions{})
	time.Sleep(50 * time.Millisecond)

	// Signal error exit
	doneCh <- io.ErrUnexpectedEOF
	time.Sleep(200 * time.Millisecond)

	r, _ := store.Get(runID)
	if r.State != run.StateFailed {
		t.Errorf("expected state Failed after error exit, got %s", r.State)
	}
	if r.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestManagerEventConsumption(t *testing.T) {
	eventsCh := make(chan StreamEvent, 10)
	doneCh := make(chan error, 1)
	rt := makeMockRuntime(eventsCh, doneCh)
	mgr, store := testManager(rt)

	runID := store.Add(&run.Run{State: run.StateQueued, CurrentSkill: "build"})

	err := mgr.Start(runID, "test", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}

	// Send a text event
	eventsCh <- StreamEvent{Type: EventText, Text: "Building feature..."}
	time.Sleep(100 * time.Millisecond)

	buf := mgr.Buffer(runID)
	if buf == nil {
		t.Fatal("expected non-nil buffer")
	}
	if buf.Len() == 0 {
		t.Fatal("expected buffer to have lines after text event")
	}

	// Send a result event with usage data
	eventsCh <- StreamEvent{
		Type: EventResult,
		Usage: &UsageData{
			InputTokens:  1000,
			OutputTokens: 500,
			TotalTokens:  1500,
			CostUSD:      0.05,
		},
	}
	time.Sleep(100 * time.Millisecond)

	r, _ := store.Get(runID)
	if r.Tokens != 1500 {
		t.Errorf("expected 1500 tokens, got %d", r.Tokens)
	}
	if r.Cost != 0.05 {
		t.Errorf("expected 0.05 cost, got %f", r.Cost)
	}

	// Cleanup
	close(eventsCh)
	doneCh <- nil
	time.Sleep(100 * time.Millisecond)
}

func TestManagerStopNonExistent(t *testing.T) {
	rt := &mockRuntime{}
	mgr, _ := testManager(rt)

	err := mgr.Stop("nonexistent")
	if err == nil {
		t.Error("expected error stopping non-existent run")
	}
}

func TestManagerPauseNonExistent(t *testing.T) {
	rt := &mockRuntime{}
	mgr, _ := testManager(rt)

	err := mgr.Pause("nonexistent")
	if err == nil {
		t.Error("expected error pausing non-existent run")
	}
}

func TestManagerDuplicateStart(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    1,
				Cmd:    &exec.Cmd{},
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}

	mgr, store := testManager(rt)
	runID := store.Add(&run.Run{State: run.StateQueued})

	mgr.Start(runID, "test", runtime.RunOptions{})
	time.Sleep(50 * time.Millisecond)

	err := mgr.Start(runID, "test again", runtime.RunOptions{})
	if err == nil {
		t.Error("expected error for duplicate start")
	}
	if !strings.Contains(err.Error(), "already has an active process") {
		t.Errorf("expected 'already has an active process', got %q", err.Error())
	}

	// Cleanup
	doneCh <- nil
	time.Sleep(100 * time.Millisecond)
}

func TestManagerActiveCount(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    1,
				Cmd:    &exec.Cmd{},
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}

	mgr, store := testManager(rt)
	runID := store.Add(&run.Run{State: run.StateQueued})

	if mgr.ActiveCount() != 0 {
		t.Errorf("expected 0 active, got %d", mgr.ActiveCount())
	}

	mgr.Start(runID, "test", runtime.RunOptions{})
	time.Sleep(50 * time.Millisecond)

	if mgr.ActiveCount() != 1 {
		t.Errorf("expected 1 active, got %d", mgr.ActiveCount())
	}

	doneCh <- nil
	time.Sleep(200 * time.Millisecond)

	if mgr.ActiveCount() != 0 {
		t.Errorf("expected 0 active after exit, got %d", mgr.ActiveCount())
	}
}

func TestManagerBufferNonExistent(t *testing.T) {
	rt := &mockRuntime{}
	mgr, _ := testManager(rt)

	buf := mgr.Buffer("nonexistent")
	if buf != nil {
		t.Error("expected nil buffer for non-existent run")
	}
}

func TestManagerRemoveBuffer(t *testing.T) {
	rt := &mockRuntime{}
	mgr, _ := testManager(rt)

	mgr.InjectBuffer("test-id", []string{"line1", "line2"})
	if mgr.Buffer("test-id") == nil {
		t.Fatal("expected non-nil buffer after inject")
	}

	mgr.RemoveBuffer("test-id")
	if mgr.Buffer("test-id") != nil {
		t.Error("expected nil buffer after RemoveBuffer")
	}
}

func TestManagerRemoveBufferNonExistent(t *testing.T) {
	rt := &mockRuntime{}
	mgr, _ := testManager(rt)

	// Should not panic
	mgr.RemoveBuffer("nonexistent")
}

func TestExtractSystemInitModel(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "system init with model",
			input: `{"type":"system","subtype":"init","model":"claude-sonnet-4-6","permissionMode":"acceptEdits","tools":["Bash"],"claude_code_version":"2.1.50"}`,
			want:  "claude-sonnet-4-6",
		},
		{
			name:  "system init without model",
			input: `{"type":"system","subtype":"init","permissionMode":"acceptEdits"}`,
			want:  "",
		},
		{
			name:  "non-system event",
			input: `{"type":"assistant","message":{"content":[{"type":"text","text":"hello"}]}}`,
			want:  "",
		},
		{
			name:  "plain text",
			input: "not json",
			want:  "",
		},
		{
			name:  "system non-init subtype",
			input: `{"type":"system","subtype":"other","model":"claude-sonnet-4-6"}`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractSystemInitModel(tt.input)
			if got != tt.want {
				t.Errorf("extractSystemInitModel(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestManagerSystemInitUpdatesModel(t *testing.T) {
	eventsCh := make(chan StreamEvent, 10)
	doneCh := make(chan error, 1)
	rt := makeMockRuntime(eventsCh, doneCh)
	mgr, store := testManager(rt)

	runID := store.Add(&run.Run{State: run.StateQueued})
	if err := mgr.Start(runID, "test", runtime.RunOptions{}); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Send system/init raw event containing the model name
	eventsCh <- StreamEvent{
		Type: EventRaw,
		Text: `{"type":"system","subtype":"init","model":"claude-opus-4-6","permissionMode":"acceptEdits","tools":["Bash"],"claude_code_version":"2.1.50"}`,
	}
	time.Sleep(100 * time.Millisecond)

	r, ok := store.Get(runID)
	if !ok {
		t.Fatal("run not found")
	}
	if r.Model != "claude-opus-4-6" {
		t.Errorf("expected model %q, got %q", "claude-opus-4-6", r.Model)
	}

	close(eventsCh)
	doneCh <- nil
	time.Sleep(50 * time.Millisecond)
}

func TestManagerDisconnectPreservesRunState(t *testing.T) {
	doneCh := make(chan error, 1)
	rt := &mockRuntime{
		startFn: func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
			return &runtime.Process{
				PID:    42,
				Cmd:    nil,
				Stdout: io.NopCloser(strings.NewReader("")),
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}

	mgr, store := testManager(rt)
	runID := store.Add(&run.Run{State: run.StateQueued, CurrentSkill: "build"})

	err := mgr.Start(runID, "test", runtime.RunOptions{})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Set disconnecting before process exits
	mgr.SetDisconnecting()

	// Signal process exit
	doneCh <- nil
	time.Sleep(200 * time.Millisecond)

	r, ok := store.Get(runID)
	if !ok {
		t.Fatal("run not found")
	}
	// State should NOT be terminal when disconnecting
	if r.State == run.StateCompleted || r.State == run.StateFailed {
		t.Errorf("expected non-terminal state when disconnecting, got %s", r.State)
	}
	// PID should be preserved
	if r.PID == 0 {
		t.Error("PID should NOT be zeroed when disconnecting")
	}
}
