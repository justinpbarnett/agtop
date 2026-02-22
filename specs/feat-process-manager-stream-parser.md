# Feature: Process Manager and Stream Parser

## Metadata

type: `feat`
task_id: `process-manager-stream-parser`
prompt: `Implement goroutine-based subprocess manager, stream-json parser, ring buffer pipe handler, and Claude Code runtime — enabling agtop to spawn, monitor, and control claude -p subprocesses with real-time log streaming and cost tracking`

## Feature Description

The process manager is the execution backbone of agtop. It spawns `claude -p` subprocesses, captures their streaming JSON output, parses events (text, tool use, cost/token data), routes parsed events into per-run ring buffers, and manages subprocess lifecycle (start, signal, kill). This step turns agtop from a static TUI displaying mock data into a system that can actually launch and monitor agent processes.

Three components work together:

1. **Process Manager** (`manager.go`) — goroutine-based subprocess pool that tracks active processes, enforces concurrency limits, and handles lifecycle operations (start, pause via SIGSTOP, resume via SIGCONT, cancel via SIGTERM/SIGKILL).
2. **Stream Parser** (`stream.go`) — line-by-line JSON parser for Claude Code's `--output-format stream-json` output. Extracts assistant text, tool use events, and result messages with token/cost data. Emits typed `StreamEvent` values.
3. **Ring Buffer** (`pipe.go`) — fixed-capacity circular buffer that stores log lines per run, preserving ANSI color codes and prefixing each line with a timestamp and skill name. Provides read access for the log viewer without blocking writes.

Additionally, this step implements the **Claude Code runtime** (`claude.go`) — the concrete `Runtime` implementation that constructs and executes `claude -p` commands with the correct flags.

## User Story

As a developer using agtop
I want to launch agent runs that execute as real subprocesses with live streaming output
So that I can see what each agent is doing in real-time and track token usage and costs as they accumulate

## Problem Statement

The current codebase has empty structs for `process.Manager`, `process.StreamParser`, `process.Pipe`, `runtime.ClaudeRuntime`, and `runtime.OpenCodeRuntime`. The TUI displays mock data seeded in `NewApp()`. There is no way to:

- Spawn a subprocess and capture its output
- Parse Claude Code's stream-json format into structured events
- Buffer log lines for display in the log viewer
- Track real token/cost data from subprocess output
- Control a running subprocess (pause, resume, cancel)
- Enforce concurrency limits on simultaneous runs

Without this infrastructure, no subsequent step (skill engine, workflow executor, run lifecycle commands) can function.

## Solution Statement

Implement the process management layer as three cooperating components plus the Claude Code runtime:

1. **`process.Manager`** — holds a map of active processes keyed by run ID, enforces `MaxConcurrentRuns` from config, spawns subprocesses in goroutines with cancellable contexts, routes `StreamEvent` values into run-specific ring buffers, updates the run store with state changes and cost/token increments. Exposes `Start`, `Stop`, `Pause`, `Resume`, and `Kill` methods.

2. **`process.StreamParser`** — reads from an `io.Reader` (subprocess stdout), scans line-by-line, unmarshals each JSON line into a typed event, and sends parsed `StreamEvent` values on a channel. Handles malformed lines gracefully (logs them as raw text events).

3. **`process.RingBuffer`** — fixed-size circular buffer of log lines with thread-safe append and read. Each line is timestamped and skill-prefixed before insertion. The log viewer reads a snapshot of the buffer without blocking writes.

4. **`runtime.ClaudeRuntime`** — constructs `os/exec.Cmd` for `claude -p` with `--output-format stream-json`, `--model`, `--allowedTools`, `--max-turns`, `--cwd`, and `--permission-mode` flags. Implements the `runtime.Runtime` interface.

The TUI integration point is a new `LogLineMsg` Bubble Tea message emitted when new log content is available for a run, and updates to the existing `RunStoreUpdatedMsg` flow for state/cost/token changes.

## Relevant Files

Use these files to implement the feature:

- `internal/process/manager.go` — Currently empty `Manager` struct. Will become the subprocess orchestrator.
- `internal/process/stream.go` — Currently empty `StreamParser` struct. Will become the stream-json line parser.
- `internal/process/pipe.go` — Currently empty `Pipe` struct. Will become the `RingBuffer` log buffer.
- `internal/runtime/runtime.go` — Defines `Runtime` interface, `RunOptions`, `Process`, and `Event` types. Will be extended with additional fields.
- `internal/runtime/claude.go` — Currently empty struct. Will implement `Runtime` for Claude Code.
- `internal/run/run.go` — `Run` struct. Will be extended with a `LogBuffer *process.RingBuffer` field and `PID int` field.
- `internal/run/store.go` — Run store. No changes needed — the manager uses existing `Add`/`Update` methods.
- `internal/tui/messages.go` — Bubble Tea message types. Will add `LogLineMsg` for log updates.
- `internal/tui/logs.go` — Log viewer. Will be updated to read from the run's ring buffer instead of mock content.
- `internal/tui/app.go` — Root model. Will hold a reference to the process manager and wire log messages.
- `internal/config/config.go` — Config structs. Already has `ClaudeConfig`, `LimitsConfig` — no changes needed.

### New Files

- `internal/process/manager_test.go` — Tests for process manager lifecycle operations.
- `internal/process/stream_test.go` — Tests for stream-json parsing with real Claude Code output samples.
- `internal/process/pipe_test.go` — Tests for ring buffer correctness, overflow, and concurrent access.

## Implementation Plan

### Phase 1: Foundation

Build the low-level components that have no dependencies on each other: the ring buffer for log storage, the stream parser for JSON event extraction, and the stream event types that both consume.

### Phase 2: Core Implementation

Implement the process manager that orchestrates subprocesses and connects the stream parser to the ring buffer. Implement the Claude Code runtime that constructs the actual `claude -p` commands. Wire the manager into the run store for state/cost updates.

### Phase 3: Integration

Connect the process manager to the TUI via Bubble Tea messages. Update the log viewer to read from ring buffers. Add the manager to the App model so run lifecycle commands can call it.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Define Stream Event Types

Update `internal/process/stream.go`:

- Define `StreamEventType` as a string type with constants:
  - `EventText` — assistant text content
  - `EventToolUse` — tool invocation (name + input)
  - `EventToolResult` — tool execution result
  - `EventResult` — final result with usage/cost data
  - `EventError` — error from the stream
  - `EventRaw` — unparseable line preserved as-is
- Define `StreamEvent` struct:
  ```go
  type StreamEvent struct {
      Type      StreamEventType
      Text      string      // for EventText, EventRaw, EventError
      ToolName  string      // for EventToolUse
      ToolInput string      // for EventToolUse (JSON string)
      Usage     *UsageData  // for EventResult
  }
  ```
- Define `UsageData` struct:
  ```go
  type UsageData struct {
      InputTokens  int
      OutputTokens int
      TotalTokens  int
      CostUSD      float64
  }
  ```
- Define internal JSON structures matching Claude Code's `stream-json` format for unmarshalling:
  - `streamMessage` — top-level envelope with `type` field (`"assistant"`, `"result"`)
  - `contentBlock` — content block with `type` (`"text"`, `"tool_use"`) and associated fields
  - `resultMessage` — result with `usage` object and `total_cost_usd` float

### 2. Implement the Stream Parser

In `internal/process/stream.go`:

- Rename `StreamParser` struct to hold:
  - `reader io.Reader` — the source (subprocess stdout)
  - `events chan StreamEvent` — output channel for parsed events
  - `done chan error` — signals completion with optional error
