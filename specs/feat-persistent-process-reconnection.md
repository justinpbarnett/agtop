# Feature: Persistent Agent Processes with Log Reconnection

## Metadata

type: `feat`
task_id: `persistent-process-reconnection`
prompt: `Closing the agtop TUI should not cancel or interrupt any runs. Opening agtop again should re-connect to the streaming logs and it should appear as if it was never closed, including retaining all logs while agtop was closed and displaying them in the log panel.`

## Feature Description

Currently, closing agtop kills all running agent processes because stdout/stderr are connected via OS pipes. When the parent process (agtop) exits, the read end of the pipes closes, and the agent subprocess receives SIGPIPE when it next writes to stdout — terminating it. Additionally, `exec.CommandContext` ties the subprocess lifetime to a Go context.

This feature decouples agent processes from the agtop TUI lifecycle by routing agent stdout/stderr through log files instead of pipes. The agent writes to files that persist on disk; agtop reads from those files using a tail-follow reader. When agtop closes, the agent keeps writing to the file. When agtop reopens, it reads the entire log file (including events produced while agtop was closed), rebuilds the in-memory buffers and cost tracking, and continues tailing for new events from still-running processes.

## User Story

As an agtop user
I want to close and reopen the dashboard without interrupting running agents
So that I can check on long-running tasks at my convenience without losing progress or log history

## Relevant Files

- `internal/runtime/runtime.go` — `Runtime` interface, `RunOptions`, `Process` struct. Needs `StdoutFile`/`StderrFile` fields on `RunOptions` and a file-based `Process` variant.
- `internal/runtime/claude.go` — `ClaudeRuntime.Start()` uses `exec.CommandContext` and `cmd.StdoutPipe()`. Must switch to `exec.Command` and file-based output.
- `internal/runtime/opencode.go` — `OpenCodeRuntime.Start()` — same changes as Claude.
- `internal/runtime/factory.go` — No changes needed.
- `internal/process/manager.go` — `Manager.Start()`, `StartSkill()`, `consumeEvents()`, `consumeSkillEvents()`, `InjectBuffer()`. Core changes for file-based reading and reconnection.
- `internal/process/pipe.go` — `RingBuffer` and `EntryBuffer`. No changes needed (still used for in-memory display).
- `internal/process/stream.go` — `StreamParser` reads from `io.Reader`. No changes needed (FollowReader satisfies `io.Reader`).
- `internal/process/stream_opencode.go` — Same, no changes needed.
- `internal/process/logentry.go` — `LogEntry`, `EntryBuffer`, `lineToEntry()`. No changes needed.
- `internal/run/persistence.go` — `SessionFile`, `Persistence.Save()`, `Rehydrate()`, `Remove()`, `WatchPIDs()`. Needs log file path storage, file-based rehydration, and cleanup of log files.
- `internal/run/run.go` — `Run` struct. No changes needed (already has `PID`, `SkillIndex`, `CurrentSkill`, `Workflow`).
- `internal/ui/app.go` — `NewApp()` rehydration flow, quit handler, delete handler. Needs reconnection integration.
- `internal/engine/executor.go` — `Executor.Execute()`, `runSkill()`, `Resume()`. Needs awareness of reconnected processes for workflow continuation.
- `cmd/agtop/main.go` — Entry point. May need cleanup of log files in the `cleanup` subcommand.

### New Files

- `internal/process/logfile.go` — `LogFile` struct for managing per-run stdout/stderr log files, and `FollowReader` that implements `io.ReadCloser` with tail-follow semantics (polls for new data at EOF until context is cancelled).
- `internal/process/logfile_test.go` — Tests for `LogFile` and `FollowReader`.

## Implementation Plan

### Phase 1: Log File Infrastructure

Create a `FollowReader` that wraps an `*os.File` and implements `io.ReadCloser`. On `Read()`, if the underlying file returns `io.EOF`, the reader polls (100ms interval) for new data until its context is cancelled. When cancelled, it returns `io.EOF` to signal completion. Also create `LogFile` helpers to create/open log files at standardized paths within the sessions directory.

