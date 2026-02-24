package selection

import (
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

// mockLines implements LinesProvider for testing.
type mockLines struct {
	lines []string
}

func (m *mockLines) Lines() []string { return m.lines }

func newMockLines(n int) *mockLines {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = "line"
	}
	return &mockLines{lines: lines}
}

// --- EnterCopyMode ---

func TestEnterCopyModeSetsCenterAnchor(t *testing.T) {
	var s Selection
	ml := newMockLines(20)
	s.EnterCopyMode(ml, 0, 10)

	if !s.Active() {
		t.Error("expected copy mode active")
	}
	if s.copyAnchor != 5 {
		t.Errorf("expected anchor at center (5), got %d", s.copyAnchor)
	}
	if s.copyCursor != 5 {
		t.Errorf("expected cursor at center (5), got %d", s.copyCursor)
	}
}

func TestEnterCopyModeWithOffset(t *testing.T) {
	var s Selection
	ml := newMockLines(20)
	s.EnterCopyMode(ml, 10, 6)

	expected := 10 + 3 // offset + height/2
	if s.copyAnchor != expected {
		t.Errorf("expected anchor=%d, got %d", expected, s.copyAnchor)
	}
}

func TestEnterCopyModeClampsToLastLine(t *testing.T) {
	var s Selection
	ml := newMockLines(5)
	s.EnterCopyMode(ml, 10, 20) // center would be 10+10=20, clamped to 4

	if s.copyAnchor != 4 {
		t.Errorf("expected anchor clamped to 4, got %d", s.copyAnchor)
	}
}

func TestEnterCopyModeEmptyLinesNoOp(t *testing.T) {
	var s Selection
	ml := &mockLines{lines: nil}
	s.EnterCopyMode(ml, 0, 10)

	if s.Active() {
		t.Error("expected copy mode not active for empty lines")
	}
}

func TestEnterCopyModeClearsMouseSelection(t *testing.T) {
	var s Selection
	s.mouseSelecting = true
	ml := newMockLines(10)
	s.EnterCopyMode(ml, 0, 10)

	if s.MouseActive() {
		t.Error("expected mouse selection cleared when entering copy mode")
	}
}

// --- UpdateCopyMode ---

type timerMsg struct{}

func TestUpdateCopyModeEscExits(t *testing.T) {
	var s Selection
	ml := newMockLines(10)
	s.EnterCopyMode(ml, 0, 10)

	vp := viewport.New(80, 10)
	gPending := false
	msg := tea.KeyMsg{Type: tea.KeyEsc}

	yank, _ := s.UpdateCopyMode(msg, ml, &vp, &gPending, timerMsg{})

	if s.Active() {
		t.Error("expected copy mode exited after esc")
	}
	if yank != "" {
		t.Error("expected no yank text on esc")
	}
}

func TestUpdateCopyModeYankReturnsText(t *testing.T) {
	var s Selection
	ml := &mockLines{lines: []string{"alpha", "beta", "gamma"}}
	s.EnterCopyMode(ml, 0, 3) // anchor=cursor=1

	// Move cursor down to select lines 1-2
	vp := viewport.New(80, 3)
	gPending := false
	s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, ml, &vp, &gPending, timerMsg{})

	yank, _ := s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}, ml, &vp, &gPending, timerMsg{})

	if yank != "beta\ngamma" {
		t.Errorf("expected 'beta\\ngamma', got %q", yank)
	}
	if s.Active() {
		t.Error("expected copy mode exited after yank")
	}
}

func TestUpdateCopyModeJKMovement(t *testing.T) {
	var s Selection
	ml := newMockLines(10)
	s.EnterCopyMode(ml, 0, 10) // anchor=cursor=5

	vp := viewport.New(80, 10)
	gPending := false

	// Move down
	s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, ml, &vp, &gPending, timerMsg{})
	if s.copyCursor != 6 {
		t.Errorf("expected cursor=6 after j, got %d", s.copyCursor)
	}

	// Move up
	s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, ml, &vp, &gPending, timerMsg{})
	if s.copyCursor != 5 {
		t.Errorf("expected cursor=5 after k, got %d", s.copyCursor)
	}
}

func TestUpdateCopyModeJClampsAtBottom(t *testing.T) {
	var s Selection
	ml := newMockLines(3)
	s.EnterCopyMode(ml, 0, 3) // cursor=1
	s.copyCursor = 2

	vp := viewport.New(80, 3)
	gPending := false
	s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}, ml, &vp, &gPending, timerMsg{})

	if s.copyCursor != 2 {
		t.Errorf("expected cursor clamped at 2, got %d", s.copyCursor)
	}
}

func TestUpdateCopyModeKClampsAtTop(t *testing.T) {
	var s Selection
	ml := newMockLines(3)
	s.EnterCopyMode(ml, 0, 3)
	s.copyCursor = 0

	vp := viewport.New(80, 3)
	gPending := false
	s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}}, ml, &vp, &gPending, timerMsg{})

	if s.copyCursor != 0 {
		t.Errorf("expected cursor clamped at 0, got %d", s.copyCursor)
	}
}

