# Feature: TUI Shell and Layout

## Metadata

type: `feat`
task_id: `tui-shell-layout`
prompt: `Implement the multi-panel TUI shell with layout engine, vim-motion navigation, themed panels, and modal system`

## Feature Description

The TUI shell is the visual and interactive core of agtop. It replaces the current placeholder "press q to quit" screen with a lazygit/btop-inspired multi-panel interface: a run list on the left, a tabbed detail panel on the right, and a global status bar at the bottom. All panels are navigable via vim motions, with a modal system for actions that need user input.

This step transforms agtop from a binary that loads config and quits into a live, resizable, keyboard-driven TUI shell ready to host the run management and process orchestration features that follow.

## User Story

As a developer using agtop
I want to see a multi-panel TUI with vim-style keyboard navigation
So that I can navigate between runs, view details/logs/diffs, and access status information without leaving the terminal

## Problem Statement

The current TUI (`app.go`) is a stub that renders a single line of text and quits on `q`. The component types (`RunList`, `Detail`, `LogViewer`, etc.) are empty structs with no behavior. There is no layout engine, no panel focus management, no keyboard routing, no terminal resize handling, and no modal system. The theme file defines color constants but no reusable styles.

## Solution Statement

Implement a full Elm-architecture Bubble Tea layout with:

1. **Window size tracking** — handle `tea.WindowSizeMsg` to compute panel dimensions dynamically
2. **Three-region layout** — left panel (~30%), right panel (~70%), bottom status bar (1 row), composed via lipgloss `JoinHorizontal` and `JoinVertical`
3. **Panel focus system** — track which panel is active, highlight its border, route keystrokes to it
4. **Component models** — each panel component implements `Init`/`Update`/`View` with its own state
5. **Vim-motion navigation** — `j`/`k` in run list, `h`/`l` for detail tabs, `Tab` to cycle focus, `G`/`gg` for jump, `?` for help
6. **Styled theme** — border styles, panel chrome, and status colors using lipgloss
7. **Modal overlay** — centered modal that captures all input when active, for help and future action dialogs

All components render with placeholder/mock content so the layout is visually complete and testable before real data flows in from later steps.

## Relevant Files

Use these files to implement the feature:

- `internal/tui/app.go` — Root model. Currently a minimal stub. Will become the layout engine, message router, and panel focus manager.
- `internal/tui/theme.go` — Color constants exist. Will be extended with reusable lipgloss `Style` definitions for borders, panels, tabs, and status bar.
- `internal/tui/runlist.go` — Empty `RunList` struct. Will become a navigable list panel with mock run items.
- `internal/tui/detail.go` — Empty `Detail` struct. Will become a tabbed container (Details/Logs/Diff) with tab cycling.
- `internal/tui/logs.go` — Empty `LogViewer` struct. Will become a scrollable viewport with placeholder log content.
- `internal/tui/diff.go` — Empty `DiffViewer` struct. Will become a scrollable viewport with placeholder diff content.
- `internal/tui/statusbar.go` — Empty `StatusBar` struct. Will render global stats and keybind hints.
- `internal/tui/modal.go` — Empty `Modal` struct. Will become a centered overlay for help and action dialogs.
- `internal/tui/input.go` — Empty `Input` struct. Not needed in this step (used by new-run modal in a later step), but will be left as a stub.
- `internal/run/run.go` — `Run` struct and `State` constants. Used to type the mock data in the run list.
- `internal/config/config.go` — `UIConfig` struct with theme/display settings. Read by theme and status bar.
- `cmd/agtop/main.go` — Entry point. No changes needed (already passes config to `NewApp`).

### New Files

- `internal/tui/keys.go` — Centralized key binding definitions using `bubbles/key`. Defines all vim-motion bindings in one place for consistency and the help view.
- `internal/tui/app_test.go` — Tests for root model: initialization, window resize, panel focus cycling, key routing, quit behavior.
- `internal/tui/runlist_test.go` — Tests for run list: navigation (j/k/G/gg), selection bounds, rendering.
- `internal/tui/detail_test.go` — Tests for detail panel: tab cycling (h/l), content switching.

## Implementation Plan

### Phase 1: Foundation