Log file paths: `~/.agtop/sessions/{projectHash}/{runID}.stdout` and `{runID}.stderr`.

### Phase 2: Runtime Changes — File-Based Output

Add `StdoutFile *os.File` and `StderrFile *os.File` fields to `runtime.RunOptions`. When set, runtimes use these as `cmd.Stdout` and `cmd.Stderr` instead of calling `cmd.StdoutPipe()`/`cmd.StderrPipe()`. Switch from `exec.CommandContext(ctx, ...)` to `exec.Command(...)` so the subprocess is not killed when the context is cancelled — process lifecycle is managed exclusively via explicit signals (SIGTERM/SIGKILL in `Stop()`).

The `Process` struct gains an optional `StdoutFile`/`StderrFile` so the manager can open FollowReaders on them. The pipe-based `Stdout`/`Stderr io.ReadCloser` fields become nil when files are used.

### Phase 3: Manager Changes — File-Based Consumption and Reconnection

Update `Manager.Start()` and `StartSkill()` to:
1. Create log files via `LogFile` helpers (needs sessions dir passed to manager)
2. Pass files to runtime via `RunOptions.StdoutFile`/`StderrFile`
3. Open `FollowReader` instances on the log files
4. Pass FollowReaders to `consumeEvents()`/`consumeSkillEvents()` instead of using `mp.proc.Stdout`/`Stderr`
5. When the process exits (via `mp.proc.Done`), cancel the FollowReader context so it stops polling

Add `Manager.Reconnect(runID string, pid int, stdoutPath string, stderrPath string) error`:
1. Open log files at the given paths
2. Create FollowReaders with a new context
3. Allocate RingBuffer and EntryBuffer
4. Start `consumeEvents()` goroutine (same as new processes — reads all historical events, then follows for new ones)
5. Register a PID monitor: when the PID dies, cancel the FollowReader context and set terminal state
6. Track as a ManagedProcess (with nil `proc` — reconnected processes use PID-based signal sending for Stop/Pause/Resume/Kill)

Update `Stop()`, `Pause()`, `Resume()`, `Kill()` to handle reconnected processes (nil `proc.Cmd`) by sending signals directly via `syscall.Kill(pid, signal)`.

Remove or deprecate `InjectBuffer()` — log file reading replaces it.

Update `RemoveBuffer()` to also close any open log file handles.

### Phase 4: Persistence Changes

Add `StdoutPath` and `StderrPath` fields to `SessionFile`. These are populated when saving a run that has log files. Keep `LogTail` for backward compatibility with old sessions.

Update `Persistence.Save()` to include log file paths.

Update `Persistence.Rehydrate()`:
- For runs with log files and a live PID: call `Manager.Reconnect()` instead of `InjectBuffer()`. This replays the full log file, rebuilding buffers and cost tracking from the actual stream data.
- For runs with log files and a dead PID: still read the log file to populate buffers (so the user sees the complete log history), then mark as failed.
- For runs without log files (old sessions): fall back to `InjectBuffer()` with `LogTail` (existing behavior).

Remove the separate `WatchPIDs()` goroutine — reconnected processes handle their own PID monitoring inside `Manager.Reconnect()`.

Update `Persistence.Remove()` to delete `.stdout` and `.stderr` files alongside the `.json` session file.

### Phase 5: App Lifecycle Integration

Update `NewApp()` rehydration to pass the manager's `Reconnect` function as a callback (alongside the existing `InjectBuffer` and `RecordCost` callbacks).

Update the quit handler (`ctrl+c`, `q`): cancel all FollowReader contexts (stop consumption goroutines) but do NOT kill processes. Close log file handles cleanly.

Update `handleDelete()`: ensure log files are cleaned up via `Persistence.Remove()`.

