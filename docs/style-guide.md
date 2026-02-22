# agtop — UI/UX Style Guide

> Design language for a terminal UI built with Go + Bubbletea + Lipgloss.
> Inspired by **btop** (information-dense panels, border-embedded labels) and **LazyVim** (clean focus states, vim-native navigation).

---

## 1. Design Principles

### 1.1 Information Density Over Minimalism

Every pixel of terminal real estate earns its place. Panels should feel _packed_ with useful data — not cluttered, but dense. If a user glances at agtop, they should immediately know: what's running, what it costs, and whether anything needs attention.

### 1.2 The Border _Is_ the Chrome

Borders are not decorative. They carry panel titles, status indicators, and keybind hints. This is the single most distinctive visual trait borrowed from btop — the border line itself doubles as a label surface, eliminating the need for separate header/footer rows and reclaiming vertical space.

### 1.3 Focus Is Obvious, Context Is Preserved

Exactly one panel is focused at any time. Focused panels use a bright accent border; unfocused panels use a dim border. Content in unfocused panels remains fully readable — only the border color changes. This is the LazyVim model: clear focus without hiding context.

### 1.4 Motion Is Vim-Native

All navigation uses `h/j/k/l`. There is no mouse-first design. Tab order exists as a fallback but the primary interaction model is spatial vim movement between panels.

---

## 2. Color Palette

Use **adaptive colors** (`lipgloss.AdaptiveColor`) everywhere so the UI works on both light and dark terminals. The palette below assumes a dark terminal (the common case for TUI power users) with light-terminal fallbacks.

### 2.1 Semantic Colors

| Token             | Dark Terminal | Light Terminal | Usage                                  |
| ----------------- | ------------- | -------------- | -------------------------------------- |
| `BorderFocused`   | `#7aa2f7`     | `#2e5cb8`      | Active panel border                    |
| `BorderUnfocused` | `#3b4261`     | `#c0c0c0`      | Inactive panel border                  |
| `TitleText`       | `#c0caf5`     | `#1a1b26`      | Panel title text in borders            |
| `KeybindKey`      | `#e0af68`     | `#8a6200`      | The key letter in keybind hints (bold) |
| `KeybindLabel`    | `#565f89`     | `#8890a8`      | The label text after the key           |
| `TextPrimary`     | `#c0caf5`     | `#1a1b26`      | Default body text                      |
| `TextSecondary`   | `#565f89`     | `#8890a8`      | Muted/secondary information            |
| `TextDim`         | `#3b4261`     | `#b0b0b0`      | Timestamps, IDs, low-priority info     |

### 2.2 Status Colors

| Token           | Dark Terminal | Light Terminal | Usage                           |
| --------------- | ------------- | -------------- | ------------------------------- |
| `StatusRunning` | `#7dcfff`     | `#0969da`      | Active/in-progress skill        |
| `StatusSuccess` | `#9ece6a`     | `#1a7f37`      | Completed successfully          |
| `StatusError`   | `#f7768e`     | `#cf222e`      | Failed/errored                  |
| `StatusWarning` | `#e0af68`     | `#8a6200`      | Warnings, high cost, slow tasks |
| `StatusPending` | `#565f89`     | `#8890a8`      | Queued/waiting                  |

### 2.3 Cost Indicator Colors

Cost text color shifts dynamically based on accumulated spend:

| Range         | Color Token     | Rationale                          |
| ------------- | --------------- | ---------------------------------- |
| `$0.00–$0.50` | `TextSecondary` | Normal, no attention needed        |
| `$0.50–$2.00` | `StatusWarning` | Noteworthy, worth a glance         |
| `$2.00+`      | `StatusError`   | Expensive — likely warrants review |

These thresholds should be user-configurable in `config.yaml`.

### 2.4 Implementation

