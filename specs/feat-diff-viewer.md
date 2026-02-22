# Feature: Diff Viewer

## Metadata

type: `feat`
task_id: `diff-viewer`
prompt: `Implement the diff viewer panel (step 12 of agtop.md): syntax-highlighted unified diff from git diff main...<worktree-branch>, navigable file list, stats summary, auto-refresh on state changes, integrated as a tab in the detail panel`

## Feature Description

The diff viewer is a scrollable, syntax-highlighted panel that displays the `git diff main...<worktree-branch>` output for the selected run. It shows a file list with per-file add/remove stats, a unified diff with color-coded additions (green), deletions (red), and diff headers (cyan), plus a summary stat line. It lives as a new "Diff" tab in the detail panel alongside the existing "Details" tab, introducing a tabbed interface to the detail panel. The diff auto-refreshes when the run store updates (state changes, new skill completions) so developers always see the latest changes.

## User Story

As a developer monitoring concurrent AI agent runs
I want to view the git diff of each run's worktree branch against main in a dedicated panel
So that I can review what the agent has changed without leaving the TUI or switching terminals

## Problem Statement

The detail panel (`internal/ui/panels/detail.go`) currently shows a static key-value display of run metadata (skill, branch, model, status, tokens, cost, worktree, error). There is no way to see what files the agent has changed or review the actual diff. The `DiffGenerator` backend exists in `internal/git/diff.go` with `Diff(branch)` and `DiffStat(branch)` methods, but it is not wired to any UI component.

Developers must leave agtop, navigate to the worktree directory, and run `git diff` manually — exactly the kind of context-switching agtop was built to eliminate.

Additionally, the detail panel has no tab system. The spec for agtop.md (§3, §12) calls for a tabbed detail view with Details / Logs / Diff tabs navigable via `h`/`l` keys, but the current implementation is a single static view.

## Solution Statement

1. **Create a `DiffView` component** (`internal/ui/panels/diffview.go`) that renders syntax-highlighted unified diff output in a `bubbles/viewport`, with vim-style scrolling (`j`/`k`/`G`/`gg`), file header navigation (`]` next file, `[` previous file), and a stats summary line.

2. **Add a tab system to the `Detail` panel** with three tabs: Details (current static view), Diff (new `DiffView`). Tab switching via `h`/`l` when the detail panel is focused. Active tab indicator in the panel title.

3. **Wire diff data flow**: On run selection change or store update, invoke `DiffGenerator.Diff()` and `DiffGenerator.DiffStat()` in a goroutine and deliver results via a Bubble Tea message. The diff view renders the result. If the run has no branch or worktree, show a placeholder.

4. **Auto-refresh**: When the run store fires `RunStoreUpdatedMsg` and the selected run's state has changed, re-fetch the diff for the selected run.

## Relevant Files

Use these files to implement the feature:

- `internal/ui/panels/detail.go` — Transform from static key-value display into a tabbed container with Details and Diff tabs. Add tab state, `h`/`l` key handling, and tab indicator rendering.
- `internal/ui/app.go` — Create `DiffGenerator` instance, pass it to the detail panel. Wire diff fetching on selection change and store updates. Add `DiffResultMsg` handling.
- `internal/git/diff.go` — Existing `DiffGenerator` with `Diff(branch)` and `DiffStat(branch)`. Add `--color=never` flag to ensure clean output for custom syntax highlighting.
- `internal/ui/styles/colors.go` — Add diff-specific colors: `DiffAdded`, `DiffRemoved`, `DiffHeader`, `DiffHunk`.
- `internal/ui/styles/theme.go` — Add diff styling: `DiffAddedStyle`, `DiffRemovedStyle`, `DiffHeaderStyle`, `DiffHunkStyle`.
- `internal/ui/panels/messages.go` — Add `DiffResultMsg` for delivering async diff results.
- `internal/ui/messages.go` — Add type alias for `DiffResultMsg`.
- `internal/run/run.go` — Reference only; no changes needed. `Run.Branch` is used to compute the diff.
- `internal/ui/border/panel.go` — Reference for `RenderPanel` API; no changes needed.
- `internal/ui/panels/logview.go` — Reference for viewport + vim motion pattern to replicate.

### New Files

- `internal/ui/panels/diffview.go` — New diff viewer component with viewport, syntax highlighting, file navigation, and stats summary.
- `internal/ui/panels/diffview_test.go` — Unit tests for diff parsing, highlighting, file navigation, and edge cases.

## Implementation Plan

### Phase 1: Diff View Component

