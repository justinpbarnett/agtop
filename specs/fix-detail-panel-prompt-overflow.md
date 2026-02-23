# Fix: Detail panel prompt text overflows without wrapping

## Metadata

type: `fix`
task_id: `detail-panel-prompt-overflow`
prompt: `the prompt in the details panel runs off the panel and is not wrapped. I want to be able to see the full prompt, but I also want to account for the fact that sometimes the prompts might be very large and I still want to see the rest of the details in the details panel.`

## Bug Description

The prompt field in the Detail panel overflows the panel boundary when the text is long. Although a `wrappedRow` function exists that wraps and caps the prompt at 6 lines (with a "+N lines" indicator for overflow), the full prompt text is never accessible. For very long prompts, the user cannot read the complete text. Conversely, for moderately long prompts, the 6-line cap wastes space by hiding content that would fit.

**What happens:** Long prompt text either runs off the panel edge or is truncated at 6 lines with no way to see the rest.

**What should happen:** The full prompt should be visible via scrolling, while other detail fields (status, tokens, cost, etc.) remain accessible without being pushed off-screen by a massive prompt.

## Reproduction Steps

1. Start agtop and create a run with a long prompt (50+ words or multi-paragraph)
2. Select the run in the run list
3. Focus the Detail panel (Tab to panel [2])
4. Observe: the prompt text is capped at 6 wrapped lines with "+N lines" indicator — the full text is inaccessible

**Expected behavior:** The detail panel should be scrollable. The prompt wraps to fit the panel width and is fully rendered. Users can scroll with `j`/`k` to navigate through all details, including long prompts and the fields below them.

## Root Cause Analysis

The Detail panel (`internal/ui/panels/detail.go`) renders content as a plain string via `renderDetails()` and passes it to `border.RenderPanel()`. The `RenderPanel` function (`internal/ui/border/panel.go:30`) crops content to `innerHeight` lines — any overflow is silently dropped.

The `wrappedRow` function (detail.go:185-211) mitigates this by capping the prompt at `maxLines=6`, but this means the full prompt text is never accessible to the user. There is no scrolling mechanism — unlike the LogView and DiffView panels which both use `viewport.Model` from the charmbracelet `bubbles` library for scrollable content.

Key code paths:
- `detail.go:218` — `wrappedRow("Prompt", r.Prompt, valStyle, 6)` caps at 6 lines
- `detail.go:38-51` — `View()` passes the full rendered string to `border.RenderPanel()`
- `border/panel.go:30-32` — `RenderPanel` crops to `innerHeight` lines, discarding overflow
- `detail.go:26-36` — `Update()` only handles `y` (yank); no scroll key handling exists

## Relevant Files

- `internal/ui/panels/detail.go` — Detail panel implementation. Needs viewport integration, scroll key handling, and removal of the prompt line cap.
- `internal/ui/border/panel.go` — `RenderPanel` function. The Detail panel will bypass the content-cropping behavior by passing pre-sized viewport output.
- `internal/ui/app.go:740-743` — Routes key messages to `detail.Update()`. No changes needed (already delegates keys).
- `internal/ui/panels/detail_test.go` — Unit tests for the Detail panel.
- `internal/ui/panels/detail_teatest_test.go` — Snapshot tests for the Detail panel.
- `internal/ui/panels/testdata/TestDetailWithRunSnapshot.golden` — Golden file that will need regeneration.
- `internal/ui/panels/logview.go` — Reference implementation for viewport-based scrolling in a panel.

## Fix Strategy

Add a `viewport.Model` to the Detail panel, following the same pattern used by LogView and DiffView. The viewport handles scrolling within the bordered panel area. The prompt `wrappedRow` call removes its `maxLines` cap so the full prompt is always rendered. When the total content exceeds the visible area, users scroll with `j`/`k` and jump with `G`/`gg`.

This approach:
- Reuses the established viewport pattern from LogView/DiffView
- Lets users see the full prompt by scrolling down
- Keeps other detail fields (status, cost, tokens) accessible by scrolling past the prompt
- Requires minimal structural changes — the `renderDetails()` output feeds the viewport instead of going directly to `RenderPanel`

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add viewport to the Detail struct

- In `internal/ui/panels/detail.go`, add `"github.com/charmbracelet/bubbles/viewport"` to imports
- Add `viewport viewport.Model` field to the `Detail` struct
- Add `gPending bool` field for `gg` double-tap support
- In `NewDetail()`, initialize with `viewport: viewport.New(0, 0)`

### 2. Update SetSize to resize the viewport

- In `SetSize()`, after setting `d.width` and `d.height`, compute inner dimensions: `innerW := w - 2`, `innerH := h - 2` (accounting for border)
- Set `d.viewport.Width = innerW` and `d.viewport.Height = innerH`
- Refresh the viewport content after resizing: call `d.viewport.SetContent(d.renderDetails())` when a run is selected

