# Fullscreen Panel Toggle

**Date:** 2026-02-23
**Specification:** specs/feat-fullscreen-panels.md

## Overview

Added a fullscreen toggle for the Detail and LogView panels, allowing users to expand any panel to fill the entire terminal by pressing `Enter` and return to the normal 3-panel layout with `Esc`. This addresses the need for more screen real estate when reading logs, diffs, or run details without leaving the dashboard.

## What Was Built

- Fullscreen mode for the Detail panel (run info, status, prompt)
- Fullscreen mode for the LogView panel (both Log and Diff tabs)
- `Esc` key to exit fullscreen and restore the 3-panel layout
- Updated help overlay with new keybind documentation
- Reassigned log entry expand/collapse from `Enter` to `Space`

## Technical Implementation

### Files Modified

- `internal/ui/app.go`: Added `fullscreenPanel` state field, `FullscreenMsg`/`ExitFullscreenMsg` handlers, fullscreen-aware `propagateSizes()`, and conditional `View()` rendering
- `internal/ui/panels/messages.go`: Added `FullscreenMsg` and `ExitFullscreenMsg` message types
- `internal/ui/panels/detail.go`: `Enter` key emits `FullscreenMsg{Panel: 2}` when focused with a selected run; added fullscreen keybind hint
- `internal/ui/panels/logview.go`: `Enter` key emits `FullscreenMsg{Panel: 1}` for both Log and Diff tabs; expand/collapse moved to `Space` only; keybind labels updated
- `internal/ui/panels/help.go`: Added `Enter`/`Esc` fullscreen entries to the Navigation section; increased overlay height

### Key Changes

- Fullscreen state is tracked centrally in `App.fullscreenPanel` (`-1` = normal, `panelDetail` or `panelLogView` = fullscreen)
- When fullscreen, `propagateSizes()` gives the active panel full terminal width and `height - 1` (reserving space for the status bar)
- `View()` conditionally renders only the fullscreen panel plus status bar, skipping the normal 3-panel grid assembly
- `Esc` handling is prioritized in the key routing — it exits fullscreen before reaching panel-specific handlers, avoiding conflicts with modal/search/copy dismissal
- Log entry expand/collapse was moved from `Enter` to `Space` to free `Enter` for fullscreen; `Space` was already a supported expand key

## How to Use

1. Navigate to the Detail panel or LogView panel using `Tab` or `h`/`l`
2. Press `Enter` to expand the focused panel to fullscreen
3. All normal panel interactions (scrolling, tab switching, copying) continue to work in fullscreen
4. Press `Esc` to return to the standard 3-panel layout
5. In the Log tab, use `Space` (instead of `Enter`) to expand/collapse structured log entries

## Testing

Run the test suite to verify fullscreen behavior and updated keybinds:

```bash
make check
```

Golden file snapshots for the help overlay and log view have been updated to reflect the new keybinds. To regenerate golden files after visual changes:

```bash
make update-golden
```

## Notes

- The Run List panel does not support fullscreen — `Enter` on the run list continues to have no effect
- Modals (help, new run, init prompt) overlay on top of fullscreen panels as expected
- Window resizes while in fullscreen correctly re-propagate sizes
