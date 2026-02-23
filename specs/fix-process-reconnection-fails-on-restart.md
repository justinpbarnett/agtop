# Fix: Process reconnection marks live processes as failed on TUI restart

## Metadata

type: `fix`
task_id: `process-reconnection-fails-on-restart`
prompt: `Runs fail with "process no longer running (agtop restarted)" when the TUI is closed and restarted, even though the underlying agent process is still alive. The persistent process reconnection feature was supposed to fix this but the bug persists.`

## Bug Description

**What happens:** When agtop TUI exits while a run is in progress (e.g., at the route step) and is restarted, the run is immediately marked as `failed` with the error "process no longer running (agtop restarted)" — even though the underlying agent process (e.g., `claude -p`) is still running.

**What should happen:** The run should reconnect to the still-running agent process via its log files and continue the workflow from where it left off.

## Reproduction Steps

1. Start agtop and create a new run with workflow `auto`
2. While the run is executing the route skill (or any skill), close the TUI with `q` or `Ctrl+C`
3. Immediately restart agtop
4. Observe: the run shows `Status: failed` with `Error: process no longer running (agtop restarted)`
5. Verify the agent process is actually still running: `ps aux | grep claude`

**Expected behavior:** The run should be detected as alive via `IsProcessAlive(pid)`, reconnected via log file tailing, and continue executing.

## Root Cause Analysis

There are two independent bugs that both cause this failure:

### Bug 1: Executor context cancellation on TUI exit

When the TUI exits, `cleanup()` at `internal/ui/app.go:608-616` calls `manager.DisconnectAll()`. This cancels all `ManagedProcess.cancel` contexts, which terminates the `FollowReader` goroutines. However, the **executor goroutine** (`executeWorkflow` at `internal/engine/executor.go:73-81`) is also running with its own context stored in `executor.active[runID]`. This context is **never cancelled** by the TUI cleanup — it remains alive as a dangling goroutine.

When `consumeSkillEvents` finishes (because its `FollowReader` stops due to context cancellation from `DisconnectAll`), it:
1. Sets `r.PID = 0` (`manager.go:614-616`)
2. Removes the process from `m.processes` (`manager.go:618-620`)
3. Sends a result on `resultCh` (`manager.go:622`)

The executor's `runSkill` receives this result, sees `result.Err != nil` (because the FollowReader cancellation propagates as an error through the `done` channel), and marks the run as **failed** (`executor.go:290-296`). The persistence layer then saves this failed state to the session file.

When agtop restarts and rehydrates, it loads the session file that now has `State: failed` and `PID: 0`. Since `IsTerminal()` returns true for `StateFailed`, the rehydration path at `persistence.go:254` (`!r.IsTerminal()`) skips the PID check entirely — the run is just loaded as-is in its failed state.

**This is the primary bug:** The agent subprocess is still alive, but the run was already marked failed before the session file was saved, so reconnection never gets a chance to happen.

### Bug 2: Race between DisconnectAll and session save

Even if Bug 1 were fixed, there's a secondary timing issue. The sequence is:

1. `cleanup()` calls `manager.DisconnectAll()` — cancels all FollowReader contexts
2. `consumeSkillEvents` goroutine detects context cancellation, sets `PID = 0`, removes from processes map
3. The persistence `BindStore` subscriber fires due to the state change and saves `PID: 0` to disk
4. TUI process exits

On restart, even if the run's state were still `running`, the saved PID is 0, so `IsProcessAlive(0)` returns false, and the run is marked failed.

### Bug 3: Reconnect silent failure (latent)

In `Manager.Reconnect()` at `manager.go:334-388`, if opening the log files fails, it logs a warning and returns without updating the run state. The run remains in `StateRunning` in the store but has no process tracking — it's orphaned. This is a latent bug that would surface once Bugs 1 and 2 are fixed.

## Relevant Files

- `internal/ui/app.go` — `cleanup()` method at line 608. Needs to coordinate executor shutdown before disconnecting processes. Also where rehydration triggers executor resume for reconnected runs.
- `internal/engine/executor.go` — `Execute()`, `executeWorkflow()`, `active` map. Needs a shutdown mechanism that preserves run state (not marking as failed) when TUI exits. Needs resume logic for reconnected runs on startup.
- `internal/process/manager.go` — `DisconnectAll()` at line 446, `consumeSkillEvents()` at line 553, `Reconnect()` at line 334. `DisconnectAll` needs to prevent `consumeSkillEvents` from marking the run as failed. `Reconnect` needs to handle file-open failures gracefully.
- `internal/run/persistence.go` — `Rehydrate()` at line 237. Needs to communicate which runs were reconnected so the executor can resume their workflows.
- `internal/run/run.go` — `Run` struct, `IsTerminal()`. No changes needed.

## Fix Strategy

The fix targets the clean shutdown path so that when the TUI exits, running processes are left undisturbed and the session file preserves their live state (PID, running state) for reconnection on restart.

### Phase 1: Graceful executor shutdown

Add a `Shutdown()` method to `Executor` that cancels all active workflow goroutines and waits for them to exit. The `executeWorkflow` loop must distinguish between a "TUI shutting down" cancellation (preserve state) vs. a "user cancelled" cancellation (mark as failed).

### Phase 2: Prevent PID zeroing on disconnect

When `DisconnectAll()` is called, `consumeSkillEvents` must not zero the PID or remove the process from the map. Introduce a "disconnecting" flag that `consumeSkillEvents` checks before modifying run state.

### Phase 3: Coordinate cleanup ordering

