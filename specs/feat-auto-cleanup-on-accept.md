# Feature: Auto-cleanup Runs After Successful Accept and Merge

## Metadata

type: `feat`
task_id: `auto-cleanup-on-accept`
prompt: `Upon successful accept and merge, clean up the run from the runs list along with its associated worktree, session file, log files, and buffers.`

## Feature Description

When a user accepts a run (presses `a`), the run undergoes a merge flow — either the auto-merge pipeline (rebase → push → PR → checks → merge) or the legacy local merge. On success, the run transitions to `StateAccepted` and its worktree is removed, but the run stays in the run list indefinitely. The user must manually press `d` to delete it.

This feature adds automatic cleanup after a successful accept-and-merge: the run is removed from the run list, its session file and log files are deleted, and its process manager buffers are freed. The user sees a brief flash message confirming the merge and cleanup, then the run disappears.

## User Story

As an agtop user
I want completed and merged runs to be automatically cleaned up from the dashboard
So that the run list stays focused on active and pending work without manual housekeeping

## Relevant Files

- `internal/ui/app.go` — `handleAccept()` (lines 763–823) orchestrates the accept flow for both auto-merge pipeline and legacy merge paths. `handleDelete()` (lines 989–1022) contains the full cleanup logic that needs to be extracted and reused. The goroutines in `handleAccept` need to call this cleanup logic after a successful merge.
- `internal/engine/pipeline.go` — `Pipeline.Run()` (lines 37–133) executes the auto-merge pipeline. After success it sets `StateAccepted` and `MergeStatus = "merged"`. The caller in `app.go` checks this state to decide whether to clean up.
- `internal/run/store.go` — `Store.Remove()` removes a run from the in-memory store and notifies subscribers.
- `internal/run/persistence.go` — `Persistence.Remove()` deletes the session JSON file for a run.
- `internal/process/manager.go` — `Manager.RemoveBuffer()` cleans up ring buffers, entry buffers, and log file handles. `Manager.LogFilePaths()` returns log file paths for cleanup.
- `internal/process/logfile.go` — `RemoveLogFiles()` deletes stdout/stderr log files from disk.
- `internal/git/worktree.go` — `WorktreeManager.Remove()` removes the git worktree and deletes the branch.
- `internal/server/devserver.go` — `DevServerManager.Stop()` stops a dev server for a run.

## Implementation Plan

### Phase 1: Extract Cleanup Helper

Extract the cleanup logic from `handleDelete()` into a reusable method (`cleanupRun`) on the `App` struct. This method takes a run ID and performs all cleanup steps: stop dev server, remove buffers, remove from store, remove worktree, remove persistence file, remove log files. The method must be safe to call from goroutines (all underlying operations are already thread-safe).

### Phase 2: Wire Cleanup Into Accept Flows

After a successful merge in both the auto-merge pipeline path and the legacy merge path, call the cleanup helper instead of only removing the worktree. Add a brief flash message so the user knows the merge succeeded.

### Phase 3: Update Tests

Update existing tests to account for the new behavior: accepted runs are removed from the store after merge. Add a test for the cleanup helper.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Extract `cleanupRun` helper from `handleDelete`

In `internal/ui/app.go`:

- Create a new method `cleanupRun(runID string)` on the `App` struct that performs:
  1. Get log file paths from manager (before removing buffers)
  2. Remove buffers from manager (`RemoveBuffer`)
  3. Stop dev server (`devServers.Stop`)
  4. Remove from store (`store.Remove`)
  5. In a goroutine: remove worktree (`worktrees.Remove`), remove persistence file (`persistence.Remove`), remove log files (`process.RemoveLogFiles`)
- Refactor `handleDelete` to call `cleanupRun` instead of duplicating the logic.

### 2. Add cleanup to auto-merge pipeline success path

In `internal/ui/app.go`, `handleAccept()`, in the auto-merge pipeline goroutine (around line 790–799):

- After `pipeline.Run(ctx, runID)` succeeds and `r.State == run.StateAccepted`:
  - Replace the current worktree-only cleanup with a call to `cleanupRun(runID)`.
  - Note: `cleanupRun` handles worktree removal internally, so remove the existing `worktrees.Remove` and `store.Update` calls.

### 3. Add cleanup to legacy merge success path

In `internal/ui/app.go`, `handleAccept()`, in the legacy merge goroutine (around line 808–819):

- After `worktrees.Merge(runID)` succeeds:
  - Replace the existing worktree-only cleanup with a call to `cleanupRun(runID)`.
  - Note: `cleanupRun` handles worktree removal internally.

### 4. Update flash messages

- Auto-merge path: change flash from "Merge pipeline started" to keep as-is (the pipeline runs async). The cleanup happens silently after completion.
- Legacy path: flash "Merged and cleaned up" instead of "Merging into current branch..." after the merge completes. Since the merge is async, update the flash from within the goroutine by sending a message through the tea.Program.

### 5. Update unit tests

- In `internal/ui/app_test.go`: update any tests that verify post-accept state to account for the run being removed from the store.
- Verify `cleanupRun` is safe when called with a run that has no buffers, no persistence, or no worktree (nil-safety for all optional subsystems).

## Testing Strategy

### Unit Tests

- Test `cleanupRun` with a fully populated run (buffers, log files, worktree, persistence) — verify all artifacts are removed.
- Test `cleanupRun` with minimal run (no manager, no persistence) — verify it doesn't panic.
- Test that `handleDelete` still works correctly after refactoring to use `cleanupRun`.
- Test accept → merge → cleanup flow for both pipeline and legacy paths.

### Edge Cases

- Run with no worktree (already removed) — cleanup should handle gracefully.
- Run with no log files (old session format) — cleanup should handle gracefully.
- Run with active dev server — dev server should be stopped before cleanup.
- Pipeline merge failure — run should NOT be cleaned up (stays in list with error).
- Legacy merge failure — run should NOT be cleaned up (stays in list with error).
- Concurrent cleanup calls for the same run — should not panic or double-delete.

## Risk Assessment

- **Run disappearing unexpectedly**: After a successful merge, the run vanishes from the list. This is the intended behavior per the user story, but could surprise users who want to review the merged run. The flash message provides visual confirmation.
- **Race condition with store listeners**: The persistence `BindStore` subscriber might try to save a run that was just removed. `Persistence.Save` handles empty IDs and `Store.List` won't include removed runs, so this is safe.
- **handleDelete regression**: Refactoring `handleDelete` to use `cleanupRun` could change behavior. The existing `handleDelete` flash message and return value must be preserved.

## Validation Commands

```bash
go vet ./...
go test ./internal/ui/... -count=1
go test ./internal/run/... -count=1
go test ./internal/process/... -count=1
make update-golden    # if golden snapshots change
```

## Open Questions (Unresolved)

None — the behavior is well-defined. Auto-cleanup on successful merge with flash confirmation.

## Sub-Tasks

Single task — no decomposition needed.