```go
package styles

import "github.com/charmbracelet/lipgloss"

// Adaptive pairs: {Light, Dark}
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

---

## 3. Typography & Text Conventions

### 3.1 No Bold Abuse

Bold is reserved for exactly two uses:

1. **Panel titles** embedded in borders.
2. **The key character** in keybind hints (e.g., the `e` in `[e]dit`).

Everything else is normal weight. This creates clear visual hierarchy without the "everything is shouting" problem.

### 3.2 Truncation Over Wrapping

Text never wraps inside bordered panels. If content exceeds available width, truncate with `…`. This prevents layout breakage — the single most common bubbletea rendering bug.

```go
func Truncate(s string, maxWidth int) string {
    if lipgloss.Width(s) <= maxWidth {
        return s
    }
    // Walk runes, accumulate width, stop at maxWidth-1 to fit "…"
    w := 0
    for i, r := range s {
        rw := runewidth.RuneWidth(r)
        if w+rw > maxWidth-1 {
            return s[:i] + "…"
        }
        w += rw
    }
    return s
}
```

### 3.3 Timestamps

Use **relative time** for anything < 24h (`3m ago`, `1h ago`). Use `Jan 02 15:04` for anything older. Never show seconds unless it's a live timer.

### 3.4 Numbers & Currency

- Cost: `$1.23` (always 2 decimal places, dollar sign prefix)
- Tokens: `12.4k` / `1.2M` (SI abbreviation, 1 decimal)
- Percentages: `87%` (no decimal unless < 10%, then `8.3%`)

---

## 4. Borders & Panels

This is the heart of the agtop aesthetic. Borders use **thin rounded** box-drawing characters with titles and keybinds embedded directly into the border line.

### 4.1 Box-Drawing Character Set

```
Corners (rounded):  ╭ ╮ ╰ ╯
Horizontal line:    ─
Vertical line:      │
T-junctions:        ├ ┤ ┬ ┴
Cross:              ┼
```

These are the Unicode Box Drawing block characters (U+2500–U+257F). The rounded corners (`╭╮╰╯`) are critical — they give the modern, softer feel that distinguishes agtop/btop from the harsher `┌┐└┘` style.

### 4.2 Border-Embedded Titles

Panel titles sit _inside_ the top border line, breaking the horizontal rule:

```
╭─ Runs ──────────────────────────────────╮
│                                         │
╰─────────────────────────────────────────╯
```

Construction technique — you build this as a string, not via lipgloss borders:

```go
func RenderBorderTop(title string, width int, focused bool) string {
    borderColor := BorderUnfocused
    titleColor := TextSecondary
    if focused {
        borderColor = BorderFocused
        titleColor = TitleText
    }

    borderStyle := lipgloss.NewStyle().Foreground(borderColor)
    titleStyle := lipgloss.NewStyle().Foreground(titleColor).Bold(true)

    // "╭─ Title ─────…─╮"
    titleRendered := titleStyle.Render(title)
    titleWidth := lipgloss.Width(titleRendered)

    // Available width for horizontal lines (subtract corners + spaces + title)
    //   ╭─·TITLE·─────╮
    //   1 1 1    1     1  = overhead of 5 + titleWidth
    fillWidth := width - 5 - titleWidth
    if fillWidth < 0 {
        fillWidth = 0
    }

    return borderStyle.Render("╭─ ") +
        titleRendered +
        borderStyle.Render(" " + strings.Repeat("─", fillWidth) + "╮")
}
```

### 4.3 Border-Embedded Keybinds

The bottom border of the **focused** panel shows available keybinds:

```
╰─ [e]dit  [k]ill  [s]top  [f]ilter ─────╯
```

Unfocused panels have a plain bottom border:

```
╰─────────────────────────────────────────╯
```

Keybind rendering:

```go
func RenderKeybind(key string, label string) string {
    keyStyle := lipgloss.NewStyle().
        Foreground(KeybindKey).
        Bold(true)
    labelStyle := lipgloss.NewStyle().
        Foreground(KeybindLabel)

    return keyStyle.Render("["+key+"]") + labelStyle.Render(label)
}