`app.cleanup()` must: (1) shut down the executor first (stopping workflow goroutines), (2) then disconnect processes (stopping FollowReaders), (3) then do a final session save with the preserved state.

### Phase 4: Resume reconnected workflows on startup

After rehydration, any run that was successfully reconnected (live PID + log files) and has `State == running` with an active workflow needs the executor to resume its workflow from the current skill index.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add disconnecting flag to Manager

- In `internal/process/manager.go`, add a `disconnecting bool` field to the `Manager` struct.
- Add a `SetDisconnecting()` method that sets this flag under the mutex.
- In `consumeSkillEvents()`, after the `done` channel fires, check `m.disconnecting`. If true: do NOT set `r.PID = 0`, do NOT set run state to failed, do NOT remove from `m.processes`. Just cancel the FollowReader context and return the result on `resultCh` with a sentinel error (e.g., `ErrDisconnected`).
- Same change in `consumeEvents()` for non-skill runs.

### 2. Add graceful shutdown to Executor

- In `internal/engine/executor.go`, add a `Shutdown()` method that:
  - Sets a `shuttingDown` flag.
  - Cancels all contexts in `e.active`.
  - Waits for all goroutines to drain (use a `sync.WaitGroup`).
- In `executeWorkflow()`, when the context is cancelled, check `e.shuttingDown`. If true: leave the run in its current state (don't mark as failed). Just exit the goroutine cleanly.
- In `runSkill()`, when `StartSkill` returns `ErrDisconnected`, return it without marking the run as failed.

### 3. Update cleanup ordering in App

- In `internal/ui/app.go` `cleanup()`:
  1. Call `a.executor.Shutdown()` first — this cancels workflow goroutines and waits for them to finish.
  2. Call `a.manager.SetDisconnecting()` — sets the flag before disconnecting.
  3. Call `a.manager.DisconnectAll()` — cancels FollowReaders.
  4. Call `a.persistence.FinalSave(a.store)` — do a final synchronous save of all runs (new method, see step 4).
  5. Then existing cleanup (devServers, pidWatchCancel).

### 4. Add FinalSave to Persistence

- In `internal/run/persistence.go`, add a `FinalSave(store *Store, getLogPaths func(runID string) (string, string))` method that synchronously saves all non-terminal runs with their current state (bypassing debounce). This ensures the last-known-good state (with PID intact) is on disk before the process exits.

### 5. Resume reconnected workflows on startup

- In `internal/run/persistence.go`, have `Rehydrate()` return reconnected run IDs (not just watchIDs).
- In `internal/ui/app.go` `NewApp()`, after rehydration, for each reconnected run that has `State == running` and a non-empty `Workflow`, call `executor.ResumeReconnected(runID)`.
- In `internal/engine/executor.go`, add `ResumeReconnected(runID string)` that reads the run's current workflow and skill index, then continues execution from that point. This is similar to `Resume()` but doesn't change state or clear errors — the run is already running.

### 6. Fix Reconnect silent failure

- In `internal/process/manager.go` `Reconnect()`, if opening log files fails, update the run state to `StateFailed` with an error message instead of silently returning.

## Regression Testing

### Tests to Add

- `internal/process/manager_test.go`: Test that `consumeSkillEvents` does NOT zero PID when `disconnecting` is true.
- `internal/process/manager_test.go`: Test that `consumeEvents` does NOT set terminal state when `disconnecting` is true.
- `internal/engine/executor_test.go`: Test that `Shutdown()` leaves running runs in `StateRunning` (not `StateFailed`).
- `internal/engine/executor_test.go`: Test that `ResumeReconnected()` picks up from the correct skill index.
- `internal/run/persistence_test.go`: Test `FinalSave` writes all non-terminal runs synchronously.
- `internal/run/persistence_test.go`: Test round-trip: save with PID → load → verify PID preserved and run not marked failed.

### Existing Tests to Verify

- `go test ./internal/process/...` — all existing manager tests must pass.
- `go test ./internal/engine/...` — all existing executor tests must pass.
- `go test ./internal/run/...` — all existing persistence and store tests must pass.
- `go test ./internal/ui/...` — all existing UI tests must pass (cleanup path changed).
- `make lint` — no new vet warnings.

## Risk Assessment

- **Executor WaitGroup drain**: If a skill subprocess hangs, `Shutdown()` could block TUI exit. Mitigate with a timeout (e.g., 2 seconds) on the WaitGroup wait.
- **Orphaned executor goroutines**: If the shutdown flag isn't checked correctly, executor goroutines could leak. Test thoroughly.
- **FinalSave race**: The final save must happen after the executor and manager have fully quiesced to avoid saving intermediate states. The sequential ordering in cleanup handles this.
- **Reconnected workflow skill mismatch**: If the skill registry changes between sessions, `ResumeReconnected` could try to run a skill that no longer exists. This already happens with `Resume()` and fails gracefully.

## Validation Commands

```bash
go test ./internal/process/... -v
go test ./internal/engine/... -v
go test ./internal/run/... -v
go test ./internal/ui/... -v
make lint
make build
```

## Open Questions (Unresolved)

- **Should failed reconnection auto-retry?** If `Reconnect()` fails to open log files, should it retry after a delay, or just fail immediately? **Recommendation:** Fail immediately with a clear error — the user can manually restart the run. Retrying adds complexity for an edge case.
- **Should `Shutdown()` have a hard timeout?** If the executor's WaitGroup doesn't drain within N seconds, should we force-exit? **Recommendation:** Yes, use a 3-second timeout. Log a warning if it fires, then proceed with cleanup anyway.
