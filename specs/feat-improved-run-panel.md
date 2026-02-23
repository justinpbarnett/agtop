# Feature: Improved Run Panel with Column Headers and Fixed Elapsed Time

## Metadata

type: `feat`
task_id: `improved-run-panel`
prompt: `Improve the run panel. Should show status icon, run/worktree ID, current state, time in run, total tokens, total cost. There should be a header to this list that says what each column is. Also the total run time counter in seconds is broken and always shows 0s. Also make sure the spacing is consistent and looks nice.`

## Feature Description

The run list panel currently shows a compact row per run: status icon, skill name, branch identifier, elapsed time, and cost. Several problems exist:

1. **Missing columns** — Run/worktree ID and current state are not displayed. Total tokens are not shown.
2. **No column header** — The column layout is not self-documenting; users must guess what each field represents.
3. **Elapsed time bug** — The timer always shows `0s` because `StartedAt` is never set. The `Executor.Execute()` method (executor.go:60-64) sets `State = StateRunning` but never sets `StartedAt`. The `Manager.StartSkill()` method (manager.go:544-546) only sets `r.PID` — unlike `Manager.Start()` (manager.go:176-180) which sets both PID and `StartedAt`. Since the executor exclusively calls `StartSkill()`, `StartedAt` remains zero for all workflow-driven runs.
4. **Inconsistent spacing** — Column widths use hardcoded `%-10s` and `%-14s` format strings with no alignment to headers.

## User Story

As an agtop user
I want the run list to show clear, labeled columns with all key run metrics
So that I can quickly scan run status, identity, timing, and cost at a glance

## Relevant Files

- `internal/ui/panels/runlist.go` — Run list panel rendering. The `renderContent()` method builds each row with `fmt.Sprintf`. Needs column header, new columns, and consistent width formatting.
- `internal/ui/panels/runlist_test.go` — Unit tests for run list navigation, filtering, and rendering. Tests need updated golden data for new columns.
- `internal/ui/panels/runlist_teatest_test.go` — Snapshot tests for run list. Golden files will change.
- `internal/ui/panels/testdata/TestRunListSnapshot.golden` — Golden snapshot for default run list rendering.
- `internal/ui/panels/testdata/TestRunListEmptySnapshot.golden` — Golden snapshot for empty run list.
- `internal/ui/panels/testdata/TestRunListScrollSnapshot.golden` — Golden snapshot for scrolled run list.
- `internal/ui/text/format.go` — `FormatElapsed()` and `FormatTokens()` formatting helpers. No changes needed but used by the new columns.
- `internal/ui/text/truncate.go` — `Truncate()` and `PadRight()` ANSI-aware helpers. Will be used for consistent column widths.
- `internal/ui/styles/theme.go` — Shared styles including `TextDimStyle`, `SelectedRowStyle`. Header will use `TextSecondaryStyle`.
- `internal/engine/executor.go` — `Execute()` method needs to set `StartedAt` when the run begins.
- `internal/run/run.go` — `Run` struct, `StatusIcon()`, `ElapsedTime()`. No changes needed.

## Implementation Plan

### Phase 1: Fix Elapsed Time Bug

Set `StartedAt` in `executor.Execute()` when the run first transitions to running, so the elapsed time counter works correctly from the moment the workflow begins.

### Phase 2: Redesign Run List Columns

Replace the current row format with a new columnar layout:

```
 ●  003  running    1m23s  12.4k  $0.42
```

Columns (left to right):
| Column | Field | Width | Alignment | Source |
|--------|-------|-------|-----------|--------|
| Icon | Status icon | 2 | left | `rn.StatusIcon()` |
| ID | Run ID | 4 | left | `rn.ID` |
| State | Current state | 10 | left | `rn.State` |
| Time | Elapsed time | 6 | right | `text.FormatElapsed(rn.ElapsedTime())` |
| Tokens | Total tokens | 6 | right | `text.FormatTokens(rn.Tokens)` |
| Cost | Total cost | 6 | right | `text.FormatCost(rn.Cost)` |

### Phase 3: Add Column Header Row

Add a dim header row at the top of the content area (below the filter bar if active, above the first run):