func TestUpdateCopyModeGJumpsToBottom(t *testing.T) {
	var s Selection
	ml := newMockLines(20)
	s.EnterCopyMode(ml, 0, 10)

	vp := viewport.New(80, 10)
	gPending := false
	s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}}, ml, &vp, &gPending, timerMsg{})

	if s.copyCursor != 19 {
		t.Errorf("expected cursor=19 after G, got %d", s.copyCursor)
	}
}

func TestUpdateCopyModeGGJumpsToTop(t *testing.T) {
	var s Selection
	ml := newMockLines(20)
	s.EnterCopyMode(ml, 0, 10)

	vp := viewport.New(80, 10)
	gPending := false

	// First g starts timer
	_, cmd := s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}, ml, &vp, &gPending, timerMsg{})
	if !gPending {
		t.Error("expected gPending=true after first g")
	}
	if cmd == nil {
		t.Error("expected timer cmd after first g")
	}

	// Second g fires
	s.UpdateCopyMode(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}}, ml, &vp, &gPending, timerMsg{})
	if gPending {
		t.Error("expected gPending=false after gg")
	}
	if s.copyCursor != 0 {
		t.Errorf("expected cursor=0 after gg, got %d", s.copyCursor)
	}
}

// --- YankSelection ---

func TestYankSelectionNormalRange(t *testing.T) {
	var s Selection
	s.copyAnchor = 1
	s.copyCursor = 3
	ml := &mockLines{lines: []string{"a", "b", "c", "d", "e"}}

	result := s.YankSelection(ml)
	if result != "b\nc\nd" {
		t.Errorf("expected 'b\\nc\\nd', got %q", result)
	}
}

func TestYankSelectionReversedRange(t *testing.T) {
	var s Selection
	s.copyAnchor = 3
	s.copyCursor = 1
	ml := &mockLines{lines: []string{"a", "b", "c", "d", "e"}}

	result := s.YankSelection(ml)
	if result != "b\nc\nd" {
		t.Errorf("expected 'b\\nc\\nd' with reversed range, got %q", result)
	}
}

func TestYankSelectionSingleLine(t *testing.T) {
	var s Selection
	s.copyAnchor = 2
	s.copyCursor = 2
	ml := &mockLines{lines: []string{"a", "b", "c"}}

	result := s.YankSelection(ml)
	if result != "c" {
		t.Errorf("expected 'c', got %q", result)
	}
}

