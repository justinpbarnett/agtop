# Feature: Collapsible Log Entries with Summary View

## Metadata

type: `feat`
task_id: `log-panel-collapsible-entries`
prompt: `Update the UI/UX of the log panel. Show a title/summary for each log entry instead of raw one-line text that extends off screen. Stream expanded output while active, collapse to summary when complete. Allow expanding entries with Enter/Space or Ctrl+O.`

## Feature Description

The log panel currently renders every line as a single flat string in the format `[HH:MM:SS skill] message`. When log messages are long (tool results, error traces, multi-line text), the content extends off the right edge of the viewport with no way to see the full text. This makes it difficult to understand what's happening during a run.

This feature restructures the log panel from a flat line-per-line buffer into a **collapsible entry model**. Each logical event (text output, tool use, tool result, completion) becomes a discrete entry that displays a one-line summary by default. Users can expand any entry to see full details. The currently-streaming entry remains expanded in real-time, then auto-collapses to its summary once complete.

## User Story

As a developer monitoring agent runs
I want to see concise summaries of each log event with the ability to expand for details
So that I can quickly scan activity without long lines running off-screen, while still accessing full details when needed

## Relevant Files

- `internal/process/stream.go` — Defines `StreamEvent`, `StreamEventType`, and the Claude Code stream parser. This is where event types (text, tool_use, tool_result, result, error, raw) are defined.
- `internal/process/manager.go` — `consumeEvents()` and `consumeSkillEvents()` format log lines via `fmt.Sprintf("[%s %s] ...")` and append to the ring buffer. This formatting pipeline needs to change to produce structured entries instead of flat strings.
- `internal/process/pipe.go` — `RingBuffer` stores `[]string` lines. Needs to be replaced or supplemented with a structured entry buffer.
- `internal/ui/panels/logview.go` — The main log viewer. Currently renders flat lines via `formatLogContent()`. Needs to render collapsible entries with summary/detail states.
- `internal/ui/panels/logview_test.go` — Existing tests for the log viewer.
- `internal/ui/panels/messages.go` — `LogLineMsg` message type. May need updating to carry entry metadata.
- `internal/ui/styles/theme.go` — Style definitions. Will need new styles for entry summaries, expanded details, and entry type indicators.
- `internal/ui/styles/colors.go` — Color constants.
- `internal/ui/keys.go` — Global key bindings. Need to add Enter/Space/Ctrl+O for expand/collapse.
- `internal/ui/app.go` — Routes messages to panels; may need minor updates for new message types.

### New Files

- `internal/process/logentry.go` — New `LogEntry` struct and `EntryBuffer` (structured ring buffer that stores entries instead of raw strings).

## Implementation Plan

### Phase 1: Data Model

