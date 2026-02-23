package panels

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/process"
)

func TestLogViewTitle(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(60, 20)
	lv.SetRun("001", "build", "feat/auth", nil, true)

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
	lv.SetRun("001", "build", "main", buf, true)
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
	lv.SetRun("001", "build", "main", buf, false)

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
	lv.SetRun("001", "build", "main", buf, false)
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
	lv.SetRun("001", "build", "main", buf, false)
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

	lv.SetRun("002", "test", "fix/bug", nil, false)

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
