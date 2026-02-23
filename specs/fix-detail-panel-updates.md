# Fix: Detail panel elapsed time, total tokens, and column alignment

## Metadata

type: `fix`
task_id: `detail-panel-updates`
prompt: `the details panel needs some updates. the time of run always shows 0s. I want total tokens to be included. also the column spacing needs adjusting.`

## Bug Description

The details panel has three display issues:

1. **Elapsed time always shows 0s** — The status line shows the run state with elapsed time (e.g., `running (0s)`), but it never updates because there is no periodic tick timer. The UI only re-renders on `RunStoreUpdatedMsg` events. Between store events, the elapsed time is frozen at whatever it was during the last render. Additionally, terminal runs (completed/failed/accepted/rejected) never show their total duration.

2. **Total tokens not shown** — The Tokens row displays `"X in / Y out"` but does not include the total token count. The `r.Tokens` field already tracks this value.

3. **Column spacing misaligned** — Key-value rows use variable-width keys (`Task`, `Prompt`, `Status`, `Skill`, `Branch`, `Model`, `Tokens`, `Cost`) so values don't align vertically. The per-skill breakdown table column widths are also tight.

## Reproduction Steps

1. Start agtop and launch a run
2. Select the run in the run list — observe the detail panel
3. **Elapsed time**: Status shows `running (0s)` and rarely updates; once the run completes, no duration is shown at all
4. **Total tokens**: The Tokens row shows only `"3.2k in / 1.8k out"` with no total
5. **Column spacing**: Values for different keys start at different horizontal positions

**Expected behavior:**
- Elapsed time ticks every second for active runs; terminal runs show their final duration
- Tokens row includes total: `"5.0k (3.2k in / 1.8k out)"`
- Key labels are right-padded to a fixed width so values align

## Root Cause Analysis

### Issue 1: Elapsed time frozen at 0s

- `internal/ui/panels/detail.go:78-79` — Displays `text.FormatElapsed(r.ElapsedTime())` only for non-terminal runs
- `internal/run/run.go:76-81` — `ElapsedTime()` computes `time.Since(r.StartedAt)` which is correct at call time, but the UI only re-renders when the store emits a change event
- `internal/ui/app.go:196-198` — `Init()` only returns `listenForChanges()`. No periodic `tea.Tick()` command exists to refresh elapsed time
- `internal/run/run.go` — No `CompletedAt` field exists, so there's no way to compute final duration for terminal runs

### Issue 2: Total tokens missing

