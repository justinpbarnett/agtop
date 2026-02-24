# Sub-task: Extract shared double-tap "gg" handler

> Part of: [Refactor: Eliminate Code Duplication](../../refactor-eliminate-duplication.md)
> Sub-task 2 of 3 for eliminate-duplication

## Scope

Extract the duplicated "gg" double-tap navigation pattern from LogView, DiffView, and Detail into a shared `DoubleTap` handler in `internal/ui/panels/gtap.go` — eliminating ~50 lines of duplication and 3 separate timer message types.

## Steps

### 1. Create the shared double-tap handler

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

### 2. Integrate into all three panels

- Remove `GTimerExpiredMsg` and `DiffGTimerExpiredMsg` and `DetailGTimerExpiredMsg` from their original files
- Add `gTap DoubleTap` field to LogView, DiffView, and Detail
- In LogView.Update: replace `gPending` check in "g" case with `l.gTap.Check()`, replace `GTimerExpiredMsg` case with `l.gTap.HandleExpiry(msg)`
- In DiffView.Update: replace `gPending` checks with `d.gTap.Check()`, replace `DiffGTimerExpiredMsg` case with `d.gTap.HandleExpiry(msg)`
- In Detail.Update: replace `gPending` checks with `d.gTap.Check()`, replace `DetailGTimerExpiredMsg` case with `d.gTap.HandleExpiry(msg)`
- Remove `gPending` field from all three structs
- Remove `gTimeout` from logview.go and `diffGTimeout` from diffview.go

## Relevant Files

### Existing Files (modify)

| File | Role | Change |
|------|------|--------|
| `internal/ui/panels/logview.go` | LogView panel | Remove `gPending`, `gTimeout`, `GTimerExpiredMsg`; add `gTap DoubleTap` field and delegate |
| `internal/ui/panels/diffview.go` | DiffView panel | Remove `gPending`, `diffGTimeout`, `DiffGTimerExpiredMsg`; add `gTap DoubleTap` field and delegate |
| `internal/ui/panels/detail.go` | Detail panel | Remove `gPending`, `DetailGTimerExpiredMsg`; add `gTap DoubleTap` field and delegate |

### New Files

| File | Role |
|------|------|
| `internal/ui/panels/gtap.go` | Shared double-tap handler and unified timer message type |

## Validation

```bash
make check          # Parallel lint + test (primary gate)
```

- All existing tests must pass unchanged
- The "gg" navigation behavior must work identically in all three panels
- Timer expiry must only affect the panel that initiated it (verified by unique IDs)
