# Feature: OpenCode Runtime Support

## Metadata

type: `feat`
task_id: `opencode-runtime-support`
prompt: `Implement OpenCode runtime support behind the Runtime interface with config-driven runtime selection, OpenCode-specific stream parsing, and integration across the registry, app, and executor.`

## Feature Description

Add OpenCode as a second runtime in agtop, fulfilling the runtime abstraction that the codebase was designed for. The `Runtime` interface already exists with `ClaudeRuntime` as the sole implementation. This feature implements `OpenCodeRuntime`, wires up config-driven runtime selection so `config.Runtime.Default` actually controls which runtime is instantiated, and adapts the stream parser to handle OpenCode's JSON output format alongside Claude's `stream-json`.

## User Story

As a developer using OpenCode as my AI coding agent
I want to run agtop workflows against OpenCode
So that I can use agtop's orchestration, cost tracking, and monitoring regardless of which agent runtime I prefer.

## Problem Statement

The codebase has a clean `Runtime` interface and `OpenCodeConfig` in the config system, but:

1. `OpenCodeRuntime` is an empty struct — no methods implemented.
2. `NewApp()` in `internal/ui/app.go:82` hardcodes `runtime.NewClaudeRuntime()` — the `config.Runtime.Default` field is never read.
3. `Registry.SkillForRun()` in `internal/engine/registry.go:151-166` hardcodes `r.cfg.Runtime.Claude.*` for model, tools, turns, and permission mode — OpenCode config is never consulted.
4. The `StreamParser` in `internal/process/stream.go` expects Claude Code's `stream-json` format. OpenCode uses `--output-format json` with a different event structure.

## Solution Statement

1. Implement `OpenCodeRuntime` with all four `Runtime` interface methods (`Start`, `Stop`, `Pause`, `Resume`), constructing the correct `opencode run` CLI invocation with `--format json`.
2. Add a runtime factory function that reads `config.Runtime.Default` and returns the appropriate `Runtime` implementation.
3. Add an `OpenCodeStreamParser` (or adapter) that translates OpenCode's JSON event format into the existing `StreamEvent` types so the process manager's event consumer works unchanged.
4. Update `Registry.SkillForRun()` to resolve options from the active runtime's config rather than hardcoding Claude.
5. Wire up runtime selection in `NewApp()`.

## Relevant Files

Use these files to implement the feature:

- `internal/runtime/runtime.go` — `Runtime` interface and shared types (`RunOptions`, `Process`). May need to extend `RunOptions` for OpenCode-specific fields.
- `internal/runtime/claude.go` — Reference implementation. OpenCode runtime follows the same pattern.
- `internal/runtime/opencode.go` — Empty struct to be implemented.
- `internal/process/stream.go` — `StreamParser` and `StreamEvent` types. Needs OpenCode format support.
- `internal/process/manager.go` — Process manager that consumes events. Should remain unchanged if the parser abstraction is clean.
- `internal/engine/registry.go` — `SkillForRun()` hardcodes Claude config. Needs runtime-aware option resolution.
- `internal/ui/app.go` — `NewApp()` hardcodes Claude runtime. Needs factory-based selection.
- `internal/config/config.go` — `RuntimeConfig`, `OpenCodeConfig` structs. May need additional fields.
- `internal/config/defaults.go` — Default values for OpenCode config.

### New Files

- `internal/runtime/factory.go` — Runtime factory function that selects implementation based on config.

## Implementation Plan

### Phase 1: Foundation

Implement the `OpenCodeRuntime` struct and factory function. This is the core enabler — once the runtime can start/stop/pause/resume OpenCode processes, the rest is integration.

Research OpenCode's CLI flags and JSON output format. OpenCode's `run` command supports `--format json` for structured output, `--model` for model selection, and `--agent` for agent selection. The JSON output format differs from Claude's `stream-json` — events use different type names and structures.

### Phase 2: Core Implementation

Build the OpenCode stream parser adapter. The key challenge is mapping OpenCode's JSON events to the existing `StreamEvent` types (`EventText`, `EventToolUse`, `EventToolResult`, `EventResult`, `EventError`, `EventRaw`). OpenCode emits events with a `type` field and `properties` object. The adapter reads these and produces `StreamEvent` values, keeping the process manager's `consumeEvents`/`consumeSkillEvents` methods untouched.

Update `SkillForRun()` to be runtime-aware. Add a `RuntimeName` concept (or use the existing `config.Runtime.Default` string) to switch between Claude and OpenCode config when resolving model, tools, turns, and permissions.

### Phase 3: Integration

Wire runtime selection into `NewApp()` via the factory function. Update the fallback logic so missing `opencode` binary falls back to Claude (and vice versa, per the agtop.md spec). Ensure the new run modal and executor work end-to-end with OpenCode.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Implement Runtime Factory