func RenderBorderBottom(keybinds []Keybind, width int, focused bool) string {
    borderColor := BorderUnfocused
    if focused {
        borderColor = BorderFocused
    }
    borderStyle := lipgloss.NewStyle().Foreground(borderColor)

    if !focused || len(keybinds) == 0 {
        return borderStyle.Render("╰" + strings.Repeat("─", width-2) + "╯")
    }

    // Build keybind string
    var parts []string
    for _, kb := range keybinds {
        parts = append(parts, RenderKeybind(kb.Key, kb.Label))
    }
    keybindStr := strings.Join(parts, "  ")
    keybindWidth := lipgloss.Width(keybindStr)

    fillWidth := width - 4 - keybindWidth // ╰─·KEYBINDS·──╯
    if fillWidth < 0 {
        fillWidth = 0
    }

    return borderStyle.Render("╰─ ") +
        keybindStr +
        borderStyle.Render(" " + strings.Repeat("─", fillWidth) + "╯")
}
```

### 4.4 Vertical Borders & Content

Side borders are rendered per-line with padding:

```go
func RenderBorderSides(content string, width int, focused bool) string {
    borderColor := BorderUnfocused
    if focused {
        borderColor = BorderFocused
    }
    borderStyle := lipgloss.NewStyle().Foreground(borderColor)

    innerWidth := width - 2 // subtract left + right border chars
    lines := strings.Split(content, "\n")
    var result []string
    for _, line := range lines {
        // Pad or truncate each line to exact inner width
        padded := PadRight(Truncate(line, innerWidth), innerWidth)
        result = append(result, borderStyle.Render("│")+padded+borderStyle.Render("│"))
    }
    return strings.Join(result, "\n")
}
```

### 4.5 Complete Panel Assembly

```go
func RenderPanel(title string, content string, keybinds []Keybind,
    width, height int, focused bool) string {

    top := RenderBorderTop(title, width, focused)
    bottom := RenderBorderBottom(keybinds, width, focused)

    // Content area height = total height - top border - bottom border
    innerHeight := height - 2
    innerWidth := width - 2

    // Pad content to fill panel, truncate if overflow
    lines := strings.Split(content, "\n")
    for len(lines) < innerHeight {
        lines = append(lines, "")
    }
    lines = lines[:innerHeight] // hard crop

    middle := RenderBorderSides(strings.Join(lines, "\n"), width, focused)

    return lipgloss.JoinVertical(lipgloss.Left, top, middle, bottom)
}
```

### 4.6 Why Not lipgloss.Border{}?

Lipgloss has built-in `RoundedBorder()`, `NormalBorder()`, etc. We deliberately **do not use them for panel borders** because:

1. They cannot embed titles or keybinds into the border line.
2. They cannot change border color per-side or mid-line.
3. They produce the border as part of `.Render()`, making it impossible to compose custom content into the border itself.

Lipgloss borders are fine for _interior_ elements (e.g., a bordered input field inside a panel). But the outer panel frame must be hand-rendered.

---

## 5. Layout System

### 5.1 Panel Grid

agtop uses a **weight-based** layout, not fixed pixel sizes. On every `tea.WindowSizeMsg`, recalculate:

```
╭─ Runs ──────────────────╮╭─ Log ────────────────────╮
│                          ││                          │
│       (list panel)       ││     (streaming log)      │
│                          ││                          │
│                          ││                          │
╰─ [n]ew  [k]ill ─────────╯╰──────────────────────────╯
╭─ Details ─────────────────────────────────────────────╮
│                  (skill detail / cost)                 │
╰─ [e]dit  [r]etry ────────────────────────────────────╯
```

### 5.2 Weight Distribution

```go
const (
    TopRowWeight    = 0.65  // Top panels get 65% of vertical space
    BottomRowWeight = 0.35  // Detail panel gets 35%
    LeftColWeight   = 0.40  // Run list gets 40% of horizontal space
    RightColWeight  = 0.60  // Log viewer gets 60%
)
```

Always subtract 2 from height before allocating to panels (for each panel's top+bottom border). This is the #1 bubbletea layout bug — forgetting that borders consume terminal rows.

### 5.3 Minimum Viable Size

If the terminal is < 80 columns or < 24 rows, show a centered message:

```
Terminal too small. Minimum: 80×24
Current: 62×18
```

Do not attempt to render a broken layout.

### 5.4 Responsive Breakpoints

| Terminal Width | Layout                                      |
| -------------- | ------------------------------------------- |
| ≥ 120 cols     | Full 2-column + detail panel                |
| 80–119 cols    | Stacked: runs on top, log below, detail tab |
| < 80 cols      | Minimum size warning                        |

---

## 6. Panel Catalog

Each panel type in agtop has specific rendering rules.

### 6.1 Run List Panel

```
╭─ Runs (3 active) ──────────────────────╮
│  ● route      feat/auth    3m  $0.12   │
│  ● build      feat/auth    1m  $0.34   │
│  ◌ test       feat/auth    ·   queued  │
│  ✓ spec       fix/nav     12m  $0.89   │
│  ✗ review     fix/nav      8m  $1.20   │
│                                         │
╰─ [n]ew  [k]ill  [f]ilter ──────────────╯
```

- **Status icons**: `●` running (StatusRunning), `◌` pending (StatusPending), `✓` success (StatusSuccess), `✗` error (StatusError)
- **Columns**: icon, skill name, branch, elapsed time, cost
- **Selected row**: full-width highlight bar using `lipgloss.NewStyle().Background()`
- **Count in title**: dynamically updates — `Runs (3 active)`

### 6.2 Log Panel

```
╭─ Log: build ─ feat/auth ───────────────╮
│ [15:03:21] Reading SKILL.md...          │
│ [15:03:22] Injecting system prompt      │
│ [15:03:23] ▍ Running claude -p ...      │
│ [15:03:24]   Creating auth middleware   │
│ [15:03:25]   Writing tests...           │
│ [15:03:25] ░░░░░░░░░░░░░░░░            │
╰─────────────────────────────────────────╯
```

- **Auto-scroll**: locked to bottom when tailing. If user scrolls up, show a `↓ new output` indicator in the bottom-right of the border.
- **Timestamps**: `TextDim` color, `HH:MM:SS` format
- **Streaming cursor**: `▍` (half-block) for active generation
- **Tool use**: indent 2 spaces, prefix with tool name in `TextSecondary`

### 6.3 Detail Panel

```
╭─ Details ─────────────────────────────────────────────╮
│  Skill: build                Branch: feat/auth        │
│  Model: claude-sonnet-4-5    Status: running (3m12s)  │
│  Tokens: 12.4k in / 3.2k out    Cost: $0.34          │
│  Worktree: /tmp/wt-feat-auth-build                    │
│  Command: claude -p "..." --output-format stream-json │
╰─ [e]dit  [r]etry  [o]pen-worktree ───────────────────╯
```

- **Key-value pairs**: left-aligned keys in `TextSecondary`, values in `TextPrimary`
- **Two-column layout**: pairs flow left-to-right, then wrap to next row
- **Cost**: uses dynamic color based on thresholds (§2.3)

### 6.4 Status Bar (Bottom Edge)

The very bottom row of the terminal is a global status bar, _not_ inside any panel:

```
 agtop v0.1.0  │  3 running  1 queued  2 done  │  Total: $2.45  │  ?: help