### 3. Add scroll key handling to Update

- In `Update()`, handle these key messages when focused and a run is selected:
  - `j`/`down` — scroll down: `d.viewport.SetYOffset(d.viewport.YOffset + 1)`
  - `k`/`up` — scroll up: `d.viewport.SetYOffset(max(d.viewport.YOffset - 1, 0))`
  - `G` — jump to bottom: `d.viewport.GotoBottom()`
  - `g` — double-tap for `gg` jump to top (use same `gPending` + timer pattern as LogView/DiffView with 300ms timeout)
- Define a `DetailGTimerExpiredMsg struct{}` type and handle it in `Update()` to clear `gPending`
- Keep existing `y` (yank) handling

### 4. Remove prompt line cap in wrappedRow

- In the `wrappedRow` call for the Prompt field (line 218), change `maxLines` from `6` to `0` (or a sentinel value)
- Update the `wrappedRow` function: when `maxLines <= 0`, render all wrapped lines without truncation (skip the truncation and "+N lines" logic)
- Keep the `maxLines=3` cap for follow-up prompts (`Update N` fields, line 223) — these are secondary and less important

### 5. Update View to use viewport output

- In `View()`, when a run is selected:
  1. Set the viewport content: `d.viewport.SetContent(d.renderDetails())`
  2. Use `d.viewport.View()` as the `content` passed to `border.RenderPanel`
- The viewport output is already sized to fit `innerHeight` lines, so `RenderPanel`'s crop logic will be a no-op
- Add keybinds to the panel border when focused: `j/k scroll`, `G/gg jump`

### 6. Refresh viewport on run change

- In `SetRun()`, after setting `d.selectedRun`, refresh the viewport content: `d.viewport.SetContent(d.renderDetails())`
- Reset scroll position to top: `d.viewport.GotoTop()`

### 7. Update tests

- In `internal/ui/panels/detail_test.go`:
  - Add a test `TestDetailPromptScrollable` that creates a Detail with a very long prompt (e.g., 200+ characters), sets a small panel size (40x10), and verifies the full prompt text appears in the rendered content by scrolling (calling `Update` with `j` keys)
  - Add a test `TestDetailScrollKeys` that verifies `j`, `k`, `G`, and `gg` key handling
- Regenerate golden files by running tests with `-update` flag

## Regression Testing

### Tests to Add

- `TestDetailPromptScrollable` — Verify a long prompt is fully rendered (not truncated) and accessible via scrolling
- `TestDetailScrollKeys` — Verify `j`/`k` scroll, `G` jumps to bottom, `gg` jumps to top
- `TestDetailScrollResetOnRunChange` — Verify viewport resets to top when a different run is selected

### Existing Tests to Verify

- `TestDetailNoRun` — Should still pass (no run = no viewport content)
- `TestDetailSetRun` — Should still pass; prompt text visible in initial viewport
- `TestDetailBorder` — Should still pass (border rendering unchanged)
- `TestDetailCostColoring` — Should still pass
- `TestDetailTerminalElapsedTime` — Should still pass
- `TestDetailNilRunHandling` — Should still pass
- `TestDetailMergeStatus`, `TestDetailMergeStatusMerged` — Should still pass
- `TestDetailPRURL` — Should still pass
- `TestDetailNoMergeStatusWhenEmpty` — Should still pass
- `TestDetailWithRunSnapshot` — Golden file will need regeneration (viewport output may differ slightly)
- `TestDetailStateVariations` — Golden files will need regeneration

## Risk Assessment

- **Viewport integration** — Low risk. The same `viewport.Model` pattern is used by LogView and DiffView. The Detail panel is simpler (no search, no copy mode) so this is a straightforward application.
- **Golden file changes** — The viewport may pad or format content slightly differently than the raw string cropping in `RenderPanel`. Golden files will need regeneration. Review the diffs carefully to ensure only formatting changes, not content changes.
- **wrappedRow unlimited mode** — Removing the line cap means very large prompts produce many lines of content. With the viewport in place, this is fine — the content scrolls. Without the viewport, it would overflow the panel. These changes must be done together.
- **Key conflicts** — `j`/`k`/`G`/`g` are only active when the Detail panel is focused. The app already routes keys to the focused panel only (`app.go:729-745`), so no conflicts with other panels.

## Validation Commands

```bash
go test ./internal/ui/panels/... -run TestDetail -v
go build ./...
```

To regenerate golden files after changes:
```bash
go test ./internal/ui/panels/... -run TestDetail -update
```

## Open Questions (Unresolved)

None — the viewport scrolling approach is well-established in this codebase and the requirements are clear.
