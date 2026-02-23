package panels

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/process"
)

func TestLogViewTitle(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(60, 20)
	lv.SetRun("001", "build", "feat/auth", nil, nil, true)

	view := lv.View()
	if !strings.Contains(view, "Log:") {
		t.Error("expected log title to contain 'Log:'")
	}
	if !strings.Contains(view, "build") {
		t.Error("expected log title to contain skill name")
	}
}

func TestLogViewDefaultContent(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)

	view := lv.View()
	if !strings.Contains(view, "No run selected") {
		t.Error("expected empty state message when no buffer is set")
	}
}

func TestLogViewAutoFollowDefault(t *testing.T) {
	lv := NewLogView()
	if !lv.follow {
		t.Error("expected follow to be true by default")
	}
}

func TestLogViewBorderPresent(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(40, 10)
	view := lv.View()

	if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
		t.Error("expected border characters in log view")
	}
}

func TestLogViewStreamingCursor(t *testing.T) {
	content := formatLogContent("line one\nline two", true, "", nil, 0)
	if !strings.Contains(content, "▍") {
		t.Error("expected streaming cursor ▍ for active content")
	}
}

func TestLogViewNoStreamingCursorForTerminal(t *testing.T) {
	content := formatLogContent("line one\nline two", false, "", nil, 0)
	if strings.Contains(content, "▍") {
		t.Error("expected no streaming cursor for inactive content")
	}
}

func TestLogViewTimestampStyling(t *testing.T) {
	content := formatLogContent("[14:32:01 route] test message", false, "", nil, 0)
	if strings.Contains(content, "[14:32:01") {
		t.Error("expected timestamp to be extracted from brackets")
	}
	if !strings.Contains(content, "14:32:01") {
		t.Error("expected timestamp value to be present")
	}
	if !strings.Contains(content, "route") {
		t.Error("expected skill name to be present")
	}
}

func TestLogViewUniformIndentation(t *testing.T) {
	content := formatLogContent("[14:32:01 route] message\n[14:32:18 build] Reading src/file.ts...", false, "", nil, 0)
	lines := strings.Split(content, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	routeIndent := len(lines[0]) - len(strings.TrimLeft(lines[0], " "))
	buildIndent := len(lines[1]) - len(strings.TrimLeft(lines[1], " "))
	if routeIndent != buildIndent {
		t.Errorf("expected uniform indentation: route=%d, build=%d", routeIndent, buildIndent)
	}
}

// --- gg state machine tests ---

func TestLogViewGGJumpsToTop(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 10)
	// Put content in the viewport so there's something to scroll
	buf := process.NewRingBuffer(100)
	for i := 0; i < 50; i++ {
		buf.Append("[14:32:01 build] line of log output")
	}
	lv.SetRun("001", "build", "main", buf, nil, true)
	// Viewport is at the bottom (follow mode). First g press:
	var cmd tea.Cmd
	lv, cmd = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if !lv.gPending {
		t.Fatal("expected gPending to be true after first g")
	}
	if cmd == nil {
		t.Fatal("expected timer cmd after first g")
	}
	// Second g press before timer:
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if lv.gPending {
		t.Error("expected gPending to be false after gg")
	}
	if lv.follow {
		t.Error("expected follow to be disabled after gg")
	}
	if lv.viewport.YOffset != 0 {
		t.Errorf("expected viewport at top (offset 0), got %d", lv.viewport.YOffset)
	}
}

func TestLogViewGTimerExpiry(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 10)
	// First g press:
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	if !lv.gPending {
		t.Fatal("expected gPending after first g")
	}
	// Timer expires:
	lv, _ = lv.Update(GTimerExpiredMsg{})
	if lv.gPending {
		t.Error("expected gPending to be cleared after timer expiry")
	}
}

func TestLogViewGCapitalFollows(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 10)
	lv.follow = false
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if !lv.follow {
		t.Error("expected G to re-enable follow")
	}
}