```
      ID  STATE       TIME  TOKENS   COST
```

The header should use `styles.TextSecondaryStyle` for a subtle, non-distracting appearance. The header row consumes one row of `availableRows`.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Fix `StartedAt` in Executor

- In `internal/engine/executor.go`, inside `Execute()` at the store update (line 60-64), add `r.StartedAt = time.Now()` alongside `r.State = run.StateRunning`.
- Also set `StartedAt` in the `Resume()` method (line 124-127) — only if `StartedAt` is still zero (avoid resetting on resume).

### 2. Redesign Run List Row Format

- In `internal/ui/panels/runlist.go`, update `renderContent()` to use the new column layout.
- Define column widths as constants at the top of the file for consistency between header and data rows:
  ```go
  const (
      colIconW   = 2
      colIDW     = 5
      colStateW  = 11
      colTimeW   = 7
      colTokensW = 8
      colCostW   = 7
  )
  ```
- Update the selected row format (plain text for background fill) and the non-selected row format to use the same column widths.
- Use `text.PadRight()` for left-aligned columns and `fmt.Sprintf("%*s", width, val)` for right-aligned columns.
- Replace `skillName` and `identifier` columns with `rn.ID` and `string(rn.State)`.
- Add `text.FormatTokens(rn.Tokens)` column between time and cost.

### 3. Add Column Header

- In `renderContent()`, after the filter bar (if active) and before the scroll-up indicator, render a header line:
  ```
       ID  STATE       TIME   TOKENS    COST
  ```
- Style the header with `styles.TextSecondaryStyle`.
- Decrement `availableRows` by 1 for the header row.

### 4. Update Tests and Golden Files

- Update `testStore()` in `runlist_test.go` to set `StartedAt` on test runs so elapsed time is non-zero in snapshots (use a fixed time relative to `CreatedAt`).
- Run `make update-golden` to regenerate golden snapshots.
- Verify `TestRunListView` still passes (it checks for "Runs" title and branch names — update branch name checks to ID checks since branches are no longer displayed).
- Verify `TestRunListFilter` still works (filtering searches on `rn.ID`, `rn.State`, etc. — the `applyFilter` method already searches these fields).

### 5. Verify Elapsed Time Fix

- Confirm the test data with `StartedAt` set produces non-zero elapsed times in the snapshot.
- Verify that `FormatElapsed` produces expected output for the test durations.

## Testing Strategy

### Unit Tests

- Update `TestRunListView` in `runlist_test.go` to check for run IDs and state strings instead of branch names (since branches are removed from the row).
- Add a test case verifying that a run with `StartedAt` set shows a non-zero elapsed time string.
- Existing navigation, bounds, jump, filter, and scrolling tests should continue to pass without changes (they test selection indices, not rendering).

### Edge Cases

- Empty store — "No runs" message unaffected.
- Very narrow terminal width — columns may truncate; the `Truncate()` call on the full line handles this.
- Run with `StartedAt` still zero (queued state) — should display `0s` as before.
- Runs with high token counts (`>1M`) — `FormatTokens` already handles this (`1.2M`).
- Runs with high costs (`>$10`) — `FormatCost` already handles this.

## Risk Assessment

- **Golden snapshot diffs** — All run list golden files will change. This is expected and handled by `make update-golden`.
- **Filter functionality** — `applyFilter()` already searches `rn.ID` and `rn.State`, so filtering will work with the new columns without code changes.
- **Detail panel** — The detail panel reads from the same `Run` struct and already displays `StartedAt`-based elapsed time. The `StartedAt` fix benefits both panels.
- **Column truncation at narrow widths** — The final `text.Truncate(line, width)` prevents overflow, but narrow terminals may cut off rightmost columns. This is acceptable — the most important info (icon, ID, state) is leftmost.

## Validation Commands

```bash
go vet ./...
go test ./internal/ui/panels/...
go test ./internal/engine/...
make update-golden  # if golden files need refresh
go test ./internal/ui/panels/... # re-run after golden update
```

## Open Questions (Unresolved)

None — the scope is well-defined and all columns map to existing `Run` struct fields.

## Sub-Tasks

Single task — no decomposition needed.
