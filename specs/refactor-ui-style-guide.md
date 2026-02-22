# refactor: UI/UX Design Overhaul per Style Guide

**Type:** refactor
**Scope:** `internal/tui/` (full rewrite), `internal/run/run.go` (icon changes)
**Reference:** `docs/style-guide.md`

---

## Summary

Refactor the entire TUI layer to match the agtop style guide. The current implementation uses lipgloss built-in borders, 256-color codes, a 2-panel layout, and a flat package structure. The style guide specifies hand-rendered borders with embedded titles/keybinds, adaptive hex colors, a 3-panel layout, and a modular package structure. This is a ground-up visual overhaul — the data layer (`run.Store`, `process.Manager`, `runtime`) is unchanged.

---

## Current State

### Problems

1. **Borders use `lipgloss.RoundedBorder()`** — cannot embed titles or keybinds into the border line (anti-pattern §4.6, §14.1)
2. **Colors are 256-color integers** (`lipgloss.Color("240")`) — no adaptive light/dark support, no semantic token names matching the style guide
3. **Layout is 2-panel** (30/70 run list + detail) — style guide requires 3-panel (run list + log top, detail bottom) with 40/60 and 65/35 weight splits
4. **Log viewer is a tab inside Detail** — style guide has it as its own top-level panel
5. **Status bar uses background color** — style guide specifies no background, `│`-separated sections with status-colored counts
6. **Help modal uses lipgloss border** — should use hand-rendered border with dimmed background overlay
7. **No text utilities** — no truncation, no relative time formatting, no SI token formatting
8. **Flat package structure** — `internal/tui/*.go` vs style guide's `ui/styles/`, `ui/border/`, `ui/panels/`, etc.
9. **Keybinds are static in status bar** — style guide puts context-sensitive keybinds in focused panel's bottom border
10. **Minimum terminal size is 40×10** — style guide requires 80×24 with descriptive message
11. **Status icons don't match** — `◐` (paused), `◉` (reviewing), `○` (queued) vs style guide's `●` (running), `◌` (pending), `✓` (success), `✗` (error)

### Current Files

```
internal/tui/
  app.go        — Root model, 2-panel layout, lipgloss borders
  runlist.go    — Run list with inline rendering
  detail.go     — Tabbed container (Details/Logs/Diff)
  logs.go       — Log viewer (viewport inside detail tab)
  diff.go       — Diff viewer (viewport inside detail tab)
  statusbar.go  — Single-line bar with background
  modal.go      — Help overlay with lipgloss border
  theme.go      — 256-color palette, lipgloss border styles
  keys.go       — Key bindings
  messages.go   — Bubble Tea message types
  input.go      — Empty stub
```

---

## Target State

### Package Structure

```
internal/
  ui/
    styles/
      colors.go        — AdaptiveColor token definitions (§2)
      theme.go         — Theme loading (§10, future — for now just the default palette)
    border/
      border.go        — RenderBorderTop, RenderBorderBottom, RenderBorderSides (§4.2–4.4)
      keybind.go       — Keybind type, RenderKeybind (§4.3)
      panel.go         — RenderPanel full assembly (§4.5)
    panels/
      runlist.go       — Run list panel (§6.1)
      logview.go       — Log viewer panel (§6.2)
      detail.go        — Detail panel (§6.3)
      statusbar.go     — Global status bar (§6.4)
      help.go          — Help overlay (§7.3)
    layout/
      layout.go        — Weight-based layout calculator (§5.1–5.2)
    text/
      truncate.go      — Truncation with ellipsis (§3.2)
      format.go        — Relative time, token SI, currency formatters (§3.3–3.4)
    app.go             — Root Bubble Tea model
    keys.go            — Key bindings
    messages.go        — Message types
```

### Key Changes