// --- Search tests ---

func TestLogViewSearchActivation(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !lv.searching {
		t.Error("expected searching to be true after /")
	}
}

func TestLogViewSearchEscClears(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	// Activate search
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}})
	if !lv.searching {
		t.Fatal("expected searching to be true")
	}
	// Esc clears
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if lv.searching {
		t.Error("expected searching to be false after Esc")
	}
	if lv.searchQuery != "" {
		t.Error("expected searchQuery to be cleared after Esc")
	}
}

func TestLogViewSearchMatchHighlight(t *testing.T) {
	// formatLogContent with a search query should preserve all text content
	content := formatLogContent("line one error here\nline two ok", false, "error", []int{0}, 0)
	if !strings.Contains(content, "error") {
		t.Error("expected 'error' to be present in highlighted output")
	}
	if !strings.Contains(content, "line one") {
		t.Error("expected surrounding text preserved")
	}
	// Non-matching line should be unchanged
	lines := strings.Split(content, "\n")
	if !strings.Contains(lines[1], "line two ok") {
		t.Errorf("expected non-matching line preserved, got %q", lines[1])
	}
}

func TestLogViewSearchNavigation(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	buf := process.NewRingBuffer(100)
	buf.Append("[14:32:01 build] first error line")
	buf.Append("[14:32:02 build] second ok line")
	buf.Append("[14:32:03 build] third error line")
	lv.SetRun("001", "build", "main", buf, nil, false)

	// Set search query manually
	lv.searchQuery = "error"
	lv.recomputeMatches()

	if len(lv.matchIndices) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(lv.matchIndices))
	}
	if lv.matchIndices[0] != 0 || lv.matchIndices[1] != 2 {
		t.Errorf("expected matches at lines 0 and 2, got %v", lv.matchIndices)
	}

	// Navigate next
	lv.currentMatch = 0
	lv.nextMatch()
	if lv.currentMatch != 1 {
		t.Errorf("expected currentMatch=1 after next, got %d", lv.currentMatch)
	}

	// Navigate next wraps
	lv.nextMatch()
	if lv.currentMatch != 0 {
		t.Errorf("expected currentMatch=0 after wrap, got %d", lv.currentMatch)
	}

	// Navigate prev wraps
	lv.prevMatch()
	if lv.currentMatch != 1 {
		t.Errorf("expected currentMatch=1 after prev wrap, got %d", lv.currentMatch)
	}
}

func TestLogViewANSIPassThrough(t *testing.T) {
	// Raw lines (not matching log pattern) should preserve ANSI sequences
	ansiLine := "\033[31mred error text\033[0m"
	content := formatLogContent(ansiLine, false, "", nil, 0)
	if !strings.Contains(content, "\033[31m") {
		t.Error("expected ANSI escape sequence to be preserved in raw line")
	}
}

func TestLogViewSearchNoMatches(t *testing.T) {
	lv := NewLogView()
	buf := process.NewRingBuffer(100)
	buf.Append("line one")
	buf.Append("line two")
	lv.SetRun("001", "build", "main", buf, nil, false)
	lv.searchQuery = "nonexistent"
	lv.recomputeMatches()

	if len(lv.matchIndices) != 0 {
		t.Errorf("expected 0 matches, got %d", len(lv.matchIndices))
	}
}

func TestLogViewScrollSpeed(t *testing.T) {
	lv := NewLogView()
	lv.SetScrollSpeed(5)
	if lv.scrollSpeed != 5 {
		t.Errorf("expected scrollSpeed=5, got %d", lv.scrollSpeed)
	}

	// Zero and negative values should not change the speed
	lv.SetScrollSpeed(0)
	if lv.scrollSpeed != 5 {
		t.Error("expected scrollSpeed to remain 5 after setting 0")
	}
	lv.SetScrollSpeed(-1)
	if lv.scrollSpeed != 5 {
		t.Error("expected scrollSpeed to remain 5 after setting -1")
	}
}

