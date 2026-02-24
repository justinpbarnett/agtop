# Refactor: Eliminate Code Duplication Across UI Panels and Utilities

## Metadata

type: `refactor`
task_id: `eliminate-duplication`
prompt: `Comprehensive review and refactoring to eliminate repeated code, optimize patterns, and reduce maintenance burden across the codebase.`

## Refactor Description

The codebase has accumulated significant duplication as new panels and features were added organically. The most impactful duplication is in the UI panel layer, where LogView, DiffView, and Detail all independently implement copy/selection, double-tap navigation ("gg"), and viewport sizing logic. Additional duplication exists in the tool summary extraction (logentry.go) and the DiffView state-setter methods.

This refactoring will extract shared abstractions without changing any external behavior or adding new features.

## Current State

### 1. Copy/Selection Mode (~400 lines duplicated)

**LogView** (`internal/ui/panels/logview.go:67-77, 861-1075`) and **DiffView** (`internal/ui/panels/diffview.go:32-42, 249-429`) independently implement:

- **State fields**: `copyMode`, `copyAnchor`, `copyCursor`, `mouseSelecting`, `mouseAnchorLine`, `mouseAnchorCol`, `mouseCurrentLine`, `mouseCurrentCol` (8 fields duplicated)
- **`enterCopyMode()`**: Identical logic — check lines, compute center, set anchor/cursor (logview:861-883, diffview:249-266)
- **`updateCopyMode(msg)`**: Identical key handlers for esc/y/j/k/G/g with viewport scrolling (logview:885-944, diffview:268-321)
- **`yankSelection()`**: Identical bounds-checked slice join (logview:946-966, diffview:323-336)
- **`copySelectionRange()`**: Identical 4-line swap (logview:968-975, diffview:338-345)
- **`StartMouseSelection()`**: Identical coordinate-to-buffer-line conversion (logview:978-999, diffview:348-364)
- **`ExtendMouseSelection()`**: Identical (logview:1002-1021, diffview:367-382)
- **`FinalizeMouseSelection()`**: Identical (logview:1025-1055, diffview:386-413)
- **`CancelMouseSelection()`**: Identical (logview:1057-1065, diffview:415-419)
- **`normalizedMouseSelection()`**: Identical (logview:1068-1075, diffview:422-429)

### 2. Double-Tap "gg" Navigation (~50 lines duplicated × 3)

Three panels implement the same `gPending` pattern with separate message types:

- **LogView** (`logview.go:19, 34, 48, 127-129, 189-203`): `gTimeout`, `GTimerExpiredMsg`, `gPending` field
- **DiffView** (`diffview.go:14, 16, 30, 138-140, 153-162`): `diffGTimeout`, `DiffGTimerExpiredMsg`, `gPending` field
- **Detail** (`detail.go:17, 26, 37-39, 69-80`): reuses `gTimeout` from logview, `DetailGTimerExpiredMsg`, `gPending` field

The timeout value (300ms) is defined as `gTimeout` in logview.go and `diffGTimeout` in diffview.go — identical values, different constants.

### 3. ToolUseSummary Repetitive Unmarshal (`logentry.go:89-167`)

The `ToolUseSummary()` function has 11 cases with the same pattern:
1. Define an anonymous struct with one JSON field
2. `json.Unmarshal` into it
3. Check if field is non-empty
4. Return formatted string

Three groups share identical struct definitions:
- `Read`, `Edit`, `Write` — all extract `file_path`
- `Glob`, `Grep` — both extract `pattern`
- Each case is 6-7 lines when 2 would suffice with a helper

### 4. DiffView State-Setter Boilerplate (`diffview.go:65-113`)

Five methods (`SetLoading`, `SetError`, `SetEmpty`, `SetNoBranch`, `SetWaiting`) follow the same pattern:
- Set one "active" field (loading/errMsg/emptyMsg)
- Clear all other fields to zero values
- Call `refreshContent()`

This is ~50 lines of near-identical code.

## Target State

### 1. `internal/ui/selection/selection.go` — Shared Selection Manager

A composable `Selection` struct that encapsulates all copy/yank and mouse selection state and logic:

```go
type Selection struct {
    // Line-level copy mode
    copyMode   bool
    copyAnchor int
    copyCursor int

    // Character-level mouse selection
    mouseSelecting   bool
    mouseAnchorLine  int
    mouseAnchorCol   int
    mouseCurrentLine int
    mouseCurrentCol  int
}
```

