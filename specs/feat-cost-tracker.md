# Feature: Cost Tracker

## Metadata

type: `feat`
task_id: `cost-tracker`
prompt: `Implement step 8 from docs/agtop.md — Cost Tracker: per-run and global token/cost aggregation, per-skill breakdowns, rate limit detection with auto-resume, and status bar/detail panel integration.`

## Feature Description

The Cost Tracker is the centralized cost accounting system for agtop. It replaces the ad-hoc cost accumulation currently scattered across the process manager with a proper `cost.Tracker` that maintains per-skill cost breakdowns, session-wide aggregates, and threshold enforcement. It also adds rate limit detection (429 errors) with automatic backoff and resume — a critical feature for long-running multi-skill workflows.

The existing codebase already has the plumbing: the stream parser extracts `UsageData` from `result` events, the process manager accumulates `run.Tokens` and `run.Cost`, the run store provides `TotalCost()`/`TotalTokens()`, and the UI displays cost with color thresholds. What's missing is: per-skill cost ledger, proper input/output token tracking, rate limit detection, backoff auto-resume, and the detail panel's per-skill cost breakdown view.

## User Story

As a developer running concurrent AI agent workflows
I want to see per-skill token and cost breakdowns with automatic rate limit handling
So that I understand exactly where my budget goes and runs recover automatically from transient API errors

## Problem Statement

1. **No per-skill visibility**: The Run struct tracks aggregate `Tokens`/`Cost` but not per-skill. A 7-skill SDLC workflow costing $3.50 gives no breakdown of which skill consumed what.
2. **Input/output tokens unused**: `Run.TokensIn` and `Run.TokensOut` fields exist but are never populated — the process manager only accumulates `TotalTokens`.
3. **Rate limits cause silent failure**: 429 errors from the Claude API are parsed as `EventError` and treated like any other error, causing the run to fail. There's no backoff/retry logic.
4. **Cost/limits stubs are empty**: `cost.Tracker` and `cost.LimitChecker` are 5-line stubs with no methods or logic.
5. **Threshold checking is inline**: Cost/token threshold logic is duplicated in both `consumeEvents` and `consumeSkillEvents` in the process manager, rather than delegated to a dedicated checker.

## Solution Statement

Implement `cost.Tracker` as the single source of truth for all cost data. The process manager delegates cost recording and threshold checking to the tracker. The tracker maintains a per-run ledger of `SkillCost` entries, populates `TokensIn`/`TokensOut`, and provides query methods for the UI. Rate limit detection is added to the stream event handling with configurable backoff and auto-resume via `SIGCONT`.

## Relevant Files

Use these files to implement the feature:

- `internal/cost/tracker.go` — Currently a stub. Will become the core cost tracking implementation with per-skill ledger, session aggregates, and query methods.
- `internal/cost/limits.go` — Currently a stub. Will become the threshold checker with rate limit detection and backoff logic.
- `internal/process/manager.go` — Contains inline cost accumulation and threshold checking (lines 285-305, 408-435). Will be refactored to delegate to `cost.Tracker`.
- `internal/process/stream.go` — Stream parser. Needs a new heuristic to detect rate limit errors (429 patterns in `EventError` text).
- `internal/run/run.go` — `Run` struct. Needs a `SkillCosts` field for per-skill breakdown.
- `internal/run/store.go` — Run store. No changes needed — existing `TotalCost()`/`TotalTokens()` work as-is.
- `internal/config/config.go` — `LimitsConfig` struct. Needs `RateLimitMaxRetries` field.
- `internal/config/defaults.go` — Defaults for new config fields.
- `internal/ui/panels/statusbar.go` — Status bar. Will add token count display.
- `internal/ui/panels/detail.go` — Detail panel. Will add per-skill cost breakdown section.
- `internal/ui/text/format.go` — Formatting utilities. Already has `FormatCost`/`FormatTokens`.
- `internal/ui/panels/messages.go` — Panel messages. May need a `CostThresholdMsg` for modal notifications.

### New Files

- `internal/cost/tracker_test.go` — Unit tests for the tracker.
- `internal/cost/limits_test.go` — Unit tests for the limit checker.

## Implementation Plan

### Phase 1: Foundation

Build the `cost.Tracker` and `cost.LimitChecker` with proper data structures, recording methods, and query APIs. These are self-contained packages with no TUI dependencies.

### Phase 2: Core Implementation

Integrate the tracker into the process manager, replacing inline cost logic. Add rate limit detection to the stream parser. Wire per-skill cost recording into the executor's skill loop. Populate `TokensIn`/`TokensOut` on the Run struct.

### Phase 3: Integration

Update the UI: add token count to the status bar, add per-skill cost breakdown to the detail panel, and add a `CostThresholdMsg` for modal notifications when thresholds are breached.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Implement cost.Tracker

Rewrite `internal/cost/tracker.go` with:

- `SkillCost` struct:
  ```go
  type SkillCost struct {
      SkillName    string
      InputTokens  int
      OutputTokens int
      TotalTokens  int
      CostUSD      float64
      StartedAt    time.Time
      CompletedAt  time.Time
  }
  ```
- `Tracker` struct with a `sync.RWMutex`-protected map of `runID → []SkillCost` and session-wide accumulators (`SessionTokens int`, `SessionCost float64`).
- `Record(runID string, sc SkillCost)` — Appends a skill cost entry, updates session totals.
- `RunCosts(runID string) []SkillCost` — Returns the per-skill ledger for a run.
- `RunTotal(runID string) (tokens int, cost float64)` — Sums a single run's costs.
- `SessionTotal() (tokens int, cost float64)` — Returns session-wide aggregates.
- `Remove(runID string)` — Cleans up ledger for a removed run (e.g., rejected parallel sub-task).

### 2. Implement cost.LimitChecker

Rewrite `internal/cost/limits.go` with:

- `LimitChecker` struct holding `MaxTokensPerRun`, `MaxCostPerRun` from config.
- `CheckRun(runTokens int, runCost float64) (exceeded bool, reason string)` — Returns whether thresholds are breached and a human-readable reason.
- `IsRateLimit(errorText string) bool` — Detects rate limit errors by matching patterns: `"rate limit"`, `"429"`, `"too many requests"`, `"overloaded"` (case-insensitive).

### 3. Add SkillCosts to Run struct

In `internal/run/run.go`:

- Import `cost` package (use a type alias or define `SkillCost` locally to avoid circular imports — prefer defining `SkillCost` in the `run` package directly and having `cost.Tracker` use `run.SkillCost`).
- Actually, to avoid circular imports, define `SkillCost` in the `cost` package and have the `Run` struct store `[]cost.SkillCost`. The `run` package imports `cost` (which has no dependencies on `run`).
- Add `SkillCosts []cost.SkillCost` field to `Run`.

### 4. Add RateLimitMaxRetries to config

In `internal/config/config.go`, add to `LimitsConfig`:

```go
RateLimitMaxRetries int `yaml:"rate_limit_max_retries"`
```

In `internal/config/defaults.go`, set default: `RateLimitMaxRetries: 3`.

### 5. Integrate Tracker into Process Manager

In `internal/process/manager.go`:

- Add `tracker *cost.Tracker` and `limiter *cost.LimitChecker` fields to the `Manager` struct.
- Update `NewManager` to accept and store these.
- In both `consumeEvents` and `consumeSkillEvents`, on `EventResult` with `Usage`:
  - Populate `r.TokensIn` and `r.TokensOut` (currently only `r.Tokens` is set):
    ```go
    r.TokensIn += event.Usage.InputTokens
    r.TokensOut += event.Usage.OutputTokens
    r.Tokens += event.Usage.TotalTokens
    r.Cost += event.Usage.CostUSD
    ```
  - Record to tracker:
    ```go
    m.tracker.Record(runID, cost.SkillCost{
        SkillName:    skillName(),
        InputTokens:  event.Usage.InputTokens,
        OutputTokens: event.Usage.OutputTokens,
        TotalTokens:  event.Usage.TotalTokens,
        CostUSD:      event.Usage.CostUSD,
        CompletedAt:  time.Now(),
    })
    ```
  - Replace inline threshold checking with `m.limiter.CheckRun(r.Tokens, r.Cost)`.
- On `EventError`, check `m.limiter.IsRateLimit(event.Text)`. If true:
  - Log a rate limit warning to the buffer.
  - Do NOT treat as a fatal error — the process manager doesn't retry (that's the executor's job via skill re-execution). Instead, log the detection so the executor can handle it.

### 6. Add Rate Limit Handling to Executor

In `internal/engine/executor.go`, in `runSkill`:

- After receiving a `SkillResult` with a non-nil `Err`, check if the error text matches rate limit patterns (delegate to a helper or check for a sentinel error).
- If rate limit detected and retries remain (track per-skill retry count up to `cfg.Limits.RateLimitMaxRetries`):
  - Log backoff countdown to the run's ring buffer.
  - Sleep for `cfg.Limits.RateLimitBackoff` seconds (respecting context cancellation).
  - Retry the skill.
- If retries exhausted, fail the run as normal.

### 7. Update Status Bar with Token Count

In `internal/ui/panels/statusbar.go`:

- Add total token count to the status bar display, using the store's existing `TotalTokens()`:
  ```
  agtop v0.1.0 │ 2 running 1 queued 3 done │ Tokens: 142.3k │ Total: $4.87 │ ?:help
  ```
- Use `text.FormatTokens(s.store.TotalTokens())` for formatting.

### 8. Add Per-Skill Cost Breakdown to Detail Panel

In `internal/ui/panels/detail.go`:

- After the existing key-value rows, add a per-skill breakdown section when `r.SkillCosts` is non-empty:
  ```
  Skill       Tokens    Cost
  route          1.2k   $0.02
  spec          12.4k   $0.38
  build         28.1k   $0.92
  ────────────────────────────
  Total         41.7k   $1.32
  ```
