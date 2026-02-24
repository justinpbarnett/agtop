package engine

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/cost"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
)

func executorTestConfig() *config.Config {
	return &config.Config{
		Workflows: map[string]config.WorkflowConfig{
			"build":      {Skills: []string{"build", "test"}},
			"plan-build": {Skills: []string{"spec", "build", "test", "review"}},
			"sdlc":       {Skills: []string{"spec", "decompose", "build", "test", "review", "document"}},
			"quick-fix":  {Skills: []string{}},
			"empty":      {Skills: []string{}},
		},
		Skills: map[string]config.SkillConfig{
			"route":     {Model: "haiku", Timeout: 300},
			"spec":      {Model: "opus"},
			"decompose": {Model: "opus"},
			"build":     {Model: "sonnet", Timeout: 1800},
			"test":      {Model: "sonnet", Timeout: 900},
			"review":    {Model: "opus"},
			"document":  {Model: "haiku"},
			"commit":    {Model: "haiku", Timeout: 300},
		},
	}
}

// --- ResolveWorkflow tests ---

func TestResolveWorkflow(t *testing.T) {
	cfg := executorTestConfig()

	skills, err := ResolveWorkflow(cfg, "build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0] != "build" || skills[1] != "test" {
		t.Errorf("expected [build, test], got %v", skills)
	}
}

func TestResolveWorkflowUnknown(t *testing.T) {
	cfg := executorTestConfig()

	_, err := ResolveWorkflow(cfg, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown workflow")
	}
}

func TestResolveWorkflowEmpty(t *testing.T) {
	cfg := executorTestConfig()

	_, err := ResolveWorkflow(cfg, "empty")
	if err == nil {
		t.Fatal("expected error for empty workflow")
	}
}

func TestResolveWorkflowSDLC(t *testing.T) {
	cfg := executorTestConfig()

	skills, err := ResolveWorkflow(cfg, "sdlc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 6 {
		t.Fatalf("expected 6 skills, got %d", len(skills))
	}
	expected := []string{"spec", "decompose", "build", "test", "review", "document"}
	for i, s := range expected {
		if skills[i] != s {
			t.Errorf("skill[%d]: expected %q, got %q", i, s, skills[i])
		}
	}
}

// --- ValidateWorkflow tests ---

func TestValidateWorkflowAllPresent(t *testing.T) {
	cfg := executorTestConfig()
	reg := NewRegistry(cfg)
	// Manually add skills to registry for testing
	reg.skills["build"] = &Skill{Name: "build"}
	reg.skills["test"] = &Skill{Name: "test"}

	missing := ValidateWorkflow([]string{"build", "test"}, reg)
	if len(missing) != 0 {
		t.Errorf("expected no missing skills, got %v", missing)
	}
}

func TestValidateWorkflowMissing(t *testing.T) {
	cfg := executorTestConfig()
	reg := NewRegistry(cfg)
	reg.skills["build"] = &Skill{Name: "build"}

	missing := ValidateWorkflow([]string{"build", "test", "review"}, reg)
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing skills, got %d: %v", len(missing), missing)
	}
	if missing[0] != "test" || missing[1] != "review" {
		t.Errorf("expected [test, review], got %v", missing)
	}
}

// --- parseRouteResult tests ---

var defaultKnownWorkflows = []string{"build", "plan-build", "sdlc", "quick-fix"}

