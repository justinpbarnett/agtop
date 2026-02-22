# Feature: Run List Panel

## Metadata

type: `feat`
task_id: `run-list-panel`
prompt: `Implement the run list panel backed by a run store with real-time updates, scrollable viewport, run filtering, and dynamic status bar aggregation`

## Feature Description

The run list panel is the primary navigation surface of agtop — the left panel where developers see every active and completed run at a glance. Step 3 (TUI Shell and Layout) established the panel with hardcoded mock data. This step replaces mock data with a proper `run.Store` that the rest of the system (process manager, workflow executor) will push runs into, adds real-time update propagation via Bubble Tea messages, makes the list scrollable when runs exceed panel height, and wires the status bar to aggregate live totals from the store.

After this step, the run list is a fully functional, data-driven component ready to receive runs from the process manager and workflow executor in later steps.

## User Story

As a developer using agtop
I want to see a live, scrollable list of all my agent runs with real-time status, token, and cost updates
So that I can monitor concurrent workflows at a glance and navigate to any run for details

## Problem Statement

The current run list (`runlist.go`) renders 4 hardcoded mock `run.Run` values constructed in `NewRunList()`. There is no backing store, no way to add or remove runs, no mechanism for state/token/cost updates to flow into the UI, no scrolling when runs exceed panel height, and no filtering. The status bar (`statusbar.go`) likewise has hardcoded totals. The `run.Store` is an empty struct with no methods.

These gaps must be resolved before the process manager (step 5) and skill engine (step 6) can push real run data into the TUI.

## Solution Statement

1. **Implement `run.Store`** — an in-memory, concurrency-safe run store with add/update/get/list operations and a subscriber model that emits Bubble Tea messages on every mutation.
2. **Refactor `RunList`** — replace the hardcoded `[]run.Run` slice with a reference to the store, subscribe to store change events, support scrolling via a viewport offset when runs exceed panel height, and implement `/` filtering with an inline text input.
3. **Add `Run` fields** — extend the `Run` struct with `CreatedAt`, `StartedAt`, `CurrentSkill`, and `Error` fields needed for display and sorting.
4. **Wire status bar** — the status bar computes its totals from the store on every change event rather than holding its own hardcoded values.
5. **Bubble Tea message flow** — define `RunStoreUpdatedMsg` as the event that triggers re-renders when any run changes.

## Relevant Files

Use these files to implement the feature:

- `internal/run/run.go` — `Run` struct and `State` constants. Will be extended with `CreatedAt`, `StartedAt`, `CurrentSkill`, and `Error` fields.
- `internal/run/store.go` — Currently an empty `Store` struct. Will become the core in-memory run store with CRUD operations, ordering, and a subscriber model.
- `internal/tui/runlist.go` — Currently renders hardcoded mock data. Will be refactored to read from the store, support scrollable viewport offset, and inline filter input.
- `internal/tui/statusbar.go` — Currently has hardcoded totals. Will compute totals from the store on update.
- `internal/tui/app.go` — Root model. Will hold the store, subscribe to updates, and propagate `RunStoreUpdatedMsg` to children.
- `internal/tui/theme.go` — May need a `FilterInputStyle` for the inline filter text input. Existing styles are sufficient otherwise.
- `internal/tui/runlist_test.go` — Existing tests. Will be updated to work with the store-backed run list.

### New Files

- `internal/run/store_test.go` — Tests for the run store: add, update, get, list, ordering, subscriber notifications, concurrency safety.
- `internal/tui/messages.go` — Shared Bubble Tea message types (`RunStoreUpdatedMsg`, etc.) used across TUI components.

## Implementation Plan

### Phase 1: Foundation

Extend the `Run` struct with fields needed for proper display and ordering. Implement `run.Store` as a concurrency-safe in-memory store with a subscriber model that sends Bubble Tea messages on mutations.

### Phase 2: Core Implementation

Refactor `RunList` to be store-backed with scrollable viewport offset and inline `/` filter. Refactor `StatusBar` to compute totals from the store. Define the `RunStoreUpdatedMsg` and wire it through `App`.

### Phase 3: Integration

Wire everything together in `App` — create the store, seed with mock data (until real runs exist), subscribe to changes, propagate messages. Update tests to use the store. Verify the full render pipeline with dynamic data.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Extend the Run Struct

Update `internal/run/run.go`:

- Add `CreatedAt time.Time` — when the run was created (for ordering)
- Add `StartedAt time.Time` — when execution began (for elapsed time display)
- Add `CurrentSkill string` — name of the currently executing skill (e.g., "build", "test")
- Add `Error string` — error message when state is `failed`
- Add import for `time` package
- Add method `func (r *Run) ElapsedTime() time.Duration` — returns duration since `StartedAt` (or zero if not started)