### Phase 6: Executor Recovery (Workflow Continuation)

When a reconnected process finishes, the run is left in its current state with `CurrentSkill` and `SkillIndex` intact. The user can manually resume (`r` key) to continue the workflow from the next skill. The existing `Executor.Resume()` already handles this — it reads `SkillIndex` and starts from the next incomplete skill.

For a future enhancement, auto-continuation could be added: after a reconnected process completes successfully, automatically call `Executor.Resume()` if there are remaining skills. This is not included in this spec to keep scope manageable.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Create FollowReader and LogFile helpers

- Create `internal/process/logfile.go` with:
  - `type FollowReader struct` — wraps `*os.File` with a `context.Context` and poll interval (100ms). `Read(p []byte)` retries on `io.EOF` with polling until context is cancelled. `Close()` cancels context and closes file.
  - `func NewFollowReader(ctx context.Context, f *os.File) *FollowReader`
  - `type LogFiles struct` — manages stdout and stderr file creation/opening for a run.
  - `func CreateLogFiles(sessionsDir, runID string) (*LogFiles, error)` — creates `{runID}.stdout` and `{runID}.stderr` files with `O_CREATE|O_WRONLY|O_APPEND`.
  - `func OpenLogFiles(stdoutPath, stderrPath string) (*LogFiles, error)` — opens existing files with `O_RDONLY` for reading.
  - `StdoutPath() string`, `StderrPath() string`, `StdoutWriter() *os.File`, `StderrWriter() *os.File`, `Close() error`
- Create `internal/process/logfile_test.go` with tests:
  - `TestFollowReader_ReadsExistingContent` — write data to file, create FollowReader, verify read
  - `TestFollowReader_WaitsForNewData` — create reader on empty file, write data in goroutine after delay, verify reader unblocks and returns data
  - `TestFollowReader_StopsOnContextCancel` — cancel context, verify reader returns `io.EOF`
  - `TestCreateLogFiles` — verify files created at correct paths
  - `TestOpenLogFiles` — verify files opened for reading

### 2. Add file-based output support to runtime

- In `internal/runtime/runtime.go`:
  - Add `StdoutFile *os.File` and `StderrFile *os.File` to `RunOptions`
  - Add `StdoutPath string` and `StderrPath string` to `Process` (for tracking file locations)
- In `internal/runtime/claude.go`:
  - Change `exec.CommandContext(ctx, ...)` to `exec.Command(...)` — process should not be killed by context cancellation
  - If `opts.StdoutFile != nil`: set `cmd.Stdout = opts.StdoutFile` instead of `cmd.StdoutPipe()`; set `proc.Stdout = nil`
  - If `opts.StderrFile != nil`: set `cmd.Stderr = opts.StderrFile` instead of `cmd.StderrPipe()`; set `proc.Stderr = nil`
  - Store file paths in `proc.StdoutPath` and `proc.StderrPath`
- In `internal/runtime/opencode.go`:
  - Apply identical changes as Claude runtime
- Update runtime tests: `internal/runtime/claude_test.go`, `internal/runtime/opencode_test.go`

### 3. Update Manager.Start() and StartSkill() for file-based logging

- Add `sessionsDir string` field to `Manager` struct; accept in `NewManager()` constructor
- In `Manager.Start()`:
  - Call `CreateLogFiles(m.sessionsDir, runID)` to create log files
  - Set `opts.StdoutFile` and `opts.StderrFile` from the LogFiles writers
  - After `rt.Start()`, open FollowReaders on the log files (separate reader handles from the writer handles)
  - Pass FollowReaders as the stdout/stderr readers to `consumeEvents()` instead of `mp.proc.Stdout`/`Stderr`
  - Store log file paths in a new `logFiles map[string]*LogFiles` field on Manager
  - When process exits (`mp.proc.Done` received in `consumeEvents`), cancel the FollowReader context
- In `Manager.StartSkill()`:
  - Same changes as `Start()`: create log files, use FollowReaders
  - When process exits (`consumeSkillEvents`), cancel FollowReader context
