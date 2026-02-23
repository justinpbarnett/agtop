# Feature: Fullscreen Panel Toggle

## Metadata

type: `feat`
task_id: `fullscreen-panels`
prompt: `Pressing enter on either the details panel, log tab, or diff tab should full screen them. Pressing esc should bring the user back to the 3 panel layout.`

## Feature Description

Add a fullscreen toggle for the Detail panel and LogView panel (both Log and Diff tabs). When a user presses `Enter` on one of these panels, it expands to fill the entire terminal (minus the status bar). Pressing `Esc` returns to the normal 3-panel layout. This allows users to focus on a single panel's content when they need more screen real estate.

## User Story

As an agtop user
I want to fullscreen the detail, log, or diff panel by pressing Enter
So that I can see more content without the other panels taking up space

## Relevant Files

- `internal/ui/app.go` — Main app struct, key routing, View assembly, and size propagation. The fullscreen state will live here and the View method will conditionally render the fullscreen panel.
- `internal/ui/layout/layout.go` — Layout calculation. Needs a new `CalculateFullscreen` function (or parameter) that gives the fullscreen panel the full terminal dimensions.
- `internal/ui/panels/detail.go` — Detail panel. Needs to handle `Enter` key to request fullscreen and `Esc` to exit.
- `internal/ui/panels/logview.go` — LogView panel (Log + Diff tabs). Currently uses `Enter`/`Space` for entry expand/collapse. `Enter` will now trigger fullscreen; expand/collapse moves to `Space` and `ctrl+o` only.
- `internal/ui/panels/help.go` — Help overlay. Needs `Enter` added to the keybind reference.
- `internal/ui/keys.go` — KeyMap struct. Add Fullscreen binding.

### New Files

No new files needed.

## Implementation Plan

### Phase 1: Foundation

Add fullscreen state tracking to the App struct and a message type for toggling fullscreen. Add a layout helper for computing fullscreen panel dimensions.

### Phase 2: Core Implementation

Handle `Enter` in each panel to emit a fullscreen request message. Handle `Esc` at the app level to exit fullscreen. Modify the `View()` method to conditionally render only the fullscreen panel when active.

### Phase 3: Integration

Update size propagation to resize the fullscreen panel to full terminal dimensions. Update the help overlay to document the new keybinds. Reassign log view entry expand/collapse from `Enter` to `Space`-only.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add fullscreen message type

- In `internal/ui/panels/messages.go`, add a new message type:
  ```go
  type FullscreenMsg struct{ Panel int }
  type ExitFullscreenMsg struct{}
  ```

### 2. Add fullscreen state to App

- In `internal/ui/app.go`, add a `fullscreenPanel` field to the `App` struct (type `int`, default `-1` meaning no fullscreen):
  ```go
  fullscreenPanel int // -1 = normal layout, panelDetail/panelLogView = fullscreen
  ```

### 3. Remove `Enter` from LogView entry expand/collapse

- In `internal/ui/panels/logview.go`, in the `Update` method's log-tab key handling (around line 212), remove the `"enter"` case from the expand/collapse switch. Keep `" "` (Space) and `"ctrl+o"` as the expand/collapse triggers.
- Update the keybinds in `View()` (around line 374): change `{Key: "⏎", Label: " expand"}` to `{Key: "␣", Label: " expand"}`.

### 4. Handle `Enter` in Detail panel to request fullscreen

- In `internal/ui/panels/detail.go`, in the `Update` method, add a case for `"enter"`:
  ```go
  case "enter":
      if d.focused && d.selectedRun != nil {
          return d, func() tea.Msg { return FullscreenMsg{Panel: 2} }
      }
  ```
- Add `FullscreenMsg` keybind hint to `View()` when focused and a run is selected:
  ```go
  {Key: "⏎", Label: " fullscreen"},
  ```

### 5. Handle `Enter` in LogView to request fullscreen

- In `internal/ui/panels/logview.go`, in the `Update` method's log-tab key handling, add an `"enter"` case that emits a fullscreen message (only when not in search/copy mode and when `entryBuffer == nil` or as a fallback after removing expand):
  ```go
  case "enter":
      if l.focused {
          return l, func() tea.Msg { return FullscreenMsg{Panel: 1} }
      }
  ```
- For the diff tab, in the switch that currently handles `"h"` / `"left"` before delegating to diffView, add `"enter"`:
  ```go
  case "enter":
      return l, func() tea.Msg { return FullscreenMsg{Panel: 1} }
  ```
- Add `⏎ fullscreen` to keybinds for both Log and Diff tabs when focused.

### 6. Handle fullscreen messages in App.Update

- In `internal/ui/app.go`, in the `Update` method, add cases for the new message types (before the `tea.KeyMsg` case):
  ```go
  case panels.FullscreenMsg:
      a.fullscreenPanel = msg.Panel
      a.propagateSizes()
      return a, nil
  case panels.ExitFullscreenMsg:
      a.fullscreenPanel = -1
      a.propagateSizes()
      return a, nil
  ```