### 2. Implement the Run Store

Rewrite `internal/run/store.go`:

- Define the `Store` struct:
  - `mu sync.RWMutex` — protects all fields
  - `runs map[string]*Run` — runs keyed by ID
  - `order []string` — insertion-ordered run IDs (newest first for display)
  - `nextID int` — auto-incrementing ID counter
  - `subscribers []func()` — callbacks invoked on every mutation
- `NewStore() *Store` — constructor
- `Add(r *Run) string` — assigns next ID if `r.ID` is empty (format `"%03d"`), stores the run, prepends to `order`, notifies subscribers, returns the ID
- `Update(id string, fn func(*Run))` — applies `fn` to the run under write lock, notifies subscribers. The callback pattern avoids exposing the pointer outside the lock.
- `Get(id string) (Run, bool)` — returns a copy (not a pointer) under read lock
- `List() []Run` — returns copies of all runs in `order` sequence under read lock
- `Remove(id string)` — removes from `runs` and `order`, notifies subscribers
- `Count() int` — returns total run count under read lock
- `Subscribe(fn func())` — appends callback. Callbacks are invoked (outside the lock) after any mutation.
- Internal `notify()` method — iterates subscribers and calls each. Called after every Add/Update/Remove.
- `ActiveRuns() int` — count runs where `State` is `running`, `routing`, `queued`, or `paused`
- `TotalTokens() int` — sum of `Tokens` across all runs
- `TotalCost() float64` — sum of `Cost` across all runs

### 3. Define Shared TUI Messages

Create `internal/tui/messages.go`:

- `RunStoreUpdatedMsg struct{}` — sent when any run in the store changes. Components that care about run data re-read the store on this message.

### 4. Refactor RunList to Use Store

Rewrite `internal/tui/runlist.go`:

- Replace `runs []run.Run` field with:
  - `store *run.Store` — reference to the shared store
  - `filtered []run.Run` — the current list of runs after filtering (re-derived on every store update or filter change)
  - `offset int` — viewport scroll offset (first visible row index)
  - `filterActive bool` — whether the filter input is showing
  - `filterText string` — current filter query
  - `filterInput textinput.Model` — bubbles text input for inline filter (imported from `github.com/charmbracelet/bubbles/textinput`)
- `NewRunList(store *run.Store) RunList` — takes the store, initializes the text input (placeholder "Filter...", width matching panel), derives initial `filtered` list
- Internal `applyFilter()` method — if `filterText` is empty, `filtered = store.List()`. Otherwise, case-insensitive substring match against ID, Branch, Workflow, State, and CurrentSkill. Resets `selected` and `offset` to 0 if the filtered set changes.
- `Update(msg tea.Msg)`:
  - On `RunStoreUpdatedMsg`: call `applyFilter()` to refresh the list. Clamp `selected` to bounds.
  - On `tea.KeyMsg`:
    - If `filterActive`: delegate to `filterInput.Update()`. On `Enter` or `Esc`, deactivate filter. On every keystroke, update `filterText` from input value and call `applyFilter()`.
    - If not filtering:
      - `j`/`k`: move selection (existing behavior), adjust `offset` to keep selection visible
      - `G`/`gg`: jump to bottom/top (existing behavior), adjust `offset`
      - `/`: activate filter — set `filterActive = true`, focus the text input
- `View()`:
  - If `filtered` is empty and filter is active: render "No matching runs."
  - If `filtered` is empty and filter is not active: render "No runs. Press n to start one."
  - Calculate visible rows: `visibleRows = height - 1` (reserve 1 row for filter bar when active, or for a header)
  - Render only `filtered[offset:offset+visibleRows]` — the visible window
  - Each row formatted as before: `{icon} #{id}  {branch:<16} {workflow:<10} {state:<8} {skill:<5} {tokens:>8}  ${cost:.2f}`
  - Show scroll indicators when content overflows: `▲` at top if `offset > 0`, `▼` at bottom if more rows below
  - If `filterActive`, render the text input at the top of the panel with a `/` prefix
- `SetSize(w, h int)` — update dimensions, recalculate visible rows, clamp offset
- `SelectedRun() *run.Run` — return from `filtered` slice (not directly from store)
- Internal `clampSelection()` — ensures `selected` is within `filtered` bounds and `offset` keeps `selected` visible
- Internal `scrollToSelection()` — adjusts `offset` so that `selected` is within the visible window

### 5. Refactor StatusBar to Use Store

Update `internal/tui/statusbar.go`:

- Replace hardcoded fields (`totalRuns`, `activeRuns`, `totalTokens`, `totalCost`) with a `store *run.Store` reference
- `NewStatusBar(store *run.Store) StatusBar` — takes the store
- `Update(msg tea.Msg)`:
  - On `RunStoreUpdatedMsg`: no-op (View reads from store directly)
- `View()`:
  - Read `store.Count()`, `store.ActiveRuns()`, `store.TotalTokens()`, `store.TotalCost()` directly
  - Render the same format: `Runs: {n} ({active} active) │ Tokens: {tok} │ Cost: ${cost}`
  - Right side shows contextual keybind hints (same as before for now)

### 6. Wire Store into App

Update `internal/tui/app.go`:

- Add `store *run.Store` field to `App`
- `NewApp(cfg *config.Config)`:
  - Create `run.NewStore()`
  - Seed with mock data: add the same 4 runs that currently exist in `NewRunList()`, setting `CreatedAt` to `time.Now()` with staggered offsets so ordering is deterministic
  - Pass store to `NewRunList(store)` and `NewStatusBar(store)`
  - Subscribe to store changes: `store.Subscribe(func() { p.Send(RunStoreUpdatedMsg{}) })` — this requires storing the `tea.Program` reference (see below)
- The store subscription needs the `tea.Program` to send messages. Since `NewApp` is called before `tea.NewProgram`, use a deferred subscription approach:
  - Add a `SetProgram(p *tea.Program)` method on `App` (or use an `Init()` command that returns a subscription)
  - Alternative: use a channel-based approach — store writes to a channel, `Init()` returns a `tea.Cmd` that listens on the channel and emits `RunStoreUpdatedMsg`. This is the idiomatic Bubble Tea pattern.
  - Recommended approach: in `Init()`, return a `tea.Cmd` that starts a goroutine listening on a store notification channel. The store exposes a `Changes() <-chan struct{}` channel method alongside (or instead of) callback subscribers.
- Update `Init()` to return the subscription command
- Route `RunStoreUpdatedMsg` in `Update()` — propagate to `runList.Update()` and `statusBar.Update()`
- Update `cmd/agtop/main.go` if needed (should not need changes since `NewApp` signature stays the same)

### 7. Add Store Notification Channel

Add to `internal/run/store.go`:

- Add `changeCh chan struct{}` field — a buffered channel (capacity 1) for non-blocking notification
- Initialize in `NewStore()` with `make(chan struct{}, 1)`
- In `notify()`, also do a non-blocking send: `select { case s.changeCh <- struct{}{}: default: }` — this coalesces rapid updates into a single notification
- `Changes() <-chan struct{}` — returns the channel for external consumers
- In `App.Init()`, return a `tea.Cmd` that does:
  ```
  func listenForChanges(ch <-chan struct{}) tea.Cmd {
      return func() tea.Msg {
          <-ch
          return RunStoreUpdatedMsg{}
      }
  }
  ```
  Then in `Update()` on `RunStoreUpdatedMsg`, return the same command to re-subscribe (continuous listening pattern)

### 8. Update Tests

**Update `internal/tui/runlist_test.go`:**

- Create a helper `testStore()` that returns a `*run.Store` pre-seeded with the 4 mock runs
- Update all test functions to use `NewRunList(testStore())` instead of `NewRunList()`
- Add new tests:
  - `TestRunListStoreUpdate` — add a run to the store, send `RunStoreUpdatedMsg`, verify the new run appears in the view
  - `TestRunListFilter` — activate filter with `/`, type "feat", verify only runs with "feat" in branch/workflow appear
  - `TestRunListFilterClear` — activate filter, type text, press `Esc`, verify all runs visible again
  - `TestRunListScrolling` — create a store with 20+ runs, set panel height to show only 5, verify offset adjusts on navigation
  - `TestRunListScrollIndicators` — verify `▲` and `▼` appear when content overflows

**Create `internal/run/store_test.go`:**