func TestLogViewDefaultScrollSpeed(t *testing.T) {
	lv := NewLogView()
	if lv.scrollSpeed != 3 {
		t.Errorf("expected default scrollSpeed=3, got %d", lv.scrollSpeed)
	}
}

func TestLogViewSearchCaseInsensitive(t *testing.T) {
	lv := NewLogView()
	buf := process.NewRingBuffer(100)
	buf.Append("ERROR: something failed")
	buf.Append("all good here")
	lv.SetRun("001", "build", "main", buf, nil, false)
	lv.searchQuery = "error"
	lv.recomputeMatches()

	if len(lv.matchIndices) != 1 {
		t.Fatalf("expected 1 case-insensitive match, got %d", len(lv.matchIndices))
	}
	if lv.matchIndices[0] != 0 {
		t.Errorf("expected match at line 0, got %d", lv.matchIndices[0])
	}
}

func TestHighlightMatchesBasic(t *testing.T) {
	result := highlightMatches("hello world hello", "hello", false)
	// Both occurrences of "hello" and the surrounding text should be preserved
	if strings.Count(result, "hello") < 2 {
		t.Error("expected both occurrences of 'hello' to be present")
	}
	if !strings.Contains(result, "world") {
		t.Error("expected 'world' to be preserved between matches")
	}
}

func TestHighlightMatchesCaseInsensitive(t *testing.T) {
	result := highlightMatches("Hello HELLO hello", "hello", false)
	// All three should be present (original case preserved)
	if !strings.Contains(result, "Hello") {
		t.Error("expected original-case 'Hello' preserved")
	}
	if !strings.Contains(result, "HELLO") {
		t.Error("expected original-case 'HELLO' preserved")
	}
}

func TestHighlightMatchesEmptyQuery(t *testing.T) {
	result := highlightMatches("hello world", "", false)
	if result != "hello world" {
		t.Error("expected empty query to return line unchanged")
	}
}

func TestLogViewSetRunClearsSearch(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.searchQuery = "test"
	lv.matchIndices = []int{0, 1}
	lv.searching = true

	lv.SetRun("002", "test", "fix/bug", nil, nil, false)

	if lv.searchQuery != "" {
		t.Error("expected searchQuery cleared on SetRun")
	}
	if lv.searching {
		t.Error("expected searching cleared on SetRun")
	}
	if lv.matchIndices != nil {
		t.Error("expected matchIndices cleared on SetRun")
	}
}

// --- Entry-based rendering tests ---

func TestLogViewEntryRendering(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "Building feature"))
	eb.Append(process.NewLogEntry("14:32:02", "build", process.EventToolUse, "Read"))
	eb.Append(process.NewLogEntry("14:32:03", "build", process.EventToolResult, "File contents"))
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)

	view := lv.View()
	if !strings.Contains(view, "Building feature") {
		t.Error("expected text entry summary in view")
	}
	if !strings.Contains(view, "Tool: Read") {
		t.Error("expected tool use summary in view")
	}
	if !strings.Contains(view, "▸") {
		t.Error("expected collapsed indicator ▸")
	}
}

func TestLogViewNoArrowWhenNotExpandable(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	// EventText with single-line detail: Detail == Summary, so not expandable
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "No extra detail here"))
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)

	view := lv.View()
	if strings.Contains(view, "▸") {
		t.Error("expected no expand arrow for non-expandable entry")
	}
	if strings.Contains(view, "▾") {
		t.Error("expected no expand arrow for non-expandable entry")
	}
}