### 7. Handle `Esc` to exit fullscreen

- In `internal/ui/app.go`, in the `tea.KeyMsg` switch (around line 503), add an `"esc"` case early (after modal checks but before panel-specific routing):
  ```go
  case "esc":
      if a.fullscreenPanel >= 0 {
          a.fullscreenPanel = -1
          a.propagateSizes()
          return a, nil
      }
  ```

### 8. Update propagateSizes for fullscreen

- In `internal/ui/app.go`, modify `propagateSizes()` to check `a.fullscreenPanel`. When a panel is fullscreen, give it the full terminal width and `height - 1` (for status bar). Other panels get their normal sizes (they won't be rendered, but this keeps state consistent):
  ```go
  func (a *App) propagateSizes() {
      l := a.layout
      if a.fullscreenPanel == panelDetail {
          a.detail.SetSize(a.width, a.height-1)
          a.runList.SetSize(l.RunListWidth, l.RunListHeight)
          a.logView.SetSize(l.LogViewWidth, l.LogViewHeight)
      } else if a.fullscreenPanel == panelLogView {
          a.logView.SetSize(a.width, a.height-1)
          a.runList.SetSize(l.RunListWidth, l.RunListHeight)
          a.detail.SetSize(l.DetailWidth, l.DetailHeight)
      } else {
          a.runList.SetSize(l.RunListWidth, l.RunListHeight)
          a.logView.SetSize(l.LogViewWidth, l.LogViewHeight)
          a.detail.SetSize(l.DetailWidth, l.DetailHeight)
      }
      a.statusBar.SetSize(l.StatusBarWidth)
  }
  ```

### 9. Update View for fullscreen rendering

- In `internal/ui/app.go`, modify the `View()` method. After rendering panels, check `a.fullscreenPanel` before assembling the layout:
  ```go
  var fullLayout string
  switch a.fullscreenPanel {
  case panelDetail:
      fullLayout = lipgloss.JoinVertical(lipgloss.Left, detailView, statusBarView)
  case panelLogView:
      fullLayout = lipgloss.JoinVertical(lipgloss.Left, logViewView, statusBarView)
  default:
      leftCol := lipgloss.JoinVertical(lipgloss.Left, runListView, detailView)
      mainArea := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, logViewView)
      fullLayout = lipgloss.JoinVertical(lipgloss.Left, mainArea, statusBarView)
  }
  ```

### 10. Update help overlay

- In `internal/ui/panels/help.go`, add `Enter` / `Esc` fullscreen keybinds to the Navigation section:
  ```go
  b.WriteString(kv("Enter", "Fullscreen panel") + "\n")
  b.WriteString(kv("Esc", "Exit fullscreen") + "\n")
  ```

### 11. Initialize fullscreenPanel in NewApp

- In `internal/ui/app.go`, in `NewApp()`, set `fullscreenPanel: -1` in the App struct initialization.

## Testing Strategy

### Unit Tests

- In `internal/ui/app_test.go` (or new test file if none exists): test that sending `FullscreenMsg{Panel: panelDetail}` sets `fullscreenPanel` correctly and that `ExitFullscreenMsg` resets it to `-1`.
- Test that `Esc` key exits fullscreen mode when `fullscreenPanel >= 0`.
- Test that `Esc` key does nothing special when not in fullscreen mode.

### Edge Cases

- Pressing `Enter` on the run list should NOT trigger fullscreen (only detail and logview support it).
- Pressing `Esc` when not in fullscreen and no modal is open should be a no-op (not quit).
- Modals should still work in fullscreen mode — help overlay, new run modal, etc. should overlay on top of the fullscreen panel.
- Window resize while in fullscreen should re-propagate sizes correctly.
- Switching the selected run while in fullscreen detail view should update the detail content.
- Log view search mode: `Enter` should commit the search query (existing behavior) not trigger fullscreen. The fullscreen `Enter` should only fire when not in search/copy mode.

## Risk Assessment

- **Enter key conflict in LogView**: The Log tab currently uses `Enter` for expand/collapse of structured entries. This spec reassigns `Enter` to fullscreen and keeps expand/collapse on `Space` and `ctrl+o`. Users familiar with `Enter` for expand will need to use `Space` instead. The `Space` keybind is already documented alongside `Enter` in the current code (line 212: `case "enter", " ":`), so this is a minor change.
- **Esc key conflicts**: `Esc` is already used to close modals, cancel search, and exit copy mode. The fullscreen exit must only trigger when no modal/search/copy is active. The existing modal/search/copy checks in `Update()` happen before the global key handler, so this is handled naturally by the existing routing order.

## Validation Commands

```bash
make check   # runs go vet and go test in parallel
make build   # ensures the binary compiles
```

## Open Questions (Unresolved)

None — the requirements are clear and the implementation approach is straightforward.

## Sub-Tasks

Single task — no decomposition needed.
