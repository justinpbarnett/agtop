package engine

import (
	"testing"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/run"
)

// ---------------------------------------------------------------------------
// parseCheckResults tests
// ---------------------------------------------------------------------------

func TestParseCheckResultsEmptyOutput(t *testing.T) {
	allPassed, pending, failed := parseCheckResults("")
	if !allPassed {
		t.Error("expected allPassed=true for empty output")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsEmptyArray(t *testing.T) {
	allPassed, pending, failed := parseCheckResults("[]")
	if !allPassed {
		t.Error("expected allPassed=true for empty array")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsEmptyArrayWithWhitespace(t *testing.T) {
	allPassed, pending, failed := parseCheckResults("  []  \n")
	if !allPassed {
		t.Error("expected allPassed=true for whitespace-padded empty array")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsAllPassed(t *testing.T) {
	input := `[
		{"name": "lint", "state": "COMPLETED", "conclusion": "SUCCESS"},
		{"name": "test", "state": "COMPLETED", "conclusion": "SUCCESS"},
		{"name": "build", "state": "COMPLETED", "conclusion": "SUCCESS"}
	]`

	allPassed, pending, failed := parseCheckResults(input)
	if !allPassed {
		t.Error("expected allPassed=true when all checks succeeded")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsPending(t *testing.T) {
	tests := []struct {
		name  string
		state string
	}{
		{"pending-check", "PENDING"},
		{"queued-check", "QUEUED"},
		{"running-check", "IN_PROGRESS"},
	}

	for _, tt := range tests {
		t.Run(tt.state, func(t *testing.T) {
			input := `[{"name": "` + tt.name + `", "state": "` + tt.state + `", "conclusion": ""}]`
			allPassed, pending, failed := parseCheckResults(input)
			if allPassed {
				t.Error("expected allPassed=false when a check is pending")
			}
			if len(pending) != 1 || pending[0] != tt.name {
				t.Errorf("expected pending=[%s], got %v", tt.name, pending)
			}
			if len(failed) != 0 {
				t.Errorf("expected no failed, got %v", failed)
			}
		})
	}
}

func TestParseCheckResultsFailed(t *testing.T) {
	tests := []struct {
		name       string
		conclusion string
	}{
		{"failing-test", "FAILURE"},
		{"timed-out-build", "TIMED_OUT"},
		{"cancelled-deploy", "CANCELLED"},
	}

	for _, tt := range tests {
		t.Run(tt.conclusion, func(t *testing.T) {
			input := `[{"name": "` + tt.name + `", "state": "COMPLETED", "conclusion": "` + tt.conclusion + `"}]`
			allPassed, pending, failed := parseCheckResults(input)
			if allPassed {
				t.Error("expected allPassed=false when a check failed")
			}
			if len(pending) != 0 {
				t.Errorf("expected no pending, got %v", pending)
			}
			if len(failed) != 1 || failed[0] != tt.name {
				t.Errorf("expected failed=[%s], got %v", tt.name, failed)
			}
		})
	}
}

func TestParseCheckResultsMixed(t *testing.T) {
	input := `[
		{"name": "lint", "state": "COMPLETED", "conclusion": "SUCCESS"},
		{"name": "test", "state": "PENDING", "conclusion": ""},
		{"name": "build", "state": "COMPLETED", "conclusion": "FAILURE"},
		{"name": "deploy", "state": "QUEUED", "conclusion": ""},
		{"name": "security", "state": "COMPLETED", "conclusion": "TIMED_OUT"}
	]`

	allPassed, pending, failed := parseCheckResults(input)
	if allPassed {
		t.Error("expected allPassed=false for mixed results")
	}

	if len(pending) != 2 {
		t.Fatalf("expected 2 pending checks, got %d: %v", len(pending), pending)
	}
	expectedPending := map[string]bool{"test": true, "deploy": true}
	for _, p := range pending {
		if !expectedPending[p] {
			t.Errorf("unexpected pending check: %s", p)
		}
	}

	if len(failed) != 2 {
		t.Fatalf("expected 2 failed checks, got %d: %v", len(failed), failed)
	}
	expectedFailed := map[string]bool{"build": true, "security": true}
	for _, f := range failed {
		if !expectedFailed[f] {
			t.Errorf("unexpected failed check: %s", f)
		}
	}
}

func TestParseCheckResultsInvalidJSON(t *testing.T) {
	allPassed, pending, failed := parseCheckResults("this is not json")
	if allPassed {
		t.Error("expected allPassed=false for invalid JSON")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 1 {
		t.Fatalf("expected 1 failed entry (parse error), got %d: %v", len(failed), failed)
	}
	if len(failed[0]) == 0 {
		t.Error("expected non-empty parse error message in failed list")
	}
	// Verify it starts with the "parse error:" prefix
	if failed[0][:12] != "parse error:" {
		t.Errorf("expected failed message to start with 'parse error:', got %q", failed[0])
	}
}

func TestParseCheckResultsEmptyChecksArray(t *testing.T) {
	// Valid JSON array that unmarshals to zero-length slice (different from "[]" string which is caught early)
	// Actually "[]" is already caught by the string check, so test with whitespace variant
	// that passes the initial string check but unmarshals to empty
	input := `[
	]`
	// This will be trimmed to "[]" which hits the early return
	allPassed, pending, failed := parseCheckResults(input)
	if !allPassed {
		t.Error("expected allPassed=true for empty checks array")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsSingleSuccess(t *testing.T) {
	input := `[{"name": "ci", "state": "COMPLETED", "conclusion": "SUCCESS"}]`
	allPassed, pending, failed := parseCheckResults(input)
	if !allPassed {
		t.Error("expected allPassed=true for single successful check")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsNeutralConclusion(t *testing.T) {
	// NEUTRAL, SKIPPED, etc. are not PENDING/QUEUED/IN_PROGRESS and not FAILURE/TIMED_OUT/CANCELLED
	// so they should be treated as passed (neither pending nor failed)
	input := `[
		{"name": "optional", "state": "COMPLETED", "conclusion": "NEUTRAL"},
		{"name": "skipped", "state": "COMPLETED", "conclusion": "SKIPPED"}
	]`
	allPassed, pending, failed := parseCheckResults(input)
	if !allPassed {
		t.Error("expected allPassed=true for neutral/skipped conclusions")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsOnlyPending(t *testing.T) {
	input := `[
		{"name": "check-a", "state": "PENDING", "conclusion": ""},
		{"name": "check-b", "state": "IN_PROGRESS", "conclusion": ""},
		{"name": "check-c", "state": "QUEUED", "conclusion": ""}
	]`
	allPassed, pending, failed := parseCheckResults(input)
	if allPassed {
		t.Error("expected allPassed=false when all checks are pending")
	}
	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d: %v", len(pending), pending)
	}
	if len(failed) != 0 {
		t.Errorf("expected no failed, got %v", failed)
	}
}

func TestParseCheckResultsOnlyFailed(t *testing.T) {
	input := `[
		{"name": "lint", "state": "COMPLETED", "conclusion": "FAILURE"},
		{"name": "test", "state": "COMPLETED", "conclusion": "CANCELLED"}
	]`
	allPassed, pending, failed := parseCheckResults(input)
	if allPassed {
		t.Error("expected allPassed=false when all checks failed")
	}
	if len(pending) != 0 {
		t.Errorf("expected no pending, got %v", pending)
	}
	if len(failed) != 2 {
		t.Errorf("expected 2 failed, got %d: %v", len(failed), failed)
	}
}

// ---------------------------------------------------------------------------
// resolveTarget tests
// ---------------------------------------------------------------------------

func TestResolveTargetFromConfig(t *testing.T) {
	store := run.NewStore()
	cfg := &config.MergeConfig{
		TargetBranch: "develop",
	}
	p := NewPipeline(nil, store, cfg, "/tmp")

	target, err := p.resolveTarget("/some/worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "develop" {
		t.Errorf("expected target 'develop', got %q", target)
	}
}

func TestResolveTargetFromConfigMain(t *testing.T) {
	store := run.NewStore()
	cfg := &config.MergeConfig{
		TargetBranch: "main",
	}
	p := NewPipeline(nil, store, cfg, "/tmp")

	target, err := p.resolveTarget("/any/path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "main" {
		t.Errorf("expected target 'main', got %q", target)
	}
}

func TestResolveTargetFromConfigCustomBranch(t *testing.T) {
	store := run.NewStore()
	cfg := &config.MergeConfig{
		TargetBranch: "release/v2",
	}
	p := NewPipeline(nil, store, cfg, "/tmp")

	target, err := p.resolveTarget("/does/not/matter")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if target != "release/v2" {
		t.Errorf("expected target 'release/v2', got %q", target)
	}
}

// ---------------------------------------------------------------------------
// Pipeline.fail tests
// ---------------------------------------------------------------------------

func TestPipelineFail(t *testing.T) {
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:  run.StateRunning,
		Branch: "feat/test",
	})

	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")
	p.fail(id, "rebase failed: conflict")

	r, ok := store.Get(id)
	if !ok {
		t.Fatal("run not found in store after fail")
	}
	if r.State != run.StateFailed {
		t.Errorf("expected state %q, got %q", run.StateFailed, r.State)
	}
	if r.MergeStatus != "failed" {
		t.Errorf("expected MergeStatus 'failed', got %q", r.MergeStatus)
	}
	if r.Error != "rebase failed: conflict" {
		t.Errorf("expected Error 'rebase failed: conflict', got %q", r.Error)
	}
	if r.CompletedAt.IsZero() {
		t.Error("expected CompletedAt to be set")
	}
	if time.Since(r.CompletedAt) > 5*time.Second {
		t.Error("expected CompletedAt to be recent (within 5s)")
	}
}

func TestPipelineFailPreservesOtherFields(t *testing.T) {
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:    run.StateRunning,
		Branch:   "feat/preserve",
		Prompt:   "build an api",
		Worktree: "/tmp/worktree",
		PRURL:    "https://github.com/org/repo/pull/42",
	})

	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")
	p.fail(id, "merge failed")

	r, ok := store.Get(id)
	if !ok {
		t.Fatal("run not found in store")
	}
	if r.Branch != "feat/preserve" {
		t.Errorf("Branch was modified: got %q", r.Branch)
	}
	if r.Prompt != "build an api" {
		t.Errorf("Prompt was modified: got %q", r.Prompt)
	}
	if r.Worktree != "/tmp/worktree" {
		t.Errorf("Worktree was modified: got %q", r.Worktree)
	}
	if r.PRURL != "https://github.com/org/repo/pull/42" {
		t.Errorf("PRURL was modified: got %q", r.PRURL)
	}
}

func TestPipelineFailNonexistentRun(t *testing.T) {
	store := run.NewStore()
	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")

	// Should not panic when failing a nonexistent run
	p.fail("nonexistent-id", "some error")
}

// ---------------------------------------------------------------------------
// Pipeline.setMergeStatus tests
// ---------------------------------------------------------------------------

func TestPipelineSetMergeStatus(t *testing.T) {
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:  run.StateRunning,
		Branch: "feat/status",
	})

	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")

	statuses := []string{"rebasing", "pushing", "pr-created", "checks-pending", "fixing", "merging"}
	for _, status := range statuses {
		p.setMergeStatus(id, status)
		r, ok := store.Get(id)
		if !ok {
			t.Fatalf("run not found in store after setting status %q", status)
		}
		if r.MergeStatus != status {
			t.Errorf("expected MergeStatus %q, got %q", status, r.MergeStatus)
		}
	}
}

func TestPipelineSetMergeStatusDoesNotChangeState(t *testing.T) {
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:  run.StateMerging,
		Branch: "feat/state-check",
	})

	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")
	p.setMergeStatus(id, "rebasing")

	r, ok := store.Get(id)
	if !ok {
		t.Fatal("run not found in store")
	}
	if r.State != run.StateMerging {
		t.Errorf("expected State to remain %q, got %q", run.StateMerging, r.State)
	}
}

func TestPipelineSetMergeStatusNonexistentRun(t *testing.T) {
	store := run.NewStore()
	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")

	// Should not panic for nonexistent run
	p.setMergeStatus("ghost", "rebasing")
}

// ---------------------------------------------------------------------------
// Pipeline success path tests
// ---------------------------------------------------------------------------

func TestPipelineSuccessUpdate(t *testing.T) {
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:       run.StateMerging,
		Branch:      "feat/success",
		MergeStatus: "merging",
		PRURL:       "https://github.com/org/repo/pull/99",
	})

	// Simulate the success update that Pipeline.Run performs at the end
	store.Update(id, func(r *run.Run) {
		r.State = run.StateAccepted
		r.MergeStatus = "merged"
		r.CompletedAt = time.Now()
	})

	r, ok := store.Get(id)
	if !ok {
		t.Fatal("run not found in store")
	}
	if r.State != run.StateAccepted {
		t.Errorf("expected state %q, got %q", run.StateAccepted, r.State)
	}
	if r.MergeStatus != "merged" {
		t.Errorf("expected MergeStatus 'merged', got %q", r.MergeStatus)
	}
	if r.CompletedAt.IsZero() {
		t.Error("expected CompletedAt to be set")
	}
	if time.Since(r.CompletedAt) > 5*time.Second {
		t.Error("expected CompletedAt to be recent")
	}
	// Ensure PRURL is preserved
	if r.PRURL != "https://github.com/org/repo/pull/99" {
		t.Errorf("expected PRURL preserved, got %q", r.PRURL)
	}
}

func TestPipelineSuccessUpdatePreservesFields(t *testing.T) {
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:    run.StateMerging,
		Branch:   "feat/all-fields",
		Prompt:   "implement feature X",
		Worktree: "/tmp/wt-123",
		Workflow: "build",
		PRURL:    "https://github.com/org/repo/pull/7",
		Tokens:   5000,
		Cost:     0.15,
	})

	store.Update(id, func(r *run.Run) {
		r.State = run.StateAccepted
		r.MergeStatus = "merged"
		r.CompletedAt = time.Now()
	})

	r, _ := store.Get(id)
	if r.Branch != "feat/all-fields" {
		t.Errorf("Branch was modified: got %q", r.Branch)
	}
	if r.Prompt != "implement feature X" {
		t.Errorf("Prompt was modified: got %q", r.Prompt)
	}
	if r.Worktree != "/tmp/wt-123" {
		t.Errorf("Worktree was modified: got %q", r.Worktree)
	}
	if r.Workflow != "build" {
		t.Errorf("Workflow was modified: got %q", r.Workflow)
	}
	if r.PRURL != "https://github.com/org/repo/pull/7" {
		t.Errorf("PRURL was modified: got %q", r.PRURL)
	}
	if r.Tokens != 5000 {
		t.Errorf("Tokens was modified: got %d", r.Tokens)
	}
	if r.Cost != 0.15 {
		t.Errorf("Cost was modified: got %f", r.Cost)
	}
}

// ---------------------------------------------------------------------------
// Pipeline PR skip on re-accept tests
// ---------------------------------------------------------------------------

func TestPipelinePRSkipOnReAccept(t *testing.T) {
	// When a run already has a PRURL, the pipeline should preserve it
	// and skip PR creation (setting status to "pr-created" without overwriting PRURL).
	store := run.NewStore()
	existingPR := "https://github.com/org/repo/pull/55"
	id := store.Add(&run.Run{
		State:    run.StateMerging,
		Branch:   "feat/retry",
		PRURL:    existingPR,
		Prompt:   "fix the thing",
		Worktree: "/tmp/wt",
	})

	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")

	// Simulate the PR stage logic from Pipeline.Run:
	// Get the run, check PRURL, skip creation if already set
	r, ok := store.Get(id)
	if !ok {
		t.Fatal("run not found")
	}

	prURL := r.PRURL
	if prURL == "" {
		t.Fatal("expected PRURL to be already set for re-accept scenario")
	}

	// The pipeline just sets merge status without creating a new PR
	p.setMergeStatus(id, "pr-created")

	r, _ = store.Get(id)
	if r.PRURL != existingPR {
		t.Errorf("expected PRURL to remain %q, got %q", existingPR, r.PRURL)
	}
	if r.MergeStatus != "pr-created" {
		t.Errorf("expected MergeStatus 'pr-created', got %q", r.MergeStatus)
	}
}

func TestPipelinePRURLSetWhenEmpty(t *testing.T) {
	// When PRURL is empty, the pipeline would create a PR and store the URL.
	// Here we simulate just the store update part.
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:  run.StateMerging,
		Branch: "feat/new-pr",
		PRURL:  "", // No existing PR
	})

	newPRURL := "https://github.com/org/repo/pull/100"
	store.Update(id, func(r *run.Run) {
		r.PRURL = newPRURL
	})

	r, _ := store.Get(id)
	if r.PRURL != newPRURL {
		t.Errorf("expected PRURL %q, got %q", newPRURL, r.PRURL)
	}
}