func TestLogViewEntryExpandCollapse(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "Short summary\nDetailed line 1\nDetailed line 2"))
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)
	lv.cursorEntry = 0

	// Initially collapsed — should not contain detail
	view := lv.View()
	if strings.Contains(view, "Detailed line 1") {
		t.Error("expected detail hidden when collapsed")
	}

	// Expand via Enter
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = lv.View()
	if !strings.Contains(view, "Detailed line 1") {
		t.Error("expected detail visible after expand")
	}
	if !strings.Contains(view, "▾") {
		t.Error("expected expanded indicator ▾")
	}

	// Collapse via Enter again
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyEnter})
	view = lv.View()
	if strings.Contains(view, "Detailed line 1") {
		t.Error("expected detail hidden after collapse")
	}
}

func TestLogViewEntryCursorNavigation(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "Entry A"))
	eb.Append(process.NewLogEntry("14:32:02", "build", process.EventText, "Entry B"))
	eb.Append(process.NewLogEntry("14:32:03", "build", process.EventText, "Entry C"))
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)

	// Cursor starts at last entry (follow mode)
	if lv.cursorEntry != 2 {
		t.Errorf("expected cursorEntry=2, got %d", lv.cursorEntry)
	}

	// Move up
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if lv.cursorEntry != 1 {
		t.Errorf("expected cursorEntry=1 after k, got %d", lv.cursorEntry)
	}

	// Move up again
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if lv.cursorEntry != 0 {
		t.Errorf("expected cursorEntry=0 after k, got %d", lv.cursorEntry)
	}

	// Can't go past 0
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if lv.cursorEntry != 0 {
		t.Errorf("expected cursorEntry=0 (clamped), got %d", lv.cursorEntry)
	}

	// Move down
	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if lv.cursorEntry != 1 {
		t.Errorf("expected cursorEntry=1 after j, got %d", lv.cursorEntry)
	}
}

func TestLogViewStreamingEntryIndicator(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "Working on it\nMore detail here"))
	buf := process.NewRingBuffer(100)
	buf.Append("[14:32:01 build] Working on it")
	lv.SetRun("001", "build", "main", buf, eb, true) // active=true

	view := lv.View()
	// Streaming entry should show cursor indicator on summary line but NOT auto-expand
	if !strings.Contains(view, "▍") {
		t.Error("expected streaming cursor ▍ on summary line")
	}
	if strings.Contains(view, "More detail here") {
		t.Error("streaming entry should not auto-expand detail")
	}
}

func TestLogViewEntrySearchAcrossEntries(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "No match here"))
	eb.Append(process.NewLogEntry("14:32:02", "build", process.EventError, "connection refused"))
	eb.Append(process.NewLogEntry("14:32:03", "build", process.EventText, "All good"))
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)

	lv.searchQuery = "refused"
	lv.recomputeMatches()

	if len(lv.matchIndices) != 1 {
		t.Fatalf("expected 1 match, got %d", len(lv.matchIndices))
	}
	if lv.matchIndices[0] != 1 {
		t.Errorf("expected match at entry 1, got %d", lv.matchIndices[0])
	}
}

func TestLogViewCtrlOExpands(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "Summary\nDetail text"))
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)
	lv.cursorEntry = 0

	lv, _ = lv.Update(tea.KeyMsg{Type: tea.KeyCtrlO})
	view := lv.View()
	if !strings.Contains(view, "Detail text") {
		t.Error("expected Ctrl+O to expand entry")
	}
}

func TestLogViewEvictionAdjustsCursorAndExpanded(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(5) // small capacity
	for i := 0; i < 5; i++ {
		eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, fmt.Sprintf("Entry %d", i)))
	}
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)

	// Manually set cursor to entry 3 and expand entry 2
	lv.cursorEntry = 3
	lv.expandedEntries = map[int]bool{2: true}
	lv.follow = false

	// Append 2 more entries, evicting 2 old entries
	eb.Append(process.NewLogEntry("14:32:02", "build", process.EventText, "Entry 5"))
	eb.Append(process.NewLogEntry("14:32:03", "build", process.EventText, "Entry 6"))

	// Simulate receiving a LogLineMsg which triggers adjustForEvictions
	lv, _ = lv.Update(LogLineMsg{RunID: "001"})

	// cursorEntry should shift from 3 to 1 (3-2=1)
	if lv.cursorEntry != 1 {
		t.Errorf("expected cursorEntry=1 after 2 evictions, got %d", lv.cursorEntry)
	}

	// expandedEntries[2] should shift to [0]
	if !lv.expandedEntries[0] {
		t.Error("expected expanded entry to shift from index 2 to index 0")
	}
	if lv.expandedEntries[2] {
		t.Error("expected old expanded index 2 to be removed")
	}
}