- Refactor `consumeEvents()` and `consumeSkillEvents()` to accept `io.Reader` for stdout and stderr instead of reading from `mp.proc.Stdout`/`Stderr` directly. This unifies started and reconnected process handling.
- Update `Manager.RemoveBuffer()` to also close and delete log file handles

### 4. Add Manager.Reconnect() for rehydrated live processes

- Add `Manager.Reconnect(runID string, pid int, stdoutPath string, stderrPath string) error`:
  - Open log files for reading via `OpenLogFiles(stdoutPath, stderrPath)`
  - Create a context with cancel for the FollowReaders
  - Create FollowReaders on stdout and stderr files
  - Allocate fresh RingBuffer (10000) and EntryBuffer (5000)
  - Create a `ManagedProcess` with `proc: nil` and the cancel function
  - Register in `m.processes`, `m.buffers`, `m.entryBuffers`, `m.logFiles`
  - Start `consumeEvents()` goroutine with the FollowReaders
  - Start a PID monitor goroutine: poll `IsProcessAlive(pid)` every 2s. When dead, cancel the FollowReader context (which causes consumeEvents to finish and set terminal state)
- Update `Stop()`, `Pause()`, `Resume()`, `Kill()`: when `mp.proc == nil` (reconnected process), send signals via `syscall.Kill(pid, signal)` using a `pid` field added to `ManagedProcess`

### 5. Update Persistence for log file paths

- Add `StdoutPath string` and `StderrPath string` to `SessionFile`
- In `Persistence.Save()`: include log file paths (obtained from manager via a new callback or stored directly)
- In `Persistence.Rehydrate()`:
  - If `sf.StdoutPath != ""` and `sf.StderrPath != ""` and PID is alive: call a new `Reconnect` callback instead of `InjectBuffer`
  - If paths exist but PID is dead: read the log files to populate buffers (one-shot, not follow), then mark as failed
  - If no paths (old sessions): fall back to `InjectBuffer` with `LogTail`
- Add `Reconnect func(runID string, pid int, stdoutPath string, stderrPath string)` to `RehydrateCallbacks`
- In `Persistence.Remove()`: delete `{runID}.stdout` and `{runID}.stderr` alongside `{runID}.json`
- Remove standalone `WatchPIDs()` — PID monitoring moves into `Manager.Reconnect()`

### 6. Update App rehydration and lifecycle

- In `NewApp()`:
  - Pass `persist.SessionsDir()` to the Manager constructor (new param)
  - Wire up the `Reconnect` callback: `cb.Reconnect = mgr.Reconnect`
  - Remove the `pidWatchCancel` field and associated `WatchPIDs` call (replaced by per-process PID monitoring in Reconnect)
- In quit handler (`ctrl+c`, `q`):
  - Remove `pidWatchCancel()` call (no longer exists)
  - Add `mgr.DisconnectAll()` — a new method that cancels all FollowReader contexts and closes log file reader handles, WITHOUT killing any processes
  - Keep `devServers.StopAll()` as-is
- In `handleDelete()`:
  - `Persistence.Remove()` already deletes log files after the update in step 5

### 7. Update the cleanup subcommand

- In `cmd/agtop/main.go` cleanup handler:
  - When removing stale sessions, also remove `.stdout` and `.stderr` files alongside `.json` files

### 8. Update tests

- `internal/process/manager_test.go` — update `NewManager()` calls with new `sessionsDir` parameter; test `Reconnect()` with a mock log file
- `internal/process/manager_skill_test.go` — update `NewManager()` calls
- `internal/run/persistence_test.go` — test log file paths in `SessionFile`, test `Remove()` deletes log files, test `Rehydrate()` with reconnect callback
- `internal/runtime/claude_test.go` — test file-based output mode
- `internal/runtime/opencode_test.go` — test file-based output mode
- `internal/ui/app_test.go` — update `NewApp()` if constructor changes