func TestYankSelectionEmptyLines(t *testing.T) {
	var s Selection
	ml := &mockLines{lines: nil}
	result := s.YankSelection(ml)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestYankSelectionClampsOutOfBounds(t *testing.T) {
	var s Selection
	s.copyAnchor = -2
	s.copyCursor = 10
	ml := &mockLines{lines: []string{"a", "b", "c"}}

	result := s.YankSelection(ml)
	if result != "a\nb\nc" {
		t.Errorf("expected 'a\\nb\\nc', got %q", result)
	}
}

// --- CopySelectionRange ---

func TestCopySelectionRangeNormalized(t *testing.T) {
	var s Selection
	s.copyAnchor = 5
	s.copyCursor = 2

	start, end := s.CopySelectionRange()
	if start != 2 || end != 5 {
		t.Errorf("expected (2,5), got (%d,%d)", start, end)
	}
}

func TestCopySelectionRangeAlreadyOrdered(t *testing.T) {
	var s Selection
	s.copyAnchor = 2
	s.copyCursor = 5

	start, end := s.CopySelectionRange()
	if start != 2 || end != 5 {
		t.Errorf("expected (2,5), got (%d,%d)", start, end)
	}
}

// --- Mouse selection ---

func TestStartMouseSetsCoordinates(t *testing.T) {
	var s Selection
	s.copyMode = true // should be cleared

	s.StartMouse(5, 3, 10)

	if s.Active() {
		t.Error("expected copy mode cleared on mouse start")
	}
	if !s.MouseActive() {
		t.Error("expected mouse selecting active")
	}
	// bufLine = 10 + (3-1) = 12, col = 5-1 = 4
	if s.mouseAnchorLine != 12 {
		t.Errorf("expected anchorLine=12, got %d", s.mouseAnchorLine)
	}
	if s.mouseAnchorCol != 4 {
		t.Errorf("expected anchorCol=4, got %d", s.mouseAnchorCol)
	}
	if s.mouseCurrentLine != 12 {
		t.Errorf("expected currentLine=12, got %d", s.mouseCurrentLine)
	}
	if s.mouseCurrentCol != 4 {
		t.Errorf("expected currentCol=4, got %d", s.mouseCurrentCol)
	}
}

func TestStartMouseClampsNegative(t *testing.T) {
	var s Selection
	s.StartMouse(0, 0, 0)

	// bufLine = 0 + (0-1) = -1 -> 0, col = 0-1 = -1 -> 0
	if s.mouseAnchorLine != 0 {
		t.Errorf("expected anchorLine clamped to 0, got %d", s.mouseAnchorLine)
	}
	if s.mouseAnchorCol != 0 {
		t.Errorf("expected anchorCol clamped to 0, got %d", s.mouseAnchorCol)
	}
}

func TestExtendMouseUpdatesEndpoint(t *testing.T) {
	var s Selection
	s.StartMouse(5, 3, 0) // anchor: line=2, col=4
	s.ExtendMouse(10, 5, 0)

	if s.mouseCurrentLine != 4 {
		t.Errorf("expected currentLine=4, got %d", s.mouseCurrentLine)
	}
	if s.mouseCurrentCol != 9 {
		t.Errorf("expected currentCol=9, got %d", s.mouseCurrentCol)
	}
}

func TestExtendMouseNoOpWhenNotSelecting(t *testing.T) {
	var s Selection
	// Don't start mouse selection
	s.ExtendMouse(10, 5, 0)

	if s.mouseCurrentLine != 0 || s.mouseCurrentCol != 0 {
		t.Error("expected no change when not selecting")
	}
}

func TestFinalizeMouseReturnsCoordsAndClears(t *testing.T) {
	var s Selection
	s.StartMouse(5, 3, 0) // anchor: line=2, col=4
	s.ExtendMouse(10, 5, 0)

	startLine, startCol, endLine, endCol, singleClick := s.FinalizeMouse(10, 5, 0)

	if singleClick {
		t.Error("expected not single click for drag")
	}
	if s.MouseActive() {
		t.Error("expected mouse selection cleared after finalize")
	}
	// Normalized: (2,4) to (4,9)
	if startLine != 2 || startCol != 4 {
		t.Errorf("expected start (2,4), got (%d,%d)", startLine, startCol)
	}
	if endLine != 4 || endCol != 9 {
		t.Errorf("expected end (4,9), got (%d,%d)", endLine, endCol)
	}
}

func TestFinalizeMouseSingleClick(t *testing.T) {
	var s Selection
	s.StartMouse(5, 3, 0)

	_, _, _, _, singleClick := s.FinalizeMouse(5, 3, 0) // same position

	if !singleClick {
		t.Error("expected single click when start equals end")
	}
}

func TestFinalizeMouseReversedDrag(t *testing.T) {
	var s Selection
	s.StartMouse(10, 5, 0) // anchor: line=4, col=9
	s.ExtendMouse(5, 3, 0)

	startLine, startCol, endLine, endCol, _ := s.FinalizeMouse(5, 3, 0)

	// Normalized: start should be before end
	if startLine != 2 || startCol != 4 {
		t.Errorf("expected start (2,4), got (%d,%d)", startLine, startCol)
	}
	if endLine != 4 || endCol != 9 {
		t.Errorf("expected end (4,9), got (%d,%d)", endLine, endCol)
	}
}

func TestCancelMouseClearsState(t *testing.T) {
	var s Selection
	s.StartMouse(5, 3, 0)
	s.CancelMouse()

	if s.MouseActive() {
		t.Error("expected mouse selection cleared after cancel")
	}
}

// --- NormalizedMouseSelection ---

func TestNormalizedMouseSelectionAlreadyOrdered(t *testing.T) {
	var s Selection
	s.mouseAnchorLine = 2
	s.mouseAnchorCol = 5
	s.mouseCurrentLine = 4
	s.mouseCurrentCol = 10

	sl, sc, el, ec := s.NormalizedMouseSelection()
	if sl != 2 || sc != 5 || el != 4 || ec != 10 {
		t.Errorf("expected (2,5,4,10), got (%d,%d,%d,%d)", sl, sc, el, ec)
	}
}

func TestNormalizedMouseSelectionReversed(t *testing.T) {
	var s Selection
	s.mouseAnchorLine = 4
	s.mouseAnchorCol = 10
	s.mouseCurrentLine = 2
	s.mouseCurrentCol = 5

	sl, sc, el, ec := s.NormalizedMouseSelection()
	if sl != 2 || sc != 5 || el != 4 || ec != 10 {
		t.Errorf("expected (2,5,4,10), got (%d,%d,%d,%d)", sl, sc, el, ec)
	}
}

func TestNormalizedMouseSelectionSameLineReversed(t *testing.T) {
	var s Selection
	s.mouseAnchorLine = 3
	s.mouseAnchorCol = 15
	s.mouseCurrentLine = 3
	s.mouseCurrentCol = 5

	sl, sc, el, ec := s.NormalizedMouseSelection()
	if sl != 3 || sc != 5 || el != 3 || ec != 15 {
		t.Errorf("expected (3,5,3,15), got (%d,%d,%d,%d)", sl, sc, el, ec)
	}
}

// --- Reset ---

func TestResetClearsAllState(t *testing.T) {
	var s Selection
	ml := newMockLines(10)
	s.EnterCopyMode(ml, 0, 10)
	s.mouseSelecting = true
	s.mouseAnchorLine = 5

	s.Reset()

	if s.Active() || s.MouseActive() {
		t.Error("expected all state cleared after reset")
	}
	if s.copyAnchor != 0 || s.copyCursor != 0 {
		t.Error("expected copy indices zeroed")
	}
	if s.mouseAnchorLine != 0 {
		t.Error("expected mouse coords zeroed")
	}
}