func TestUpdateRunMetaPreservesActiveTab(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(100)
	eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, "Entry A"))
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, true)

	// Switch to diff tab
	lv.activeTab = tabDiff

	// UpdateRunMeta should NOT reset the tab
	lv.UpdateRunMeta("test", "feat/new", buf, eb, true)

	if lv.activeTab != tabDiff {
		t.Errorf("expected activeTab=%d (tabDiff), got %d", tabDiff, lv.activeTab)
	}
	// Verify metadata was updated
	if lv.skill != "test" {
		t.Errorf("expected skill='test', got %q", lv.skill)
	}
	if lv.branch != "feat/new" {
		t.Errorf("expected branch='feat/new', got %q", lv.branch)
	}
}

func TestUpdateRunMetaPreservesSearchState(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	buf := process.NewRingBuffer(100)
	buf.Append("error line one")
	buf.Append("ok line two")
	lv.SetRun("001", "build", "main", buf, nil, true)

	// Set up search state
	lv.searchQuery = "error"
	lv.recomputeMatches()
	origMatches := len(lv.matchIndices)

	// UpdateRunMeta should preserve search
	lv.UpdateRunMeta("test", "main", buf, nil, false)

	if lv.searchQuery != "error" {
		t.Errorf("expected searchQuery='error', got %q", lv.searchQuery)
	}
	if len(lv.matchIndices) != origMatches {
		t.Errorf("expected %d matches preserved, got %d", origMatches, len(lv.matchIndices))
	}
}

func TestSetRunResetsActiveTab(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, nil, true)

	// Switch to diff tab
	lv.activeTab = tabDiff

	// SetRun with a new run ID should reset the tab
	lv.SetRun("002", "test", "feat/new", buf, nil, true)

	if lv.activeTab != tabLog {
		t.Errorf("expected activeTab=%d (tabLog), got %d", tabLog, lv.activeTab)
	}
}

func TestLogViewEvictionClampsToZero(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	eb := process.NewEntryBuffer(3)
	for i := 0; i < 3; i++ {
		eb.Append(process.NewLogEntry("14:32:01", "build", process.EventText, fmt.Sprintf("Entry %d", i)))
	}
	buf := process.NewRingBuffer(100)
	lv.SetRun("001", "build", "main", buf, eb, false)

	// Cursor at entry 1, expand entry 0
	lv.cursorEntry = 1
	lv.expandedEntries = map[int]bool{0: true}
	lv.follow = false

	// Evict 3 entries (more than cursor position)
	eb.Append(process.NewLogEntry("14:32:02", "build", process.EventText, "Entry 3"))
	eb.Append(process.NewLogEntry("14:32:03", "build", process.EventText, "Entry 4"))
	eb.Append(process.NewLogEntry("14:32:04", "build", process.EventText, "Entry 5"))

	lv, _ = lv.Update(LogLineMsg{RunID: "001"})

	// cursorEntry should clamp to 0 (1-3 = -2 -> 0)
	if lv.cursorEntry != 0 {
		t.Errorf("expected cursorEntry clamped to 0, got %d", lv.cursorEntry)
	}
	// expandedEntries[0] should be evicted (0-3 = -3, dropped)
	if len(lv.expandedEntries) != 0 {
		t.Errorf("expected all expanded entries evicted, got %d", len(lv.expandedEntries))
	}
}