Add the `bubbles` dependency (was specified in step 1 of the design doc but not yet added). Extend the theme with reusable lipgloss styles. Implement window size tracking and the three-region layout in `app.go` with empty bordered panels.

### Phase 2: Core Implementation

Build out each component model — RunList with mock data and j/k navigation, Detail with tab cycling, LogViewer and DiffViewer as scrollable viewports, StatusBar with static content. Wire message routing from App to child components based on panel focus.

### Phase 3: Integration

Add modal system for the help overlay (`?`). Implement panel focus cycling (`Tab`). Add quit-with-confirmation when active runs would exist (stub the check for now). Write tests for key behaviors.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add bubbles Dependency

- Run `go get github.com/charmbracelet/bubbles` to add the bubbles component library
- Run `go mod tidy`
- This provides `viewport`, `key`, `help`, and `textinput` components used throughout the TUI

### 2. Define Key Bindings

Create `internal/tui/keys.go`:

- Define a `KeyMap` struct with `key.Binding` fields for every vim-motion:
  - `Up` / `Down` — `k` / `j` (run list navigation)
  - `Top` / `Bottom` — `gg` / `G` (jump to top/bottom)
  - `TabNext` / `TabPrev` — `l` / `h` (detail tab cycling)
  - `FocusNext` — `Tab` (panel focus cycling)
  - `Filter` — `/` (filter runs, placeholder for now)
  - `Help` — `?` (toggle help modal)
  - `Quit` — `q` (quit application)
- Each binding includes `key.WithKeys(...)` and `key.WithHelp(...)` for the help view
- Define a `DefaultKeyMap()` constructor
- Implement `ShortHelp()` and `FullHelp()` on KeyMap to satisfy `help.KeyMap` interface

### 3. Extend Theme Styles

Update `internal/tui/theme.go`:

- Keep existing color constants
- Add reusable `lipgloss.Style` variables:
  - `PanelBorder` — `lipgloss.RoundedBorder()` in `BorderColor`, applied to inactive panels
  - `ActivePanelBorder` — same rounded border in `ActiveColor` for the focused panel
  - `TabStyle` — base style for inactive tab labels
  - `ActiveTabStyle` — bold/underlined style for the active tab label
  - `StatusBarStyle` — full-width style with background color for the bottom bar
  - `RunStateStyle(state State) lipgloss.Style` — returns a style with the appropriate foreground color for a run state (running=blue, paused=yellow, etc.)
  - `ModalStyle` — centered overlay with border and padding
- All styles are functions that take width/height parameters where needed, not fixed-size globals

### 4. Implement Root Layout Engine

Rewrite `internal/tui/app.go`:

- Add fields to `App` struct:
  - `width`, `height int` — terminal dimensions from `tea.WindowSizeMsg`
  - `focusedPanel int` — index of focused panel (0=run list, 1=detail)
  - `runList RunList` — run list component
  - `detail Detail` — detail panel component
  - `statusBar StatusBar` — status bar component
  - `modal *Modal` — optional modal overlay (nil when hidden)
  - `keys KeyMap` — key bindings
  - `ready bool` — true after first `WindowSizeMsg`
- `Init()` — return `nil` (wait for `WindowSizeMsg` before rendering)
- `Update()`:
  - On `tea.WindowSizeMsg`: store dimensions, propagate to children with computed widths/heights, set `ready = true`
  - On `tea.KeyMsg`: if modal is active, route to modal. Otherwise route based on `focusedPanel`. Handle global keys (`Tab`, `?`, `q`) at the app level before panel-specific routing.
  - Panel width calculation: `leftWidth = width * 30 / 100`, `rightWidth = width - leftWidth - 3` (accounting for borders), `contentHeight = height - 3` (status bar + borders)
- `View()`:
  - If not `ready`, return a centered "loading..." message
  - Render left panel: `runList.View()` wrapped in a border style (active or inactive based on focus)
  - Render right panel: `detail.View()` wrapped in a border style
  - Render bottom: `statusBar.View()`
  - Compose: `lipgloss.JoinHorizontal(lipgloss.Top, left, right)` then `lipgloss.JoinVertical(lipgloss.Left, panels, statusBar)`
  - If modal is non-nil, overlay it centered on the composed layout