Create the standalone `DiffView` component that takes raw unified diff text and renders it with syntax highlighting in a scrollable viewport. This is self-contained and testable independently of the tab system.

### Phase 2: Detail Panel Tab System

Transform the `Detail` panel from a static view into a tabbed container. Add tab state, key handling for `h`/`l`, and render the active tab's content. The Details tab retains the existing `renderDetails()` output. The Diff tab delegates to the `DiffView` component.

### Phase 3: Data Wiring and Auto-Refresh

Connect the `DiffGenerator` to the UI via async Bubble Tea commands. Fetch diff on selection change and on store updates. Deliver results via `DiffResultMsg`. Handle loading states and errors gracefully.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add Diff Colors and Styles

- Add to `internal/ui/styles/colors.go`:
  - `DiffAdded` — green adaptive color (Light: `"#1a7f37"`, Dark: `"#9ece6a"`)
  - `DiffRemoved` — red adaptive color (Light: `"#cf222e"`, Dark: `"#f7768e"`)
  - `DiffHeader` — cyan adaptive color (Light: `"#0969da"`, Dark: `"#7dcfff"`)
  - `DiffHunk` — magenta adaptive color (Light: `"#8250df"`, Dark: `"#bb9af7"`)
- Add to `internal/ui/styles/theme.go`:
  - `DiffAddedStyle` — `lipgloss.NewStyle().Foreground(DiffAdded)`
  - `DiffRemovedStyle` — `lipgloss.NewStyle().Foreground(DiffRemoved)`
  - `DiffHeaderStyle` — `lipgloss.NewStyle().Foreground(DiffHeader).Bold(true)`
  - `DiffHunkStyle` — `lipgloss.NewStyle().Foreground(DiffHunk)`

### 2. Add DiffResultMsg

- Add to `internal/ui/panels/messages.go`:
  ```go
  type DiffResultMsg struct {
      RunID    string
      Diff     string
      DiffStat string
      Err      error
  }
  ```
- Add type alias to `internal/ui/messages.go`:
  ```go
  type DiffResultMsg = panels.DiffResultMsg
  ```

### 3. Ensure Clean Diff Output

- In `internal/git/diff.go`, update `Diff()` to use `--color=never` flag: `git diff --color=never main...<branch>`. This ensures agtop applies its own syntax highlighting rather than inheriting terminal colors from git.
- Similarly update `DiffStat()` to use `--color=never`.

### 4. Create DiffView Component

- Create `internal/ui/panels/diffview.go` with:
  - `DiffView` struct: `viewport viewport.Model`, `width int`, `height int`, `rawDiff string`, `diffStat string`, `fileOffsets []int` (line offsets of `diff --git` headers for file navigation), `currentFile int`, `loading bool`, `errMsg string`, `focused bool`
  - `NewDiffView() DiffView` — initialize viewport
  - `SetDiff(diff, stat string)` — store raw diff, parse file offsets, render styled content into viewport
  - `SetLoading()` — show "Loading diff..." placeholder
  - `SetError(err string)` — show error message
  - `SetEmpty()` — show "No changes" or "No branch" placeholder
  - `SetSize(w, h int)` — set dimensions, resize viewport to `(w-2, h-2)` to account for borders
  - `SetFocused(focused bool)` — track focus state
  - `Update(msg tea.Msg) (DiffView, tea.Cmd)` — handle vim motions:
    - `j`/`k` — scroll up/down by 1 line
    - `G` — jump to bottom
    - `g` + `g` — jump to top (reuse `gPending` + timer pattern from LogView)
    - `]` — jump to next file header (`currentFile++`, `viewport.SetYOffset(fileOffsets[currentFile])`)
    - `[` — jump to previous file header
  - `View() string` — return `border.RenderPanel("Diff", content, keybinds, w, h, focused)` where keybinds show `]/[` for file nav when focused
  - `renderStyledDiff() string` — parse each line of `rawDiff` and apply styles:
    - Lines starting with `diff --git` → `DiffHeaderStyle`
    - Lines starting with `index`, `---`, `+++` → `DiffHeaderStyle`
    - Lines starting with `@@` → `DiffHunkStyle`
    - Lines starting with `+` (not `+++`) → `DiffAddedStyle`
    - Lines starting with `-` (not `---`) → `DiffRemovedStyle`
    - Context lines → no styling (default text color)
  - `parseFileOffsets()` — scan lines for `diff --git` prefixes, record their line indices in `fileOffsets`

### 5. Add Tab System to Detail Panel

