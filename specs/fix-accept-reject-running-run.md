# Fix: Accept/reject allowed on a running run

## Metadata

type: `fix`
task_id: `accept-reject-running-run`
prompt: `Should not be able to accept or deny a currently running run`

## Bug Description

A user can press `a` (accept) or `x` (reject) on a run that is still actively being processed, and the action goes through. The expected behavior is that accept and reject are only possible after the run's workflow has fully completed and no executor worker is active for it.

**What happens:** Accept/reject succeeds on a run whose agent process is still executing or whose executor worker is still running between skills.

**What should happen:** Accept/reject should be blocked whenever the run has an active executor worker, regardless of the `State` field value.

## Reproduction Steps

1. Start a multi-skill workflow (e.g., `plan-build` with skills `["spec", "build", "test", "review"]`)
2. While the run is actively executing, press `a` to accept
3. Observe: the action may succeed instead of being blocked

**Expected behavior:** Flash message "Cannot accept: run is running" and no state change.

## Root Cause Analysis

The guards in `handleAccept()` (`internal/ui/app.go:928-933`) and `handleReject()` (`internal/ui/app.go:986-989`) check the run's `State` field from the `RunList.SelectedRun()` cache. This has two weaknesses:

### 1. Stale cached state (`internal/ui/panels/runlist.go:290-295`)

`SelectedRun()` returns a copy from `RunList.filtered`, which is only refreshed when `RunStoreUpdatedMsg` is processed. The store can be mutated by goroutines (process manager, executor) between `RunStoreUpdatedMsg` refreshes. The notification channel (`changeCh`) is buffered with size 1 and uses non-blocking sends — so rapid successive store updates can coalesce, leaving the filtered list briefly stale.

### 2. State alone is insufficient

The `State` field doesn't reliably indicate whether work is still in progress:

- **Between skills in a workflow**: After a skill subprocess exits, `consumeSkillEvents` (`internal/process/manager.go:614-616`) clears the PID and sends the result. The executor then does post-processing (auto-commit at `executor.go:454-456`) before looping to the next skill. If a `RunStoreUpdatedMsg` happens to refresh the cache during this window, the run may appear idle.
- **Reconnected runs**: `Reconnect()` (`internal/process/manager.go:416`) uses `consumeEvents()` which sets `State = StateCompleted` on process exit (`manager.go:675`). If the executor's `ResumeReconnected` worker is still active, the state says "completed" but work continues.
- **Post-workflow processing**: After the final skill, the executor calls `commitAfterStep` and sets the terminal state (`executor.go:510-515`). Between the last skill exiting and the terminal state being written, there's a brief window.

The fundamental issue: **the guard should check whether the executor has an active worker for the run, not just the `State` field.**

## Relevant Files

- `internal/ui/app.go` — `handleAccept()` (line 921) and `handleReject()` (line 980) contain the guards to fix
- `internal/engine/executor.go` — `active` map (line 26) tracks running workers; needs a public query method
- `internal/ui/panels/runlist.go` — `SelectedRun()` (line 290) returns stale copies from cache
- `internal/run/store.go` — `Store.Get()` (line 50) for fresh reads
- `internal/run/run.go` — `Run` struct and `State` constants

## Fix Strategy

Two-part fix: (A) add a way to check whether the executor has an active worker for a run, and (B) use that check in the accept/reject guards alongside a fresh store read.

### Part A: Expose executor activity check

Add an `IsActive(runID string) bool` method to `Executor` that checks the `active` map under the mutex. This is the authoritative source of whether a workflow goroutine is still running for a given run.

### Part B: Strengthen the guards

In both `handleAccept()` and `handleReject()`:
1. Read the run state fresh from the store (`a.store.Get(runID)`) instead of relying on the cached `SelectedRun()` copy for state checks
2. Before the existing state check, add an executor activity check: if `a.executor.IsActive(runID)`, block with a flash message

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `IsActive` method to Executor

- In `internal/engine/executor.go`, add a public method:

```go
func (e *Executor) IsActive(runID string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.active[runID]
	return ok
}
```

### 2. Update `handleAccept()` guard

- In `internal/ui/app.go`, modify `handleAccept()` (starting at line 921):
  - Keep using `a.runList.SelectedRun()` for the nil/no-selection check and to get the `runID`
  - After extracting `runID`, add an executor activity check:
    ```go
    if a.executor != nil && a.executor.IsActive(runID) {
        a.statusBar.SetFlashWithLevel("Cannot accept: run is still executing", panels.FlashError)
        return a, flashClearCmd()
    }
    ```
  - After the executor check, re-read state fresh from the store for the existing state guard:
    ```go
    fresh, ok := a.store.Get(runID)
    if !ok {
        return a, nil
    }
    ```
  - Replace `selected.State` with `fresh.State` in the existing guard condition

### 3. Update `handleReject()` guard

- In `internal/ui/app.go`, modify `handleReject()` (starting at line 980):
  - Same pattern as accept: after extracting `runID`, check `a.executor.IsActive(runID)`
  - Re-read fresh state from the store
  - Replace `selected.State` with `fresh.State` in the guard condition

### 4. Add unit tests

- In `internal/engine/executor_test.go`, add a test for `IsActive()`:
  - Start a workflow, verify `IsActive` returns `true` while running
  - After workflow completes, verify `IsActive` returns `false`

- In `internal/ui/app_test.go`, add tests for the strengthened guards:
  - Test that accept is blocked when executor reports the run as active
  - Test that reject is blocked when executor reports the run as active
  - Test that accept/reject still work normally for completed runs with no active executor worker

## Regression Testing

### Tests to Add

- `TestIsActive` in `internal/engine/executor_test.go` — verifies `IsActive` returns correct values during and after workflow execution
- `TestAcceptBlockedWhileExecutorActive` in `internal/ui/app_test.go` — verifies accept is blocked when executor has an active worker
- `TestRejectBlockedWhileExecutorActive` in `internal/ui/app_test.go` — same for reject

### Existing Tests to Verify

- `TestAcceptCompletedRun` in `internal/ui/app_test.go` — must still pass (accept works for genuinely completed runs)
- All existing executor tests in `internal/engine/executor_test.go`
- `make test` — full test suite

## Risk Assessment

- **Low risk**: The `active` map already exists and is already mutex-protected. The `IsActive` method is a simple read.
- **Behavioral change**: Accept/reject will be slightly more restrictive. A run must have no active executor worker AND be in a valid state. This is strictly more correct.
- **Nil executor**: The app may run without an executor (`a.executor == nil` when the engine is disabled). The guard must check for nil before calling `IsActive`.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
make check
```

## Open Questions (Unresolved)

None — the fix is targeted and minimal.