### 5. Implement Run List Component

Rewrite `internal/tui/runlist.go`:

- Add fields to `RunList`:
  - `runs []run.Run` — list of runs (initialized with mock data)
  - `selected int` — currently selected index
  - `width`, `height int` — panel dimensions
- `NewRunList()` — returns a RunList with 4 mock runs in different states:
  - `#001` — `running`, workflow `sdlc`, skill 3/7, 12400 tokens, $0.42
  - `#002` — `paused`, workflow `quick-fix`, skill 1/3, 3100 tokens, $0.08
  - `#003` — `completed`, workflow `plan-build`, reviewing, 45200 tokens, $1.23
  - `#004` — `failed`, workflow `build`, skill 2/3, 8700 tokens, $0.31
- `Update(msg tea.Msg)` — handle `j`/`k` to move selection (clamped to bounds), `G` to jump to last, `gg` to jump to first
- `View()` — render each run as a row:
  - Status icon: `●` running (blue), `◐` paused (yellow), `✓` completed (green), `✗` failed (red)
  - Format: `{icon} #{id}  {branch:<16} {workflow:<10} {state:<12} {tokens:>8} tok  ${cost:.2f}`
  - Highlighted row (selected): reverse video or bold with accent color
  - Terminal-state rows (completed, failed, accepted, rejected): dimmed
- `SetSize(w, h int)` — update dimensions for rendering
- `SelectedRun() *run.Run` — return the currently selected run (used by detail panel)

### 6. Implement Detail Panel with Tabs

Rewrite `internal/tui/detail.go`:

- Add fields to `Detail`:
  - `activeTab int` — 0=Details, 1=Logs, 2=Diff
  - `tabNames []string` — `{"Details", "Logs", "Diff"}`
  - `logViewer LogViewer` — log viewer sub-component
  - `diffViewer DiffViewer` — diff viewer sub-component
  - `width`, `height int` — panel dimensions
  - `selectedRun *run.Run` — the run to display details for
- `NewDetail()` — initialize with tab 0, create sub-components
- `Update(msg tea.Msg)` — handle `h`/`l` to cycle tabs (wrap around), delegate to active sub-component for scrolling keys
- `View()`:
  - Render tab bar at top: tab names with `ActiveTabStyle` for the active tab, `TabStyle` for others, separated by ` │ `
  - Render content area below tabs based on `activeTab`:
    - Tab 0 (Details): render run metadata — ID, branch, workflow, state, skill progress, tokens, cost, worktree path, elapsed time (all from `selectedRun`)
    - Tab 1 (Logs): render `logViewer.View()`
    - Tab 2 (Diff): render `diffViewer.View()`
  - If `selectedRun` is nil, render "No run selected" centered
- `SetRun(r *run.Run)` — update the displayed run
- `SetSize(w, h int)` — propagate to sub-components (subtract 2 for tab bar)

### 7. Implement Log Viewer Component

Rewrite `internal/tui/logs.go`:

- Add fields to `LogViewer`:
  - `viewport viewport.Model` — bubbles viewport for scrollable content
  - `content string` — log content (mock data for now)
  - `width`, `height int`
- `NewLogViewer()` — create viewport, set mock log content:
  - Generate ~30 lines of realistic-looking log entries with timestamps and skill prefixes
  - Example: `[14:32:05 build] Reading src/auth.ts...`
  - Example: `[14:32:07 build] Editing src/auth.ts — adding JWT validation`
  - Example: `[14:32:12 test] Running npm test...`
- `Update(msg tea.Msg)` — delegate to viewport (handles `j`/`k` scrolling, `G`/`gg`)
- `View()` — return `viewport.View()`
- `SetSize(w, h int)` — resize viewport

### 8. Implement Diff Viewer Component

Rewrite `internal/tui/diff.go`:

- Add fields to `DiffViewer`:
  - `viewport viewport.Model` — scrollable viewport
  - `content string` — diff content (mock data)
  - `width`, `height int`
- `NewDiffViewer()` — create viewport with mock unified diff content:
  - A realistic-looking `diff --git` output with `+`/`-` lines
  - Include file header, hunk headers, added (green-tinted) and removed (red-tinted) lines
