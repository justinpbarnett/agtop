# Feature: Log Viewer

## Metadata

type: `feat`
task_id: `log-viewer`
prompt: `Implement the full log viewer panel (step 11 of agtop.md): ring buffer per run (10k lines), ANSI color support, vim motions (j/k scroll, G follow, gg top, / search), auto-scroll on active runs with user scroll-up disabling follow, timestamped skill-prefixed log lines`

## Feature Description

The log viewer is the primary observability surface in agtop — a scrollable, searchable, real-time log panel that streams output from AI agent subprocess runs. It renders timestamped, skill-prefixed log lines with ANSI color support, provides vim-style navigation, and includes inline search with match highlighting. It auto-follows new output for active runs but disengages when the user scrolls up, re-engaging on `G`.

A basic `LogView` already exists in `internal/ui/panels/logview.go` with a viewport, ring buffer integration, log line parsing/styling, auto-follow via `G`, and a streaming cursor. This spec completes the remaining features: `gg` (double-tap jump-to-top), `/` search with highlighting, configurable scroll speed, and ANSI pass-through.

## User Story

As a developer monitoring concurrent AI agent runs
I want to scroll, search, and follow streaming logs in real time with vim motions
So that I can quickly find relevant output, track progress, and diagnose failures without leaving the TUI

## Problem Statement

The current `LogView` implementation is functional but incomplete:

1. **No `gg` sequence**: The `g` key disables follow but does not implement the standard vim `gg` (jump-to-top) as a double-tap. There is a `GPrefix` key binding in `internal/ui/keys.go` but no timer/state machine to resolve the `g` → `gg` sequence.
2. **No `/` search**: There is no way to search log content. For long runs producing thousands of lines, finding a specific error or tool invocation requires manually scrolling.
3. **No ANSI pass-through**: `formatLogContent` applies its own styling but strips any ANSI sequences that the underlying agent process emitted. Certain agent outputs (colored test results, error highlighting) lose their formatting.
4. **Scroll speed not configurable**: The `ui.log_scroll_speed` config field exists in the schema but is not wired to the viewport.

## Solution Statement

Enhance `LogView` with:

1. A `g`-prefix state machine: first `g` press starts a short timer (~300ms); second `g` within the window fires `gg` (jump-to-top + disable follow). If the timer expires without a second `g`, it's a no-op (or single-g behavior).
2. A `/` search mode: pressing `/` opens a single-line text input at the bottom of the log panel. Typing filters/highlights matching lines. `Enter` confirms and jumps to next match. `n`/`N` cycle matches. `Esc` cancels search.
3. ANSI color pass-through: preserve any ANSI escape sequences present in raw log lines rather than stripping them during `formatLogContent`.
4. Wire `log_scroll_speed` from config to the viewport's `MouseWheelDelta` and keyboard scroll step.

## Relevant Files

Use these files to implement the feature:

- `internal/ui/panels/logview.go` — Main file to modify. Contains `LogView` struct, `Update`, `View`, `formatLogContent`, and all log rendering logic.
- `internal/ui/panels/logview_test.go` — Tests for the log viewer. Add tests for `gg`, search, and ANSI handling.
- `internal/ui/panels/messages.go` — Panel-level Bubble Tea messages. May need a new message type for search state changes.
- `internal/ui/app.go` — Routes key events to the focused panel. May need to suppress global `g`/`/` handling when log view is focused and in search mode.
- `internal/ui/keys.go` — Key map definitions. `GPrefix` binding exists but is unused; wire it into the `gg` state machine.
- `internal/process/pipe.go` — `RingBuffer` implementation. No changes needed (10k default is correct).
- `internal/ui/styles/theme.go` — Style definitions. May need a search highlight style.
- `internal/ui/styles/colors.go` — Color constants. May need a search match color.
- `internal/ui/border/panel.go` — `RenderPanel` used by `LogView.View()`. No changes expected.
- `internal/config/config.go` — Config struct. Verify `UI.LogScrollSpeed` field exists.

### New Files

No new files needed. All changes are additions to existing files.

## Implementation Plan

### Phase 1: `gg` Jump-to-Top

Add a state machine to `LogView` that tracks whether a `g` press is pending. On first `g`, set a flag and start a Bubble Tea timer (~300ms). On second `g` before the timer fires, execute jump-to-top and clear the flag. On timer expiry, clear the flag. This matches how lazygit and other vim-style TUIs handle the `gg` sequence.

### Phase 2: `/` Search

