# Feature: Workflow Executor

## Metadata

type: `feat`
task_id: `workflow-executor`
prompt: `Implement the workflow executor — the orchestrator that chains skills into sequential workflows, executes each as a separate claude -p subprocess, passes minimal context between skills, supports parallel execution for decomposed tasks, and manages checkpoint/resume on failure`

## Feature Description

The Workflow Executor is the central orchestrator that drives agtop's core value: deterministic, chained skill execution. It takes a workflow definition (an ordered list of skill names from config), a user's task prompt, and a run ID, then executes each skill sequentially as a separate `claude -p` subprocess — assembling prompts via the Skill Engine, launching processes via the Process Manager, waiting for completion, capturing result text, and passing minimal context forward to the next skill.

The executor manages the run's state machine throughout its lifecycle: `queued` → `running` → skill-by-skill progression → `completed`/`failed`. It handles the special behaviors of specific skills (route determines workflow, decompose produces parallel groups), enforces timeouts, and supports checkpoint/resume so a failed run can restart from the last incomplete skill.

This is the component that transforms agtop from "a dashboard that shows mock data" into "a working agent orchestrator."

## User Story

As a developer launching an AI agent workflow
I want skills to execute automatically in sequence, with each skill receiving only the context it needs from the previous skill's output
So that I get deterministic, cost-efficient workflows where every skill operates with fresh, hyper-targeted context rather than an ever-growing conversation

## Problem Statement

The current codebase has all the building blocks but no way to chain them together:

- **Skill Engine** (`internal/engine/`) — Can discover, parse, and resolve skills; can build prompts with context injection. Fully working.
- **Process Manager** (`internal/process/manager.go`) — Can spawn `claude -p` subprocesses, parse streaming output, manage lifecycle (pause/resume/kill), track tokens/cost, and auto-pause on thresholds. Fully working.
- **Run Store** (`internal/run/`) — Tracks run state, skill progress (`SkillIndex`/`SkillTotal`), and notifies the TUI on changes. Fully working.
- **Config System** (`internal/config/`) — Defines workflows as ordered skill lists with per-skill model/timeout/tool overrides. Fully working.
- **Workflow/Executor stubs** (`internal/engine/workflow.go`, `executor.go`) — Empty structs with no logic.

The gap: nothing connects these components. There is no code that:
1. Reads a workflow definition and iterates through its skills
2. Calls `BuildPrompt()` then `Manager.Start()` for each skill in sequence
3. Waits for a skill subprocess to complete before starting the next
4. Captures the result text from one skill to pass as `PreviousOutput` to the next
5. Handles per-skill timeouts, route-skill workflow selection, or decompose-skill parallel dispatch
6. Updates `SkillIndex`/`CurrentSkill` as skills progress

Without the executor, agtop is a beautifully rendered dashboard with mock data. With it, agtop becomes a working agent orchestrator.

## Solution Statement

Implement the Workflow Executor as two tightly coupled changes:

1. **Process Manager extension** — Add a `StartSkill()` method that launches a subprocess and returns a completion channel with the result text, without setting the run's terminal state. This lets the executor chain multiple subprocesses within a single run, managing state transitions itself.

2. **Executor** (`internal/engine/executor.go`) — A goroutine-based orchestrator that:
   - Resolves workflow → skill list from config
   - Iterates skills sequentially, calling `BuildPrompt()` → `StartSkill()` → wait for result
   - Passes the previous skill's result text as `PreviousOutput` in the next skill's `PromptContext`
   - Updates `SkillIndex`, `CurrentSkill`, and run state at each transition
   - Handles special skill behaviors: `route` can override the workflow; `decompose` produces parallel groups
   - Enforces per-skill timeouts via `context.WithTimeout`
   - Supports pause/resume/cancel by checking run state between skills and responding to signals
   - Checkpoints progress so a resumed run restarts from the last incomplete skill
   - On workflow completion, transitions to `StateReviewing` (if the workflow warrants review) or `StateCompleted`

The executor runs in its own goroutine per run, communicates with the TUI via the existing store notification system, and is fully cancelable.

## Relevant Files

Use these files to implement the feature:

- `internal/engine/executor.go` — Currently empty `Executor` struct. Will be rewritten with the full orchestration logic.
- `internal/engine/workflow.go` — Currently defines empty `Workflow` struct. Will add helper methods for workflow resolution from config.
- `internal/engine/decompose.go` — `DecomposeResult` and `DecomposeTask` types. Will add JSON parsing of decompose skill output.
- `internal/engine/prompt.go` — `BuildPrompt()` and `PromptContext`. No changes needed — already complete.
- `internal/engine/registry.go` — `Registry` with `Get()`, `SkillForRun()`, `BuildPrompt()`. No changes needed.
- `internal/engine/skill.go` — `Skill` struct with all fields. No changes needed.
- `internal/process/manager.go` — `Manager` with `Start()`, `Stop()`, `Pause()`, `Resume()`. Will add `StartSkill()` method and `SkillResult` type.
- `internal/process/stream.go` — `StreamParser`, `StreamEvent`, event types. No changes needed — already captures result text.
- `internal/run/run.go` — `Run` struct with `SkillIndex`, `SkillTotal`, `CurrentSkill`, state machine. No changes needed.
- `internal/run/store.go` — `Store` with `Update()`, `Get()`, `Changes()`. No changes needed.
- `internal/config/config.go` — `WorkflowConfig`, `SkillConfig`, `LimitsConfig`. No changes needed.
- `internal/config/defaults.go` — Default workflow and skill definitions. No changes needed.
- `internal/ui/app.go` — `App` struct holds `manager`, `registry`, `store`. Will integrate executor instantiation and run launching.

### New Files

- `internal/engine/executor_test.go` — Unit tests for the executor: sequential execution, parallel execution, error handling, pause/resume, checkpoint/resume, timeout, route override.
- `internal/process/manager_skill_test.go` — Tests for `StartSkill()` completion channel behavior.
- `internal/engine/testdata/decompose-result.json` — Test fixture for decompose result parsing.

## Implementation Plan

### Phase 1: Foundation — Process Manager Skill Mode

Extend the process manager to support the executor's needs. The current `Start()` method sets `r.State = StateCompleted/StateFailed` when a subprocess exits — the executor needs a variant that returns the result to the caller instead.

Add `StartSkill()` which:
- Launches a subprocess identically to `Start()` (reusing all existing code)
- Returns a `<-chan SkillResult` that receives the exit status and result text
- Does NOT set the run's terminal state on exit (the executor manages state)
- Still logs to ring buffer, still tracks tokens/cost, still supports pause/resume

Also capture the `result` event's text content, which the current manager ignores. The `result` event from stream-json contains the final text output — this is the "previous skill output" that gets passed forward.

### Phase 2: Core Implementation — Sequential Executor

Build the executor's main loop:
1. Resolve workflow name → skill list from config
2. For each skill: build prompt → start skill → wait for completion → capture result
3. Pass result text as `PreviousOutput` to next skill's `PromptContext`
4. Update `SkillIndex`/`CurrentSkill`/`State` at each step
5. Handle errors: set `StateFailed` with error message, record failed skill index for resume
6. Handle timeouts: per-skill timeout from config, context cancellation

### Phase 3: Integration — Special Skills, Parallel Execution, TUI Hookup

- **Route skill**: Parse route output to determine workflow, then execute the resolved workflow's remaining skills
- **Decompose skill**: Parse JSON output into `DecomposeResult`, dispatch parallel groups as concurrent `StartSkill()` calls
- **TUI integration**: Wire executor into `app.go` so the `n` (new run) command triggers execution
- **Pause/resume**: Check run state between skills; respect external pause signals
- **Checkpoint**: Record skill index to enable resume from last incomplete skill

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add SkillResult Type and StartSkill to Process Manager

In `internal/process/manager.go`:

- Define `SkillResult` struct:

```go
// SkillResult captures the outcome of a single skill subprocess execution.
type SkillResult struct {
	ResultText string // Final result text from the stream-json "result" event
	Err        error  // Non-nil if the process exited with error
}
```

- Add `StartSkill()` method to `Manager`:

```go
// StartSkill launches a skill subprocess and returns a channel that receives
// the result when the process exits. Unlike Start(), it does NOT set the run's
// terminal state (Completed/Failed) on exit — the caller manages state.
// It still logs to the ring buffer and tracks tokens/cost.
func (m *Manager) StartSkill(runID string, prompt string, opts runtime.RunOptions) (<-chan SkillResult, error)
```

