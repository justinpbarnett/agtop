# Sub-task: Extract shared selection/copy/mouse package

> Part of: [Refactor: Eliminate Code Duplication](../../refactor-eliminate-duplication.md)
> Sub-task 1 of 3 for eliminate-duplication

## Scope

Extract all duplicated copy/selection and mouse selection logic from LogView and DiffView into a shared `internal/ui/selection` package, then integrate it into both panels — eliminating ~400 lines of duplication.

## Steps

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

### 2. Integrate selection manager into DiffView

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

### 3. Integrate selection manager into LogView

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

## Relevant Files

### Existing Files (modify)

| File | Role | Change |
|------|------|--------|
| `internal/ui/panels/logview.go` | Primary panel with copy/selection logic | Remove duplicated selection state and methods, embed `selection.Selection`, delegate to it |
| `internal/ui/panels/diffview.go` | Diff panel with copy/selection logic | Remove duplicated selection state and methods, embed `selection.Selection`, delegate to it |

### New Files

| File | Role |
|------|------|
| `internal/ui/selection/selection.go` | Shared selection/copy/mouse state and logic |

## Validation

```bash
make check          # Parallel lint + test (primary gate)
make test           # Unit tests
make lint           # go vet
```

- All existing tests must pass unchanged
- Golden file tests in `*_teatest_test.go` must pass without regeneration
- No behavioral changes to copy mode, yank, or mouse selection