```

- No border. Uses full terminal width.
- Sections separated by `│` in `TextDim`.
- Counts use their respective status colors.
- `?` keybind hint for help overlay.

---

## 7. Interaction Patterns

### 7.1 Navigation Model

```
     ┌─────────────┐  ┌─────────────┐
     │  Run List    │  │  Log Panel  │
     │   (panel 0)  │  │  (panel 1)  │
     └─────────────┘  └─────────────┘
     ┌─────────────────────────────────┐
     │        Detail Panel             │
     │         (panel 2)               │
     └─────────────────────────────────┘

  h/l = move focus left/right within a row
  j/k = move focus up/down between rows (and scroll within list/log)
  Tab = cycle focus forward through panels
```

When a panel is focused, `j/k` scrolls _within_ that panel. `h/l` moves focus _between_ panels. This is the lazygit model — movement keys are overloaded based on context.

### 7.2 Contextual Keybinds

Keybinds shown in the bottom border are **context-sensitive** — they change based on which panel is focused and what's selected:

| Panel     | Selected State | Available Keybinds                 |
| --------- | -------------- | ---------------------------------- |
| Run List  | Running skill  | `[k]ill  [l]og  [d]etail`          |
| Run List  | Failed skill   | `[r]etry  [l]og  [d]etail`         |
| Run List  | No selection   | `[n]ew  [f]ilter`                  |
| Log Panel | Tailing        | `[w]rap  [c]opy`                   |
| Log Panel | Scrolled up    | `[↓] tail  [w]rap  [c]opy`         |
| Detail    | Any            | `[e]dit  [r]etry  [o]pen-worktree` |

### 7.3 The Help Overlay

Pressing `?` from anywhere opens a centered modal overlay (not a new panel):

```
╭─ Keybinds ──────────────────────╮
│                                  │
│  Navigation                      │
│    h/l     switch panel          │
│    j/k     scroll / move         │
│    Tab     cycle panels          │
│                                  │
│  Actions                         │
│    n       new run               │
│    k       kill selected run     │
│    e       edit configuration    │
│    r       retry failed run      │
│    f       filter runs           │
│    /       search log            │
│                                  │
│  Global                          │
│    ?       toggle this help      │
│    q       quit                  │
│    :       command palette       │
│                                  │
╰─ Press ? or Esc to close ───────╯
```

The overlay dims the background (render all panels first, then overlay a semi-transparent dark layer — in practice, re-render panel content with `TextDim` color).

---

## 8. Animations & Transitions

### 8.1 Spinners

Active skills show a spinner in the run list. Use `charmbracelet/bubbles/spinner` with the **Dot** style:

```go
spinner.Dot // ⣾ ⣽ ⣻ ⢿ ⡿ ⣟ ⣯ ⣷
```

Braille dot spinners are compact (1 cell) and consistent with the btop aesthetic of using braille characters for dense visualizations. Tick rate: 100ms.

### 8.2 Progress Indicators

For skills with known step counts, show a thin progress bar in the detail panel:

```
Progress: ████████░░░░░░░░  4/8 steps
```

Use half-block characters (`█` fill, `░` empty) for sub-character resolution. Match the `StatusRunning` color for fill.

### 8.3 No Gratuitous Animation

Do not animate panel transitions, focus changes, or layout shifts. Immediate state changes only. The terminal redraws fast enough that animation would feel sluggish, not smooth. The only moving elements should be spinners, the streaming cursor, and auto-scrolling logs.

---

## 9. Braille Graphs (Stretch Goal)

Like btop's CPU graphs, agtop can show cost-over-time or token-throughput sparklines using braille characters (U+2800–U+28FF). Each braille cell encodes a 2×4 dot grid, giving effective resolution of 2× horizontal and 4× vertical per character cell.

```
╭─ Cost Rate ($/min) ─────────────╮
│ ⠀⠀⠀⠀⠀⠀⠀⠀⣀⣀⡀⠀⠀⠀⠀⢠⣤⣤⠀⠀⣿⣿  │
│ ⠀⠀⠀⠀⢀⣤⣶⣿⣿⣿⣿⣶⣤⡀⣿⣿⣿⣿⣤⣿⣿  │
╰─────────────────────────────────╯
```

This is a nice-to-have for v2. The core value is the sparkline in the detail panel or status bar.

---

## 10. Theming Architecture

### 10.1 Theme File Format

Themes are YAML files in `~/.config/agtop/themes/`:

```yaml
# tokyo-night.yaml (default)
name: "Tokyo Night"
colors:
  border_focused: "#7aa2f7"
  border_unfocused: "#3b4261"
  title_text: "#c0caf5"
  keybind_key: "#e0af68"
  keybind_label: "#565f89"
  text_primary: "#c0caf5"
  text_secondary: "#565f89"
  text_dim: "#3b4261"
  status_running: "#7dcfff"
  status_success: "#9ece6a"
  status_error: "#f7768e"
  status_warning: "#e0af68"
  status_pending: "#565f89"
