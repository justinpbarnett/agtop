# Fix: Status bar flash messages overflow terminal width, corrupting layout

## Metadata

type: `fix`
task_id: `statusbar-flash-overflow`
prompt: `Error flash messages in the status bar (e.g. "unparseable workflow") render below the status line, push all panels up by one row, and corrupt the TUI layout until the user switches panels to force a re-render.`

## Bug Description

When a flash message is appended to the status bar and the total rendered width exceeds the terminal width, the line wraps in the terminal. Since the layout allocates exactly 1 row for the status bar (`usableHeight := termHeight - 1` in `layout.go:58`), the wrapped second line creates an extra row in the view. Bubbletea renders the full view string each frame — when it's 1 line taller than the terminal, the terminal scrolls, hiding the top row of the UI and showing the overflow below the status bar.

The user sees the error text appearing "just below the status line," all panels shifted up by one row, and a broken layout that persists until a panel switch forces a clean re-render. Errors like "unparseable workflow" that recur frequently make this especially painful since each recurrence resets the 5-second flash timer.

## Reproduction Steps

1. Start agtop with a configuration that triggers a route skill returning an unparseable workflow name
2. Observe the status bar renders: ` agtop vX.X.X │ N running N queued N done │ Tokens: Xk │ Total: $X.XX │ ✗ <error message> │ ?:help`
3. On a terminal narrower than ~120 columns, the status bar content exceeds terminal width
4. The line wraps, creating a second row below the status bar
5. All panels shift up by one row, clipping the top border of the run list panel

**Expected behavior:** Flash messages should never cause the status bar to exceed its 1-row allocation. If the message is too long, it should be truncated to fit within the available width.

## Root Cause Analysis

In `internal/ui/panels/statusbar.go:91-108`, the flash message is appended to the `left` section without checking whether the total width will exceed `s.width`:

```go
left := " " + version + sep + counts + sep + tokensStr + sep + costStr

if s.flash != "" && time.Now().Before(s.flashUntil) {
    // ...
    flashStr := lipgloss.NewStyle().Foreground(color).Bold(true).Render(icon + " " + s.flash)
    left += sep + flashStr  // <-- no width check
}
```

At line 114, `gap` is clamped to a minimum of 1 when `leftWidth + rightWidth > s.width`, but this doesn't prevent the total rendered string from exceeding terminal width — it just ensures a minimum 1-char gap. The final output `left + gap + right` can be significantly wider than `s.width`.

The layout in `layout.go:58` allocates exactly `termHeight - 1` rows for panels and 1 row for the status bar. When the status bar wraps to 2 lines, the view becomes `termHeight + 1` lines, causing the terminal to scroll.

## Relevant Files

- `internal/ui/panels/statusbar.go` — Status bar rendering and flash message logic; this is the file to fix
- `internal/ui/layout/layout.go` — Layout calculation that allocates 1 row for the status bar
- `internal/ui/app.go` (lines 601-670) — View assembly that joins panels with status bar
- `internal/ui/text/text.go` — Text utilities; may contain or need a truncation helper

## Fix Strategy

Truncate the status bar output to fit within `s.width` in the `View()` method. The approach:

1. After computing the full `left` string (including any flash message), measure its visual width with `lipgloss.Width()`
2. If `leftWidth + rightWidth + 1 > s.width`, truncate the flash message text (not the icon) to fit the available space, adding an ellipsis
3. If even without the flash message the bar is too wide, truncate the left section itself
4. Ensure the final returned string never has a visual width exceeding `s.width`

This is the minimal fix: it directly prevents the overflow that causes layout corruption, while still showing as much of the flash message as will fit.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add width-aware flash truncation in statusbar.go

- In `internal/ui/panels/statusbar.go`, in the `View()` method, after building the `left` string with the flash message (line 108), calculate how much space is available for the flash
- Compute `availableForFlash = s.width - lipgloss.Width(leftWithoutFlash) - rightWidth - lipgloss.Width(sep) - 1` (1 for minimum gap)
- If the flash content (icon + message) exceeds `availableForFlash`, truncate the message text and append `…`
- If `availableForFlash` is too small to show even the icon, omit the flash entirely

### 2. Add a final safety truncation

- After composing the final output string (`left + gap + right`), use `lipgloss.Width()` to verify the total width
- If it exceeds `s.width`, use `ansi.Truncate()` from `github.com/muesli/ansi` (already a transitive dependency via lipgloss) or the `text.Truncate()` helper to hard-cap the output to `s.width` characters
- This acts as a safety net for any edge cases in width calculation (multi-byte characters, wide Unicode, etc.)

### 3. Verify the fix handles edge cases

- Terminal width at minimum (80 columns): flash should be omitted or heavily truncated
- Very long error messages: truncated with ellipsis
- No flash active: no change to current behavior
- Flash message exactly fitting: no truncation needed

## Regression Testing

### Tests to Add

- Add a test in `internal/ui/panels/statusbar_test.go` (create if needed) that:
  - Sets a flash message longer than the available width and asserts `lipgloss.Width(bar.View()) <= bar.width`
  - Sets a flash message that fits and asserts the full message appears
  - Sets width to minimum (80) with a flash and asserts no overflow
  - Verifies the ellipsis appears when truncation occurs

### Existing Tests to Verify

- Run `make test` to verify all existing tests pass
- Run `make lint` to verify no vet issues
- Check golden file tests in `internal/ui/` still match (run `make update-golden` if the status bar layout changed in golden files)

## Risk Assessment

- **Low risk**: The change is confined to `statusbar.go`'s `View()` method — a pure rendering function with no side effects
- **Truncation visibility**: Very long error messages will be cut off, but this is strictly better than the current behavior (layout corruption). Users can still see the full error in the detail panel's Error field for failed runs
- **ANSI-aware truncation**: Must use ANSI-aware width measurement and truncation (lipgloss.Width / ansi.Truncate) to avoid cutting in the middle of ANSI escape sequences

## Validation Commands

```bash
make lint
make test
```

## Open Questions (Unresolved)

- **Should the flash duration be shortened for errors?** Currently all flash levels use 5 seconds. Recurring errors that reset the timer make the corruption persistent. Recommendation: keep at 5 seconds for now; the truncation fix addresses the layout issue regardless of duration.
