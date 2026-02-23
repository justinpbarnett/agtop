# Feature: Log Panel Cursor Follows Viewport Scroll

## Metadata

type: `feat`
task_id: `log-panel-cursor-follows-scroll`
prompt: `Navigating up and down in the log panel should also move the cursor/selected log entry.`

## Feature Description

The log panel has two independent navigation mechanisms that can get out of sync:

1. **Entry-level cursor** (`cursorEntry`) — tracks the highlighted log entry. Moved explicitly by `j`/`k`/`up`/`down` keys, which also call `scrollToCursorEntry()` to keep the cursor visible.
2. **Viewport scroll** — the underlying `viewport.Model` has its own scroll state. Unhandled key messages (e.g., `ctrl+d`, `ctrl+u`, `ctrl+f`, `ctrl+b`) and mouse wheel events fall through to `viewport.Update()` at `logview.go:265`, which scrolls the viewport **without** updating `cursorEntry`.

When the viewport scrolls independently, the cursor entry becomes invisible (off-screen) or stranded at a position that doesn't match what the user is looking at. The user expects the highlighted entry to track with their scroll position — navigating down should advance the cursor to the next entry, and navigating up should move it to the previous entry.

## User Story

As a developer monitoring agent runs
I want the highlighted log entry to move when I scroll the log panel
So that the cursor always reflects what I'm looking at and I can expand/interact with visible entries

## Relevant Files

- `internal/ui/panels/logview.go` — The main log viewer. Contains `cursorEntry`, key handling (lines 130–261), the viewport fallthrough at line 265, `scrollToCursorEntry()` (lines 672–700), and `renderEntries()` (lines 529–630). This is the primary file to modify.
- `internal/ui/panels/logview_test.go` — Existing tests for log viewer behavior.
- `internal/process/logentry.go` — `EntryBuffer` struct and methods used to compute entry counts and access entries.

### New Files

None required.

## Implementation Plan

### Phase 1: Intercept Viewport Scroll Keys

Capture the viewport scroll keys (`ctrl+d`, `ctrl+u`, `ctrl+f`, `ctrl+b`) explicitly in the `tea.KeyMsg` switch within `logview.go` Update method, before they fall through to `viewport.Update()`. For each, compute the scroll amount (half-page or full-page) and move `cursorEntry` by the corresponding number of entries, then scroll the viewport and refresh.

### Phase 2: Sync Cursor After Viewport Scroll

For mouse wheel events and any other viewport scrolling that falls through to `viewport.Update()`, add a post-scroll cursor sync. After `viewport.Update()` returns, determine which entry is now at the top of the visible viewport and set `cursorEntry` to that entry. This ensures the cursor tracks with arbitrary scroll events.

### Phase 3: Clamp and Refresh

Ensure `cursorEntry` is always clamped to `[0, entryBuffer.Len()-1]` after any movement. Call `refreshContent()` to re-render with the updated cursor highlight. Disable `follow` mode on any manual scroll action (consistent with existing `j`/`k` behavior).

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add explicit handling for half-page and full-page scroll keys

- In `logview.go`, inside the `tea.KeyMsg` switch (after the `k`/`up` case at line 260), add cases for `ctrl+d`, `ctrl+u`, `ctrl+f`, and `ctrl+b`.
- When `entryBuffer != nil`:
  - `ctrl+d` / `ctrl+f`: Move `cursorEntry` forward by `viewport.Height / 2` (half-page) or `viewport.Height` (full-page) entries, clamped to `entryBuffer.Len() - 1`.
  - `ctrl+u` / `ctrl+b`: Move `cursorEntry` backward by the same amounts, clamped to `0`.
  - Set `follow = false`.
  - Call `refreshContent()` and `scrollToCursorEntry()`.
  - Return early (do not fall through to `viewport.Update()`).
- When `entryBuffer == nil`, let the keys fall through to viewport as they do today.

### 2. Add cursor sync after viewport.Update for mouse scroll

- After the `viewport.Update()` call at line 265, when `entryBuffer != nil`, compute which entry is visible at the top of the viewport by walking through entries and counting rendered lines (similar to the approach in `scrollToCursorEntry()` but in reverse — given a `YOffset`, find the entry at that line).
- Add a helper method `entryAtViewportTop() int` that returns the entry index visible at the current `viewport.YOffset`.
- After `viewport.Update()`, if the viewport offset changed and `entryBuffer != nil`, set `cursorEntry` to `entryAtViewportTop()` and call `refreshContent()`.
- Set `follow = false` when mouse scroll changes the offset.

### 3. Implement `entryAtViewportTop` helper

- Add method `func (l *LogView) entryAtViewportTop() int` to `logview.go`.
- Walk through `entryBuffer.Entries()`, counting one line per summary plus detail lines for expanded entries (matching the logic in `scrollToCursorEntry()`).
- Return the index of the first entry whose summary line is at or below `viewport.YOffset`.
- If no entry found, return `entryBuffer.Len() - 1`.

### 4. Update tests

- In `logview_test.go`, add test cases that verify:
  - `ctrl+d` moves cursor forward by half a viewport height in entries.
  - `ctrl+u` moves cursor backward by half a viewport height in entries.
  - Cursor stays clamped at boundaries (first/last entry).
  - `follow` is disabled after page scroll.

## Testing Strategy

### Unit Tests

- Add tests in `logview_test.go` for the new scroll key handlers (`ctrl+d`, `ctrl+u`, `ctrl+f`, `ctrl+b`) verifying cursor position changes.
- Add tests for `entryAtViewportTop()` with various viewport offsets and mixed expanded/collapsed entries.
- Add tests that verify cursor sync after simulated viewport offset changes.

### Edge Cases

- Empty entry buffer (0 entries) — cursor should remain 0, no panic.
- Single entry — all scroll operations clamp to entry 0.
- All entries expanded — half-page scroll moves fewer entries (since each entry takes multiple lines).
- Cursor at first/last entry — scroll keys should clamp without wrapping.
- Evicted entries — `cursorEntry` must remain valid after eviction adjustment.
- Follow mode active — manual page scroll should disable follow.

## Risk Assessment

- **Low risk**: Changes are confined to `logview.go` key handling and a new helper method. No data model changes, no cross-panel interactions.
- **Viewport state coupling**: The `entryAtViewportTop()` helper duplicates some line-counting logic from `scrollToCursorEntry()`. Consider extracting shared line-counting into a helper to keep them in sync.
- **Expanded entry line counting**: The word-wrap width affects how many lines an expanded entry occupies. `scrollToCursorEntry()` currently uses `strings.Count(e.Detail, "\n") + 1` which doesn't account for word-wrap. The new helper should use the same counting approach for consistency (even if slightly inaccurate for wrapped lines).

## Validation Commands

```bash
make lint
make build
go test ./internal/ui/panels/... -run TestLogView -v
```

## Open Questions (Unresolved)

- **Mouse wheel granularity**: Should mouse wheel scroll move the cursor one entry at a time or track the viewport position? Recommendation: track viewport position (snap cursor to topmost visible entry) since mouse wheel scroll amounts vary by platform/terminal.

## Sub-Tasks

Single task — no decomposition needed.