- `Update(msg tea.Msg)` — delegate to viewport
- `View()` — return `viewport.View()`
- `SetSize(w, h int)` — resize viewport

### 9. Implement Status Bar

Rewrite `internal/tui/statusbar.go`:

- Add fields to `StatusBar`:
  - `width int`
  - `totalRuns`, `activeRuns int`
  - `totalTokens int`
  - `totalCost float64`
  - `keybindHints string`
- `NewStatusBar()` — initialize with mock data: 4 runs, 2 active, 69.4k tokens, $2.04
- `Update(msg tea.Msg)` — no-op for now (will react to run events later)
- `View()`:
  - Left side: `Runs: 4 (2 active) │ Tokens: 69.4k │ Cost: $2.04`
  - Right side: `j/k:navigate  h/l:tabs  Tab:focus  ?:help  q:quit`
  - Render as a single-line bar with left/right justification using lipgloss `Place` or padding
  - Apply `StatusBarStyle` (contrasting background)
- `SetSize(w int)` — update width for padding calculation

### 10. Implement Modal System

Rewrite `internal/tui/modal.go`:

- Add fields to `Modal`:
  - `title string`
  - `content string`
  - `width`, `height int` — modal dimensions (not terminal dimensions)
- `NewHelpModal()` — create a modal with:
  - Title: "Keybindings"
  - Content: formatted list of all keybindings grouped by category:
    - Navigation: `j`/`k` move, `G`/`gg` jump, `Tab` focus
    - Detail: `h`/`l` tabs
    - Actions: `n` new run, `p` pause, `r` resume, `c` cancel, `a` accept, `x` reject
    - General: `/` filter, `?` help, `q` quit
  - Width: 50 chars, height: fits content
- `Update(msg tea.Msg)` — `Esc` or `?` or `q` closes the modal (returns a `CloseModalMsg`)
- `View()` — render content inside `ModalStyle` with title
- Define `CloseModalMsg` as a `tea.Msg` type that `App.Update` handles to set `modal = nil`

### 11. Wire Focus and Key Routing in App

Update `internal/tui/app.go` Update method:

- **Global keys** (handled before panel routing):
  - `Tab` — increment `focusedPanel`, wrap around (0 → 1 → 0)
  - `?` — toggle help modal (create if nil, close if active)
  - `q` — `tea.Quit` (later: confirm if active runs)
  - `ctrl+c` — `tea.Quit` unconditionally
- **Panel-specific routing**:
  - `focusedPanel == 0` (run list): forward `j`, `k`, `G`, `gg` to `runList.Update()`; after update, call `detail.SetRun(runList.SelectedRun())`
  - `focusedPanel == 1` (detail): forward `h`, `l` to `detail.Update()` for tab cycling; forward `j`, `k`, `G`, `gg` to detail for viewport scrolling in logs/diff tabs
- **Modal active**: all keys go to `modal.Update()`; on `CloseModalMsg`, set `modal = nil`

### 12. Write Tests

Create test files:

**`internal/tui/app_test.go`:**
- `TestAppInitialState` — new app has `ready == false`, `focusedPanel == 0`
- `TestAppWindowResize` — send `tea.WindowSizeMsg`, verify `ready == true`, dimensions stored
- `TestAppFocusCycle` — send `Tab` key, verify `focusedPanel` cycles 0 → 1 → 0
- `TestAppHelpToggle` — send `?`, verify modal is non-nil; send `?` again, verify nil
- `TestAppQuit` — send `q`, verify `tea.Quit` command returned
- `TestAppViewNotReady` — verify `View()` returns loading message before `WindowSizeMsg`
- `TestAppViewReady` — send `WindowSizeMsg` then verify `View()` contains panel borders

**`internal/tui/runlist_test.go`:**
- `TestRunListNavigation` — send `j` keys, verify selected index increments; `k` decrements
- `TestRunListBounds` — verify selected can't go below 0 or above len(runs)-1
- `TestRunListJumpTop` — send `G` then `gg`, verify selection is first
- `TestRunListJumpBottom` — send `gg` then `G`, verify selection is last
- `TestRunListView` — verify rendered output contains status icons and run info