- Modify `internal/ui/panels/detail.go`:
  - Add tab constants: `tabDetails = 0`, `tabDiff = 1`
  - Add fields: `activeTab int`, `diffView DiffView`
  - Update `NewDetail()`: initialize `diffView` via `NewDiffView()`
  - Add `SetDiff(diff, stat string)` method that delegates to `d.diffView.SetDiff(diff, stat)`
  - Add `SetDiffLoading()` that delegates to `d.diffView.SetLoading()`
  - Add `SetDiffError(err string)` that delegates to `d.diffView.SetError(err)`
  - Add `SetDiffEmpty()` that delegates to `d.diffView.SetEmpty()`
  - Update `Update(msg tea.Msg)`:
    - Handle `h`/`l` keys when focused: cycle `activeTab` between `tabDetails` and `tabDiff`
    - When `activeTab == tabDiff`, delegate remaining key events to `d.diffView.Update(msg)`
  - Update `View()`:
    - Build tab indicator in title: `"Details [1/2]"` or `"Diff [2/2]"`. Use highlight style for active tab name.
    - When `activeTab == tabDetails`: render current `renderDetails()` content
    - When `activeTab == tabDiff`: render `d.diffView.View()` — but only the inner content, since the detail panel already wraps in `RenderPanel`. Actually, the `DiffView.View()` should return just the content string (not wrapped in a panel), and the `Detail.View()` wraps everything. Adjust `DiffView.View()` to return raw content, and add a separate `DiffView.Keybinds()` method that returns the keybind list.
    - Show tab-cycling keybinds `h`/`l` in the border when detail is focused
  - Update `SetSize(w, h int)`: propagate inner size to `diffView.SetSize(w-2, h-2)` (accounting for detail panel borders)
  - Update `SetFocused(focused bool)`: propagate to `diffView.SetFocused(focused && activeTab == tabDiff)`

### 6. Wire Diff Fetching in App