Introduce a `LogEntry` struct that represents a single logical log event with summary and detail text. Create an `EntryBuffer` that stores entries (replacing the flat string ring buffer for the log viewer's purposes). The existing `RingBuffer` stays for backward compatibility with any raw-line consumers.

### Phase 2: Entry Generation

Update the `consumeEvents()` / `consumeSkillEvents()` pipelines in the process manager to produce `LogEntry` structs. Each `StreamEvent` maps to one entry. The entry captures: timestamp, skill name, event type, a short summary string, and the full detail text. Tool uses get summaries like `"Tool: Read — /path/to/file.go"`, tool results get summaries like `"Result: (243 chars)"`, text events show the first ~80 chars, etc.

### Phase 3: Log Viewer Rendering

Refactor the log viewer to render entries instead of flat lines. Each entry renders as either its summary line (collapsed) or its full detail block (expanded). Track which entries are expanded via a set. The "cursor" entry (for navigation) is visually indicated. The last entry in an active run stays expanded (streaming mode) and auto-collapses when a new entry arrives or the run completes.

### Phase 4: Interaction & Keybindings

Add entry-level navigation and expand/collapse controls. `j`/`k` move the cursor between entries (not raw lines). `Enter`/`Space` toggle expand/collapse on the cursor entry. `Ctrl+O` also toggles (Claude Code convention). Existing search, copy mode, and `gg`/`G` navigation adapt to the entry model.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Define LogEntry struct and EntryBuffer

- Create `internal/process/logentry.go`
- Define `LogEntry` struct with fields:
  - `Timestamp string` — e.g. `"14:32:01"`
  - `Skill string` — e.g. `"build"`, may be empty
  - `Type StreamEventType` — reuse existing event types (text, tool_use, tool_result, result, error, raw)
  - `Summary string` — one-line summary (pre-computed, max ~80 visible chars)
  - `Detail string` — full content (may be multi-line)
  - `Complete bool` — true when the entry is finalized (not still streaming)
- Define `EntryBuffer` struct:
  - Thread-safe (sync.RWMutex)
  - Circular buffer of `*LogEntry` with configurable capacity (default 5000 entries)
  - `Append(entry *LogEntry)` — add a new entry
  - `UpdateLast(fn func(*LogEntry))` — update the most recent entry in-place (for streaming text accumulation)
  - `Entries() []*LogEntry` — return all entries in order
  - `Len() int`
  - `Get(index int) *LogEntry` — get entry by index
- Write unit tests in `internal/process/logentry_test.go`

### 2. Add summary generation helpers

- In `internal/process/logentry.go`, add a `NewLogEntry(ts, skill string, eventType StreamEventType, detail string) *LogEntry` constructor that auto-generates summaries:
  - `EventText`: First line of detail text, truncated to 80 chars with `…` suffix
  - `EventToolUse`: `"Tool: {toolName}"` (tool name extracted; detail holds full input JSON)
  - `EventToolResult`: `"Result: {first 60 chars}…"` or `"Result: ({len} chars)"` if detail is very long (>200 chars)
  - `EventResult`: `"Completed — {tokens} tokens, ${cost}"`
  - `EventError`: `"ERROR: {first 60 chars}…"`
  - `EventRaw`: First line, truncated to 80 chars
- Add a `ToolUseSummary(toolName, toolInput string) string` helper that produces a readable one-line tool summary (e.g., `"Tool: Read — src/main.go"`, `"Tool: Bash — git status"`, `"Tool: Edit — src/app.go"`)
- Unit test the summary generation for each event type

### 3. Update process manager to produce LogEntries

- In `internal/process/manager.go`, add an `entryBuffers map[string]*EntryBuffer` field to `Manager`
- In `NewManager()`, initialize `entryBuffers`
- Add `EntryBuffer(runID string) *EntryBuffer` accessor method
- Add `InjectEntryBuffer(runID string, entries []*LogEntry)` for session restoration
- In `consumeEvents()` and `consumeSkillEvents()`:
  - Alongside the existing `buf.Append(logLine)` calls, also create a `LogEntry` and append it to the entry buffer
  - For `EventText`: create entry with full text as detail, first line as summary
  - For `EventToolUse`: create entry with tool name summary, full input JSON as detail
  - For `EventToolResult`: create entry with truncated summary, full result as detail
  - For `EventResult`: create entry with completion summary
  - For `EventError`: create entry with error summary, full error as detail
  - For `EventRaw`: create entry with first-line summary, full text as detail
- Keep the existing `RingBuffer` (`buf`) and `buf.Append()` calls unchanged for backward compatibility (raw log export, search, etc.)

### 4. Add new styles for log entries

- In `internal/ui/styles/theme.go`, add:
  - `LogEntrySummaryStyle` — normal weight, primary foreground, for collapsed entry summaries
  - `LogEntryActiveStyle` — slightly brighter or with a subtle left-border indicator for the cursor entry
  - `LogEntryExpandedStyle` — for the detail block of an expanded entry (slightly indented, dim background or left border)
  - `LogEntryTypeIcon` styles — small type indicators: `"▸"` for collapsed, `"▾"` for expanded; dim tool/result/error type labels

### 5. Refactor LogView to render entries

- In `internal/ui/panels/logview.go`:
  - Add new fields to `LogView`:
    - `entryBuffer *EntryBuffer` — reference to the structured entry buffer
    - `expandedEntries map[int]bool` — set of entry indices that are expanded
    - `cursorEntry int` — index of the currently-highlighted entry (for expand/collapse navigation)
    - `streamingExpanded bool` — whether the last entry is force-expanded because it's streaming
  - Update `SetRun()` to also receive and store the `*EntryBuffer`
  - Create `renderEntries() string` method:
    - Iterate over `entryBuffer.Entries()`
    - For each entry, render summary line or expanded detail based on `expandedEntries` set
    - The last entry in an active run: render expanded (streaming mode) with the `▍` cursor
    - The cursor entry gets `LogEntryActiveStyle` highlight
    - Format: `[HH:MM:SS skill] ▸ Summary text` (collapsed) or `[HH:MM:SS skill] ▾ Summary text\n  detail line 1\n  detail line 2...` (expanded)
  - Update `renderContentBase()` to call `renderEntries()` when `entryBuffer` is available, falling back to the existing flat rendering when it's not (backward compat)
  - Update `refreshContent()` accordingly

### 6. Add entry-level navigation and expand/collapse keybindings

- In the `Update()` method of `LogView` (normal mode, log tab):
  - `j`/`down`: Move `cursorEntry` down by 1 entry, scroll viewport to keep it visible
  - `k`/`up`: Move `cursorEntry` up by 1 entry, scroll viewport to keep it visible
  - `Enter`/`Space`: Toggle `expandedEntries[cursorEntry]` — if collapsed, expand; if expanded, collapse
  - `ctrl+o`: Same as Enter/Space (Claude Code convention)
  - `G`: Move cursor to last entry, enable follow
  - `gg`: Move cursor to first entry
  - Keep `y` (copy mode), `/` (search), `l`/`r` (tab switch) working as before
- When `follow` is true and new entries arrive:
  - Auto-advance `cursorEntry` to the last entry
  - Auto-collapse the previously-streaming entry, auto-expand the new last entry
- Update the keybind hints in `View()` to show the new bindings:
  - `Enter` expand, `y` yank/copy, `G` bottom, `gg` top, `/` search

### 7. Handle streaming entry lifecycle

- When a new `LogLineMsg` arrives for the active run:
  - If `follow` is true: auto-collapse the current last entry's streaming expansion, append the new entry, expand the new last entry
  - If the run transitions from active to inactive: collapse all streaming expansions, mark all entries as `Complete`
- In `SetRun()`: reset `expandedEntries`, `cursorEntry` to 0, `streamingExpanded` to true

### 8. Adapt search to entry model

- Update `recomputeMatches()` to search across entry summaries AND details
- `matchIndices` should reference entry indices (not raw line indices)
- When jumping to a match, auto-expand the matching entry if it's collapsed
- Highlight search terms in both summary and detail views

### 9. Adapt copy mode to entry model

- Copy mode should work on the rendered content (already viewport-based), so it should mostly work as-is
- Ensure `yankSelection()` extracts text from the rendered entry view (summaries + expanded details)
- No changes needed to mouse selection (it operates on rendered viewport content)

### 10. Update app.go to pass EntryBuffer

- In `internal/ui/app.go`, where `SetRun()` is called on the log view, also pass the `EntryBuffer` from the manager
- Ensure `LogLineMsg` handling triggers entry-aware refresh

### 11. Write tests

- In `internal/ui/panels/logview_test.go`:
  - Test collapsed entry rendering (shows summary only)
  - Test expanded entry rendering (shows summary + detail)
  - Test Enter/Space toggles expand/collapse
  - Test streaming auto-expand behavior
  - Test cursor navigation between entries
  - Test search across entries
- In `internal/process/logentry_test.go`:
  - Test `EntryBuffer` append, capacity wrap, thread safety
  - Test summary generation for each event type
  - Test `UpdateLast()` for streaming accumulation

## Testing Strategy

### Unit Tests

- `internal/process/logentry_test.go` — EntryBuffer CRUD, capacity, concurrency, summary generation
- `internal/ui/panels/logview_test.go` — Entry rendering, navigation, expand/collapse, search integration

### Edge Cases

- Empty entry buffer (no entries yet) — show "Waiting for output..."
- Very long detail text (>1000 lines) — viewport should handle scrolling within expanded entry
- Entry with empty detail (e.g., tool use with no input) — show summary only, expand shows nothing extra
- Rapid entry arrival during follow mode — don't flicker; batch collapse/expand
- Search match in collapsed entry detail — auto-expand when jumping to match
- Copy mode across mixed collapsed/expanded entries — yank what's visible
- Switching runs resets all expand state
- Entry buffer wrapping (>5000 entries) — oldest entries discarded, cursor/expand indices stay valid

## Risk Assessment

- **Scroll position drift**: Expanding/collapsing entries changes the total line count, which can shift the viewport. Mitigation: recalculate viewport offset after any expand/collapse to keep the cursor entry visible.
- **Performance with many expanded entries**: If a user expands hundreds of entries with large details, rendering could slow. Mitigation: only render entries within the viewport window (virtual scrolling), not the entire buffer.
- **Backward compatibility**: The existing `RingBuffer` and flat log format are kept. The entry model is layered on top. If `entryBuffer` is nil, the viewer falls back to flat rendering.
- **Search regression**: Search currently operates on raw buffer lines. Adapting to entries changes match semantics. Mitigation: thorough test coverage of search across collapsed/expanded entries.

## Validation Commands

```bash
go build ./...
go vet ./...
go test ./internal/process/... -v
go test ./internal/ui/panels/... -v
go test ./... -count=1
```

## Open Questions (Unresolved)

1. **Should j/k navigate entries or lines within expanded entries?**
   - Recommendation: j/k navigate between entries. When an entry is expanded and its detail is taller than the viewport, the viewport scrolls within that entry automatically. This keeps navigation snappy. If users need line-level control within an expanded entry, they can use copy mode.

2. **Should all entries auto-collapse when a run finishes, or keep their current state?**
   - Recommendation: When a run finishes, only the streaming-expanded last entry collapses. User-manually-expanded entries stay expanded. This respects user intent.

3. **What's the visual treatment for the cursor entry?**
   - Recommendation: A subtle highlight (e.g., slightly brighter foreground or a `>` prefix) on the summary line of the cursor entry. Similar to how run list highlights the selected run. Keep it minimal to not clash with search highlights.

4. **Should the raw RingBuffer be phased out or kept alongside EntryBuffer?**
   - Recommendation: Keep both for now. The RingBuffer serves as a simple raw log export source and is used for session persistence. The EntryBuffer is purely for UI rendering. Phasing out can be a future refactor.

## Sub-Tasks

Single task — no decomposition needed.