**`internal/tui/detail_test.go`:**
- `TestDetailTabCycle` — send `l`, verify activeTab increments; `h` decrements; wraps around
- `TestDetailNoRun` — verify view shows "No run selected" when selectedRun is nil
- `TestDetailSetRun` — set a run, verify details tab shows run metadata

## Testing Strategy

### Unit Tests

- **App model tests**: verify initialization, resize handling, focus cycling, modal toggling, and quit behavior by constructing the model and sending `tea.Msg` values through `Update()`
- **RunList tests**: verify navigation bounds, selection state, and rendered output
- **Detail tests**: verify tab cycling, content switching, and null-run handling
- All tests construct models directly and call `Update()` / `View()` — no need for a running terminal

### Edge Cases

- Terminal too small (< 40 cols or < 10 rows) — render a "terminal too small" message instead of the layout
- Terminal resize mid-session — all panels reflow correctly
- Empty run list (0 runs) — run list shows "No runs. Press n to start one." message
- `gg` key binding requires detecting `g` pressed twice — implement as a two-key sequence with a short timeout or track last-key state
- Modal open during resize — modal re-centers
- Focus on detail panel with no runs — detail shows placeholder, j/k are no-ops in detail

## Acceptance Criteria

- [ ] App renders three distinct regions: left panel, right panel, bottom status bar
- [ ] Left panel is ~30% width, right panel is ~70% width
- [ ] Panels have rounded borders (`lipgloss.RoundedBorder()`)
- [ ] Focused panel has a highlighted border color, unfocused panels have dim gray borders
- [ ] `j`/`k` navigate the run list up and down
- [ ] `h`/`l` cycle through detail tabs (Details / Logs / Diff)
- [ ] `G` jumps to bottom, `gg` jumps to top in the run list
- [ ] `Tab` cycles panel focus between run list and detail
- [ ] `?` toggles a centered help modal with all keybindings listed
- [ ] `Esc` closes the modal
- [ ] `q` quits the application
- [ ] Run list shows 4 mock runs with status icons, IDs, branches, workflows, states, tokens, and costs
- [ ] Selected run in the list is visually highlighted
- [ ] Terminal-state runs are visually dimmed
- [ ] Detail panel shows run metadata on the Details tab
- [ ] Detail Logs tab shows mock log entries in a scrollable viewport
- [ ] Detail Diff tab shows mock diff output in a scrollable viewport
- [ ] Status bar shows run count, active count, token total, cost total, and keybind hints
- [ ] Terminal resize reflows all panels correctly
- [ ] Terminal too small shows a warning message
- [ ] All tests pass: `go test ./internal/tui/...`
- [ ] `go vet ./...` and `go build ./...` pass cleanly

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Dependencies resolve
go mod tidy

# All packages compile
go build ./...

# No vet issues
go vet ./...

# TUI tests pass
go test ./internal/tui/... -v

# All tests pass
go test ./...

# Binary builds and runs
make build

# Manual smoke test — launch and verify layout
./bin/agtop
```

## Notes

- The `bubbles` library (`github.com/charmbracelet/bubbles`) was listed as a step 1 dependency in the design doc but was not added during project scaffolding. This step adds it, providing `viewport.Model` for scrollable content and `key.Binding` for structured key definitions.
- Mock data in run list and logs is essential for visual development and testing. Real data integration happens in later steps (process manager, skill engine).
- The `gg` keybinding (jump to top) requires tracking whether the previous key was `g`. Use a `lastKey` field on RunList with a timeout or immediate resolution: if `g` is pressed and the previous key within 500ms was also `g`, treat as `gg`. Otherwise `g` alone is a no-op. This mirrors vim's actual behavior.
- `input.go` is intentionally left as a stub — it's used by the new-run modal in step 13, not the TUI shell.
- The modal system is minimal by design (lazygit-style). Modals are simple content overlays with keybind-driven actions, not form-based dialogs with field tabbing.
- Panel focus is binary (run list or detail) rather than including the status bar, since the status bar is read-only and never needs focus.
- The status bar keybind hints show actions available in the current context. For now they're static; later steps will make them dynamic based on selected run state.