Add a search mode to `LogView` with a text input (from `bubbles/textinput`). When activated, render a search bar at the bottom of the log viewport. As the user types, scan the buffer for matches and highlight them. Support `n`/`N` for next/previous match navigation, and `Esc` to exit search mode. The search operates on the plain text of log lines (after stripping ANSI for matching, but displaying with ANSI intact).

### Phase 3: ANSI Pass-Through and Scroll Speed

Update `formatLogContent` to preserve existing ANSI sequences in non-parsed lines (lines that don't match the `[HH:MM:SS skill]` pattern). For parsed lines, continue applying custom styling but don't strip ANSI from the message portion. Wire `config.UI.LogScrollSpeed` to the viewport's scroll delta.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `gg` State Machine to LogView

- Add fields to `LogView`: `gPending bool` to track whether a single `g` has been pressed and is awaiting a second `g`
- Define a new `GTimerExpiredMsg` in `internal/ui/panels/messages.go` (or locally in logview.go as an unexported type)
- In `LogView.Update`, handle `g` key:
  - If `gPending` is false: set `gPending = true`, disable follow, return a `tea.Tick` command for 300ms that sends `GTimerExpiredMsg`
  - If `gPending` is true: set `gPending = false`, call `l.viewport.GotoTop()`, disable follow, return nil
- Handle `GTimerExpiredMsg`: set `gPending = false`, return nil
- Ensure `G` (capital) still works: set `follow = true`, `viewport.GotoBottom()`

### 2. Add Search Mode

- Add fields to `LogView`: `searching bool`, `searchInput textinput.Model`, `searchQuery string`, `matchIndices []int` (line indices of matches), `currentMatch int`
- In `NewLogView()`, initialize `searchInput` with `textinput.New()`, set placeholder to "Search...", set prompt to "/"
- In `LogView.Update`, handle `/` key (when not already searching): set `searching = true`, focus the text input, return `textinput.Blink`
- When `searching` is true, route all key events to the text input except:
  - `Esc`: exit search mode, clear `searchQuery` and highlights
  - `Enter`: confirm search, exit input mode but keep `searchQuery` active, jump to first match
- When `searchQuery` is active (not empty) and not in input mode:
  - `n`: jump to next match (`currentMatch++` wrapped)
  - `N`: jump to previous match (`currentMatch--` wrapped)
  - `/`: re-enter search input mode with current query
- Add `findMatches()` method: scan `buffer.Lines()` for lines containing `searchQuery` (case-insensitive), populate `matchIndices`
- Add `jumpToMatch()` method: calculate the viewport offset needed to show `matchIndices[currentMatch]` and call `viewport.SetYOffset()`

### 3. Add Search Highlighting to Log Rendering

- Update `formatLogContent` signature or add a new wrapper that accepts a search query
- In the rendering loop, for lines that contain the search query, wrap the matching substring in a highlight style (e.g., black text on yellow background)
- Add `SearchHighlightStyle` to `internal/ui/styles/theme.go`: `lipgloss.NewStyle().Background(lipgloss.Color("11")).Foreground(lipgloss.Color("0"))` (yellow bg, black fg)
- Current match should use a distinct style (e.g., black on orange/bright yellow) to differentiate from other matches

### 4. Update LogView.View for Search Bar

- When `searching` is true, reduce viewport height by 1 row to make room for the search input bar
- Render the search input at the bottom of the log panel (inside the border, below the viewport)
- When `searchQuery` is active but not in input mode, show a status line like `"Match 3/17 (n/N to navigate, / to edit, Esc to clear)"`
- Update keybinds shown in the border: when focused, add `{Key: "/", Label: "search"}` to the keybinds list

### 5. Preserve ANSI Escape Sequences

- In `formatLogContent`, for lines matching the `logLineRe` pattern, apply custom styling to timestamp and skill name but pass through the message portion (`m[3]`) without stripping ANSI sequences
- For lines that do NOT match the pattern (raw output), pass them through unchanged — do not apply any styling that would interfere with embedded ANSI codes
- The Bubble Tea viewport and Lip Gloss already support ANSI pass-through, so this is mainly about not wrapping raw lines in additional `lipgloss.Render()` calls

### 6. Wire Scroll Speed from Config

- Add a `scrollSpeed int` field to `LogView`
- Add `SetScrollSpeed(speed int)` method
- In `LogView.Update`, for `j`/`k` keys, scroll by `scrollSpeed` lines instead of the default 1
- In `app.go`, after creating `NewLogView()`, call `lv.SetScrollSpeed(cfg.UI.LogScrollSpeed)` if the value is > 0
- Default to 3 lines per keypress if not configured (the viewport default of 1 is too slow for 10k-line buffers)

### 7. Update Tests

- Add `TestLogViewGGJumpsToTop`: simulate pressing `g` twice in quick succession, verify viewport is at top
- Add `TestLogViewGTimerExpiry`: simulate pressing `g` once, advance past timer, verify `gPending` is false
- Add `TestLogViewSearchActivation`: simulate pressing `/`, verify `searching` is true
- Add `TestLogViewSearchMatchHighlight`: set a search query, format content with a matching line, verify highlight styling is present
- Add `TestLogViewSearchNavigation`: set matches, verify `n`/`N` cycle through them
- Add `TestLogViewANSIPassThrough`: pass content containing ANSI escape sequences to `formatLogContent`, verify they are preserved in output
- Add `TestLogViewSearchEscClears`: enter search mode, press Esc, verify search state is cleared

## Testing Strategy

### Unit Tests

- Test `gg` state machine: `g` once sets pending, `g` twice goes to top, timer expiry clears pending
- Test search: activation via `/`, match finding, `n`/`N` navigation, `Esc` cancellation, `Enter` confirmation
- Test `formatLogContent` with ANSI sequences: verify sequences survive formatting
- Test search highlighting: verify match substrings are wrapped in highlight style
- Test scroll speed: verify custom scroll delta is applied

### Edge Cases

- **Empty buffer + search**: `/` in an empty log should show "No matches" immediately
- **Search with no matches**: display "0/0 matches" status, `n`/`N` are no-ops
- **Search while auto-following**: searching should disable follow; `Esc` from search should not re-enable follow
- **Buffer wrap during search**: if the ring buffer wraps (>10k lines), `matchIndices` need to be recomputed. On new `LogLineMsg`, re-run `findMatches()` if `searchQuery` is active
- **`gg` during search**: should be routed to text input as literal characters, not trigger jump-to-top
- **Very long lines**: search highlighting must handle lines that exceed viewport width (truncated by viewport)
- **Regex-special characters in search**: search should use literal string matching (not regex) to avoid crashes on special chars like `[`, `(`, `*`
- **Terminal resize during search**: viewport height adjustment must account for the search bar row

## Acceptance Criteria

- [ ] `gg` (double-tap within 300ms) jumps to the top of the log buffer
- [ ] Single `g` after 300ms timeout is a no-op (no jump)
- [ ] `G` jumps to bottom and re-enables auto-follow
- [ ] `j`/`k` scroll by configurable number of lines (default 3)
- [ ] `/` activates search mode with a text input at the bottom of the log panel
- [ ] Search is case-insensitive literal string matching
- [ ] Matching lines are highlighted (yellow background)
- [ ] Current match has a distinct highlight (brighter/orange)
- [ ] `Enter` confirms search and jumps to first match
- [ ] `n` jumps to next match, `N` jumps to previous match (wrapping)
- [ ] `Esc` exits search mode and clears highlights
- [ ] Match count shown: "Match 3/17"
- [ ] ANSI escape sequences in log lines are preserved and rendered
- [ ] Ring buffer default is 10,000 lines (already implemented)
- [ ] Log lines prefixed with `[HH:MM:SS skill]` (already implemented)
- [ ] Streaming cursor `▍` shown for active runs (already implemented)
- [ ] `log_scroll_speed` from `agtop.yaml` controls scroll delta
- [ ] All new behavior has unit tests

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Lint
go vet ./...

# Run all tests
go test ./...

# Run log viewer tests specifically
go test ./internal/ui/panels/ -run TestLogView -v

# Build to verify compilation
go build -o bin/agtop ./cmd/agtop
```

## Notes

- The `bubbles/textinput` component is already a dependency of the project (used by other Bubble Tea apps in the ecosystem). If not yet in `go.mod`, it will be pulled in as a transitive dependency of `bubbles`.
- The viewport component from `bubbles` supports `SetYOffset()` for programmatic scrolling, which is needed for search match jumping.
- The `gg` state machine pattern is well-established in TUI frameworks — lazygit uses the same timer-based approach.
- Search highlighting requires re-rendering content on every query change. For 10k lines this should be fast enough in Go, but if profiling shows issues, consider only highlighting visible lines.
- The `/` search in the log viewer should not conflict with the `/` filter binding on the run list panel, because key events are routed to the focused panel only.