```

### 10.2 Built-in Themes

Ship with 3 themes:

1. **Tokyo Night** (default) — the palette documented above
2. **Catppuccin Mocha** — pastel on dark, popular in the ricing community
3. **Monochrome** — grayscale only, for maximum compatibility and accessibility

### 10.3 User Overrides

Users can override individual tokens without creating a full theme:

```yaml
# ~/.config/agtop/config.yaml
theme: "tokyo-night"
theme_overrides:
  status_error: "#ff0000" # I want ERROR to scream
```

---

## 11. Accessibility Notes

### 11.1 256-Color Fallback

If `COLORTERM` is not `truecolor`, convert hex colors to nearest 256-color (6×6×6 cube + grayscale ramp). The `lipgloss` library handles this automatically when using `lipgloss.Color("#hex")` — but test explicitly.

### 11.2 No Color-Only Semantics

Status is conveyed by **icon + color**, never color alone:

- `●` running, `◌` pending, `✓` success, `✗` error
- If someone is colorblind, the icons still communicate state.

### 11.3 Minimum Contrast

All text-on-background combinations should meet WCAG AA contrast ratio (4.5:1) against a `#1a1b26` (dark) or `#ffffff` (light) background. The Tokyo Night palette satisfies this.

---

## 12. File Structure

