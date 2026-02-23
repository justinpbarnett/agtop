# Fix: Log updates reset active tab from diff to log

## Metadata

type: `fix`
task_id: `log-update-resets-diff-tab`
prompt: `new logs records being added to the log tab should not force swapping from the diff tab to the log tab`

## Bug Description

When the user is viewing the diff tab in the LogView panel (panel [3]), any run store update — including new log lines arriving — snaps the active tab back to the log tab. The user cannot stay on the diff tab while a run is active because the tab resets on every update cycle.

**What happens:** User switches to the diff tab with `l`. A log entry arrives, triggering `RunStoreUpdatedMsg` → `syncSelection()` → `SetRun()` → `activeTab = tabLog`. The diff tab disappears and the log tab is shown.

**What should happen:** The diff tab should remain active. Only explicitly switching runs (selecting a different run in the run list) should reset the tab to the log tab.

## Reproduction Steps

1. Start a run with a workflow (e.g., `build`)
2. While the run is active and producing log output, press `l` to switch to the diff tab
3. Observe: within ~1 second, the view snaps back to the log tab

**Expected behavior:** The diff tab stays active until the user explicitly presses `h` to switch back, or selects a different run.

## Root Cause Analysis

The call chain is:

1. `RunStoreUpdatedMsg` arrives in `App.Update()` (`internal/ui/app.go:284`)
2. This calls `a.syncSelection()` (`internal/ui/app.go:288`)
3. `syncSelection()` unconditionally calls `a.logView.SetRun(...)` (`internal/ui/app.go:692`)
4. `SetRun()` unconditionally resets `l.activeTab = tabLog` (`internal/ui/panels/logview.go:441`)

The problem is that `syncSelection()` does not differentiate between:
- **Selection changed** — the user picked a different run (tab reset is correct)
- **Same run updated** — the currently-selected run got new data (tab reset is wrong)

`RunStoreUpdatedMsg` fires on every store change: state transitions, skill progress, log entries, cost updates — all of which trigger the full `syncSelection()` → `SetRun()` path.

## Relevant Files

- `internal/ui/app.go` — `syncSelection()` at line 682 calls `SetRun()` unconditionally on every store update
- `internal/ui/panels/logview.go` — `SetRun()` at line 418 resets `activeTab = tabLog` on every call

## Fix Strategy

Split `syncSelection()` into two paths based on whether the selected run ID has changed:

1. **Run changed** (different run selected): Call `SetRun()` as before — full reset including `activeTab = tabLog` is correct.
2. **Same run, metadata updated** (skill name, branch, active state changed): Update only the metadata fields that changed without resetting tab state, search state, cursor position, or expanded entries. Add a new `UpdateRunMeta()` method to `LogView` for this purpose.

Track the last-synced run ID in the `App` struct to detect selection changes.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `UpdateRunMeta` method to LogView

In `internal/ui/panels/logview.go`, add a new method that updates run metadata without resetting UI state:

- Add `UpdateRunMeta(skill, branch string, buf *process.RingBuffer, eb *process.EntryBuffer, active bool)` method
- This method updates `l.skill`, `l.branch`, `l.buffer`, `l.entryBuffer`, and `l.active` only
- It does NOT reset `activeTab`, `follow`, `searchQuery`, `cursorEntry`, `expandedEntries`, `copyMode`, or `mouseSelecting`
- Call `l.refreshContent()` at the end to pick up any new content

### 2. Track last-synced run ID in App

In `internal/ui/app.go`:

- Add a `lastSyncedRunID string` field to the `App` struct
- In `syncSelection()`, compare `selected.ID` against `a.lastSyncedRunID`
- If the IDs match (same run): call `a.logView.UpdateRunMeta(...)` instead of `a.logView.SetRun(...)`
- If the IDs differ (new selection) or there's no selection: call `SetRun()` as before and update `a.lastSyncedRunID`

### 3. Update tests

- Add a test in `internal/ui/panels/logview_test.go` verifying that `UpdateRunMeta()` preserves `activeTab` when set to `tabDiff`
- Add a test verifying that `SetRun()` still resets `activeTab` to `tabLog` (existing behavior)

## Regression Testing

### Tests to Add

- `TestUpdateRunMetaPreservesActiveTab` — Set `activeTab = tabDiff`, call `UpdateRunMeta()`, assert `activeTab` is still `tabDiff`
- `TestUpdateRunMetaPreservesSearchState` — Enter search mode, call `UpdateRunMeta()`, assert search query is preserved
- `TestSetRunResetsActiveTab` — Verify `SetRun()` continues to reset `activeTab` to `tabLog` (unchanged behavior)

### Existing Tests to Verify

- `go test ./internal/ui/...` — All existing UI tests must pass
- `go test ./...` — Full test suite must pass
- Golden snapshot tests via `make update-golden` if any view output changes

## Risk Assessment

Low risk. The change is narrowly scoped:
- `SetRun()` behavior is completely unchanged — it still resets everything on run switch
- The new `UpdateRunMeta()` path only activates when the same run is re-synced
- No changes to message handling, key routing, or tab switching logic

## Validation Commands

```bash
go vet ./...
go test ./internal/ui/panels/...
go test ./...
```

## Open Questions (Unresolved)

None — the fix is straightforward and doesn't require design decisions.