- In `internal/ui/app.go`:
  - Add `diffGen *gitpkg.DiffGenerator` field to `App`
  - In `NewApp()`, initialize: `diffGen: gitpkg.NewDiffGenerator(projectRoot)`
  - Add `fetchDiff(runID, branch string) tea.Cmd` method that returns a `tea.Cmd` running `diffGen.Diff(branch)` and `diffGen.DiffStat(branch)` in a goroutine, returning `DiffResultMsg`
  - Update `syncSelection()`:
    - After setting `detail.SetRun(selected)`, check if `selected.Branch != ""`:
      - If yes: call `detail.SetDiffLoading()`, and return the `fetchDiff` command (store it and batch it with other cmds)
      - If no: call `detail.SetDiffEmpty()`
  - Handle `DiffResultMsg` in `Update()`:
    - If `msg.Err != nil`: call `detail.SetDiffError(msg.Err.Error())`
    - If `msg.RunID` matches current selected run: call `detail.SetDiff(msg.Diff, msg.DiffStat)`
    - Otherwise discard (stale result)
  - Update `RunStoreUpdatedMsg` handler: after `syncSelection()`, if the selected run has a branch, also re-fetch the diff (debounce by checking if branch hasn't changed since last fetch to avoid redundant calls)
  - Since `syncSelection()` needs to return commands now, refactor it to return `tea.Cmd` instead of being void. Call sites batch the returned command.
  - Route `DiffResultMsg` to the detail panel if needed, or handle at the app level

### 7. Handle Edge Cases

- **No selected run**: `Detail.View()` shows "No run selected" (already handled)
- **Run with no branch**: `DiffView` shows "No branch — diff unavailable"
- **Run with branch but no changes**: `DiffView` shows "No changes on branch"
- **Diff fetch error** (e.g., branch deleted): `DiffView` shows error message in `StatusError` color
- **Loading state**: `DiffView` shows "Loading diff..." while the goroutine is running
- **Large diffs**: The viewport handles this natively via scrolling. File navigation via `]`/`[` provides quick jumping.
- **Terminal resize**: `SetSize` propagates through Detail → DiffView → viewport
- **Queued/routing runs** (no worktree yet): Show "Waiting for worktree..."

### 8. Write Tests

- Create `internal/ui/panels/diffview_test.go`:
  - `TestDiffViewRenderStyledDiff` — pass a sample unified diff, verify added lines are styled green, removed lines red, headers cyan, hunks magenta
  - `TestDiffViewParseFileOffsets` — pass a multi-file diff, verify `fileOffsets` contains correct line indices
  - `TestDiffViewFileNavigation` — set a multi-file diff, simulate `]` and `[` keys, verify `currentFile` and viewport offset change correctly
  - `TestDiffViewEmptyDiff` — set empty diff string, verify "No changes" placeholder
  - `TestDiffViewError` — call `SetError`, verify error message is rendered
  - `TestDiffViewLoading` — call `SetLoading`, verify loading placeholder
  - `TestDiffViewGGJumpsToTop` — simulate `g` twice within timeout, verify viewport at top
  - `TestDetailTabSwitching` — create `Detail`, simulate `l` key, verify `activeTab` changes to `tabDiff`, simulate `h`, verify back to `tabDetails`
  - `TestDetailDiffIntegration` — create `Detail`, call `SetDiff` with sample data, switch to diff tab, verify diff content is rendered

## Testing Strategy

### Unit Tests

- `DiffView` rendering: verify each diff line type gets the correct style applied
- `DiffView` file offset parsing: verify multi-file diffs produce correct offset arrays
- `DiffView` navigation: `j`/`k`/`G`/`gg`/`]`/`[` all work correctly
- `Detail` tab system: `h`/`l` cycle tabs, content switches correctly
- `Detail` diff integration: `SetDiff`/`SetDiffLoading`/`SetDiffError`/`SetDiffEmpty` propagate to `DiffView`

### Edge Cases

- Empty diff (no changes on branch) — show placeholder, not empty panel
- Error from git (branch deleted, not a git repo) — show error, not crash
- Very large diff (thousands of lines) — viewport scrolls smoothly, file nav works
- Diff with binary files — `Binary files differ` line rendered as plain text
- Diff with no file headers (shouldn't happen normally) — `fileOffsets` empty, `]`/`[` are no-ops
- Tab switching while diff is loading — loading state preserved
- Rapid selection changes — stale `DiffResultMsg` for old run discarded
- Run with no branch (queued state) — "Waiting for worktree..." placeholder

## Acceptance Criteria

- [ ] `DiffView` component renders syntax-highlighted unified diff output
- [ ] Added lines green, removed lines red, diff headers cyan, hunk markers magenta
- [ ] Scrollable via `j`/`k`, `G` (bottom), `gg` (top)
- [ ] `]`/`[` navigate between file headers in the diff
- [ ] Diff stats summary shown (files changed, insertions, deletions)
- [ ] Detail panel has a tab system with Details and Diff tabs
- [ ] `h`/`l` keys cycle tabs when detail panel is focused
- [ ] Active tab indicated in panel title
- [ ] Diff auto-fetched on run selection change
- [ ] Diff auto-refreshed on run store updates
- [ ] Loading state shown while diff is being fetched
- [ ] Placeholder shown for runs with no branch or no changes
- [ ] Error state shown if diff fetch fails
- [ ] Stale diff results for non-selected runs are discarded
- [ ] All new behavior has unit tests
- [ ] Keybinds (`]`/`[`, `h`/`l`) shown in panel border when focused

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Lint
go vet ./...

# Run all tests
go test ./...

# Run diff viewer tests specifically
go test ./internal/ui/panels/ -run TestDiffView -v

# Run detail panel tests
go test ./internal/ui/panels/ -run TestDetail -v

# Build to verify compilation
go build -o bin/agtop ./cmd/agtop
```

## Notes

- The `DiffView` follows the same viewport + vim motion pattern as `LogView` (`internal/ui/panels/logview.go`). The `gg` state machine (gPending + timer) is identical.
- The `DiffGenerator` already shells out to `git diff main...<branch>`. Adding `--color=never` ensures consistent output regardless of the user's git config (`color.diff` setting).
- Tab switching in the detail panel intentionally does NOT include a Logs tab. The log viewer is a separate top-row panel (`panelLogView`). The detail panel tabs are Details and Diff only, per the current layout where logs have their own dedicated panel.
- File navigation (`]`/`[`) is inspired by vim's `]c`/`[c` diff navigation. Keeping it to just `]`/`[` avoids multi-key sequences.
- The async diff fetch pattern (goroutine → `DiffResultMsg`) prevents blocking the UI on potentially slow git operations, especially for large repos. The RunID check on receipt prevents stale results from overwriting a newer selection's diff.
- `syncSelection()` currently returns nothing. Refactoring it to return `tea.Cmd` is a small but necessary change since diff fetching must be triggered as a Bubble Tea command. All call sites (`routeKey` → `syncSelection`, `RunStoreUpdatedMsg` handler) need to batch the returned command.
