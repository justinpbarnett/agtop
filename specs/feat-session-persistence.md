# Feature: Session Persistence and Recovery

## Metadata

type: `feat`
task_id: `session-persistence`
prompt: `Implement session persistence so run state is serialized to disk on every transition and rehydrated on startup, with PID-based process reconnection and an agtop cleanup subcommand for stale sessions and orphaned worktrees.`

## Feature Description

Add session persistence to agtop so that run state survives process restarts. Every state transition serializes the run to a JSON file at `~/.agtop/sessions/<project-hash>/<run-id>.json`. On startup, agtop discovers session files for the current project, rehydrates them into the run store, checks PID liveness for non-terminal runs, and marks dead processes as failed. A new `agtop cleanup` subcommand removes stale session files and orphaned git worktrees.

## User Story

As a developer running concurrent AI agent workflows
I want my run history and state to persist across agtop restarts
So that I don't lose visibility into completed runs, cost data, and log context when I close and reopen agtop.

## Problem Statement

Currently all run state is in-memory only. When agtop exits:

1. **All run history is lost.** Completed runs, their cost data, skill breakdowns, and log buffers vanish entirely. The mock data in `seedMockData()` confirms this — the store starts empty every time.
2. **Active runs become orphaned.** If a `claude -p` process is still running when agtop exits, there's no way to know about it on restart. The worktree remains, the process may still be running, but agtop has no record of it.
3. **No cleanup mechanism.** Orphaned worktrees from crashed sessions accumulate in `.agtop/worktrees/` with no way to discover or remove them.
4. **Cost tracking resets.** The cost tracker's session totals (`sessionTokens`, `sessionCost`) and per-run cost breakdowns are lost on every restart.

## Solution Statement

Implement a persistence layer in `internal/run/persistence.go` that:

1. **Serializes on every mutation** by subscribing to `store.Subscribe()`. Each run is written as a self-contained JSON file with the run state plus a log tail snapshot.
2. **Rehydrates on startup** by scanning `~/.agtop/sessions/<project-hash>/` for JSON files, deserializing them into the store, and restoring log buffers.
3. **Checks PID liveness** for non-terminal runs using `syscall.Kill(pid, 0)`. Dead processes are marked as `failed`. Live PIDs are monitored with a polling goroutine (since stdout/stderr pipes can't be re-attached to an already-running process).
4. **Provides `agtop cleanup`** to remove stale sessions and orphaned worktrees by cross-referencing session files against live worktrees.

## Relevant Files

Use these files to implement the feature:

- `internal/run/persistence.go` — Stub file. This is where the serialization/deserialization logic goes.
- `internal/run/run.go` — `Run` struct needs JSON tags added to all fields for serialization.
- `internal/run/store.go` — `Store.Subscribe()` is the hook for auto-save. `Store.Add()` is used during rehydration. The `nextID` counter must be restored.
- `internal/process/pipe.go` — `RingBuffer.Tail(n)` provides log lines for session files. `RingBuffer.Append()` is used to restore logs on rehydration.
- `internal/cost/tracker.go` — `Tracker.Record()` restores per-run cost data on rehydration. `SkillCost` already has JSON tags.
- `internal/ui/app.go` — `NewApp()` initialization flow. Replace `seedMockData()` with session rehydration. Pass persistence dependencies through.
- `cmd/agtop/main.go` — Add `cleanup` subcommand routing alongside existing `init`.
- `internal/git/worktree.go` — `WorktreeManager.List()` and `Remove()` for orphan detection and cleanup.
- `internal/config/config.go` — May need session directory config or project hash utility.
- `internal/process/manager.go` — `Manager.Buffer()` provides log buffers. Need to support injecting pre-populated buffers for rehydrated runs.

### New Files

- `cmd/agtop/cleanup.go` — `agtop cleanup` subcommand implementation. Follows the pattern established by `cmd/agtop/init.go`.
- `internal/run/persistence_test.go` — Tests for serialization, deserialization, PID checking, and cleanup logic.

## Implementation Plan

### Phase 1: Foundation

Add JSON tags to the `Run` struct and implement the core `SessionFile` type with serialize/deserialize methods. This is purely data layer work with no integration — everything is testable in isolation.

The session directory structure is `~/.agtop/sessions/<project-hash>/` where `<project-hash>` is an 8-character hex string derived from the absolute path of the project root using FNV-32a (consistent with the hashing approach already used in `internal/server/devserver.go`).

### Phase 2: Core Implementation

Wire up auto-save via `store.Subscribe()` and implement the rehydration flow. This replaces `seedMockData()` in `NewApp()` with a call that loads real session data. Implement PID liveness checking and a background watcher goroutine for live rehydrated processes.

### Phase 3: Integration

Add the `agtop cleanup` subcommand, handle edge cases (concurrent agtop instances, corrupt session files, ID counter restoration), and remove the `seedMockData()` function.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add JSON Tags to Run Struct

- Edit `internal/run/run.go` and add `json:"..."` tags to every field on the `Run` struct.
- Use snake_case JSON keys: `id`, `prompt`, `branch`, `worktree`, `workflow`, `state`, `skill_index`, `skill_total`, `tokens`, `tokens_in`, `tokens_out`, `cost`, `created_at`, `started_at`, `current_skill`, `model`, `command`, `error`, `pid`, `skill_costs`, `dev_server_port`, `dev_server_url`.
- The `State` type is `string`, so it serializes naturally. `time.Time` fields serialize to RFC3339 by default.

### 2. Define SessionFile Type and Core Methods

- In `internal/run/persistence.go`, define the `SessionFile` struct:

```go
type SessionFile struct {
    Version  int       `json:"version"`
    Run      Run       `json:"run"`
    LogTail  []string  `json:"log_tail"`
    SavedAt  time.Time `json:"saved_at"`
}
```

- Define the `Persistence` struct with fields:
  - `sessionsDir string` — base path `~/.agtop/sessions/<project-hash>/`
  - `store *Store` — reference to the run store
  - `mu sync.Mutex` — protects concurrent writes

- Implement `NewPersistence(projectRoot string) (*Persistence, error)`:
  - Compute project hash: `fmt.Sprintf("%08x", fnv.New32a().Sum32())` on the absolute path of `projectRoot`.
  - Build sessions dir: `filepath.Join(homeDir, ".agtop", "sessions", projectHash)`.
  - Create the directory with `os.MkdirAll`.

- Implement `Save(r Run, logTail []string) error`:
  - Create a `SessionFile` with `Version: 1`, the run, log tail, and current time.
  - Marshal to JSON with `json.MarshalIndent`.
  - Write to `<sessionsDir>/<run-id>.json` atomically (write to `.tmp` then rename).

- Implement `Load() ([]SessionFile, error)`:
  - Read all `.json` files from the sessions directory.
  - Deserialize each into a `SessionFile`. Skip files that fail to parse (log a warning).
  - Return the list sorted by `Run.CreatedAt` (oldest first, so IDs are added in order).

- Implement `Remove(runID string) error`:
  - Delete `<sessionsDir>/<run-id>.json`.

### 3. Implement Auto-Save via Store Subscription

- Add a method `BindStore(store *Store, getLogTail func(runID string) []string)` to `Persistence`.
- Inside, call `store.Subscribe(fn)` where `fn`:
  - Iterates `store.List()`.
  - For each run, calls `Save(run, getLogTail(run.ID))`.
- The `getLogTail` callback is provided by the caller (app.go) and calls `manager.Buffer(runID).Tail(1000)` to save the last 1000 log lines.
- Debounce writes: track last save time per run, skip if < 500ms since last save and state hasn't changed. Always save on terminal state transitions.

### 4. Implement Rehydration

- Add a method `Rehydrate(store *Store, buffers func(runID string, lines []string)) (int, error)` to `Persistence`:
  - Call `Load()` to get all session files.
  - For each session file:
    - If run is terminal, add directly to store via `store.Add(&sf.Run)`.
    - If run is non-terminal and has a PID > 0, check liveness with `isProcessAlive(pid)`.
    - If PID is alive: add to store with current state intact. Note the run ID for PID watching.
    - If PID is dead: set `State = StateFailed`, `Error = "process no longer running (agtop restarted)"`, `PID = 0`. Add to store.
    - If run is non-terminal with PID == 0: set `State = StateFailed` with appropriate error.
    - Call `buffers(runID, sf.LogTail)` to restore log lines into a RingBuffer.
  - Return the count of rehydrated runs and any error.
  - After all runs are added, update the store's `nextID` to `max(all numeric IDs) + 1` to avoid collisions.

- Implement `isProcessAlive(pid int) bool`:
  - Use `syscall.Kill(pid, 0)` — returns nil if process exists, `ESRCH` if not.
  - Return true only if err is nil (process exists and we have permission to signal it).

### 5. Add Store.SetNextID Method

- Add `SetNextID(id int)` to `Store` in `internal/run/store.go`:
  - Sets `s.nextID = id` under lock.
  - Only sets if `id > s.nextID` to avoid regression.

### 6. Add Buffer Injection to Process Manager

- Add `InjectBuffer(runID string, lines []string)` to `Manager` in `internal/process/manager.go`:
  - Creates a new `RingBuffer(10000)` in `m.buffers[runID]`.
  - Appends each line from `lines` to the buffer.
  - This allows rehydrated runs to have their log history restored.

### 7. Implement PID Watcher

- Add `WatchPIDs(runIDs []string, store *Store, interval time.Duration) context.CancelFunc` to `Persistence`:
  - Spawns a goroutine that periodically (default 5s) checks `isProcessAlive(pid)` for each tracked run ID.
  - When a PID dies, updates the store: `State = StateFailed`, `Error = "process exited while agtop was not running"`, `PID = 0`.
  - Removes the run ID from the watch list.
  - Returns a cancel function to stop the watcher.
  - When the watch list is empty, the goroutine exits.

### 8. Wire Up Persistence in NewApp

- In `internal/ui/app.go`, modify `NewApp()`:
  - Create `Persistence` after the store is created: `persist, err := run.NewPersistence(projectRoot)`.
  - Call `persist.Rehydrate(store, manager.InjectBuffer)` before UI panel creation.
  - Call `persist.BindStore(store, func(id string) []string { ... })` to enable auto-save.
  - Remove the `seedMockData(store)` call.
  - Store `persist` as a field on `App` for cleanup access.
  - If a PID watcher is needed (rehydrated runs with live PIDs), start it and store the cancel func.

### 9. Implement Cleanup Subcommand

- Create `cmd/agtop/cleanup.go` with `runCleanup(cfg *config.Config) error`:
  - Compute the project hash and sessions directory (same logic as `Persistence`).
  - Load all session files.
  - Categorize:
    - **Stale terminal runs**: `State` is terminal and `SavedAt` is older than 7 days (configurable). Remove session file.
    - **Dead non-terminal runs**: PID is dead. Remove session file.
    - **Orphaned worktrees**: Worktrees in `.agtop/worktrees/` that don't match any session file's `Run.ID`. Remove via `WorktreeManager.Remove()`.
  - Print summary: `Removed N session files, M orphaned worktrees`.
  - Support `--dry-run` flag to preview without deleting.

- In `cmd/agtop/main.go`, add routing:
  ```go
  if len(os.Args) > 1 && os.Args[1] == "cleanup" {
      if err := runCleanup(cfg); err != nil { ... }
      return
  }
  ```

### 10. Handle Edge Cases

- **Corrupt session files**: `Load()` skips files that fail JSON parsing with `log.Printf` warning. Don't crash.
- **Concurrent agtop instances**: Atomic file writes (write-then-rename) prevent partial reads. Two instances of the same project will race on writes, but each will have a consistent snapshot. A lockfile (`~/.agtop/<project-hash>.lock`) is future work — note it in the code as a TODO.
- **Empty sessions directory**: `Load()` returns empty slice, `Rehydrate()` returns 0 runs. No error.
- **ID counter restoration**: Parse all run IDs as integers, take the max, set `nextID` to max + 1. Non-numeric IDs (from parallel sub-tasks like `001:taskname`) are skipped.
- **Log tail size**: Cap at 1000 lines per run to keep session files under ~500KB each.
- **Permission errors**: Log and continue. A single unreadable file shouldn't block startup.

## Testing Strategy

### Unit Tests

In `internal/run/persistence_test.go`:

- **TestSaveAndLoad**: Create a `SessionFile`, save it, load it, verify all fields round-trip correctly including `time.Time`, `State`, `SkillCosts`, and `LogTail`.
- **TestSaveAtomic**: Verify that a concurrent read during write returns either the old or new version, never partial JSON (test the temp-file-then-rename approach).
- **TestLoadSkipsCorruptFiles**: Write a valid session file and a corrupt one. Verify `Load()` returns only the valid one without error.
- **TestRehydrateTerminalRuns**: Create session files for completed/failed/accepted/rejected runs. Verify they're added to the store with correct state.
- **TestRehydrateDeadProcess**: Create a session file with a non-terminal state and a PID that doesn't exist. Verify the run is marked as `failed` with appropriate error.
- **TestRehydrateRestoresNextID**: Add runs with IDs "003", "007", "012". Verify `nextID` is set to 13.
- **TestRehydrateRestoresLogTail**: Save a session with 500 log lines. Rehydrate. Verify the buffer callback receives those lines.
- **TestRemove**: Save a session, remove it, verify the file is deleted.
- **TestProjectHash**: Verify that the same project root always produces the same hash, and different roots produce different hashes.
- **TestSessionFileVersion**: Verify `Version` field is set to 1 on save.

### Edge Cases

- Session file with `Version` != 1 (future-proofing: skip with warning).
- Session file with empty `Run.ID` (skip).
- Runs with PID 0 and non-terminal state (mark as failed).
- Very large log tail (verify truncation to 1000 lines).
- Multiple agtop sessions for different projects in the same `~/.agtop/sessions/` directory (verify project hash isolation).
- `Rehydrate()` called on empty directory returns 0 runs, no error.
- `Save()` when sessions directory doesn't exist (auto-create).

## Acceptance Criteria

- [ ] Run state persists across agtop restarts — close and reopen, see previous runs.
- [ ] Session files are written to `~/.agtop/sessions/<project-hash>/<run-id>.json`.
- [ ] Every store mutation triggers a save (debounced at 500ms, immediate on terminal state).
- [ ] Rehydration on startup restores runs, cost data, skill breakdowns, and log tails.
- [ ] Non-terminal runs with dead PIDs are marked as `failed` on rehydration.
- [ ] Non-terminal runs with live PIDs are shown with their saved state and monitored.
- [ ] `nextID` counter is restored to avoid ID collisions.
- [ ] `seedMockData()` is removed — real persistence replaces it.
- [ ] `agtop cleanup` removes stale session files and orphaned worktrees.
- [ ] Corrupt session files are skipped with a warning, not a crash.
- [ ] Log tail capped at 1000 lines per session file.
- [ ] All unit tests pass: `go test ./internal/run/...`

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Run all tests
go test ./...

# Run persistence-specific tests
go test ./internal/run/ -v -run TestPersistence

# Build successfully
go build -o bin/agtop ./cmd/agtop

# Lint
go vet ./...

# Manual validation: start agtop, create a run, exit, restart — run should appear
bin/agtop
# (create a run with 'n', wait for state change, press 'q')
bin/agtop
# (previous run should be visible in the run list)

# Manual validation: cleanup
bin/agtop cleanup
```

## Notes

- **Process reconnection limitations**: Unix pipes cannot be re-attached to a running process after the parent exits. Rehydrated runs with live PIDs will show their saved state and log tail, but no new log lines will appear until the process exits. The PID watcher detects when the process dies and marks it as failed. Full reconnection would require an intermediate log broker (e.g., writing to a file), which is out of scope.
- **Lockfile for concurrent instances**: The spec mentions `~/.agtop/<project-hash>.lock` in the edge cases section of `docs/agtop.md`. This is deferred to a follow-up — atomic writes provide sufficient safety for the common case.
- **Session file versioning**: The `Version: 1` field enables future schema migration. If a session file has an unknown version, skip it with a warning.
- **Cost tracker restoration**: The cost tracker's `Record()` method can replay `SkillCost` entries from rehydrated runs to restore session totals. This should be called during rehydration for each run's `SkillCosts` slice.
- **Debounce strategy**: The 500ms debounce per-run prevents disk thrashing during rapid event streaming (e.g., token-by-token updates). Terminal state changes bypass the debounce to ensure critical transitions are always persisted immediately.
