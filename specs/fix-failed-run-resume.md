# Fix: Prevent Resuming Truly Failed Runs via Space Key

## Metadata

type: `fix`
task_id: `failed-run-resume`
prompt: `Should not be able to press space and resume a failed run if it's really failed. If it's just hung somehow it should be marked as paused/suspended so that it can be resumed, but if a run has really failed it should not be able to be resumed.`

## Bug Description

**What happens:** Pressing `Space` on a run in `StateFailed` calls `executor.Resume()`, which restarts the workflow from the failed skill. This means any run that exits with a non-zero status — including genuinely fatal errors — can be "resumed" as if it were just paused.

**What should happen:** `StateFailed` should be a true terminal state. Space should not resume failed runs. If a process is hung/unresponsive but hasn't actually exited, it should remain in `StatePaused` (or a new suspended state) so that `Space` can resume it. The `r` (restart) key already exists for re-running terminal runs from scratch, which is the appropriate action for failed runs.

## Reproduction Steps

1. Start agtop and launch a run that will fail (e.g., a workflow where a skill errors out)
2. Wait for the run to reach `StateFailed`
3. Press `Space` on the failed run
4. Observe: the run resumes from the failed skill instead of showing an error

**Expected behavior:** Status bar shows "Cannot resume: run has failed" (or similar). The user should use `r` to restart the run from scratch if desired.

## Root Cause Analysis

The issue is in `handleTogglePause()` at `internal/ui/app.go:1074-1082`. The space key handler has an explicit `case run.StateFailed:` branch that calls `executor.Resume()`, treating failed runs as resumable:

```go
case run.StateFailed:
    if a.executor != nil {
        if err := a.executor.Resume(selected.ID, selected.Prompt); err != nil {
            ...
        }
        return a, nil
    }
    return a, nil
```

Additionally, `Executor.Resume()` at `internal/engine/executor.go:157` explicitly accepts `StateFailed`:

```go
if r.State != run.StateFailed && r.State != run.StatePaused {
    return fmt.Errorf("run %s is %s, not resumable", runID, r.State)
}
```

The design conflates two different scenarios:
1. **Truly failed** — process exited with an error (non-zero exit, skill error, process died). These should not be resumable via space.
2. **Hung/suspended** — process is alive but unresponsive. These should be `StatePaused`, not `StateFailed`.

## Relevant Files

- `internal/ui/app.go` — `handleTogglePause()` (line 1046) contains the `StateFailed` branch that needs to be removed
- `internal/engine/executor.go` — `Resume()` (line 152) accepts `StateFailed` and needs to only accept `StatePaused`
- `internal/run/run.go` — State constants and `IsTerminal()` (line 75) — `StateFailed` is already marked terminal, which is correct
- `internal/process/manager.go` — `Pause()` / `Resume()` (lines 229-269) — process-level pause/resume, no changes needed
- `internal/ui/panels/help.go` — Help overlay (line 56) shows "Space → Pause / Resume" — no change needed since it still pauses/resumes running/paused runs

## Fix Strategy

The fix is minimal — remove the `StateFailed` case from `handleTogglePause()` and tighten the executor's `Resume()` guard. Failed runs already fall through to the `default:` case which displays an appropriate error message.

For the "hung process" scenario: if a process is still alive but the user wants to suspend it, that's already handled by the `StateRunning → Pause` path (space on a running process sends SIGSTOP and transitions to `StatePaused`). If a process appears hung, the user presses space to pause it (SIGSTOP), then space again to resume (SIGCONT). No new state is needed — the existing `StatePaused` already covers this.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Remove `StateFailed` branch from `handleTogglePause()`

- In `internal/ui/app.go`, remove the `case run.StateFailed:` block (lines 1074-1082) from the `handleTogglePause()` method
- Failed runs will now fall through to the `default:` case, which displays: `"Cannot pause/resume: run is {state}"`

### 2. Tighten `Executor.Resume()` to reject failed runs

- In `internal/engine/executor.go`, change the guard at line 157 from:
  ```go
  if r.State != run.StateFailed && r.State != run.StatePaused {
  ```
  to:
  ```go
  if r.State != run.StatePaused {
  ```
- Update the comment on line 151 from "Resume restarts a failed or paused run" to "Resume restarts a paused run from its last incomplete skill."

### 3. Add unit test for `handleTogglePause` rejecting failed runs

- In `internal/ui/app_test.go`, add a test that verifies pressing space on a `StateFailed` run does **not** call `executor.Resume()` and instead produces a flash error message
- If no test infrastructure exists for this handler, add a test in `internal/engine/executor_test.go` that verifies `Executor.Resume()` returns an error when called with a `StateFailed` run

### 4. Add unit test for `Executor.Resume()` guard

- In `internal/engine/executor_test.go`, add a test case that calls `Resume()` on a run in `StateFailed` and asserts it returns an error with "not resumable"
- Verify the existing test (if any) for resuming a `StatePaused` run still passes

## Regression Testing

### Tests to Add

- `TestExecutorResumeRejectsFailedRun` — calls `Resume()` on a `StateFailed` run, expects error
- `TestExecutorResumePausedRun` — calls `Resume()` on a `StatePaused` run, expects success (ensures the tightened guard didn't break paused resume)

### Existing Tests to Verify

- `go test ./internal/engine/...` — all executor tests
- `go test ./internal/ui/...` — all UI tests
- `go test ./internal/process/...` — process manager tests (pause/resume unaffected)

## Risk Assessment

**Low risk.** The change removes a code path rather than adding one. The `r` (restart) key at `handleRestart()` (line 1090) already handles re-running terminal runs, so users have a clear alternative. The only behavioral change is that space no longer resumes failed runs — it shows an error instead.

**Edge case:** If users were relying on space to retry failed workflow skills, they'll need to use `r` (restart) instead, which restarts from the beginning rather than from the failed skill. This is intentional — a failed skill likely needs a fresh start, not a retry from a potentially corrupt intermediate state.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
make check
```

## Open Questions (Unresolved)

- **Should the default error message be more specific for failed runs?** Currently the default case says `"Cannot pause/resume: run is failed"`. A more helpful message might be `"Run has failed — use 'r' to restart"`. **Recommendation:** Yes, add a specific `case run.StateFailed:` that shows a hint about using `r` to restart, rather than the generic default message.
