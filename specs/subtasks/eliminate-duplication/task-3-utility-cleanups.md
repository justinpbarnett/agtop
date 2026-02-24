# Sub-task: Utility cleanups — ToolUseSummary helper and DiffView state setters

> Part of: [Refactor: Eliminate Code Duplication](../../refactor-eliminate-duplication.md)
> Sub-task 3 of 3 for eliminate-duplication

## Scope

Simplify `ToolUseSummary()` with a `toolField()` helper to reduce repetitive unmarshal boilerplate (~60 lines), and consolidate DiffView's five state-setter methods with a `resetState()` helper (~30 lines).

## Steps

### 1. Simplify ToolUseSummary with helper function

- In `internal/process/logentry.go`, add a `toolField(toolInput, fieldName string) string` helper
- Group `Read`, `Edit`, `Write` into a single case using fallthrough or comma-separated case list, calling `toolField(toolInput, "file_path")` and `shortenPath()`
- Group `Glob`, `Grep` into a single case calling `toolField(toolInput, "pattern")`
- Simplify remaining cases (`Bash`, `WebSearch`, `WebFetch`, `Task`, `TodoWrite`/`TaskCreate`) to use `toolField()`
- Run `make check` to verify

### 2. Consolidate DiffView state setters

- Add a `resetState()` method to DiffView that clears `loading`, `errMsg`, `emptyMsg`, `rawDiff`, `diffStat`, `fileOffsets`
- Simplify `SetLoading()`: `d.resetState(); d.loading = true; d.refreshContent()`
- Simplify `SetError(err)`: `d.resetState(); d.errMsg = err; d.refreshContent()`
- Simplify `SetEmpty()`: `d.resetState(); d.emptyMsg = "No changes on branch"; d.refreshContent()`
- Simplify `SetNoBranch()`: `d.resetState(); d.emptyMsg = "No branch — diff unavailable"; d.refreshContent()`
- Simplify `SetWaiting()`: `d.resetState(); d.emptyMsg = "Waiting for worktree..."; d.refreshContent()`
- Run `make check` to verify

## Relevant Files

### Existing Files (modify)

| File | Role | Change |
|------|------|--------|
| `internal/process/logentry.go` | Log entry parsing with ToolUseSummary | Add `toolField()` helper, simplify switch cases |
| `internal/ui/panels/diffview.go` | DiffView panel | Add `resetState()` method, simplify 5 setter methods |

### New Files

None.

## Validation

```bash
make check          # Parallel lint + test (primary gate)
```

- All existing tests must pass unchanged
- `ToolUseSummary()` output must be identical for all tool types
- DiffView state transitions must behave identically