With methods: `EnterCopyMode()`, `UpdateCopyMode()`, `YankSelection()`, `CopySelectionRange()`, `StartMouse()`, `ExtendMouse()`, `FinalizeMouse()`, `CancelMouse()`, `NormalizedMouseSelection()`, `Active() bool`.

The selection manager uses a `LinesProvider` interface to abstract over LogView's `buffer.Lines()` vs DiffView's `rawLines()`:

```go
type LinesProvider interface {
    Lines() []string
}
```

LogView and DiffView embed `Selection` and delegate to it.

### 2. `internal/ui/panels/gtap.go` — Shared Double-Tap Handler

A single `GTimerExpiredMsg` type and a `DoubleTap` struct:

```go
type GTimerExpiredMsg struct{ ID int }

type DoubleTap struct {
    Pending bool
    id      int
}
```

With `Check(id int) (fired bool, cmd tea.Cmd)` — returns true if the double-tap fired, or a timer command if starting the window. Each panel gets a unique ID to distinguish timer messages.

### 3. `logentry.go` — Tool Summary Helper

A `toolField` helper function:

```go
func toolField(toolInput, fieldName string) string {
    var m map[string]interface{}
    if json.Unmarshal([]byte(toolInput), &m) != nil {
        return ""
    }
    if v, ok := m[fieldName].(string); ok {
        return v
    }
    return ""
}
```

This reduces each case in `ToolUseSummary` from 6-7 lines to 1-2 lines, and tools sharing the same field (Read/Edit/Write) can be grouped into a single case.

### 4. DiffView State Consolidation

Replace the five setter methods with a single internal `resetState` helper and thin setter wrappers:

```go
func (d *DiffView) resetState() {
    d.loading = false
    d.errMsg = ""
    d.emptyMsg = ""
    d.rawDiff = ""
    d.diffStat = ""
    d.fileOffsets = nil
}
```

Each setter becomes a 3-liner: `resetState()`, set one field, `refreshContent()`.

## Relevant Files

- `internal/ui/panels/logview.go` — Primary source of copy/selection, gg, and search logic
- `internal/ui/panels/diffview.go` — Duplicates copy/selection and gg from logview
- `internal/ui/panels/detail.go` — Duplicates gg from logview
- `internal/ui/panels/messages.go` — Custom message types (YankMsg, FullscreenMsg)
- `internal/process/logentry.go` — ToolUseSummary duplication
- `internal/ui/styles/theme.go` — SelectionStyle used by selection highlight functions

### New Files

- `internal/ui/selection/selection.go` — Shared selection/copy/mouse state and logic
- `internal/ui/panels/gtap.go` — Shared double-tap handler and message type

## Migration Strategy

Each refactoring area is independent and can be done incrementally. Within each area, the strategy is:

1. **Extract** — Create the new shared abstraction with tests
2. **Integrate** — Embed or use the abstraction in one consumer (e.g., DiffView first since it's simpler)
3. **Propagate** — Apply to remaining consumers (LogView, Detail)
4. **Delete** — Remove the old duplicated code

No behavior changes. All existing tests must continue to pass. The `applySelectionHighlight`, `applyCharSelectionHighlight`, and `extractCharSelection` free functions stay in logview.go (they're already shared between logview and diffview via direct function calls).

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Create the shared selection package

- Create `internal/ui/selection/selection.go`
- Define the `LinesProvider` interface with a single `Lines() []string` method
- Define the `Selection` struct with all 8 state fields (copyMode, copyAnchor, copyCursor, mouseSelecting, mouseAnchorLine, mouseAnchorCol, mouseCurrentLine, mouseCurrentCol)
- Implement `EnterCopyMode(lines LinesProvider, viewportYOffset, viewportHeight int)` — ported from logview.go:861-883
- Implement `UpdateCopyMode(msg tea.KeyMsg, lines LinesProvider, viewport *viewport.Model, gPending *bool, gTimerMsg tea.Msg) (yankText string, cmd tea.Cmd)` — ported from logview.go:885-944
- Implement `YankSelection(lines LinesProvider) string` — ported from logview.go:946-966
- Implement `CopySelectionRange() (start, end int)` — ported from logview.go:968-975
- Implement `StartMouse(relX, relY, viewportYOffset int)` — ported from logview.go:978-999
- Implement `ExtendMouse(relX, relY, viewportYOffset int)` — ported from logview.go:1002-1021
- Implement `FinalizeMouse(relX, relY, viewportYOffset int) (startLine, startCol, endLine, endCol int, singleClick bool)` — returns normalized coords and single-click flag; callers handle content extraction
- Implement `CancelMouse()`
- Implement `NormalizedMouseSelection() (startLine, startCol, endLine, endCol int)` — ported from logview.go:1068-1075
- Implement `Active() bool` — returns `copyMode`
- Implement `MouseActive() bool` — returns `mouseSelecting`
- Implement `Reset()` — clears all state

### 2. Create the shared double-tap handler

- Create `internal/ui/panels/gtap.go`
- Move `gTimeout` constant (300ms) here as the single source of truth
- Define `GTimerExpiredMsg struct{ ID int }` — replaces `GTimerExpiredMsg`, `DiffGTimerExpiredMsg`, and `DetailGTimerExpiredMsg`
- Define `DoubleTap struct { Pending bool; id int }` with `NewDoubleTap(id int) DoubleTap`
- Implement `func (dt *DoubleTap) Check() (fired bool, cmd tea.Cmd)`:
  - If `dt.Pending`, set `Pending = false`, return `fired = true, nil`
  - Otherwise, set `Pending = true`, return `fired = false, tea.Tick(gTimeout, ...)`
- Implement `func (dt *DoubleTap) HandleExpiry(msg GTimerExpiredMsg) bool`:
  - If `msg.ID == dt.id`, set `Pending = false`, return `true` (handled)
  - Otherwise return `false`
- Assign unique IDs: LogView=1, DiffView=2, Detail=3

### 3. Integrate selection manager into DiffView

- Add `sel selection.Selection` field to DiffView struct
- Create a `diffLinesProvider` adapter that wraps `rawLines()` to satisfy `LinesProvider`
- Replace `enterCopyMode()` body with `d.sel.EnterCopyMode(...)` call
- Replace `updateCopyMode()` body with `d.sel.UpdateCopyMode(...)` delegation
- Replace `yankSelection()` with `d.sel.YankSelection(...)`
- Replace `copySelectionRange()` with `d.sel.CopySelectionRange()`
- Replace `StartMouseSelection()` with `d.sel.StartMouse(...)`
- Replace `ExtendMouseSelection()` with `d.sel.ExtendMouse(...)`
- Replace `FinalizeMouseSelection()` — use `d.sel.FinalizeMouse(...)` + `extractCharSelection`
- Replace `CancelMouseSelection()` with `d.sel.CancelMouse()` + `refreshContent()`
- Replace `normalizedMouseSelection()` with `d.sel.NormalizedMouseSelection()`
- Update `refreshContent()` to use `d.sel.Active()` and `d.sel.MouseActive()`
- Update `Content()` to use `d.sel.CopySelectionRange()`
- Remove the 8 now-unused state fields from DiffView struct
- Remove the old method implementations
- Run `make check` to verify

### 4. Integrate selection manager into LogView

- Add `sel selection.Selection` field to LogView struct
- Create a `logLinesProvider` adapter that wraps `l.buffer.Lines()` to satisfy `LinesProvider`
- Replace all selection methods in LogView with delegation to `l.sel.*`
- Replace `l.copyMode` with `l.sel.Active()`, `l.mouseSelecting` with `l.sel.MouseActive()`
- Update `renderContent()` to use `l.sel.CopySelectionRange()` and `l.sel.NormalizedMouseSelection()`
- Update `SetRun()` to call `l.sel.Reset()`
- Update `ConsumesKeys()` to use `l.sel.Active()`
- Update `View()` to use `l.sel.Active()`, `l.sel.CopySelectionRange()`
- Remove the 8 now-unused state fields from LogView struct
- Remove the old method implementations
- Delegate LogView.StartMouseSelection -> check activeTab, then `l.sel.StartMouse(...)` (keep the tab delegation to diffView)
- Run `make check` to verify

### 5. Integrate double-tap handler into all three panels

- In `gtap.go`, remove `GTimerExpiredMsg` and `DiffGTimerExpiredMsg` and `DetailGTimerExpiredMsg` from their original files
- Add `gTap DoubleTap` field to LogView, DiffView, and Detail
- In LogView.Update: replace `gPending` check in "g" case with `l.gTap.Check()`, replace `GTimerExpiredMsg` case with `l.gTap.HandleExpiry(msg)`
- In DiffView.Update: replace `gPending` checks with `d.gTap.Check()`, replace `DiffGTimerExpiredMsg` case with `d.gTap.HandleExpiry(msg)`
- In Detail.Update: replace `gPending` checks with `d.gTap.Check()`, replace `DetailGTimerExpiredMsg` case with `d.gTap.HandleExpiry(msg)`
- Remove `gPending` field from all three structs
- Remove `gTimeout` from logview.go and `diffGTimeout` from diffview.go
- Run `make check` to verify

### 6. Simplify ToolUseSummary with helper function

- In `internal/process/logentry.go`, add a `toolField(toolInput, fieldName string) string` helper
- Group `Read`, `Edit`, `Write` into a single case using fallthrough or comma-separated case list, calling `toolField(toolInput, "file_path")` and `shortenPath()`
- Group `Glob`, `Grep` into a single case calling `toolField(toolInput, "pattern")`
- Simplify remaining cases (`Bash`, `WebSearch`, `WebFetch`, `Task`, `TodoWrite`/`TaskCreate`) to use `toolField()`
- Run `make check` to verify

### 7. Consolidate DiffView state setters

- Add a `resetState()` method to DiffView that clears `loading`, `errMsg`, `emptyMsg`, `rawDiff`, `diffStat`, `fileOffsets`
- Simplify `SetLoading()`: `d.resetState(); d.loading = true; d.refreshContent()`
- Simplify `SetError(err)`: `d.resetState(); d.errMsg = err; d.refreshContent()`
- Simplify `SetEmpty()`: `d.resetState(); d.emptyMsg = "No changes on branch"; d.refreshContent()`
- Simplify `SetNoBranch()`: `d.resetState(); d.emptyMsg = "No branch — diff unavailable"; d.refreshContent()`
- Simplify `SetWaiting()`: `d.resetState(); d.emptyMsg = "Waiting for worktree..."; d.refreshContent()`
- Run `make check` to verify

## Testing Strategy

**No new behavior is introduced.** All existing tests must pass unchanged as the primary correctness gate:

- `make test` — all unit tests
- `make lint` — go vet
- `make check` — parallel lint + test

Since the selection, copy, and double-tap behaviors are tested through the teatest golden file assertions in `*_teatest_test.go`, any behavioral regression will show up as golden file mismatches.

For the new `internal/ui/selection/selection.go` package, add unit tests for:
- `EnterCopyMode` sets anchor/cursor to center
- `UpdateCopyMode` handles j/k/G/gg/y/esc correctly
- `YankSelection` returns correct slice for normal and reversed ranges
- `CopySelectionRange` normalizes start > end
- Mouse methods compute correct buffer coordinates
- `FinalizeMouse` detects single-click (no drag)

## Risk Assessment

- **Low risk**: All changes are behavior-preserving internal refactoring within the `internal/` package (no public API changes)
- **Primary risk**: Subtle behavioral differences if the ported selection logic doesn't exactly match per-panel variations (e.g., LogView delegates mouse events to DiffView on the diff tab — this delegation must be preserved)
- **Mitigation**: The DiffView integration step happens first (simpler panel), validating the abstraction before applying to LogView
- **Golden file updates**: If rendering changes at all (it shouldn't), `make update-golden` can regenerate the golden files after manual visual verification

## Validation Commands

```bash
make check          # Parallel lint + test (primary gate)
make test           # Unit tests only
make lint           # go vet only
make update-golden  # Only if golden files break (should NOT be needed)
```

## Open Questions (Unresolved)

None — all refactoring targets are clearly defined with no ambiguity in approach.

## Sub-Tasks

The spec is structured as 7 sequential steps. Steps 1-2 are foundational (new packages), steps 3-5 integrate those packages, and steps 6-7 are independent cleanups. If decomposition is desired:

- **Sub-task A** (Steps 1-4): Selection extraction — highest impact, ~400 lines removed
- **Sub-task B** (Step 5): Double-tap extraction — medium impact, ~50 lines removed, simplifies message types
- **Sub-task C** (Steps 6-7): Utility cleanups — low impact, independent of A and B