The implementation follows the same pattern as `Start()`:
- Check concurrency limit
- Create context with cancel
- Call `m.rt.Start(ctx, prompt, opts)` to spawn the subprocess
- Register in `m.processes` and `m.buffers`
- Launch `consumeSkillEvents()` goroutine (similar to `consumeEvents()` but captures result text and sends `SkillResult` on the returned channel instead of setting terminal state)
- Update run: set `PID`, `StartedAt`

- Add `consumeSkillEvents()` private method. This is nearly identical to `consumeEvents()` with two differences:
  1. It captures the `Text` field from `EventResult` events into a `resultText` variable
  2. On process exit, it sends `SkillResult{ResultText: resultText, Err: exitErr}` on the channel instead of calling `store.Update()` with terminal state
  3. It still cleans up `m.processes[runID]` on exit

- Add `ResultText` capture to both `consumeEvents` and `consumeSkillEvents`. In the `EventResult` case, after handling usage, also store `event.Text` — this is the final text output from the `result` stream-json message.

### 2. Add Per-Skill Timeout Support to StartSkill

- Accept an optional timeout parameter. If `skill.Timeout > 0`, wrap the context with `context.WithTimeout(ctx, time.Duration(skill.Timeout) * time.Second)`.
- On timeout, the context cancels, `consumeSkillEvents` detects it and sends `SkillResult{Err: context.DeadlineExceeded}`.
- Log the timeout to the ring buffer: `[HH:MM:SS skillName] TIMEOUT: skill exceeded Ns deadline`.

### 3. Implement Workflow Resolution

In `internal/engine/workflow.go`:

- Add a `ResolveWorkflow` function:

```go
// ResolveWorkflow returns the ordered list of skill names for a workflow.
// Returns an error if the workflow name is not found in config.
func ResolveWorkflow(cfg *config.Config, workflowName string) ([]string, error) {
	wf, ok := cfg.Workflows[workflowName]
	if !ok {
		return nil, fmt.Errorf("unknown workflow: %q", workflowName)
	}
	if len(wf.Skills) == 0 {
		return nil, fmt.Errorf("workflow %q has no skills", workflowName)
	}
	return wf.Skills, nil
}
```

- Add a `ValidateWorkflow` function that checks all skill names in a workflow exist in the registry:

```go
// ValidateWorkflow checks that every skill in the workflow is available
// in the registry. Returns the names of any missing skills.
func ValidateWorkflow(skills []string, reg *Registry) []string
```

### 4. Implement Decompose Result Parser

In `internal/engine/decompose.go`:

- Add `ParseDecomposeResult()` that parses JSON output from the decompose skill:

```go
// ParseDecomposeResult parses the JSON output from a decompose skill
// into structured tasks with parallel groups and dependencies.
func ParseDecomposeResult(jsonText string) (*DecomposeResult, error)
```

- The decompose skill outputs a JSON array of tasks, each with `name`, `parallel_group`, and `dependencies`.

- Add `GroupByParallel()` method on `DecomposeResult`:

```go
// GroupByParallel returns tasks grouped by parallel group, ordered
// so that groups with no dependencies come first.
func (d *DecomposeResult) GroupByParallel() [][]DecomposeTask
```

### 5. Implement the Executor Struct and Constructor

In `internal/engine/executor.go`:

- Define the `Executor` struct with all dependencies:

```go
type Executor struct {
	store    *run.Store
	manager  *process.Manager
	registry *Registry
	cfg      *config.Config
	mu       sync.Mutex
	active   map[string]context.CancelFunc // runID → cancel function
}

func NewExecutor(store *run.Store, manager *process.Manager, registry *Registry, cfg *config.Config) *Executor
```

- The `active` map tracks running executor goroutines for cancellation from TUI commands.

### 6. Implement Sequential Skill Execution Loop

In `internal/engine/executor.go`:

- Add the `Execute()` method — the main entry point:

```go
// Execute runs a workflow for the given run. It runs in a goroutine
// and communicates progress via the run store. Call Cancel() to stop.
func (e *Executor) Execute(runID string, workflowName string, userPrompt string)
```

- `Execute()` spawns a goroutine that calls `executeWorkflow()`:

```go
func (e *Executor) executeWorkflow(ctx context.Context, runID string, skills []string, userPrompt string) {
	r, _ := e.store.Get(runID)
	var previousOutput string

	for i, skillName := range skills {
		// Check if cancelled
		select {
		case <-ctx.Done():
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = "cancelled"
			})
			return
		default:
		}

		// Check if paused — wait for resume
		e.waitIfPaused(ctx, runID)

		// Update progress
		e.store.Update(runID, func(r *run.Run) {
			r.SkillIndex = i + 1
			r.CurrentSkill = skillName
			r.State = run.StateRunning
		})

		// Get skill and options
		skill, opts, ok := e.registry.SkillForRun(skillName)
		if !ok {
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = fmt.Sprintf("skill not found: %s", skillName)
			})
			return
		}

		// Set worktree from run
		r, _ = e.store.Get(runID)
		opts.WorkDir = r.Worktree

		// Build prompt
		prompt := BuildPrompt(skill, PromptContext{
			WorkDir:        r.Worktree,
			Branch:         r.Branch,
			PreviousOutput: previousOutput,
			UserPrompt:     userPrompt,
		})

		// Execute skill
		result, err := e.runSkill(ctx, runID, prompt, opts, skill.Timeout)
		if err != nil {
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = fmt.Sprintf("skill %s failed: %v", skillName, err)
			})
			return
		}

		previousOutput = result.ResultText
	}

	// Workflow complete
	e.store.Update(runID, func(r *run.Run) {
		r.State = run.StateCompleted
		r.CurrentSkill = ""
	})
}
```

- `runSkill()` is a helper that calls `manager.StartSkill()` and waits on the result channel:

```go
func (e *Executor) runSkill(ctx context.Context, runID string, prompt string, opts runtime.RunOptions, timeout int) (process.SkillResult, error) {
	// Apply timeout if configured
	skillCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		skillCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	ch, err := e.manager.StartSkill(runID, prompt, opts)
	if err != nil {
		return process.SkillResult{}, err
	}

	select {
	case result := <-ch:
		return result, result.Err
	case <-skillCtx.Done():
		_ = e.manager.Stop(runID)
		return process.SkillResult{}, skillCtx.Err()
	}
}
```

### 7. Implement Pause/Resume Handling

- Add `waitIfPaused()` to the executor. Between skills, check if the run is paused. If so, block until the run state changes to something other than paused:

```go
func (e *Executor) waitIfPaused(ctx context.Context, runID string) {
	for {
		r, ok := e.store.Get(runID)
		if !ok || r.State != run.StatePaused {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-e.store.Changes():
			// State changed, re-check
		}
	}
}
```

- Add `Cancel()` method:

```go
// Cancel stops execution of a run. Safe to call if the run isn't active.
func (e *Executor) Cancel(runID string) {
	e.mu.Lock()
	cancel, ok := e.active[runID]
	e.mu.Unlock()
	if ok {
		cancel()
	}
}
```

- Add `Pause()` and `Resume()` methods that delegate to the process manager and update state. The executor's main loop detects paused state via `waitIfPaused()` between skills. During a skill, the process manager's `Pause()`/`Resume()` handles the subprocess directly.

### 8. Implement Route Skill Handling

The `route` skill is special: its output determines which workflow to use. After executing route:

- Parse the result text for a workflow name (the route skill's output should be a single workflow name or a JSON object with a `workflow` field)
- Validate the resolved workflow exists in config
- Replace the remaining skill list with the resolved workflow's skills
- Log the workflow selection to the ring buffer

```go
// parseRouteResult extracts a workflow name from the route skill's output.
func parseRouteResult(resultText string) string
```

In the main loop, after executing a skill named "route":

```go
if skillName == "route" {
	resolvedWorkflow := parseRouteResult(previousOutput)
	if resolvedWorkflow != "" {
		newSkills, err := ResolveWorkflow(e.cfg, resolvedWorkflow)
		if err == nil {
			skills = newSkills
			i = -1 // restart loop with new skill list
			e.store.Update(runID, func(r *run.Run) {
				r.Workflow = resolvedWorkflow
				r.SkillTotal = len(newSkills)
			})
			continue
		}
	}
}
```

### 9. Implement Decompose Skill and Parallel Execution

After the `decompose` skill completes:

- Parse its result text as `DecomposeResult` JSON via `ParseDecomposeResult()`
- Group tasks by `ParallelGroup` via `GroupByParallel()`
- For each group, launch all tasks in the group concurrently via `StartSkill()`
- Wait for all tasks in a group to complete before starting the next group
- Collect results and merge them as `PreviousOutput` for the next sequential skill

```go
func (e *Executor) executeParallelGroup(ctx context.Context, runID string, tasks []DecomposeTask, userPrompt string, previousOutput string) (string, error) {
	var wg sync.WaitGroup
	results := make([]string, len(tasks))
	errs := make([]error, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t DecomposeTask) {
			defer wg.Done()
			// Build task-specific prompt using build skill with task context
			skill, opts, _ := e.registry.SkillForRun("build")
			r, _ := e.store.Get(runID)
			opts.WorkDir = r.Worktree

			taskPrompt := BuildPrompt(skill, PromptContext{
				WorkDir:        r.Worktree,
				Branch:         r.Branch,
				PreviousOutput: previousOutput,
				UserPrompt:     fmt.Sprintf("%s\n\nSub-task: %s", userPrompt, t.Name),
			})

			result, err := e.runSkill(ctx, runID, taskPrompt, opts, skill.Timeout)
			results[idx] = result.ResultText
			errs[idx] = err
		}(i, task)
	}

	wg.Wait()

	// Check for errors
	for i, err := range errs {
		if err != nil {
			return "", fmt.Errorf("parallel task %q failed: %w", tasks[i].Name, err)
		}
	}

	// Merge results
	var merged strings.Builder
	for i, task := range tasks {
		merged.WriteString(fmt.Sprintf("### %s\n%s\n\n", task.Name, results[i]))
	}
	return merged.String(), nil
}
```

Note: Parallel execution requires the process manager to support multiple concurrent processes for the same run ID. This means `StartSkill()` needs to use a sub-ID (e.g., `runID/taskName`) for process tracking, or accept that multiple processes share the same ring buffer.

The simpler approach: parallel sub-tasks log to the same run's buffer (prefixed with task name) and share token/cost tracking. The manager tracks them with composite keys (`runID:taskName`).

### 10. Implement Checkpoint for Resume Support

Add checkpoint tracking to enable resume from the last incomplete skill:

- When a skill starts, record `SkillIndex` in the run (already done via `store.Update`)
- When a skill fails, `SkillIndex` remains at the failed skill's position
- On resume: read `SkillIndex` from the run, skip already-completed skills

```go
// Resume restarts a failed or paused run from its last incomplete skill.
func (e *Executor) Resume(runID string, userPrompt string) error {
	r, ok := e.store.Get(runID)
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}

	skills, err := ResolveWorkflow(e.cfg, r.Workflow)
	if err != nil {
		return err
	}

	// Start from the failed skill (SkillIndex is 1-based)
	startIdx := r.SkillIndex - 1
	if startIdx < 0 {
		startIdx = 0
	}
	remainingSkills := skills[startIdx:]

	e.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRunning
		r.Error = ""
	})

	go func() {
		ctx, cancel := context.WithCancel(context.Background())
		e.mu.Lock()
		e.active[runID] = cancel
		e.mu.Unlock()
		defer func() {
			e.mu.Lock()
			delete(e.active, runID)
			e.mu.Unlock()
		}()

		e.executeWorkflow(ctx, runID, remainingSkills, userPrompt)
	}()

	return nil
}
```

Note: Resume doesn't replay previous skill outputs. The resumed skill starts with an empty `PreviousOutput`. This is acceptable because each skill should be self-contained enough to operate on the worktree state, and the worktree preserves all changes from prior skills.

### 11. Integrate Executor into TUI App

In `internal/ui/app.go`:

- Add `executor *engine.Executor` field to `App` struct
- Instantiate in `NewApp()`:

```go
var exec *engine.Executor
if mgr != nil {
	exec = engine.NewExecutor(store, mgr, reg, cfg)
}
```

- Add a Bubble Tea message type for launching runs:

```go
type StartRunMsg struct {
	Prompt   string
	Workflow string
}
```

- Handle `StartRunMsg` in `Update()`:
  1. Create a new `Run` in the store with `StateQueued`
  2. Set the branch name (for now, use a generated name like `agtop/<run-id>`)
  3. Set the worktree path (for now, use project root — actual worktree management is step 10)
  4. Call `executor.Execute(runID, workflow, prompt)`

- Wire up the `n` key to emit `StartRunMsg` (placeholder until the new run modal is implemented in step 13):
  - For now, use a hardcoded prompt and workflow for testing
  - The actual modal will be implemented in a later step

### 12. Add Workflow Complete Transition Logic

When the executor finishes all skills:

- If the workflow includes a `review` skill: transition to `StateReviewing` (awaiting user accept/reject)
- If the workflow ends with `commit` or `pr`: transition to `StateCompleted` directly
- Otherwise: transition to `StateCompleted`

```go
func terminalState(skills []string) run.State {
	if len(skills) == 0 {
		return run.StateCompleted
	}
	last := skills[len(skills)-1]
	switch last {
	case "review":
		return run.StateReviewing
	default:
		return run.StateCompleted
	}
}
```

### 13. Write Unit Tests

In `internal/engine/executor_test.go`:

- **TestResolveWorkflow**: Verify workflow resolution from config, unknown workflow error, empty workflow error.
- **TestValidateWorkflow**: Verify missing skill detection.
- **TestParseRouteResult**: Various route output formats (plain text, JSON).
- **TestParseDecomposeResult**: Valid JSON, empty tasks, malformed JSON.
- **TestGroupByParallel**: Correct grouping and dependency ordering.
- **TestTerminalState**: Correct terminal state for different workflow endings.
- **TestExecuteSequential**: Mock manager and registry, verify skill execution order, state transitions, and context passing. Use a mock `StartSkill()` that returns a channel with canned results.
- **TestExecuteWithFailure**: Verify run transitions to `StateFailed` with error message on skill failure.
- **TestExecuteWithTimeout**: Verify timeout handling for slow skills.
- **TestResumeFromCheckpoint**: Start a workflow, fail mid-way, resume, verify it continues from the correct skill.
- **TestExecuteCancel**: Start a workflow, cancel it, verify state transition.

In `internal/process/manager_skill_test.go`:

- **TestStartSkillReturnsChannel**: Verify `StartSkill()` returns a channel and the channel receives `SkillResult` on process exit.
- **TestStartSkillDoesNotSetTerminalState**: Verify the run state is NOT set to Completed/Failed by the manager when using `StartSkill()`.
- **TestStartSkillCapturesResultText**: Verify the result text from the stream-json `result` event is captured in `SkillResult.ResultText`.

In `internal/engine/testdata/decompose-result.json`:

```json
{
  "tasks": [
    {"name": "implement-auth-middleware", "parallel_group": "core", "dependencies": []},
    {"name": "implement-auth-routes", "parallel_group": "core", "dependencies": []},
    {"name": "implement-auth-ui", "parallel_group": "frontend", "dependencies": ["implement-auth-routes"]},
    {"name": "implement-auth-tests", "parallel_group": "testing", "dependencies": ["implement-auth-middleware", "implement-auth-routes"]}
  ]
}
```

## Testing Strategy

### Unit Tests

- **Workflow resolution**: Config lookup, unknown workflow, empty skills list
- **Decompose parsing**: Valid JSON, malformed JSON, empty tasks
- **Parallel grouping**: Independent groups, dependency ordering, single group
- **Route parsing**: Plain text workflow name, JSON format, unrecognized output
- **Sequential execution**: Mock manager → verify prompt assembly, skill ordering, context passing
- **Error propagation**: Skill failure → run state, error message
- **Timeout**: Per-skill timeout → context cancellation → error
- **Checkpoint/resume**: Fail at skill N, resume skips skills 0..N-1
- **Cancel**: External cancellation between skills and during a skill
- **Terminal state**: Correct state based on workflow ending skill

### Edge Cases

- **Workflow with single skill**: Should work without any context passing
- **Skill not in registry**: Fail immediately with clear error, not panic
- **All skills fail on first**: Run should be `StateFailed` with `SkillIndex=1`
- **Route skill returns unknown workflow**: Fall back to original workflow, log warning
- **Decompose returns empty task list**: Skip parallel phase, continue with next sequential skill
- **Decompose returns single-task groups**: Execute "parallel" group of 1 sequentially (no goroutine overhead)
- **Concurrent runs**: Two runs executing simultaneously should not interfere (separate contexts, separate ring buffers)
- **Pause during skill execution**: Process manager handles SIGSTOP; executor detects paused state between skills
- **Pause then cancel**: Cancel should work even if paused (context cancellation wakes `waitIfPaused`)
- **Resume already-completed run**: Should return error, not restart
- **Rate limit during skill**: Process manager auto-pauses; executor's `waitIfPaused` blocks until resume
- **Empty `PreviousOutput`**: First skill in workflow should work with empty previous output (already handled by `BuildPrompt`)
- **Very long result text**: Should be passed as-is; truncation is a future concern (noted but not implemented)

## Acceptance Criteria

- [ ] `StartSkill()` launches subprocess and returns `<-chan SkillResult` with result text on completion
- [ ] `StartSkill()` does NOT set run terminal state (Completed/Failed)
- [ ] `StartSkill()` still logs to ring buffer and tracks tokens/cost
- [ ] `ResolveWorkflow()` returns skill list for valid workflow, error for unknown
- [ ] `ValidateWorkflow()` detects missing skills in registry
- [ ] `Executor.Execute()` runs skills sequentially in correct order
- [ ] Each skill receives `PreviousOutput` from the preceding skill's result text
- [ ] `SkillIndex` and `CurrentSkill` update at each skill transition
- [ ] Run state transitions: `StateRunning` during execution, `StateCompleted`/`StateReviewing` on success, `StateFailed` on error
- [ ] Per-skill timeouts from config are enforced
- [ ] Route skill output overrides the workflow
- [ ] Decompose skill output is parsed and parallel groups execute concurrently
- [ ] `Cancel()` stops the executor goroutine and running subprocess
- [ ] `Resume()` restarts from the last incomplete skill (based on `SkillIndex`)
- [ ] Executor pauses between skills when run state is `StatePaused`
- [ ] TUI can launch a run that executes a real workflow via the executor
- [ ] All unit tests pass
- [ ] `go build ./...` succeeds with no errors
- [ ] `go vet ./...` reports no issues

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Build the project
go build ./...

# Run all tests
go test ./internal/engine/ -v -run TestExecutor
go test ./internal/engine/ -v -run TestResolveWorkflow
go test ./internal/engine/ -v -run TestParseDecompose
go test ./internal/engine/ -v -run TestParseRouteResult
go test ./internal/process/ -v -run TestStartSkill

# Run full test suite
go test ./...

# Vet for correctness
go vet ./...
```

## Notes

### Process Manager Extension Strategy

The key design decision is adding `StartSkill()` alongside the existing `Start()` rather than modifying `Start()`. This preserves backward compatibility — `Start()` continues to work for single-skill runs or direct process launches, while `StartSkill()` provides the executor with the control it needs. The two methods share almost all code; the only difference is how process exit is handled.

### Context Passing: The Sniper Pattern

Each skill gets only what it needs: the SKILL.md instructions, the worktree path, the branch name, a summary of the previous skill's output, and the user's task. There is no accumulated conversation history, no growing context window, no "memory" beyond what `PreviousOutput` carries. This is intentional — it keeps each skill invocation cheap and focused.

The tradeoff: if a skill needs detailed context from two skills ago, it must find that context in the worktree (where previous skills wrote files like SPEC.md) rather than in the prompt. This is a feature, not a bug — it forces skills to leave artifacts rather than relying on invisible context.

### Parallel Execution Complexity

Parallel execution (via decompose) is the most complex part of this feature. The current process manager tracks processes by run ID, but parallel execution means multiple processes per run. The spec handles this by using composite keys (`runID:taskName`). An alternative is to create sub-runs, but that adds complexity to the run store and TUI for marginal benefit.

If parallel execution proves too complex for the initial implementation, it can be deferred — the executor should work correctly with sequential-only execution first, and parallel support layered on afterward. The decompose types are already defined; parsing can be implemented without the parallel dispatch.

### Worktree Dependency

The executor references `r.Worktree` and `r.Branch` but actual git worktree creation is step 10. For this step, the executor should work with the project root as the "worktree" and a placeholder branch name. The TUI integration (step 11 in the implementation) sets these values before calling `Execute()`.

### Future: Rate Limit Detection

The current process manager auto-pauses on cost/token thresholds but doesn't specifically detect rate limits (HTTP 429). Rate limit detection would look for specific patterns in stderr or error events and trigger a backoff-then-resume cycle. This is noted for future work and not implemented in this step.
