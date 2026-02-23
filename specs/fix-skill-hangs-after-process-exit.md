# Fix: Skill subprocess hangs after process exits when using log files

## Metadata

type: `fix`
task_id: `skill-hangs-after-process-exit`
prompt: `Run 002 with auto workflow never progressed from the route skill to an actual workflow. The route skill consumed 1.3k tokens and $0.03 but the executor never advanced to the resolved workflow.`

## Bug Description

When a skill subprocess exits naturally (e.g., the route skill finishes its analysis), the executor hangs forever waiting for the result. The workflow never progresses past the completed skill. This affects all `StartSkill`-based skill execution when log files are enabled (which is the default when session persistence is active).

**What happens:** The route skill runs, produces output (1.3k tokens, $0.03), and the `claude` process exits. But the executor never receives the result, so the workflow stays stuck at "route (1/1)" with `Workflow: auto` — the route handling code that resolves and replaces the workflow is never reached.

**What should happen:** After the route skill completes and the `claude` process exits, the executor should receive the skill result, parse the workflow name from it, resolve the new workflow, and continue execution.

## Reproduction Steps

1. Start agtop with session persistence enabled (default)
2. Create a new run with workflow "auto"
3. Enter any prompt (e.g., "instead of one hotkey per model type...")
4. Observe: the route skill runs, tokens and cost accrue, but the run stays at "route (1/1)" indefinitely
5. The `Workflow` field never changes from "auto" to the resolved workflow name

**Expected behavior:** After the route skill completes (~5-10s for haiku), the workflow should change to the resolved name (e.g., "build") and execution should proceed with the first skill of that workflow.

## Root Cause Analysis

The deadlock is in `internal/process/manager.go` in `consumeSkillEvents` (line 584) and `consumeEvents` (line 663). Both functions have the same structural issue:

```go
func (m *Manager) consumeSkillEvents(..., done <-chan error, resultCh chan<- SkillResult) {
    // Phase 1: Process events from stream parser
    for event := range parser.Events() { ... }  // <-- BLOCKS FOREVER

    // Phase 2: Wait for process exit (never reached)
    var exitErr error
    select { case exitErr = <-done: ... }

    // Phase 3: Cancel FollowReader (never reached)
    mp.cancel()

    // Phase 4: Send result (never reached)
    resultCh <- SkillResult{...}
}
```

The deadlock chain:
1. `parser.Events()` channel only closes when `parser.Parse()` returns
2. `parser.Parse()` only returns when `scanner.Scan()` returns false
3. `scanner.Scan()` calls `FollowReader.Read()`, which polls on EOF until its context is cancelled
4. The context is cancelled by `mp.cancel()` — but that call is in Phase 3
5. Phase 3 is unreachable because Phase 1 blocks forever

When **pipes** are used (no log files), the pipe closes when the process exits, causing `Read()` to return `io.EOF` immediately, which breaks the scanner loop. When **FollowReader** is used (log files enabled), EOF triggers a poll loop that only stops when the context is cancelled — and nothing cancels it.

This bug was introduced by `e0dd58e feat: add log files for process reconnection`, which changed `StartSkill` to use log files + FollowReader instead of raw pipes.

The `consumeEvents` function has the same structure and is affected for reconnected processes that exit naturally (via the PID monitor `done` channel).

## Relevant Files

- `internal/process/manager.go` — Contains both `consumeSkillEvents` (line 584) and `consumeEvents` (line 663) with the deadlock. The fix goes here.
- `internal/process/logfile.go` — `FollowReader` implementation. Polls on EOF until context cancelled (line 34-51). No changes needed.
- `internal/process/manager_test.go` — Existing manager tests. Needs new test for the fix.
- `internal/engine/executor.go` — Calls `runSkill` → `StartSkill`. No changes needed but useful for understanding the call chain.

## Fix Strategy

Add a goroutine in both `consumeSkillEvents` and `consumeEvents` that monitors the `done` channel and calls `mp.cancel()` when the process exits. This breaks the FollowReader poll loop, causing the stream parser to drain, closing the events channel, and unblocking the event loop.

The goroutine captures the exit error so the main flow can use it after the event loop exits.

### In `consumeSkillEvents`:

```go
func (m *Manager) consumeSkillEvents(runID string, mp *ManagedProcess, buf *RingBuffer, eb *EntryBuffer, stdout io.Reader, stderr io.Reader, done <-chan error, resultCh chan<- SkillResult) {
    defer close(resultCh)

    var resultText string

    // When the process exits, cancel the FollowReader so the parser drains.
    var exitErr error
    exitDone := make(chan struct{})
    go func() {
        exitErr = <-done
        close(exitDone)
        mp.cancel()
    }()

    // ... existing event processing loop (unchanged) ...

    for event := range parser.Events() {
        // ... existing event handling (unchanged) ...
    }

    // Wait for exit goroutine to complete (instant — it already fired
    // since the event loop only exits after mp.cancel() is called).
    <-exitDone

    // If the TUI is shutting down, preserve state for reconnection.
    if m.isDisconnecting() {
        resultCh <- SkillResult{ResultText: resultText, Err: ErrDisconnected}
        return
    }

    m.store.Update(runID, func(r *run.Run) {
        r.PID = 0
    })

    m.mu.Lock()
    delete(m.processes, runID)
    m.mu.Unlock()

    resultCh <- SkillResult{ResultText: resultText, Err: exitErr}
}
```

### In `consumeEvents`:

Apply the same pattern: goroutine watches `done`, calls `mp.cancel()`, captures exit error.

### Safety considerations:

- `exitErr` is written by the goroutine before `close(exitDone)`, and read by the main goroutine after `<-exitDone`. The channel synchronization provides a happens-before guarantee — no mutex needed.
- `mp.cancel()` is safe to call multiple times (it's a `context.CancelFunc`).
- The stderr goroutine is also unblocked by `mp.cancel()` since it reads from a FollowReader with the same context.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Fix `consumeSkillEvents` deadlock

- In `internal/process/manager.go`, refactor `consumeSkillEvents` (line 584):
  - Add a goroutine before the event loop that reads from `done`, stores the exit error, closes a signal channel, and calls `mp.cancel()`
  - Remove the `select { case exitErr = <-done: ... }` block after the event loop
  - Replace it with `<-exitDone` to wait for the goroutine
  - Use the captured `exitErr` in the result

### 2. Fix `consumeEvents` deadlock

- In `internal/process/manager.go`, refactor `consumeEvents` (line 663):
  - Apply the same goroutine pattern as `consumeSkillEvents`
  - Remove the `select { case exitErr = <-done: ... }` block
  - Replace with `<-exitDone` and use captured exit error

### 3. Add regression test

- In `internal/process/manager_test.go` or `internal/process/manager_skill_test.go`:
  - Create a test that starts a skill subprocess using log files (not pipes)
  - Have the mock process write a result event and exit immediately
  - Assert that the result channel receives the result within a reasonable timeout (e.g., 5s)
  - Without the fix, this test would hang forever

### 4. Verify existing tests pass

- Run `go test ./internal/process/...` — existing tests should still pass
- Run `go test ./internal/engine/...` — executor tests should still pass
- Run `go vet ./...` — no lint issues

## Regression Testing

### Tests to Add

- Test in `manager_skill_test.go`: `TestStartSkillWithLogFilesCompletes` — mock process writes result and exits; verify the result channel receives within timeout. Uses a temp directory as `sessionsDir` to enable log files.

### Existing Tests to Verify

- `internal/process/manager_test.go` — All existing manager tests
- `internal/process/manager_skill_test.go` — All existing skill manager tests
- `internal/engine/executor_test.go` — `TestExecutorShutdownPreservesRunState`, `TestExecutorResumeReconnectedSkillIndex`, and all route/workflow tests

## Risk Assessment

- **Low risk**: The fix adds a goroutine that monitors an existing channel and calls an existing cancel function. No new data structures or synchronization primitives beyond a simple signal channel.
- **mp.cancel() idempotency**: `context.CancelFunc` is safe to call multiple times. External callers (Stop, Kill, DisconnectAll) may also call `mp.cancel()` — this is fine.
- **Reconnected process monitoring**: The same fix in `consumeEvents` ensures reconnected processes that exit naturally also unblock properly.

## Validation Commands

```bash
go test ./internal/process/...
go test ./internal/engine/...
go vet ./...
```

## Open Questions (Unresolved)

None — the root cause and fix are clear.