// ---------------------------------------------------------------------------
// NewPipeline constructor test
// ---------------------------------------------------------------------------

func TestNewPipeline(t *testing.T) {
	store := run.NewStore()
	cfg := &config.MergeConfig{
		TargetBranch:  "main",
		AutoMerge:     true,
		MergeStrategy: "squash",
		FixAttempts:   5,
		PollInterval:  15,
		PollTimeout:   300,
	}

	p := NewPipeline(nil, store, cfg, "/home/user/repo")
	if p == nil {
		t.Fatal("expected non-nil pipeline")
	}
	if p.store != store {
		t.Error("pipeline store does not match")
	}
	if p.cfg != cfg {
		t.Error("pipeline config does not match")
	}
	if p.repoRoot != "/home/user/repo" {
		t.Errorf("expected repoRoot '/home/user/repo', got %q", p.repoRoot)
	}
	if p.executor != nil {
		t.Error("expected executor to be nil when passed nil")
	}
}

// ---------------------------------------------------------------------------
// Pipeline state transition sequences
// ---------------------------------------------------------------------------

func TestPipelineMergeStatusTransitions(t *testing.T) {
	// Verify that a typical sequence of setMergeStatus calls produces
	// the correct final state at each step.
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:  run.StateMerging,
		Branch: "feat/transitions",
	})

	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")

	transitions := []string{
		"rebasing",
		"pushing",
		"pr-created",
		"checks-pending",
		"fixing",
		"pushing",
		"checks-pending",
		"merging",
	}

	for i, status := range transitions {
		p.setMergeStatus(id, status)
		r, ok := store.Get(id)
		if !ok {
			t.Fatalf("step %d: run not found", i)
		}
		if r.MergeStatus != status {
			t.Errorf("step %d: expected MergeStatus %q, got %q", i, status, r.MergeStatus)
		}
		// State should remain unchanged throughout
		if r.State != run.StateMerging {
			t.Errorf("step %d: expected State %q, got %q", i, run.StateMerging, r.State)
		}
	}
}

func TestPipelineFailAfterPartialProgress(t *testing.T) {
	store := run.NewStore()
	id := store.Add(&run.Run{
		State:       run.StateMerging,
		Branch:      "feat/partial",
		MergeStatus: "checks-pending",
		PRURL:       "https://github.com/org/repo/pull/10",
	})

	p := NewPipeline(nil, store, &config.MergeConfig{}, "/tmp")
	p.fail(id, "checks still failing after 3 fix attempts: lint, test")

	r, _ := store.Get(id)
	if r.State != run.StateFailed {
		t.Errorf("expected StateFailed, got %q", r.State)
	}
	if r.MergeStatus != "failed" {
		t.Errorf("expected MergeStatus 'failed', got %q", r.MergeStatus)
	}
	if r.Error != "checks still failing after 3 fix attempts: lint, test" {
		t.Errorf("unexpected error message: %q", r.Error)
	}
	// PRURL should be preserved even after failure
	if r.PRURL != "https://github.com/org/repo/pull/10" {
		t.Errorf("expected PRURL preserved after fail, got %q", r.PRURL)
	}
}