## Testing Strategy

### Unit Tests

- **FollowReader**: Test read of existing data, blocking on EOF until new data arrives, unblocking on context cancel, handling of large files, concurrent write+read.
- **LogFile creation/opening**: Verify file paths, permissions, create+open round-trip.
- **Manager.Reconnect**: Create a log file with known events, call Reconnect, verify buffers are populated with correct entries and cost is tracked.
- **Manager.Start (file-based)**: Verify log files are created, FollowReaders are opened, process output appears in buffers.
- **Persistence round-trip**: Save a session with log file paths, load it, verify paths are preserved. Remove and verify all files deleted.
- **Runtime file mode**: Verify that when StdoutFile/StderrFile are set, pipes are not created and the process writes to files.

### Integration Tests

- Start a real process (e.g., `echo` or `sleep`), verify output appears in log file.
- Start a process, close the manager (cancel FollowReaders), verify process keeps running (check PID).
- Start a process with file output, reconnect to it, verify all events are replayed.

### Edge Cases

- Process exits while agtop is closed: on restart, full log is available, run marked as failed with appropriate error.
- Multiple restarts: agtop closed and reopened multiple times while process runs — each time, all historical logs are visible.
- Very large log files (>100MB): verify memory stays bounded (RingBuffer/EntryBuffer circular eviction).
- Process exits immediately: FollowReader sees EOF, context is cancelled, no hang.
- Log file missing on rehydrate: fall back to LogTail gracefully, log a warning.
- Old session files without log paths: backward-compatible LogTail injection.

## Risk Assessment

- **SIGPIPE on old sessions**: Processes started before this change (using pipes) will still die on agtop exit. No migration path — this only affects new processes started after the change.
- **File handle limits**: Each running process needs 4 file handles (2 writer, 2 reader). With the default `max_concurrent_runs` of 5, this is 20 handles — well within OS limits.
- **Disk space**: Log files grow unbounded during a run. A very long-running agent could produce large files. Mitigation: the cleanup subcommand already handles stale sessions; log files are cleaned up alongside session files.
- **Stdout buffering**: When agent writes to a file instead of a pipe, buffering behavior may differ. Node.js (Claude CLI) and Go (OpenCode) both flush writes immediately, so this should not cause visible lag. If it does, `stdbuf -oL` can be prepended to the command as a fallback.
- **Breaking change to Manager constructor**: Adding `sessionsDir` parameter changes the `NewManager()` signature. All callers must be updated (there's only one: `NewApp()`).
- **Executor workflow gap**: When agtop exits mid-workflow, the current skill keeps running but subsequent skills don't auto-start. The user must press `r` to resume. This is a known limitation documented for future work.

## Validation Commands

```bash
make lint          # go vet ./...
go test ./internal/process/...
go test ./internal/runtime/...
go test ./internal/run/...
go test ./internal/ui/...
go test ./...
make build         # verify compilation
```

## Open Questions (Unresolved)

1. **FollowReader poll interval**: 100ms is proposed. Too fast wastes CPU; too slow causes visible lag in log updates. Recommendation: 100ms is a good default — it's imperceptible to users and the CPU cost of a stat+read on a file is negligible. Could be made configurable later.

2. **Log file rotation/cleanup for long-running processes**: Should we cap log file size or rotate? Recommendation: No — keep it simple. Log files are cleaned up when runs are deleted or via `agtop cleanup`. Very long-running agents (hours) produce manageable file sizes (stream-json events are compact).

3. **Auto-resume workflow after reconnected process completes**: Should the executor automatically continue the workflow? Recommendation: Not in this iteration. Manual resume (`r` key) already works. Auto-continuation adds complexity around error handling and user expectations. Add it as a follow-up feature.

## Sub-Tasks

Single task — no decomposition needed. The phases are sequential and tightly coupled (each builds on the previous). A single implementation pass is appropriate.