```
internal/
  ui/
    styles/
      colors.go        # Color token definitions (§2)
      theme.go         # Theme loading & override logic (§10)
    border/
      border.go        # RenderBorderTop, RenderBorderBottom, RenderBorderSides
      keybind.go       # RenderKeybind, keybind types
      panel.go         # RenderPanel — full assembly
    panels/
      runlist.go       # Run list panel (§6.1)
      logview.go       # Log viewer panel (§6.2)
      detail.go        # Detail panel (§6.3)
      statusbar.go     # Global status bar (§6.4)
      help.go          # Help overlay (§7.3)
    layout/
      layout.go        # Weight-based layout calculator (§5)
      responsive.go    # Breakpoint detection & stacking logic
    text/
      truncate.go      # Truncation utilities (§3.2)
      format.go        # Number/currency/time formatters (§3.3, §3.4)
```

---

## 13. Reference: Unicode Characters Used

| Character | Codepoint | Name                                   | Usage               |
| --------- | --------- | -------------------------------------- | ------------------- |
| `╭`       | U+256D    | BOX DRAWINGS LIGHT ARC DOWN AND RIGHT  | Top-left corner     |
| `╮`       | U+256E    | BOX DRAWINGS LIGHT ARC DOWN AND LEFT   | Top-right corner    |
| `╰`       | U+2570    | BOX DRAWINGS LIGHT ARC UP AND RIGHT    | Bottom-left corner  |
| `╯`       | U+256F    | BOX DRAWINGS LIGHT ARC UP AND LEFT     | Bottom-right corner |
| `─`       | U+2500    | BOX DRAWINGS LIGHT HORIZONTAL          | Horizontal border   |
| `│`       | U+2502    | BOX DRAWINGS LIGHT VERTICAL            | Vertical border     |
| `├`       | U+251C    | BOX DRAWINGS LIGHT VERTICAL AND RIGHT  | Left T-junction     |
| `┤`       | U+2524    | BOX DRAWINGS LIGHT VERTICAL AND LEFT   | Right T-junction    |
| `┬`       | U+252C    | BOX DRAWINGS LIGHT DOWN AND HORIZONTAL | Top T-junction      |
| `┴`       | U+2534    | BOX DRAWINGS LIGHT UP AND HORIZONTAL   | Bottom T-junction   |
| `●`       | U+25CF    | BLACK CIRCLE                           | Running status      |
| `◌`       | U+25CC    | DOTTED CIRCLE                          | Pending status      |
| `✓`       | U+2713    | CHECK MARK                             | Success status      |
| `✗`       | U+2717    | BALLOT X                               | Error status        |
| `▍`       | U+258D    | LEFT THREE EIGHTHS BLOCK               | Streaming cursor    |
| `█`       | U+2588    | FULL BLOCK                             | Progress fill       |
| `░`       | U+2591    | LIGHT SHADE                            | Progress empty      |
| `…`       | U+2026    | HORIZONTAL ELLIPSIS                    | Truncation          |

---

## 14. Anti-Patterns (Do Not Do This)

1. **Don't use lipgloss `.Border()` for outer panels.** It can't embed titles. Hand-render.
2. **Don't auto-wrap text in bordered panels.** It breaks border alignment. Always truncate.
3. **Don't hardcode panel dimensions.** Use weight-based proportional layout + `tea.WindowSizeMsg`.
4. **Don't show all keybinds at once.** Only show keybinds relevant to the focused panel and selected item.
5. **Don't use thick/double borders.** They waste horizontal space and fight the "thin line" aesthetic.
6. **Don't mix rounded and square corners.** Use `╭╮╰╯` consistently everywhere.
7. **Don't animate panel transitions.** Instant state changes only.
8. **Don't rely on color alone for status.** Always pair with an icon.
9. **Don't render the help overlay as a separate panel.** It's a modal that overlays existing panels.
10. **Don't forget to subtract border height.** Every panel's usable height = allocated height − 2.