- `NewStreamParser(r io.Reader, bufSize int) *StreamParser` — constructor. `bufSize` is the channel buffer size (default 256).
- `Parse(ctx context.Context)` — the main parse loop, intended to run in a goroutine:
  - Create a `bufio.Scanner` on the reader, set max token size to 1MB (agent output can include large tool results)
  - For each line:
    - If context is cancelled, return
    - Try `json.Unmarshal` into `streamMessage`
    - If it's an `"assistant"` type: iterate `content` blocks, emit `EventText` for text blocks and `EventToolUse` for tool_use blocks
    - If it's a `"result"` type: extract `usage` and `total_cost_usd`, emit `EventResult` with `UsageData`
    - If unmarshal fails: emit `EventRaw` with the raw line (stderr passthrough, progress indicators, etc.)
  - On scanner error or EOF, send error on `done` channel and close `events`
- `Events() <-chan StreamEvent` — returns the events channel
- `Done() <-chan error` — returns the done channel

### 3. Implement the Ring Buffer

Rewrite `internal/process/pipe.go`:

- Rename `Pipe` to `RingBuffer` struct:
  - `mu sync.RWMutex`
  - `lines []string` — fixed-size backing array
  - `capacity int` — max lines (from config, default 10000)
  - `head int` — next write position
  - `count int` — current number of lines stored
  - `totalWritten int` — total lines ever written (for detecting new content)
- `NewRingBuffer(capacity int) *RingBuffer` — constructor
- `Append(line string)` — write-lock, write at `head`, advance `head = (head + 1) % capacity`, increment `count` (capped at `capacity`) and `totalWritten`
- `Lines() []string` — read-lock, return a copy of all lines in insertion order (oldest first). Handle the wrap-around: if `count < capacity`, return `lines[:count]`; otherwise return `lines[head:] + lines[:head]`
- `Tail(n int) []string` — read-lock, return the last `n` lines (or fewer if buffer has less)
- `Len() int` — read-lock, return `count`
- `TotalWritten() int` — read-lock, return `totalWritten` (callers use this to detect whether new lines have been added since last read)
- `Reset()` — write-lock, zero out head, count, totalWritten

### 4. Implement the Claude Code Runtime

Rewrite `internal/runtime/claude.go`:

- Define `ClaudeRuntime` struct:
  - `claudePath string` — path to the `claude` binary (resolved at init via `exec.LookPath`)
- `NewClaudeRuntime() (*ClaudeRuntime, error)` — resolve `claude` binary path. Return error if not found with message: `"claude binary not found in PATH — install from https://docs.anthropic.com/en/docs/claude-code"`.
- Implement `Start(ctx context.Context, prompt string, opts RunOptions) (*Process, error)`:
  - Build the command args:
    ```
    claude -p <prompt>
      --output-format stream-json
      --model <opts.Model>
      --max-turns <opts.MaxTurns>
      --cwd <opts.WorkDir>
    ```
  - If `opts.AllowedTools` is non-empty, add `--allowedTools "<comma-separated>"`.
  - If `opts.PermissionMode` is non-empty, add `--permission-mode <mode>`.
  - Create `exec.CommandContext(ctx, claudePath, args...)`.
  - Set `cmd.Dir = opts.WorkDir`.
  - Get `cmd.StdoutPipe()` and `cmd.StderrPipe()`.
  - Call `cmd.Start()`.
  - Create a `StreamParser` on the stdout pipe.
  - Start a goroutine that copies stderr lines into the events channel as `EventRaw` events.
  - Start `parser.Parse(ctx)` in a goroutine.
  - Start a goroutine that waits for the process (`cmd.Wait()`), then sends the exit error on `Process.Done`.
  - Return `&Process{PID: cmd.Process.Pid, Cmd: cmd, Events: parser.Events(), Done: doneCh}`.
- Implement `Stop(proc *Process) error`:
  - Send `SIGTERM` to the process.
  - Start a goroutine with a 5-second timer. If the process hasn't exited by then, send `SIGKILL`.
  - Return nil (actual exit status comes via `Process.Done`).
- Add `Pause(proc *Process) error` — send `SIGSTOP` (Unix only).
- Add `Resume(proc *Process) error` — send `SIGCONT` (Unix only).

### 5. Extend the Runtime Interface and Types

Update `internal/runtime/runtime.go`:

- Add `Pause` and `Resume` to the `Runtime` interface:
  ```go
  type Runtime interface {
      Start(ctx context.Context, prompt string, opts RunOptions) (*Process, error)
      Stop(proc *Process) error
      Pause(proc *Process) error
      Resume(proc *Process) error
  }
  ```
- Add `PermissionMode` field to `RunOptions`:
  ```go
  type RunOptions struct {
      Model          string
      WorkDir        string
      AllowedTools   []string
      MaxTurns       int
      PermissionMode string
  }
  ```
- Update `Process` struct to hold the `exec.Cmd` reference (needed for signaling):
  ```go
  type Process struct {
      PID    int
      Cmd    *exec.Cmd
      Events <-chan process.StreamEvent
      Done   <-chan error
  }
  ```
  Note: `Events` changes from `chan Event` to `<-chan process.StreamEvent` to use the richer typed events.
- Remove the old `Event` struct (replaced by `process.StreamEvent`).

### 6. Implement the Process Manager

Rewrite `internal/process/manager.go`:

- Define `Manager` struct:
  - `store *run.Store` — the run store for state/cost updates
  - `runtime runtime.Runtime` — the agent runtime
  - `cfg *config.LimitsConfig` — concurrency and cost limits
  - `mu sync.Mutex` — protects the processes map
  - `processes map[string]*ManagedProcess` — active processes keyed by run ID
  - `buffers map[string]*RingBuffer` — log buffers keyed by run ID
  - `program *tea.Program` — Bubble Tea program for sending messages (set after init)
- Define `ManagedProcess` struct:
  - `proc *runtime.Process` — the underlying process
  - `cancel context.CancelFunc` — cancels the process context
  - `runID string` — associated run ID
- `NewManager(store *run.Store, rt runtime.Runtime, cfg *config.LimitsConfig) *Manager` — constructor
- `SetProgram(p *tea.Program)` — called after `tea.NewProgram` is created, stores the reference for sending TUI messages
- `Start(runID string, prompt string, opts runtime.RunOptions) error`:
  - Check concurrency limit: if `len(processes) >= cfg.MaxConcurrentRuns`, return error
  - Create a `RingBuffer` for this run (capacity 10000)
  - Create a cancellable context: `ctx, cancel := context.WithCancel(context.Background())`
  - Call `runtime.Start(ctx, prompt, opts)`
  - Store the `ManagedProcess` and `RingBuffer` in maps
  - Update run store: set `State = StateRunning`, `PID` (add PID field to run — see step 7)
  - Start the event consumption goroutine (see below)
  - Return nil