| Aspect | Current | Target |
|--------|---------|--------|
| Colors | `lipgloss.Color("240")` | `lipgloss.AdaptiveColor{Light: "#c0c0c0", Dark: "#3b4261"}` |
| Borders | `lipgloss.RoundedBorder()` | Hand-rendered `╭─ Title ──╮` with embedded titles/keybinds |
| Layout | 2-panel (30/70) | 3-panel: top row (40/60) + bottom row (100%), vertical 65/35 |
| Panels | RunList + Detail(tabs) | RunList + LogView + Detail (no tabs on detail) |
| Keybinds | Static in status bar | Context-sensitive in focused panel's bottom border |
| Status bar | Background color, inline hints | No bg, `│`-separated, status-colored counts, `?:help` only |
| Min size | 40×10 | 80×24 with descriptive message |
| Icons | `○◐◉` | `◌` (pending), `●` (running) only — per §6.1 |
| Focus | 2-panel cycle | 3-panel spatial: h/l horizontal, j/k vertical + scroll overload |
| Modal | lipgloss border | Hand-rendered border, dimmed background |

---

## Migration Strategy

Implement bottom-up: utilities first, then border rendering, then panels, then layout/app. Each phase produces testable, independently verifiable code. The old `internal/tui/` package is replaced entirely — no incremental migration.

---

## Implementation Phases

### Phase 1: Text Utilities (`internal/ui/text/`)

Create foundational text formatting functions used by all panels.

#### Task 1.1: `truncate.go`

Create `internal/ui/text/truncate.go`:

```go
package text

// Truncate truncates s to maxWidth, appending "…" if truncated.
// Uses lipgloss.Width for accurate ANSI-aware width measurement.
func Truncate(s string, maxWidth int) string

// PadRight pads s with spaces to exactly width. If s is wider, returns s unchanged.
func PadRight(s string, width int) string
```

- Use `github.com/mattn/go-runewidth` for `RuneWidth` (already a transitive dep via lipgloss)
- Walk runes, accumulate width, stop at `maxWidth-1` to fit `…`
- Reference implementation in style guide §3.2

#### Task 1.2: `format.go`

Create `internal/ui/text/format.go`:

```go
package text

// RelativeTime formats a duration as "3m ago", "1h ago", or "Jan 02 15:04" if > 24h.
func RelativeTime(t time.Time) string

// FormatTokens formats token counts: 12400 → "12.4k", 1200000 → "1.2M"
func FormatTokens(n int) string

// FormatCost formats cost with 2 decimal places: 1.234 → "$1.23"
func FormatCost(cost float64) string

// FormatPercent formats percentages: 87 → "87%", 8.3 → "8.3%"
func FormatPercent(pct float64) string

// FormatElapsed formats a duration as "3m", "1h12m", "25m" (no seconds unless < 1m).
func FormatElapsed(d time.Duration) string
```

- Token formatting: `≥1M` → `"1.2M"`, `≥1k` → `"12.4k"`, else raw number
- Relative time: `<1m` → `"<1m ago"`, `<60m` → `"3m ago"`, `<24h` → `"1h ago"`, else `"Jan 02 15:04"`
- Percentage: `<10%` → 1 decimal (`"8.3%"`), else no decimal (`"87%"`)

#### Task 1.3: Tests

Create `internal/ui/text/truncate_test.go` and `internal/ui/text/format_test.go`:

- Truncation: empty string, within limit, exact limit, over limit, multi-byte chars
- PadRight: shorter, exact, longer
- RelativeTime: seconds ago, minutes ago, hours ago, > 24h
- FormatTokens: 0, 500, 1000, 12400, 1200000
- FormatCost: 0, 0.12, 1.234, 99.999
- FormatElapsed: 30s, 3m, 1h12m

---

### Phase 2: Color System (`internal/ui/styles/`)

#### Task 2.1: `colors.go`

Create `internal/ui/styles/colors.go` with all semantic color tokens from §2:

```go
package styles

import "github.com/charmbracelet/lipgloss"

// Semantic colors — AdaptiveColor{Light, Dark}
var (
    BorderFocused   = lipgloss.AdaptiveColor{Light: "#2e5cb8", Dark: "#7aa2f7"}
    BorderUnfocused = lipgloss.AdaptiveColor{Light: "#c0c0c0", Dark: "#3b4261"}
    TitleText       = lipgloss.AdaptiveColor{Light: "#1a1b26", Dark: "#c0caf5"}
    KeybindKey      = lipgloss.AdaptiveColor{Light: "#8a6200", Dark: "#e0af68"}
    KeybindLabel    = lipgloss.AdaptiveColor{Light: "#8890a8", Dark: "#565f89"}
    TextPrimary     = lipgloss.AdaptiveColor{Light: "#1a1b26", Dark: "#c0caf5"}
    TextSecondary   = lipgloss.AdaptiveColor{Light: "#8890a8", Dark: "#565f89"}
    TextDim         = lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#3b4261"}

    StatusRunning = lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#7dcfff"}
    StatusSuccess = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#9ece6a"}
    StatusError   = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f7768e"}
    StatusWarning = lipgloss.AdaptiveColor{Light: "#8a6200", Dark: "#e0af68"}
    StatusPending = lipgloss.AdaptiveColor{Light: "#8890a8", Dark: "#565f89"}
)
```

Also add a `CostColor(cost float64) lipgloss.AdaptiveColor` function implementing the §2.3 thresholds.

#### Task 2.2: `theme.go` (minimal)

Create `internal/ui/styles/theme.go` — for now, just export reusable lipgloss styles that panels will reference:

```go
package styles

// Common reusable styles built from the color tokens.
var (
    TextPrimaryStyle   = lipgloss.NewStyle().Foreground(TextPrimary)
    TextSecondaryStyle = lipgloss.NewStyle().Foreground(TextSecondary)
    TextDimStyle       = lipgloss.NewStyle().Foreground(TextDim)
    TitleStyle         = lipgloss.NewStyle().Foreground(TitleText).Bold(true)
    SelectedRowStyle   = lipgloss.NewStyle().Background(lipgloss.AdaptiveColor{Light: "#e0e0e0", Dark: "#292e42"})
)
```

Full theme file loading (§10) is out of scope for this refactor — it's a separate feature.

---

### Phase 3: Border Rendering (`internal/ui/border/`)

This is the core visual change. Hand-rendered borders replace lipgloss `.Border()`.

#### Task 3.1: `keybind.go`

Create `internal/ui/border/keybind.go`:

```go
package border

// Keybind represents a single keybind hint: [e]dit, [k]ill, etc.
type Keybind struct {
    Key   string // The key character, e.g. "e"
    Label string // The label after the key, e.g. "dit"
}

// RenderKeybind renders a single keybind: [e]dit with Key in KeybindKey color (bold), label in KeybindLabel.
func RenderKeybind(kb Keybind) string
```

#### Task 3.2: `border.go`

Create `internal/ui/border/border.go`:

```go
package border

// RenderBorderTop renders: ╭─ Title ────────────╮
// Title is bold TitleText (focused) or TextSecondary (unfocused).
// Border chars use BorderFocused or BorderUnfocused.
func RenderBorderTop(title string, width int, focused bool) string

// RenderBorderBottom renders the bottom border.
// If focused and keybinds provided: ╰─ [e]dit  [k]ill ──╯
// Otherwise: ╰────────────────────╯
func RenderBorderBottom(keybinds []Keybind, width int, focused bool) string

// RenderBorderSides wraps content lines with │ on each side.
// Each line is truncated/padded to innerWidth (width - 2).
func RenderBorderSides(content string, width int, focused bool) string
```

- Characters: `╭ ╮ ╰ ╯ ─ │` (rounded corners, thin lines)
- All border chars colored with `BorderFocused`/`BorderUnfocused`
- Use `text.Truncate` and `text.PadRight` for content lines

#### Task 3.3: `panel.go`

Create `internal/ui/border/panel.go`:

```go
package border

// RenderPanel assembles a complete bordered panel:
//   top border (with title)
//   content lines (with side borders)
//   bottom border (with keybinds if focused)
// Content is padded/cropped to exactly fill height-2 rows × width-2 cols.
func RenderPanel(title string, content string, keybinds []Keybind,
    width, height int, focused bool) string
```

- Inner height = `height - 2` (top + bottom border)
- Inner width = `width - 2` (left + right border)
- Pad content with empty lines if fewer than `innerHeight`
- Hard crop at `innerHeight` lines
- Join with `lipgloss.JoinVertical(lipgloss.Left, top, middle, bottom)`

#### Task 3.4: Tests

Create `internal/ui/border/border_test.go`:

- `RenderBorderTop`: correct width, title embedded, focused vs unfocused colors
- `RenderBorderBottom`: plain (no keybinds), with keybinds, width calculation
- `RenderBorderSides`: correct padding, truncation of long lines
- `RenderPanel`: full assembly, height padding, content cropping
- `RenderKeybind`: correct format `[k]ey`

**Verification approach**: Strip ANSI codes, assert exact character widths and corner characters.

---

### Phase 4: Layout Engine (`internal/ui/layout/`)

#### Task 4.1: `layout.go`

Create `internal/ui/layout/layout.go`:

```go
package layout

// Layout holds the computed pixel dimensions for all panels.
type Layout struct {
    TermWidth    int
    TermHeight   int
    TooSmall     bool   // true if terminal < 80×24

    // Top row panels
    RunListWidth  int
    RunListHeight int
    LogViewWidth  int
    LogViewHeight int

    // Bottom row panel
    DetailWidth  int
    DetailHeight int

    // Status bar
    StatusBarWidth int
}

const (
    MinWidth  = 80
    MinHeight = 24

    TopRowWeight    = 0.65
    BottomRowWeight = 0.35
    LeftColWeight   = 0.40
    RightColWeight  = 0.60
)

// Calculate computes panel dimensions from terminal size.
// Subtracts 1 row for the status bar before splitting.
// Returns Layout with TooSmall=true if under minimum.
func Calculate(termWidth, termHeight int) Layout
```

Calculation logic:
1. If `termWidth < 80 || termHeight < 24` → `TooSmall = true`, return
2. Usable height = `termHeight - 1` (status bar)
3. Top row height = `int(float64(usableHeight) * TopRowWeight)`
4. Bottom row height = `usableHeight - topRowHeight`
5. Run list width = `int(float64(termWidth) * LeftColWeight)`
6. Log view width = `termWidth - runListWidth`
7. Detail width = `termWidth`
8. Status bar width = `termWidth`

#### Task 4.2: Tests

Create `internal/ui/layout/layout_test.go`:

- Too-small detection (79×24, 80×23)
- Standard 120×40: verify all dimensions sum correctly, no off-by-one
- Edge case 80×24: minimum viable layout
- Verify `topHeight + bottomHeight + 1 == termHeight`
- Verify `leftWidth + rightWidth == termWidth`

---

### Phase 5: Panels (`internal/ui/panels/`)

#### Task 5.1: `runlist.go` — Run List Panel

Create `internal/ui/panels/runlist.go`. Rewrite the run list to:

- Use `border.RenderPanel` for the outer frame
- Title: `"Runs (N active)"` with dynamic count in the border
- Status icons per §6.1: `●` running, `◌` pending, `✓` success, `✗` error
- Columns: icon, skill name (or current skill), branch, elapsed time, cost
- Selected row: full-width highlight using `styles.SelectedRowStyle`
- Terminal-state rows: dimmed with `styles.TextDimStyle`
- Bottom border keybinds: context-sensitive per §7.2
- Content truncated, never wrapped (§3.2)
- Cost colored per §2.3 thresholds

Port filter logic, `j/k` scrolling, `gg/G` jumps from current `runlist.go`. The Bubble Tea model behavior stays the same, only rendering changes.

#### Task 5.2: `logview.go` — Log Panel

Create `internal/ui/panels/logview.go`. Promote log viewer from a detail tab to its own top-level panel:

- Use `border.RenderPanel` for outer frame
- Title: `"Log: <skill> ─ <branch>"` showing selected run's current skill and branch
- Timestamps in `TextDim` color, `HH:MM:SS` format
- Auto-scroll locked to bottom when tailing
- If user scrolls up, show `↓ new output` indicator in bottom-right border area
- Streaming cursor: `▍` for active generation
- Tool use lines: indent 2 spaces, tool name in `TextSecondary`
- Use `bubbles/viewport` internally (same as current `LogViewer`)

Port `SetRun`, `follow` logic, ring buffer reading from current `logs.go`.

#### Task 5.3: `detail.go` — Detail Panel

Create `internal/ui/panels/detail.go`. Simplify from tabbed container to key-value display:

- Use `border.RenderPanel` for outer frame
- Title: `"Details"`
- Two-column key-value layout per §6.3:
  ```
  Skill: build                Branch: feat/auth
  Model: claude-sonnet-4-5    Status: running (3m12s)
  Tokens: 12.4k in / 3.2k out    Cost: $0.34
  Worktree: /tmp/wt-feat-auth-build
  Command: claude -p "..." --output-format stream-json
  ```
- Keys in `TextSecondary`, values in `TextPrimary`
- Cost uses dynamic color per §2.3
- Status uses `RunStateColor` helper
- Bottom border keybinds: `[e]dit  [r]etry  [o]pen-worktree` when focused

Remove the tab system (Details/Logs/Diff). Logs are now a separate panel. Diff viewer is deferred to a future spec (it can be re-added as a separate panel or modal later).

#### Task 5.4: `statusbar.go` — Global Status Bar

Create `internal/ui/panels/statusbar.go`:

- No border, no background color
- Full terminal width, single row
- Format: ` agtop v0.1.0  │  3 running  1 queued  2 done  │  Total: $2.45  │  ?: help`
- Separators `│` in `TextDim`
- Counts colored with respective status colors (`StatusRunning`, `StatusPending`, `StatusSuccess`)
- Cost colored per §2.3 thresholds
- `?` keybind hint at far right

#### Task 5.5: `help.go` — Help Overlay

Create `internal/ui/panels/help.go`:

- Use `border.RenderPanel` (not lipgloss border) for the overlay frame
- Title: `"Keybinds"` in the top border
- Bottom border: `"Press ? or Esc to close"`
- Content organized by section: Navigation, Actions, Global (per §7.3)
- Keys in `KeybindKey` color (bold), descriptions in `TextPrimary`
- Background dimming: re-render panels with `TextDim` color (or render a dark overlay)

#### Task 5.6: Panel Tests

Create test files for each panel:

- `runlist_test.go`: row rendering, icon selection, selection highlight, filter, scroll indicators
- `logview_test.go`: auto-follow toggle, buffer reading, title formatting
- `detail_test.go`: key-value rendering, cost coloring, nil run handling
- `statusbar_test.go`: count aggregation, cost coloring, width handling
- `help_test.go`: content structure, key/esc close behavior

---

### Phase 6: App Model & Wiring (`internal/ui/`)

#### Task 6.1: `app.go` — Root Model

Create `internal/ui/app.go`. Rewrite the root model:

- **3-panel focus system**: `focusedPanel int` — 0=run list, 1=log view, 2=detail
- **Spatial navigation** per §7.1:
  - `h/l`: move focus left/right within a row (run list ↔ log view)
  - `Tab`: cycle focus forward (0→1→2→0)
  - When a panel is focused, `j/k` scroll _within_ that panel
- **Layout**: use `layout.Calculate(width, height)` for all dimensions
- **Minimum size check**: display centered message per §5.3 with current dimensions
- **Panel rendering**: call `border.RenderPanel` per panel, `lipgloss.JoinHorizontal` for top row, `lipgloss.JoinVertical` for full layout
- **Modal handling**: render help overlay on top when active