- Create `internal/runtime/factory.go` with a `NewRuntime(cfg *config.RuntimeConfig) (Runtime, error)` function.
- Read `cfg.Default` to select `"claude"` or `"opencode"`.
- For `"claude"`: call `NewClaudeRuntime()`.
- For `"opencode"`: call `NewOpenCodeRuntime()`.
- If the selected runtime binary is missing, attempt the other as fallback (per agtop.md edge case: "Missing `opencode` binary: Fall back to claude if available"). Log a warning on fallback.
- Return an error only if neither runtime is available.

### 2. Implement OpenCodeRuntime

- In `internal/runtime/opencode.go`, add fields: `opencodePath string`.
- Implement `NewOpenCodeRuntime() (*OpenCodeRuntime, error)`:
  - Use `exec.LookPath("opencode")` to find the binary.
  - Return a clear error message if not found.
- Implement `BuildArgs(prompt string, opts RunOptions) []string`:
  - Base: `["run", prompt]`.
  - `--format json` for structured output.
  - `--model <model>` if `opts.Model` is set.
  - OpenCode does not support `--allowedTools` or `--permission-mode` — skip these. OpenCode auto-approves all permissions in non-interactive mode.
  - `--quiet` to suppress spinner (not useful for subprocess).
- Implement `Start(ctx, prompt, opts)`:
  - Build args, create `exec.CommandContext`, set `cmd.Dir` to `opts.WorkDir`.
  - Pipe stdout/stderr, start process, return `*Process` with done channel.
  - Follow the same pattern as `ClaudeRuntime.Start()`.
- Implement `Stop(proc)`: SIGTERM → 5s grace → SIGKILL (same as Claude).
- Implement `Pause(proc)`: SIGSTOP signal.
- Implement `Resume(proc)`: SIGCONT signal.

### 3. Extend RunOptions for OpenCode

- Add an `Agent string` field to `RunOptions` in `internal/runtime/runtime.go`.
- OpenCode supports `--agent <name>` to select which agent handles the prompt. Claude ignores this field.
- Update `OpenCodeRuntime.BuildArgs()` to include `--agent` when set.

### 4. Implement OpenCode Stream Parser

- OpenCode's `--format json` outputs JSON objects, one per line. The event structure differs from Claude's `stream-json`:
  - OpenCode events have a `type` field with values like `"message.part.updated"`, `"message.updated"`, `"session.updated"`, etc.
  - Content is nested differently (e.g., `properties.part.text` for text content).
  - Usage/cost data may appear in session or message metadata.
- Create an `OpenCodeStreamParser` in `internal/process/stream.go` (or a new file `internal/process/stream_opencode.go`) that:
  - Reads line-by-line JSON from stdout (same as `StreamParser`).
  - Maps OpenCode event types to `StreamEvent` types:
    - Text content → `EventText`
    - Tool invocations → `EventToolUse`
    - Tool results → `EventToolResult`
    - Session/message completion with usage → `EventResult`
    - Errors → `EventError`
    - Unknown events → `EventRaw`
  - Implements the same `Events() <-chan StreamEvent` and `Done() <-chan error` interface as `StreamParser`.
- Extract a `StreamParserInterface` (or just use the existing struct pattern) so the process manager can work with either parser.

### 5. Add Parser Selection to Process Manager

- The process manager currently creates `NewStreamParser(mp.proc.Stdout, 256)` directly in `consumeEvents` and `consumeSkillEvents`.
- Add a field or method to `Manager` that indicates which parser to use (based on the runtime type).
- Option A: Add a `ParserFactory func(io.Reader, int) StreamParserInterface` to `Manager`.
- Option B: Add a `runtimeName string` field to `Manager` and select the parser in `consumeEvents`/`consumeSkillEvents`.
- Option B is simpler and recommended. The manager already holds the `runtime.Runtime` — add a method `Runtime.Name() string` to the interface, or pass the name separately.

### 6. Update Registry for Runtime-Aware Option Resolution

- In `internal/engine/registry.go`, `SkillForRun()` currently hardcodes:
  ```go
  model := r.cfg.Runtime.Claude.Model
  allowedTools := r.cfg.Runtime.Claude.AllowedTools
  opts := runtime.RunOptions{
      MaxTurns:       r.cfg.Runtime.Claude.MaxTurns,
      PermissionMode: r.cfg.Runtime.Claude.PermissionMode,
  }
  ```
- Change this to read `r.cfg.Runtime.Default` and select the appropriate config:
  - If `"claude"`: use `r.cfg.Runtime.Claude.*` (current behavior).
  - If `"opencode"`: use `r.cfg.Runtime.OpenCode.Model` for model, `r.cfg.Runtime.OpenCode.Agent` for the new `RunOptions.Agent` field, and skip tools/turns/permission since OpenCode doesn't support them.
- Skill-level model overrides still take precedence regardless of runtime.

### 7. Wire Runtime Selection in NewApp

- In `internal/ui/app.go`, replace:
  ```go
  rt, err := runtime.NewClaudeRuntime()
  ```
  with:
  ```go
  rt, err := runtime.NewRuntime(&cfg.Runtime)
  ```
