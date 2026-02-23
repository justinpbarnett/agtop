# Feature: Copy text from panels

## Metadata

type: `feat`
task_id: `copy-text`
prompt: `Give the user the ability to highlight and copy text in the logs or any of the panels. The most common will be logs and run IDs.`

## Feature Description

Users need to copy text from agtop panels — most commonly log output and run IDs. Because the TUI runs in alt screen with mouse cell motion tracking (`tea.WithMouseCellMotion()`), native terminal text selection is intercepted. This feature adds vim-style yank/copy functionality with OSC52 clipboard support so users can copy text from any panel without leaving the dashboard.

## User Story

As an agtop user
I want to copy log lines, run IDs, and other text from the dashboard
So that I can paste them into other tools, bug reports, or chat messages

## Relevant Files

- `internal/ui/app.go` — Top-level key routing; will add `y` keybind dispatch
- `internal/ui/keys.go` — KeyMap definitions; will add Yank binding
- `internal/ui/panels/logview.go` — Log panel; will add copy mode (visual line selection + yank)
- `internal/ui/panels/runlist.go` — Run list panel; will yank selected run ID
- `internal/ui/panels/detail.go` — Detail panel; will yank selected field value
- `internal/ui/panels/help.go` — Help overlay; will document `y` keybind
- `internal/ui/panels/statusbar.go` — Flash messages; will show "Copied!" confirmation
- `internal/ui/styles/colors.go` — May need a visual selection highlight color
- `cmd/agtop/main.go` — Already uses `tea.WithMouseCellMotion()`

### New Files

- `internal/ui/clipboard/clipboard.go` — Clipboard abstraction using OSC52 with `atotto/clipboard` fallback

## Implementation Plan

### Phase 1: Clipboard Abstraction

Create a small clipboard package that writes to the system clipboard. Use OSC52 escape sequences as the primary method (works across SSH, tmux, etc.) with `atotto/clipboard` (already an indirect dep) as fallback.

### Phase 2: Quick-Yank from Run List and Detail

Add `y` keybind to run list and detail panels that copies the most useful contextual value — the run ID from the run list, and the run ID from the detail panel. Show a flash message confirming the copy.

### Phase 3: Copy Mode in Log View

Add a visual-line copy mode to the log view. Press `y` to enter copy mode (anchor on current viewport line), use `j/k` to extend the selection, press `y` again to yank the selected lines. Press `Esc` to cancel. Selected lines are highlighted with a distinct background color.

### Phase 4: Copy Mode in Diff View

Same copy mode for the diff tab — `y` to start selection, `j/k` to extend, `y` to yank, `Esc` to cancel.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Create clipboard package