Wire:
- `RunStoreUpdatedMsg` → refresh run list, update detail panel selection
- `LogLineMsg` → update log viewer
- `tea.WindowSizeMsg` → recalculate layout, propagate sizes
- `tea.KeyMsg` → route to focused panel or handle global keys

#### Task 6.2: `keys.go` — Key Bindings

Move to `internal/ui/keys.go` (same structure, updated for 3-panel navigation):

- Add spatial navigation bindings (`h/l` for panel focus in top row)
- Keep `Tab` for cycle
- Keep `j/k` for scroll within panels
- Keep `?`, `q`, `/`, `G`, `gg`

#### Task 6.3: `messages.go` — Message Types

Move to `internal/ui/messages.go` (same content):

- `RunStoreUpdatedMsg`
- `LogLineMsg`
- `CloseModalMsg`

#### Task 6.4: Entry Point Update

Update `cmd/agtop/main.go` to import from `internal/ui` instead of `internal/tui`.

---

### Phase 7: Status Icon Update

#### Task 7.1: Update `internal/run/run.go`

Update `StatusIcon()` to match style guide §6.1:

| State | Current Icon | Target Icon |
|-------|-------------|-------------|
| Running/Routing | `●` | `●` (unchanged) |
| Paused | `◐` | `◐` (keep — style guide doesn't define paused) |
| Completed/Accepted | `✓` | `✓` (unchanged) |
| Failed/Rejected | `✗` | `✗` (unchanged) |
| Reviewing | `◉` | `◉` (keep — style guide doesn't define reviewing) |
| Queued | `○` | `◌` (change to match §6.1 pending icon) |

---

### Phase 8: Cleanup

#### Task 8.1: Delete Old Package

Delete the entire `internal/tui/` directory after the new `internal/ui/` package is complete and wired up.

#### Task 8.2: Update Test Imports

Update any test files that import `internal/tui` to import `internal/ui` or its subpackages.

#### Task 8.3: Verify Build

Run `go build ./...` and `go vet ./...` to verify clean compilation.

#### Task 8.4: Run All Tests

Run `go test ./...` to verify all tests pass.

---

## Relevant Files

### Modified

| File | Change |
|------|--------|
| `cmd/agtop/main.go` | Import path: `internal/tui` → `internal/ui` |
| `internal/run/run.go` | Queued icon: `○` → `◌` |

### New Files

| File | Purpose |
|------|---------|
| `internal/ui/styles/colors.go` | AdaptiveColor token definitions |
| `internal/ui/styles/theme.go` | Reusable style helpers |
| `internal/ui/border/border.go` | Hand-rendered border top/bottom/sides |
| `internal/ui/border/keybind.go` | Keybind type and renderer |
| `internal/ui/border/panel.go` | Full panel assembly |
| `internal/ui/layout/layout.go` | Weight-based layout calculator |
| `internal/ui/text/truncate.go` | Truncation and padding utilities |
| `internal/ui/text/format.go` | Time, token, cost, percent formatters |
| `internal/ui/panels/runlist.go` | Run list panel |
| `internal/ui/panels/logview.go` | Log viewer panel |
| `internal/ui/panels/detail.go` | Detail panel (key-value, no tabs) |
| `internal/ui/panels/statusbar.go` | Global status bar |
| `internal/ui/panels/help.go` | Help overlay |
| `internal/ui/app.go` | Root Bubble Tea model |
| `internal/ui/keys.go` | Key bindings |
| `internal/ui/messages.go` | Message types |
| `internal/ui/text/truncate_test.go` | Truncation tests |
| `internal/ui/text/format_test.go` | Formatter tests |
| `internal/ui/border/border_test.go` | Border rendering tests |
| `internal/ui/layout/layout_test.go` | Layout calculation tests |
| `internal/ui/panels/runlist_test.go` | Run list tests |
| `internal/ui/panels/logview_test.go` | Log viewer tests |
| `internal/ui/panels/detail_test.go` | Detail tests |
| `internal/ui/panels/statusbar_test.go` | Status bar tests |
| `internal/ui/panels/help_test.go` | Help overlay tests |

### Deleted

| File | Reason |
|------|--------|
| `internal/tui/app.go` | Replaced by `internal/ui/app.go` |
| `internal/tui/runlist.go` | Replaced by `internal/ui/panels/runlist.go` |
| `internal/tui/detail.go` | Replaced by `internal/ui/panels/detail.go` |
| `internal/tui/logs.go` | Replaced by `internal/ui/panels/logview.go` |
| `internal/tui/diff.go` | Deferred — not in style guide's core layout |
| `internal/tui/statusbar.go` | Replaced by `internal/ui/panels/statusbar.go` |
| `internal/tui/modal.go` | Replaced by `internal/ui/panels/help.go` |
| `internal/tui/theme.go` | Replaced by `internal/ui/styles/colors.go` + `theme.go` |
| `internal/tui/keys.go` | Replaced by `internal/ui/keys.go` |
| `internal/tui/messages.go` | Replaced by `internal/ui/messages.go` |
| `internal/tui/input.go` | Empty stub, not needed |
| `internal/tui/*_test.go` | Replaced by new package tests |

---

## Out of Scope

- **Theme file loading** (§10) — YAML theme files, user overrides. Separate feature spec.
- **Responsive breakpoints** (§5.4) — Stacked layout for 80–119 cols. Future enhancement.
- **Braille graphs** (§9) — Cost/token sparklines. Stretch goal per style guide.
- **Spinners** (§8.1) — Braille dot spinners for active runs. Separate small feature.
- **Diff viewer** — Removed from main layout. Can be re-added as a modal or 4th panel.
- **Run action commands** — `n`, `k`, `e`, `r`, etc. keybinds trigger actions but the action handlers are not part of this refactor (they're process manager features).

---

## Validation

```bash
# Build
go build ./cmd/agtop

# Vet
go vet ./...

# Tests
go test ./internal/ui/... -v

# Run (manual visual inspection)
go run ./cmd/agtop
```

### Visual Checklist

- [ ] Panel borders use `╭╮╰╯─│` (rounded thin corners)
- [ ] Focused panel border is bright blue (`#7aa2f7`)
- [ ] Unfocused panel borders are dim (`#3b4261`)
- [ ] Panel titles embedded in top border line
- [ ] Keybinds embedded in focused panel's bottom border
- [ ] 3-panel layout: run list (top-left), log (top-right), detail (bottom)
- [ ] Status bar at bottom with `│`-separated sections
- [ ] Help overlay centered with hand-rendered border
- [ ] `Tab` cycles through 3 panels
- [ ] `h/l` navigates between top-row panels
- [ ] `j/k` scrolls within focused panel
- [ ] Text truncated with `…`, never wraps
- [ ] Relative timestamps (`3m ago`) in run list
- [ ] SI token formatting (`12.4k`)
- [ ] Cost colored by threshold (normal/warning/error)
- [ ] Status icons: `●` running, `◌` pending, `✓` success, `✗` error

---

## Dependencies

No new external dependencies required. All needed packages are already in `go.mod`:
- `github.com/charmbracelet/bubbletea`
- `github.com/charmbracelet/bubbles`
- `github.com/charmbracelet/lipgloss`
- `github.com/mattn/go-runewidth` (transitive via lipgloss)

---

## Risk Notes

1. **ANSI width calculation**: Hand-rendered borders require exact character width math. Off-by-one errors cause misaligned corners. Mitigation: always use `lipgloss.Width()` for styled strings, never `len()`.
2. **3-panel focus routing**: Overloading `j/k` for both scroll and focus navigation needs careful context handling. Mitigation: clear state machine — if panel has scrollable content, `j/k` scrolls; `h/l` always switches panels.
3. **Viewport integration**: The log panel uses `bubbles/viewport` which has its own key handling. Must ensure `j/k` are correctly routed. Mitigation: viewport's `Update` only called when log panel is focused.