- `TestStoreAdd` — add a run, verify it's retrievable and ID was assigned
- `TestStoreUpdate` — add a run, update its state, verify the change persists
- `TestStoreGet` — verify get returns a copy (modifying the copy doesn't affect the store)
- `TestStoreList` — add multiple runs, verify list returns them in insertion order
- `TestStoreRemove` — add then remove, verify it's gone
- `TestStoreAggregates` — add runs with known tokens/costs, verify `TotalTokens()`, `TotalCost()`, `ActiveRuns()`
- `TestStoreSubscriber` — subscribe, add a run, verify the subscriber was called
- `TestStoreChangesChannel` — verify the `Changes()` channel receives a notification after add/update
- `TestStoreConcurrency` — launch multiple goroutines doing concurrent add/update, verify no panics or data corruption

**Update `internal/tui/app_test.go`:**

- Update `NewApp` calls to work with the new store-backed constructor (should be transparent since `NewApp` signature takes only `*config.Config`)
- Add `TestAppStoreUpdate` — modify a run in the store, send `RunStoreUpdatedMsg`, verify the run list and status bar reflect the change

## Testing Strategy

### Unit Tests

- **Store tests** (`store_test.go`): verify all CRUD operations, ordering, aggregate calculations, subscriber notification, channel notification, and concurrency safety via `go test -race`
- **RunList tests** (`runlist_test.go`): verify navigation, filtering, scrolling/offset behavior, and store-driven updates by constructing a store, mutating it, and sending `RunStoreUpdatedMsg` through `Update()`
- **App tests** (`app_test.go`): verify the message propagation pipeline — store change → `RunStoreUpdatedMsg` → run list and status bar re-render
- **StatusBar tests**: verify that `View()` output reflects store aggregates

### Edge Cases

- **Empty store** — run list shows "No runs. Press n to start one." and `SelectedRun()` returns nil
- **Single run** — navigation keys are no-ops (selection clamped)
- **Many runs (50+)** — scrolling viewport works correctly, offset clamps to bounds, scroll indicators show/hide
- **Filter matches nothing** — "No matching runs." displayed, selection is nil
- **Filter with special characters** — treated as literal substring, no regex parsing
- **Rapid store updates** — channel coalescing prevents message flooding; UI stays responsive
- **Concurrent store access** — mutex protects all reads/writes; `go test -race` passes
- **Run removed while selected** — selection clamps to new bounds
- **Panel resize while scrolled** — offset adjusts to keep selection visible
- **Very long branch names** — truncated to fit column width without breaking layout alignment
- **Zero tokens or cost** — displayed as "0" and "$0.00" respectively

## Acceptance Criteria

- [ ] `run.Store` is implemented with Add, Update, Get, List, Remove, Count, ActiveRuns, TotalTokens, TotalCost
- [ ] Store is concurrency-safe (`go test -race ./internal/run/...` passes)
- [ ] Store notifies subscribers and sends on `Changes()` channel on every mutation
- [ ] `RunList` reads runs from the store, not from a hardcoded slice
- [ ] `RunList` re-renders on `RunStoreUpdatedMsg`
- [ ] `RunList` scrolls when runs exceed panel height, with `▲`/`▼` indicators
- [ ] `RunList` supports `/` to open inline filter, `Esc` to close, live filtering by substring
- [ ] `StatusBar` computes totals from the store dynamically
- [ ] `Run` struct includes `CreatedAt`, `StartedAt`, `CurrentSkill`, `Error` fields
- [ ] `App.Init()` returns a subscription command that listens for store changes
- [ ] Mock data is seeded via the store in `NewApp()`, not hardcoded in `NewRunList()`
- [ ] Run rows display: status icon, ID, branch, workflow, state, skill progress, tokens, cost
- [ ] Selected row is highlighted; terminal-state rows are dimmed
- [ ] All existing TUI navigation still works (`j`/`k`, `G`/`gg`, `Tab`, `h`/`l`, `?`, `q`)
- [ ] All new and updated tests pass
- [ ] `go vet ./...` and `go build ./...` pass cleanly

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# All packages compile
go build ./...

# No vet issues
go vet ./...

# Run store tests with race detector
go test -race ./internal/run/... -v

# TUI tests pass
go test ./internal/tui/... -v

# All tests pass with race detector
go test -race ./...

# Binary builds
make build

# Manual smoke test — launch and verify run list renders from store
./bin/agtop
```

## Notes

- The store's `Update(id, func(*Run))` callback pattern is deliberate — it keeps mutation logic co-located with the caller while ensuring the mutex is held for the duration of the update. Callers never get a raw pointer to a run outside a lock scope.
- The `Changes()` channel with capacity 1 and non-blocking send is a standard coalescing pattern for Bubble Tea subscriptions. Multiple rapid mutations collapse into a single `RunStoreUpdatedMsg`, preventing UI thrashing.
- The `filtered` slice in `RunList` is re-derived from the store on every update. For the expected scale (< 100 runs per session), this is fast enough without caching or incremental diffing.
- The filter is a simple substring match, not regex. This matches the lazygit filter UX — type a few characters to narrow down, `Esc` to clear.
- Mock data remains as the seed in `NewApp()` until the process manager (step 5) creates real runs. This ensures the TUI is always visually testable during development.
- The `textinput` component from `bubbles` is already available as a transitive dependency. The import for `github.com/charmbracelet/bubbles/textinput` should resolve without additional `go get`.