- Internal `consumeEvents(runID string, mp *ManagedProcess, buf *RingBuffer)` — runs in a goroutine:
  - Select on `mp.proc.Events` and `mp.proc.Done`:
  - On `StreamEvent`:
    - Format the event as a log line: `[HH:MM:SS skill] <text>` (skill name comes from the run's `CurrentSkill`)
    - `buf.Append(formattedLine)`
    - If event type is `EventResult` and has `UsageData`: call `store.Update(runID, func(r) { r.Tokens += usage.TotalTokens; r.Cost += usage.CostUSD })`. Check cost/token thresholds — if exceeded, call `Pause(runID)` and log a warning.
    - If event type is `EventText` or `EventToolUse`: just append to buffer (already done above)
    - If `program` is set, send `LogLineMsg{RunID: runID}` to trigger log viewer refresh
  - On `Done` (process exited):
    - Check the error: if nil or exit code 0, update run state to `StateCompleted`. If non-nil, update to `StateFailed` with `Error` set to the error message.
    - Clean up: remove from `processes` map (keep the buffer for log viewing)
    - Return (goroutine exits)
- `Stop(runID string) error` — look up process, call `runtime.Stop(proc)`, call `cancel()`
- `Pause(runID string) error` — call `runtime.Pause(proc)`, update run state to `StatePaused`
- `Resume(runID string) error` — call `runtime.Resume(proc)`, update run state to `StateRunning`
- `Kill(runID string) error` — send SIGKILL immediately, call `cancel()`
- `Buffer(runID string) *RingBuffer` — return the ring buffer for a run (used by log viewer)
- `ActiveCount() int` — return `len(processes)`

### 7. Extend the Run Struct

Update `internal/run/run.go`:

- Add `PID int` field — the OS process ID of the current subprocess (0 if not running)
- Add `LogBuffer` as a field would create a circular dependency (run → process → run). Instead, the process manager holds the buffer map separately and exposes `Buffer(runID)`. No change to run.go needed beyond `PID`.

### 8. Add TUI Messages for Log Streaming

Update `internal/tui/messages.go`:

- Add `LogLineMsg`:
  ```go
  type LogLineMsg struct {
      RunID string
  }
  ```
  This message triggers the log viewer to re-read the ring buffer for the specified run.

### 9. Update the Log Viewer

Update `internal/tui/logs.go`:

- Add a `manager` field (type `ProcessManager` interface — see below) and `runID string` to `LogViewer`
- Define a minimal interface to avoid importing the process package directly:
  ```go
  type LogSource interface {
      Buffer(runID string) *process.RingBuffer
  }
  ```
  Or pass the buffer directly: `SetBuffer(buf *process.RingBuffer)`
- Update `NewLogViewer()` to not set mock content
- Add `SetRun(runID string, buf *process.RingBuffer)` — called when the selected run changes. If `buf` is nil, show "No logs available." If non-nil, set the viewport content to `strings.Join(buf.Lines(), "\n")`.
- On `LogLineMsg` in `Update()`: if the message's `RunID` matches, refresh the viewport content from the buffer. If the viewport is at the bottom (auto-follow mode), scroll to the new bottom.
- Keep the mock content function for when no process manager is available (development mode) — check if buffer is nil and fall back to mock

### 10. Wire the Process Manager into App

Update `internal/tui/app.go`:

- Add `manager *process.Manager` field to `App`
- Update `NewApp(cfg *config.Config)`:
  - Attempt to create `ClaudeRuntime` via `runtime.NewClaudeRuntime()`. If it fails (binary not found), log a warning and leave `manager` as nil (TUI still works with mock data for development).
  - If runtime is available, create `process.NewManager(store, rt, &cfg.Limits)`.
- Add a `SetProgram(p *tea.Program)` method on `App` that forwards to `manager.SetProgram(p)` if manager is non-nil.
- Update `cmd/agtop/main.go`:
  - After `tea.NewProgram(app)`, call `app.SetProgram(p)` before `p.Run()`. Note: this requires `app` to be a pointer or returned by reference. If `NewApp` returns a value, store `p := tea.NewProgram(app)` then access through the program. Alternative: use `Init()` to set up the program reference — but Bubble Tea doesn't expose the program to `Init`. The simplest approach: make `App` hold a `programReady chan *tea.Program` and have `Init()` return a command that receives it.
  - Pragmatic approach: accept that `SetProgram` is called on the app's copy, not the Bubble Tea model's copy. Instead, have the `Manager` accept the program via a setter called from main: `manager.SetProgram(p)` before `p.Run()`.
- Route `LogLineMsg` in `Update()`:
  - If the message's `RunID` matches the currently selected run, propagate to the detail panel's log viewer.

### 11. Update Detail Panel for Log Integration

Update `internal/tui/detail.go`:

- When the selected run changes (`SetRun`), if the process manager is available, call `logViewer.SetRun(runID, manager.Buffer(runID))`.
- Propagate `LogLineMsg` to the log viewer in `Update()`.

### 12. Write Tests

**Create `internal/process/stream_test.go`:**

- `TestParseTextEvent` — feed a JSON line with `type: "assistant"` and a text content block, verify `EventText` is emitted with correct text.
- `TestParseToolUseEvent` — feed a JSON line with a `tool_use` content block, verify `EventToolUse` with tool name and input.
- `TestParseResultEvent` — feed a result JSON with `usage` and `total_cost_usd`, verify `EventResult` with correct `UsageData`.
- `TestParseMultipleContentBlocks` — a single assistant message with both text and tool_use blocks emits multiple events.
- `TestParseMalformedLine` — non-JSON line emits `EventRaw`.
- `TestParseEmptyLine` — blank lines are skipped.
- `TestParseContextCancellation` — cancel the context mid-parse, verify the parser stops.
- `TestParseLargeLine` — line near the 1MB scanner limit is parsed correctly.
- Sample JSON fixtures based on real Claude Code `stream-json` output format.

**Create `internal/process/pipe_test.go`:**

- `TestRingBufferAppend` — append lines, verify `Lines()` returns them in order.
- `TestRingBufferOverflow` — append more than capacity, verify oldest lines are dropped and `Lines()` returns correct order.
- `TestRingBufferTail` — verify `Tail(n)` returns the last n lines.
- `TestRingBufferTotalWritten` — verify counter increments on every append.
- `TestRingBufferReset` — verify reset clears all state.
- `TestRingBufferConcurrent` — multiple goroutines appending and reading simultaneously, run with `-race`.

**Create `internal/process/manager_test.go`:**

- `TestManagerStart` — start a process (use a mock runtime that returns a fake process), verify the run state is updated to `StateRunning`.
- `TestManagerConcurrencyLimit` — set max concurrent to 2, start 3 processes, verify the 3rd returns an error.
- `TestManagerStop` — start and stop a process, verify the run state transitions to completed or failed.
- `TestManagerPause` — start and pause, verify state is `StatePaused`.
- `TestManagerResume` — pause then resume, verify state is `StateRunning`.
- `TestManagerEventConsumption` — send events through the mock process's channel, verify they appear in the ring buffer and run store tokens/costs are updated.
- `TestManagerProcessExit` — close the mock process's done channel, verify cleanup happens and run state is terminal.
- Use a `mockRuntime` that implements `runtime.Runtime` with channels for controlled testing.

## Testing Strategy

### Unit Tests

- **Stream parser tests** — use `strings.NewReader` with sample JSON lines to test all event types. No real subprocess needed.
- **Ring buffer tests** — pure data structure tests plus concurrent access with `-race` flag.
- **Manager tests** — use a mock `Runtime` implementation that returns fake `Process` values with controllable channels. Tests verify state machine transitions, concurrency limits, event routing, and cleanup.
- **Claude runtime tests** — test command construction (args, flags) without actually executing `claude`. Use `exec.LookPath` mocking or test the arg builder function separately.

### Edge Cases

- **Malformed JSON in stream** — parser emits `EventRaw`, doesn't crash
- **Empty stream** (process exits immediately with no output) — run transitions to completed/failed based on exit code
- **Very large tool output** (near 1MB scanner limit) — scanner doesn't truncate; test with large line
- **Rapid events** (thousands per second) — ring buffer handles high-throughput appends; channel buffer prevents backpressure on the parser goroutine
- **Process killed externally** — `cmd.Wait()` returns an error, manager detects it and updates run state to failed
- **Context cancelled during parse** — parser exits cleanly, events channel closed
- **Concurrent Start/Stop on same run** — mutex protects processes map; Stop on non-existent run returns error
- **Ring buffer exactly at capacity** — verify wrap-around produces correct line ordering
- **Manager Stop called twice** — second call is a no-op (process already removed from map)
- **Cost threshold exceeded** — manager auto-pauses the run and logs a warning in the buffer
- **Rate limit (429) in stream** — detected by error text pattern in stream events, triggers pause with backoff (future step integration point — log the event for now)
- **Binary not found** — `NewClaudeRuntime` returns descriptive error; app continues with nil manager and mock data

## Acceptance Criteria

- [ ] `StreamParser` parses `assistant` messages into `EventText` and `EventToolUse` events
- [ ] `StreamParser` parses `result` messages into `EventResult` with `UsageData` (tokens + cost)
- [ ] `StreamParser` handles malformed lines as `EventRaw` without crashing
- [ ] `StreamParser` respects context cancellation
- [ ] `RingBuffer` stores up to `capacity` lines, drops oldest on overflow
- [ ] `RingBuffer` is safe for concurrent read/write (`go test -race` passes)
- [ ] `RingBuffer.Lines()` returns lines in correct insertion order through wrap-around
- [ ] `RingBuffer.Tail(n)` returns the most recent n lines
- [ ] `Manager.Start()` spawns a subprocess and updates run state to `StateRunning`
- [ ] `Manager.Start()` enforces `MaxConcurrentRuns` limit
- [ ] `Manager.Stop()` sends SIGTERM, then SIGKILL after 5s grace period
- [ ] `Manager.Pause()` sends SIGSTOP and sets run state to `StatePaused`
- [ ] `Manager.Resume()` sends SIGCONT and sets run state to `StateRunning`
- [ ] Manager's event consumer routes `StreamEvent` content to the run's `RingBuffer`
- [ ] Manager's event consumer updates run store tokens/cost on `EventResult`
- [ ] Manager detects process exit and transitions run to `StateCompleted` or `StateFailed`
- [ ] `ClaudeRuntime` constructs correct `claude -p` command with all flags
- [ ] `ClaudeRuntime` returns descriptive error when `claude` binary is not found
- [ ] `Runtime` interface includes `Pause` and `Resume` methods
- [ ] `LogLineMsg` is defined and routed through the TUI to trigger log viewer refresh
- [ ] Log viewer reads from `RingBuffer` instead of mock content when available
- [ ] Log lines are formatted as `[HH:MM:SS skill] <text>`
- [ ] All new tests pass with `go test -race ./internal/process/...`
- [ ] `go vet ./...` and `go build ./...` pass cleanly

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# All packages compile
go build ./...

# No vet issues
go vet ./...

# Process package tests with race detector
go test -race ./internal/process/... -v

# Runtime package tests
go test -race ./internal/runtime/... -v

# TUI tests still pass
go test ./internal/tui/... -v

# All tests pass with race detector
go test -race ./...

# Binary builds
make build
```

## Notes

- The `process.StreamEvent` type replaces the placeholder `runtime.Event` type. The `runtime.Process.Events` channel now carries `process.StreamEvent` values, creating a dependency from `runtime` → `process`. This is acceptable — the `process` package is a lower-level infrastructure layer. If the dependency direction is problematic, the event types can be moved to a shared `internal/event` package, but the current layout matches the project structure in `docs/agtop.md`.

- The Claude Code `stream-json` format outputs one JSON object per line. Key shapes:
  ```json
  {"type":"assistant","message":{"content":[{"type":"text","text":"..."}]}}
  {"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"..."}}]}}
  {"type":"result","result":"...","usage":{"input_tokens":1234,"output_tokens":567},"total_cost_usd":0.042}
  ```
  The exact schema should be validated against real `claude -p --output-format stream-json` output during implementation. The parser should be lenient — unmarshal only the fields it needs and ignore unknown fields (`json.Decoder` with `DisallowUnknownFields` OFF).

- SIGSTOP/SIGCONT are Unix-only. The `Pause`/`Resume` methods should be implemented in a `_unix.go` build-tagged file if Windows support is ever needed. For now, assume Linux/macOS only.

- The `Manager.SetProgram` pattern (setting the `*tea.Program` after construction) is a pragmatic workaround for Bubble Tea's architecture where the program is created after the model. The manager needs the program to send `LogLineMsg` messages. An alternative is to have the manager write to a channel and have `App.Init()` return a command that listens on it, similar to the store's `Changes()` pattern. The channel approach is more idiomatic for Bubble Tea and should be preferred if `SetProgram` proves awkward.

- The mock data seeding in `NewApp()` should remain as a fallback when the process manager is nil (e.g., `claude` binary not installed). This lets developers work on the TUI without having Claude Code installed.

- The ring buffer capacity (10000 lines) is not yet configurable via `agtop.yaml`. A future config field `ui.log_buffer_size` can be added, but the default is sufficient for now.

- The stream parser's 1MB max scanner token size accommodates large tool outputs (e.g., file reads, grep results). Claude Code's stream-json can produce very long lines for tool results.