- Keep the existing fallback logic: if `rt` is nil, log warning and seed mock data.

### 8. Extend OpenCodeConfig

- In `internal/config/config.go`, add fields to `OpenCodeConfig` if needed:
  - `MaxTurns int` — if OpenCode supports turn limits in future.
  - For now, `Model` and `Agent` are sufficient.
- In `internal/config/defaults.go`, verify the defaults are sensible:
  - `Model: "anthropic/claude-sonnet-4-5"` — reasonable default.
  - `Agent: "build"` — the OpenCode default agent.

### 9. Add Tests

- `internal/runtime/opencode_test.go`:
  - Test `BuildArgs()` produces correct CLI flags for various `RunOptions`.
  - Test `NewOpenCodeRuntime()` error when binary not found (mock `exec.LookPath`).
- `internal/runtime/factory_test.go`:
  - Test factory returns `ClaudeRuntime` for `Default: "claude"`.
  - Test factory returns `OpenCodeRuntime` for `Default: "opencode"`.
  - Test fallback logic when one binary is missing.
- `internal/process/stream_opencode_test.go`:
  - Test parsing of OpenCode JSON events into `StreamEvent` types.
  - Test edge cases: malformed JSON, unknown event types, missing usage data.
- `internal/engine/registry_test.go`:
  - Test `SkillForRun()` returns Claude-derived options when default is `"claude"`.
  - Test `SkillForRun()` returns OpenCode-derived options when default is `"opencode"`.

## Testing Strategy

### Unit Tests

- **OpenCodeRuntime.BuildArgs**: Verify correct arg construction for all combinations of model, agent, workdir, and format flags.
- **OpenCodeStreamParser**: Feed captured OpenCode JSON output through the parser and assert correct `StreamEvent` types, text content, tool names, and usage data.
- **Runtime factory**: Verify correct runtime selection and fallback behavior.
- **Registry.SkillForRun**: Verify runtime-aware option resolution for both runtimes with and without skill-level overrides.

### Edge Cases

- OpenCode binary not installed → fallback to Claude with warning log.
- Claude binary not installed → fallback to OpenCode with warning log.
- Neither binary installed → error, mock data mode.
- OpenCode emits unknown event types → treated as `EventRaw`, no crash.
- OpenCode emits empty JSON lines → skipped silently.
- OpenCode process exits with non-zero code → `SkillResult.Err` populated, run marked failed.
- OpenCode doesn't provide usage/cost data → run shows 0 tokens, $0.00 (no crash).
- Config sets `runtime.default: "opencode"` but no `opencode` section → use defaults from `defaults.go`.
- Pause/resume signals work on Linux (SIGSTOP/SIGCONT) — same mechanism as Claude.

## Acceptance Criteria

- [ ] `OpenCodeRuntime` implements all four `Runtime` interface methods.
- [ ] `opencode run` is invoked with correct flags: `--format json`, `--model`, `--agent`, `--quiet`.
- [ ] `runtime.NewRuntime()` factory selects runtime based on `config.Runtime.Default`.
- [ ] Fallback: missing preferred binary falls back to the other runtime with a logged warning.
- [ ] OpenCode JSON events are parsed into `StreamEvent` types that the process manager can consume.
- [ ] Token/cost tracking works for OpenCode runs (or gracefully shows zero if OpenCode doesn't report usage).
- [ ] `Registry.SkillForRun()` resolves options from the active runtime's config.
- [ ] `NewApp()` uses the factory instead of hardcoding `NewClaudeRuntime()`.
- [ ] All existing Claude functionality is unaffected.
- [ ] Unit tests pass for runtime, factory, parser, and registry changes.
- [ ] `make build` compiles without errors.
- [ ] `make lint` passes.

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
make build
make lint
go test ./internal/runtime/... -v
go test ./internal/process/... -v
go test ./internal/engine/... -v
```

## Notes

- **OpenCode's streaming format is not fully documented.** The `--format json` flag outputs line-delimited JSON, but the exact event schema may need to be reverse-engineered from OpenCode's source or by running `opencode run --format json` and inspecting output. The parser should be defensive — unknown event types become `EventRaw`.
- **OpenCode auto-approves all permissions in non-interactive mode.** The `--allowedTools` and `--permission-mode` flags from Claude have no OpenCode equivalent. The `RunOptions` fields are simply ignored by `OpenCodeRuntime.BuildArgs()`.
- **Cost tracking may be limited.** OpenCode may not report `total_cost_usd` in the same way Claude does. The parser should handle missing cost data gracefully.
- **The `--agent` flag** selects which OpenCode agent handles the prompt (e.g., `"build"`, `"code"`). This maps to `config.Runtime.OpenCode.Agent` and the new `RunOptions.Agent` field.
- **Future: per-run runtime override.** The new run modal could eventually allow selecting the runtime per-run instead of globally. This is out of scope for this spec but the factory pattern supports it.