- Use `text.FormatTokens` and `text.FormatCost` for formatting.
- Apply `styles.CostColor` to each row's cost value.

### 9. Add CostThresholdMsg for Modal Notification

In `internal/ui/panels/messages.go`, add:

```go
type CostThresholdMsg struct {
    RunID  string
    Reason string
}
```

In the process manager, when a threshold is breached, send this message via `m.program.Send(...)` so the TUI can display a modal or highlighted warning. The app model in `ui/app.go` should handle this message by showing the reason in the status bar or a brief flash notification (not a blocking modal — the run is already paused).

### 10. Write Tests

- `internal/cost/tracker_test.go`:
  - Test `Record` and `RunCosts` return correct entries.
  - Test `RunTotal` sums correctly across multiple skill entries.
  - Test `SessionTotal` aggregates across multiple runs.
  - Test `Remove` cleans up a run's ledger.
  - Test concurrent `Record` calls (goroutine safety).

- `internal/cost/limits_test.go`:
  - Test `CheckRun` returns exceeded for cost over threshold.
  - Test `CheckRun` returns exceeded for tokens over threshold.
  - Test `CheckRun` returns not exceeded when under both thresholds.
  - Test `CheckRun` with zero thresholds (disabled).
  - Test `IsRateLimit` matches "rate limit", "429", "too many requests", "overloaded" (case-insensitive).
  - Test `IsRateLimit` rejects unrelated error messages.

### 11. Update Constructor Call Sites

Update all call sites that construct `process.Manager`:

- `cmd/agtop/main.go` (or wherever `NewManager` is called): pass the new `tracker` and `limiter` arguments.
- Ensure the tracker is also accessible to the UI for per-skill breakdown queries (either via the run store's `SkillCosts` field or by passing the tracker to detail panel).

## Testing Strategy

### Unit Tests

- **cost.Tracker**: Record, query, remove, concurrent safety, session aggregation.
- **cost.LimitChecker**: Threshold checking (cost, tokens, both, neither, disabled). Rate limit pattern matching.
- **Process manager integration**: Verify that after processing a `result` event, `Run.TokensIn`, `Run.TokensOut`, and `Run.SkillCosts` are populated correctly. Existing `manager_test.go` tests should continue passing.

### Edge Cases

- Rate limit on the very first skill of a workflow (no previous output to preserve).
- Rate limit retries exhausted — run transitions to failed state with clear error message.
- Cost threshold breached mid-parallel-execution (multiple sub-tasks running) — only the parent run is paused.
- Zero-cost result events (some tool-only turns may report 0 cost) — should be recorded but not trigger thresholds.
- Run removal while tracker has entries — `Remove` should not panic.
- Concurrent threshold checks from parallel sub-tasks accumulating to the same parent run.

## Acceptance Criteria

- [ ] `cost.Tracker` records per-skill cost entries and provides `RunCosts`, `RunTotal`, `SessionTotal` queries
- [ ] `cost.LimitChecker.CheckRun` correctly enforces `max_tokens_per_run` and `max_cost_per_run` thresholds
- [ ] `cost.LimitChecker.IsRateLimit` detects 429/rate-limit error patterns
- [ ] `Run.TokensIn` and `Run.TokensOut` are populated from stream-json usage data
- [ ] `Run.SkillCosts` contains a per-skill cost breakdown after workflow execution
- [ ] Process manager delegates threshold checking to `LimitChecker` instead of inline logic
- [ ] Rate-limited skills are retried up to `rate_limit_max_retries` with `rate_limit_backoff` delay
- [ ] Status bar displays session token count alongside cost: `Tokens: 142.3k │ Total: $4.87`
- [ ] Detail panel shows per-skill cost breakdown table when a run is selected
- [ ] All existing tests pass (`go vet ./...`, `go test ./...`)
- [ ] New unit tests for `cost.Tracker` and `cost.LimitChecker` pass

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
go vet ./...
go build ./...
go test ./internal/cost/...
go test ./internal/process/...
go test ./internal/engine/...
go test ./internal/run/...
go test ./internal/ui/...
go test ./...
```

## Notes

- **Circular import avoidance**: `cost.SkillCost` is defined in the `cost` package. The `run` package imports `cost` to use the type. The `cost.Tracker` does NOT import `run` — it receives `runID string` and `SkillCost` values, keeping the dependency graph clean.
- **Parallel sub-task cost**: The executor already accumulates parallel sub-task cost to the parent run (see `executor.go:413-418`). The tracker should also record these under the parent run's ledger with a skill name like `"build (sub-task-name)"` for granularity.
- **Future: session persistence**: The tracker's data is in-memory only. Step 15 (session persistence) will serialize tracker state to disk. The tracker's data structures should be JSON-serializable for this future integration.
- **Rate limit backoff strategy**: Use fixed backoff (not exponential) from `config.Limits.RateLimitBackoff` for simplicity. Exponential backoff can be added later if needed.