- `internal/ui/panels/detail.go:117-118` — Formats tokens as `"%s in / %s out"` using `r.TokensIn` and `r.TokensOut` only. The `r.Tokens` (total) field is available but unused in the main details section (it's only used in the per-skill total row at line 161)

### Issue 3: Column alignment

- `internal/ui/panels/detail.go:88-93` — The `row()` and `styledRow()` functions render `key + ": " + val` with no fixed-width formatting on the key, causing value misalignment

## Relevant Files

- `internal/ui/panels/detail.go` — Detail panel rendering. All three issues are fixed here (token display, column alignment, elapsed time display)
- `internal/ui/app.go` — App init and update loop. Needs a periodic tick timer for elapsed time refresh
- `internal/run/run.go` — Run struct and `ElapsedTime()` method. Needs `CompletedAt` field and updated method
- `internal/ui/panels/detail_test.go` — Existing detail tests; needs updates for new token format
- `internal/process/manager.go` — Sets `StartedAt` when run begins; needs to set `CompletedAt` when run ends
- `internal/run/persistence.go` — Session persistence; `CompletedAt` should be included in serialization (already handled by JSON tags on the Run struct)

## Fix Strategy

### 1. Add `CompletedAt` field to Run and update `ElapsedTime()`

Add `CompletedAt time.Time` to the `Run` struct. Update `ElapsedTime()` to return `CompletedAt - StartedAt` for terminal runs and `time.Since(StartedAt)` for active runs.

### 2. Set `CompletedAt` in the process manager

When a run reaches a terminal state in the process manager's event consumer, set `CompletedAt = time.Now()`.

### 3. Add a periodic tick timer to the app

Add a 1-second `tea.Tick()` command that fires a `TickMsg`. On receiving `TickMsg`, return another tick command to keep the timer running. This causes the UI to re-render, updating the elapsed time display.

### 4. Update token display to include total

Change the Tokens row format from `"X in / Y out"` to `"X (Y in / Z out)"` where X is the total.

### 5. Fix column alignment with fixed-width keys

Pad all key labels to a consistent width (e.g., 10 characters) using `fmt.Sprintf("%-10s", key)` or a `lipgloss.Width()` constraint so values align vertically.

### 6. Show elapsed time for terminal runs

Remove the `!r.IsTerminal()` guard so both active and completed runs show their duration. The `ElapsedTime()` method handles both cases with the new `CompletedAt` field.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `CompletedAt` to Run struct and update `ElapsedTime()`

- In `internal/run/run.go`, add `CompletedAt time.Time` field after `StartedAt` with json tag `"completed_at"`
- Update `ElapsedTime()`: if `CompletedAt` is non-zero, return `r.CompletedAt.Sub(r.StartedAt)`; otherwise return `time.Since(r.StartedAt)`

### 2. Set `CompletedAt` when runs terminate

- In `internal/process/manager.go`, find where run state transitions to terminal states (completed, failed). In the store update callback, set `r.CompletedAt = time.Now()`
- Search for all `r.State = run.StateCompleted` and `r.State = run.StateFailed` assignments in `manager.go` and add `r.CompletedAt = time.Now()` alongside them

### 3. Add periodic tick timer to app

- In `internal/ui/app.go`, define a `TickMsg struct{}` type
- In `Init()`, return `tea.Batch(listenForChanges(a.store.Changes()), tickCmd())` where `tickCmd()` returns `tea.Tick(time.Second, func(time.Time) tea.Msg { return TickMsg{} })`
- In `Update()`, handle `TickMsg` by returning `tickCmd()` to keep the timer running (the re-render is implicit)

### 4. Update detail panel token display

- In `internal/ui/panels/detail.go:117-118`, change the token format string to include total:
  ```go
  tokStr := fmt.Sprintf("%s (%s in / %s out)", text.FormatTokens(r.Tokens), text.FormatTokens(r.TokensIn), text.FormatTokens(r.TokensOut))
  ```

### 5. Fix detail panel column alignment

- In `internal/ui/panels/detail.go`, update the `row()` function to use a fixed-width key. Determine the max key length from the labels used (e.g., "Worktree" = 8, "DevServer" = 9, "Command" = 7). Use a constant like `keyWidth := 9` and format as:
  ```go
  row := func(key, val string) string {
      paddedKey := fmt.Sprintf("%-9s", key)
      return keyStyle.Render(paddedKey + ": ") + valStyle.Render(val)
  }
  ```
- Apply the same padding to `styledRow()`

### 6. Show elapsed time for terminal runs

- In `internal/ui/panels/detail.go:78`, change the condition from `!r.IsTerminal() && !r.StartedAt.IsZero()` to just `!r.StartedAt.IsZero()` so both active and completed runs show their duration

### 7. Update tests

- In `internal/ui/panels/detail_test.go`, update `TestDetailSetRun` to check for the new token format that includes total (e.g., `"5.0k"` for total and `"3.2k in"`)
- Add a test for terminal run elapsed time display with `CompletedAt` set

## Regression Testing

### Tests to Add

- Test that terminal runs with `CompletedAt` set display the correct elapsed duration
- Test that the new token format includes total, in, and out values
- Test that key labels are padded consistently (values start at the same column)

### Existing Tests to Verify

- `TestDetailNoRun` — should still pass (no run selected)
- `TestDetailSetRun` — update expected token format from `"3.2k in"` to also check for total `"5.0k"`
- `TestDetailBorder` — should still pass (border rendering)
- `TestDetailCostColoring` — should still pass (cost display)
- `TestDetailNilRunHandling` — should still pass (nil handling)

## Risk Assessment

- **Tick timer performance** — A 1-second tick is lightweight for a TUI. Bubbletea handles this efficiently. No risk of performance degradation.
- **CompletedAt persistence** — The new field serializes via JSON tags automatically. Existing persisted sessions without `CompletedAt` will default to zero time, which `ElapsedTime()` handles by falling back to `time.Since(StartedAt)`. For terminal runs loaded from old sessions this means the elapsed time will be inaccurate (showing time since original start), but this is acceptable for a transitional period.
- **Column width** — A fixed 9-character key width accommodates all current labels. If new keys are added longer than 9 chars, the constant needs updating.

## Validation Commands

```bash
go test ./internal/ui/panels/... -run TestDetail -v
go test ./internal/run/... -v
go build ./...
```

## Open Questions (Unresolved)

None — all requirements are clear from the request.