- Create `internal/ui/clipboard/clipboard.go`
- Implement `func Write(text string) error` that writes to clipboard via OSC52 (`\x1b]52;c;<base64>\x07`)
- Use `atotto/clipboard` as fallback if OSC52 fails (it's already an indirect dependency — promote to direct)
- The OSC52 output should go to stdout (Bubble Tea's output), so accept an `io.Writer` parameter or use `os.Stdout` directly

### 2. Add visual selection style

- In `internal/ui/styles/colors.go`, add a `SelectionBg` adaptive color (dim blue background like vim visual mode)
- Add a `SelectionStyle` in `internal/ui/styles/theme.go` with the selection background color

### 3. Add yank keybind to KeyMap

- In `internal/ui/keys.go`, add `Yank key.Binding` to `KeyMap` struct
- Bind to `y` key with help text `"yank/copy"`

### 4. Implement quick-yank in RunList

- In `internal/ui/panels/runlist.go`, handle `y` key in `Update()`
- When pressed, copy the selected run's ID to clipboard
- Return a new `YankMsg{Text: runID}` message so the app can show a flash

### 5. Implement quick-yank in Detail

- In `internal/ui/panels/detail.go`, handle `y` key in a new `Update()` method
- Copy the selected run's ID to clipboard
- Return `YankMsg{Text: runID}`

### 6. Implement copy mode in LogView

- In `internal/ui/panels/logview.go`, add copy mode state:
  - `copyMode bool` — whether copy mode is active
  - `copyAnchor int` — the viewport line where copy started
  - `copyCursor int` — the current cursor line (extends selection)
- On `y` key when copy mode is off: enter copy mode, set anchor to current viewport center line, set cursor to anchor
- On `j`/`k` in copy mode: move cursor down/up to extend selection
- On `y` in copy mode: yank selected lines (from min(anchor,cursor) to max(anchor,cursor)), exit copy mode, return `YankMsg`
- On `Esc` in copy mode: exit copy mode, clear selection
- `ConsumesKeys()` should return true when in copy mode
- In `renderContent()`: when copy mode is active, render lines between anchor and cursor with `SelectionStyle` background
- Add `y` to the logview keybinds when focused: `{Key: "y", Label: "ank/copy"}`

### 7. Implement copy mode in DiffView

- In `internal/ui/panels/diffview.go`, add the same copy mode state and logic as LogView
- `y` to enter, `j/k` to extend, `y` to yank, `Esc` to cancel
- Selected lines highlighted with `SelectionStyle`

### 8. Wire yank into App

- In `internal/ui/app.go`, add `YankMsg` type: `type YankMsg struct { Text string }`
- In `Update()`, handle `YankMsg`: call `clipboard.Write(msg.Text)`, show flash "Copied to clipboard"
- In the global `tea.KeyMsg` handler, route `y` to the focused panel (run list → yank run ID, detail → yank run ID, log view → enters/completes copy mode)
- For detail panel: add `y` handler since detail doesn't currently have an `Update` for key messages

### 9. Update help overlay

- In `internal/ui/panels/help.go`, add `y` keybind under Actions section: `y — Yank / copy`

### 10. Update keybind hints

- In `internal/ui/panels/logview.go`, add `y` to the keybinds array when focused
- In `internal/ui/panels/runlist.go`, add `y` to the keybinds array when focused
- Copy mode keybinds: when in copy mode, show `{Key: "y", Label: "ank"}, {Key: "Esc", Label: " cancel"}`

## Testing Strategy

### Unit Tests

- `internal/ui/clipboard/clipboard_test.go` — Test that `Write()` doesn't panic, test base64 encoding of OSC52 sequence
- `internal/ui/panels/logview_test.go` — Test copy mode: entering, extending selection, yanking, canceling
- `internal/ui/panels/diffview_test.go` — Same copy mode tests for diff view
- `internal/ui/app_test.go` — Test that `y` key dispatches correctly per focused panel

### Edge Cases

- Empty log buffer when trying to yank
- Copy mode with no run selected
- Copy mode selection spanning entire log (boundary: anchor at 0, cursor at last line)
- Yanking a single line (anchor == cursor)
- Entering copy mode while in search mode (should be blocked — search takes priority)
- OSC52 write failure falling back to atotto/clipboard

## Risk Assessment

- **OSC52 compatibility**: Not all terminals support OSC52. The `atotto/clipboard` fallback covers local usage; SSH/tmux users may need terminal OSC52 support. This is acceptable — document in README.
- **Key conflict**: `y` is currently unused globally. No conflicts.
- **Copy mode + search mode interaction**: Copy mode should not be enterable while search is active, and vice versa. Guard with `ConsumesKeys()`.

## Validation Commands

```bash
go vet ./...
go test ./internal/ui/... ./internal/ui/panels/... ./internal/ui/clipboard/...
go build ./cmd/agtop
```

## Open Questions (Unresolved)

- **Should `Y` (shift-y) do a different action?** Suggestion: `Y` copies the entire log/diff content, `y` enters visual selection mode. This mirrors vim's `yy` (yank line) vs `y` (yank motion). Recommend implementing `Y` as "yank all visible" in a follow-up.

## Sub-Tasks

Single task — no decomposition needed.