func TestParseRouteResultPlainText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"build", "build"},
		{"plan-build", "plan-build"},
		{"sdlc\n", "sdlc"},
		{"  quick-fix  ", "quick-fix"},
		{"quick_fix", "quick_fix"},
		// Workflow name on first line with trailing text
		{"sdlc\nsome extra text", "sdlc"},
		// Workflow name on last line (common case: model adds explanation)
		{"Based on my analysis, this is a complex task.\nplan-build", "plan-build"},
		{"This task requires multi-file changes.\n\nbuild", "build"},
		// Multi-line triage assessment with workflow name at the end
		{"The task involves fixing auto-resume.\nRecommended approach: plan with spec first.\n\nplan-build\n", "plan-build"},
		// Workflow name buried in the middle
		{"Some preamble text.\nsdlc\nSome trailing explanation.", "sdlc"},
		// Wrapped in backticks (common LLM output)
		{"`build`", "build"},
		{"`plan-build`\n", "plan-build"},
		// Wrapped in quotes
		{`"sdlc"`, "sdlc"},
		{"'quick-fix'", "quick-fix"},
		// Trailing punctuation
		{"build.", "build"},
		{"plan-build,", "plan-build"},
		// Backticks on the last line after explanation
		{"Based on analysis:\n`build`", "build"},
		// Markdown bold (LLMs sometimes wrap the answer)
		{"**build**", "build"},
		{"**plan-build**", "plan-build"},
		{"Based on analysis:\n**sdlc**", "sdlc"},
		// Markdown bold with trailing punctuation
		{"**quick-fix**.", "quick-fix"},
	}

	for _, tt := range tests {
		got := parseRouteResult(tt.input, defaultKnownWorkflows)
		if got != tt.expected {
			t.Errorf("parseRouteResult(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseRouteResultJSON(t *testing.T) {
	got := parseRouteResult(`{"workflow": "plan-build"}`, defaultKnownWorkflows)
	if got != "plan-build" {
		t.Errorf("expected plan-build, got %q", got)
	}
}

func TestParseRouteResultEmpty(t *testing.T) {
	got := parseRouteResult("", defaultKnownWorkflows)
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParseRouteResultInvalidChars(t *testing.T) {
	got := parseRouteResult("I think you should use the build workflow", defaultKnownWorkflows)
	if got != "" {
		t.Errorf("expected empty for sentence input, got %q", got)
	}
}

func TestParseRouteResultAllSentences(t *testing.T) {
	// No valid workflow name on any line
	got := parseRouteResult("I recommend the build workflow.\nIt seems like a good fit.", defaultKnownWorkflows)
	if got != "" {
		t.Errorf("expected empty when all lines are sentences, got %q", got)
	}
}

func TestParseRouteResultCustomWorkflow(t *testing.T) {
	// Custom workflow name should be found when included in knownWorkflows
	custom := append(defaultKnownWorkflows, "my-custom")
	got := parseRouteResult("use my-custom", custom)
	if got != "my-custom" {
		t.Errorf("expected my-custom, got %q", got)
	}

	// But not when using default list
	got = parseRouteResult("use my-custom", defaultKnownWorkflows)
	if got != "" {
		t.Errorf("expected empty without custom in known list, got %q", got)
	}
}

func TestIsWorkflowName(t *testing.T) {
	valid := []string{"build", "plan-build", "sdlc", "quick_fix", "my-workflow-v2"}
	for _, name := range valid {
		if !isWorkflowName(name) {
			t.Errorf("isWorkflowName(%q) = false, want true", name)
		}
	}

	invalid := []string{"", "has spaces", "has.dots", "has:colons", "has,commas"}
	for _, name := range invalid {
		if isWorkflowName(name) {
			t.Errorf("isWorkflowName(%q) = true, want false", name)
		}
	}
}

// --- ParseDecomposeResult tests ---

func TestParseDecomposeResultFromFile(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "decompose-result.json"))
	if err != nil {
		t.Fatalf("read test fixture: %v", err)
	}

	result, err := ParseDecomposeResult(string(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(result.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(result.Tasks))
	}
	if result.Tasks[0].Name != "implement-auth-middleware" {
		t.Errorf("first task: expected implement-auth-middleware, got %q", result.Tasks[0].Name)
	}
	if result.Tasks[0].ParallelGroup != "core" {
		t.Errorf("first task group: expected core, got %q", result.Tasks[0].ParallelGroup)
	}
}

func TestParseDecomposeResultMalformed(t *testing.T) {
	_, err := ParseDecomposeResult("not json")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseDecomposeResultEmpty(t *testing.T) {
	result, err := ParseDecomposeResult(`{"tasks": []}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(result.Tasks))
	}
}

// --- GroupByParallel tests ---

func TestGroupByParallelBasic(t *testing.T) {
	result := &DecomposeResult{
		Tasks: []DecomposeTask{
			{Name: "a", ParallelGroup: "group1", Dependencies: nil},
			{Name: "b", ParallelGroup: "group1", Dependencies: nil},
			{Name: "c", ParallelGroup: "group2", Dependencies: []string{"a"}},
		},
	}

	groups := result.GroupByParallel()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// group1 should come first (no deps)
	if len(groups[0]) != 2 {
		t.Errorf("first group should have 2 tasks, got %d", len(groups[0]))
	}
	if len(groups[1]) != 1 {
		t.Errorf("second group should have 1 task, got %d", len(groups[1]))
	}
}

func TestGroupByParallelNoDeps(t *testing.T) {
	result := &DecomposeResult{
		Tasks: []DecomposeTask{
			{Name: "a", ParallelGroup: "alpha"},
			{Name: "b", ParallelGroup: "beta"},
		},
	}

	groups := result.GroupByParallel()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestGroupByParallelEmpty(t *testing.T) {
	result := &DecomposeResult{Tasks: nil}
	groups := result.GroupByParallel()
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestGroupByParallelSingleTask(t *testing.T) {
	result := &DecomposeResult{
		Tasks: []DecomposeTask{
			{Name: "only-one", ParallelGroup: "solo"},
		},
	}

	groups := result.GroupByParallel()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 1 {
		t.Errorf("group should have 1 task, got %d", len(groups[0]))
	}
}

// --- terminalState tests ---

func TestTerminalStateReview(t *testing.T) {
	state := terminalState([]string{"build", "test", "review"})
	if state != "reviewing" {
		t.Errorf("expected reviewing, got %s", state)
	}
}

func TestTerminalStateCompleted(t *testing.T) {
	tests := [][]string{
		{"build", "test"},
		{"build", "test", "commit"},
		{"build"},
		{},
	}

	for _, skills := range tests {
		state := terminalState(skills)
		if state != "completed" {
			t.Errorf("terminalState(%v) = %s, want completed", skills, state)
		}
	}
}

// --- isNonModifyingSkill tests ---

func TestIsNonModifyingSkillTrue(t *testing.T) {
	for _, name := range []string{"route", "decompose", "review"} {
		if !isNonModifyingSkill(name) {
			t.Errorf("isNonModifyingSkill(%q) = false, want true", name)
		}
	}
}

func TestIsNonModifyingSkillFalse(t *testing.T) {
	for _, name := range []string{"build", "test", "commit", "spec", "document", ""} {
		if isNonModifyingSkill(name) {
			t.Errorf("isNonModifyingSkill(%q) = true, want false", name)
		}
	}
}

// --- Executor integration tests ---

// executorMockRuntime implements runtime.Runtime for executor tests.
type executorMockRuntime struct {
	startFn func(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error)
	stopFn  func(proc *runtime.Process) error
}

func (m *executorMockRuntime) Start(ctx context.Context, prompt string, opts runtime.RunOptions) (*runtime.Process, error) {
	if m.startFn != nil {
		return m.startFn(ctx, prompt, opts)
	}
	return nil, nil
}
func (m *executorMockRuntime) Stop(proc *runtime.Process) error {
	if m.stopFn != nil {
		return m.stopFn(proc)
	}
	return nil
}
func (m *executorMockRuntime) Pause(_ *runtime.Process) error  { return nil }
func (m *executorMockRuntime) Resume(_ *runtime.Process) error { return nil }

func newTestExecutor(rt runtime.Runtime) (*Executor, *run.Store) {
	store := run.NewStore()
	cfg := executorTestConfig()
	tracker := cost.NewTracker()
	limiter := &cost.LimitChecker{}
	mgr := process.NewManager(store, rt, "claude", "", &config.LimitsConfig{MaxConcurrentRuns: 5}, tracker, limiter, nil)

	reg := NewRegistry(cfg)
	reg.skills["build"] = &Skill{Name: "build"}
	reg.skills["test"] = &Skill{Name: "test"}
	reg.skills["spec"] = &Skill{Name: "spec"}
	reg.skills["commit"] = &Skill{Name: "commit"}
	reg.skills["review"] = &Skill{Name: "review"}

	exec := NewExecutor(store, mgr, reg, cfg)
	return exec, store
}

// blockingRuntime returns a mock runtime whose Start() blocks stdout until
// the returned cancel function is called. Each call to Start creates fresh pipes.
func blockingRuntime() (*executorMockRuntime, func()) {
	var mu sync.Mutex
	var cancels []func()

	rt := &executorMockRuntime{
		startFn: func(ctx context.Context, _ string, _ runtime.RunOptions) (*runtime.Process, error) {
			pr, pw := io.Pipe()
			doneCh := make(chan error, 1)

			go func() {
				// Write one event so the stream parser starts
				pw.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Working..."}]}}` + "\n"))
				// Block until context cancels
				<-ctx.Done()
				pw.Close()
				doneCh <- nil
			}()

			mu.Lock()
			cancels = append(cancels, func() { pw.Close(); doneCh <- nil })
			mu.Unlock()

			return &runtime.Process{
				PID:    12345,
				Stdout: pr,
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}

	cleanup := func() {
		mu.Lock()
		defer mu.Unlock()
		for _, c := range cancels {
			c()
		}
	}
	return rt, cleanup
}

// completingRuntime returns a mock runtime whose Start() immediately writes
// a result event and completes. Tracks skill invocations via the returned slice.
func completingRuntime() (*executorMockRuntime, *[]string) {
	var mu sync.Mutex
	invocations := &[]string{}

	rt := &executorMockRuntime{
		startFn: func(_ context.Context, prompt string, _ runtime.RunOptions) (*runtime.Process, error) {
			mu.Lock()
			*invocations = append(*invocations, prompt)
			mu.Unlock()

			pr, pw := io.Pipe()
			doneCh := make(chan error, 1)

			go func() {
				pw.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"Done"}]}}` + "\n"))
				pw.Write([]byte(`{"type":"result","result":"ok","usage":{"input_tokens":10,"output_tokens":5},"total_cost_usd":0.001}` + "\n"))
				pw.Close()
				doneCh <- nil
			}()

			return &runtime.Process{
				PID:    12345,
				Stdout: pr,
				Stderr: io.NopCloser(strings.NewReader("")),
				Done:   doneCh,
			}, nil
		},
	}
	return rt, invocations
}

func TestExecutorShutdownPreservesRunState(t *testing.T) {
	rt, cleanup := blockingRuntime()
	defer cleanup()
	exec, store := newTestExecutor(rt)

	store.Add(&run.Run{State: run.StateQueued})
	exec.Execute("001", "build", "test prompt")

	// Wait for run to reach running state
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := store.Get("001")
		if r.State == run.StateRunning {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	r, _ := store.Get("001")
	if r.State != run.StateRunning {
		t.Fatalf("expected StateRunning before shutdown, got %s", r.State)
	}

	// Shutdown should cancel workflows but preserve run state
	exec.Shutdown()

	r, _ = store.Get("001")
	if r.State != run.StateRunning {
		t.Errorf("expected StateRunning after shutdown, got %s", r.State)
	}
}

func TestExecutorResumeReconnectedSkillIndex(t *testing.T) {
	rt, invocations := completingRuntime()
	exec, store := newTestExecutor(rt)
	_ = rt

	// Simulate a reconnected run at SkillIndex=2 in "plan-build" workflow
	// plan-build = ["spec", "build", "test", "review"]; SkillIndex 2 means
	// the 2nd skill ("build") was in progress, so resume should start from "build".
	store.Add(&run.Run{
		State:      run.StateRunning,
		Workflow:   "plan-build",
		SkillIndex: 2,
		SkillTotal: 4,
		Prompt:     "implement feature",
	})

	exec.ResumeReconnected("001", "implement feature")

	// Wait for workflow to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := store.Get("001")
		if r.IsTerminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	r, _ := store.Get("001")
	// plan-build ends with "review", so terminal state is StateReviewing
	if r.State != run.StateReviewing {
		t.Errorf("expected StateReviewing, got %s (error: %s)", r.State, r.Error)
	}

	// Should have executed "build", "test", "review", plus auto-commits
	if len(*invocations) < 3 {
		t.Errorf("expected at least 3 skill invocations (build + test + review), got %d", len(*invocations))
	}
}

func TestExecutorResumeRejectsFailedRun(t *testing.T) {
	rt, _ := completingRuntime()
	exec, store := newTestExecutor(rt)

	store.Add(&run.Run{
		State:    run.StateFailed,
		Workflow: "build",
		Prompt:   "test prompt",
		Error:    "skill failed",
	})

	err := exec.Resume("001", "test prompt")
	if err == nil {
		t.Fatal("expected error when resuming a failed run")
	}
	if !strings.Contains(err.Error(), "not resumable") {
		t.Errorf("expected 'not resumable' in error, got: %v", err)
	}

	// Verify the run state was not changed
	r, _ := store.Get("001")
	if r.State != run.StateFailed {
		t.Errorf("expected state to remain StateFailed, got %s", r.State)
	}
}

func TestIsActiveWhileRunning(t *testing.T) {
	rt, cleanup := blockingRuntime()
	defer cleanup()
	exec, store := newTestExecutor(rt)

	store.Add(&run.Run{State: run.StateQueued})

	if exec.IsActive("001") {
		t.Error("expected IsActive false before Execute")
	}

	exec.Execute("001", "build", "test prompt")

	// Wait for the worker to register
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if exec.IsActive("001") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !exec.IsActive("001") {
		t.Error("expected IsActive true while workflow is running")
	}

	// Shutdown cancels the worker; after shutdown, IsActive should be false
	exec.Shutdown()
	if exec.IsActive("001") {
		t.Error("expected IsActive false after Shutdown")
	}
}

func TestIsActiveAfterCompletion(t *testing.T) {
	rt, _ := completingRuntime()
	exec, store := newTestExecutor(rt)

	store.Add(&run.Run{State: run.StateQueued})
	exec.Execute("001", "build", "test prompt")

	// Wait for workflow to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := store.Get("001")
		if r.IsTerminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if exec.IsActive("001") {
		t.Error("expected IsActive false after workflow completion")
	}
}

func TestExecutorResumePausedRun(t *testing.T) {
	rt, _ := completingRuntime()
	exec, store := newTestExecutor(rt)

	store.Add(&run.Run{
		State:      run.StatePaused,
		Workflow:   "build",
		Prompt:     "test prompt",
		SkillIndex: 1,
		SkillTotal: 2,
	})

	err := exec.Resume("001", "test prompt")
	if err != nil {
		t.Fatalf("Resume on paused run should succeed: %v", err)
	}

	// Wait for the run to transition out of paused
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := store.Get("001")
		if r.State != run.StatePaused {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	r, _ := store.Get("001")
	if r.State == run.StatePaused {
		t.Error("expected run to transition from StatePaused after Resume")
	}
}

func TestResumePreservesSkillIndexAcrossMultipleResumes(t *testing.T) {
	// Simulate: 4-skill workflow, pause at skill 3, resume, pause at skill 3
	// again, resume — verify it starts at skill 3 both times (not skill 1).
	rt, invocations := completingRuntime()
	exec, store := newTestExecutor(rt)
	_ = rt

	// First run: pause at SkillIndex=3 (1-based) in plan-build workflow
	// plan-build = ["spec", "build", "test", "review"]
	store.Add(&run.Run{
		State:      run.StatePaused,
		Workflow:   "plan-build",
		Prompt:     "implement feature",
		SkillIndex: 3, // 1-based: spec(1), build(2), test(3)
		SkillTotal: 4,
	})

	// First resume: should start from skill index 2 (0-based) = "test"
	err := exec.Resume("001", "implement feature")
	if err != nil {
		t.Fatalf("first Resume failed: %v", err)
	}

	// Wait for workflow to complete
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := store.Get("001")
		if r.IsTerminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	r, _ := store.Get("001")
	if r.State != run.StateReviewing {
		t.Fatalf("expected StateReviewing after first resume, got %s (error: %s)", r.State, r.Error)
	}

	// SkillIndex should be 4 (all 4 skills completed), not reset to a low value
	if r.SkillIndex != 4 {
		t.Errorf("expected SkillIndex=4 after completing all skills, got %d", r.SkillIndex)
	}

	// Now simulate a second pause at skill 3 again
	store.Update("001", func(r *run.Run) {
		r.State = run.StatePaused
		r.SkillIndex = 3
	})

	*invocations = nil // reset invocation tracking

	err = exec.Resume("001", "implement feature")
	if err != nil {
		t.Fatalf("second Resume failed: %v", err)
	}

	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		r, _ := store.Get("001")
		if r.IsTerminal() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	r, _ = store.Get("001")
	// After second resume, SkillIndex should again be 4 (not 1 or 2)
	if r.SkillIndex != 4 {
		t.Errorf("expected SkillIndex=4 after second resume, got %d (Bug B regression: SkillIndex was reset to a relative value)", r.SkillIndex)
	}
}
