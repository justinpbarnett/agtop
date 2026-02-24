package selection

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const gTimeout = 300 * time.Millisecond

// LinesProvider abstracts over the different line sources used by each panel.
type LinesProvider interface {
	Lines() []string
}

// Selection manages copy/yank and mouse selection state, shared between panels.
type Selection struct {
	// Line-level copy mode
	copyMode   bool
	copyAnchor int
	copyCursor int

	// Character-level mouse selection
	mouseSelecting   bool
	mouseAnchorLine  int
	mouseAnchorCol   int
	mouseCurrentLine int
	mouseCurrentCol  int
}

// Active returns true if keyboard copy mode is active.
func (s *Selection) Active() bool { return s.copyMode }

// MouseActive returns true if a mouse drag selection is in progress.
func (s *Selection) MouseActive() bool { return s.mouseSelecting }

// Reset clears all selection state.
func (s *Selection) Reset() { *s = Selection{} }

// EnterCopyMode activates keyboard copy mode anchored at the viewport center.
// Does nothing if lines is empty.
func (s *Selection) EnterCopyMode(lines LinesProvider, viewportYOffset, viewportHeight int) {
	all := lines.Lines()
	if len(all) == 0 {
		return
	}
	centerLine := viewportYOffset + viewportHeight/2
	if centerLine >= len(all) {
		centerLine = len(all) - 1
	}
	if centerLine < 0 {
		centerLine = 0
	}
	s.copyMode = true
	s.mouseSelecting = false
	s.copyAnchor = centerLine
	s.copyCursor = centerLine
}

// UpdateCopyMode handles keyboard input during copy mode.
// Returns yankText when "y" is pressed (caller should emit YankMsg).
// Returns a tea.Cmd for the "g" double-tap timer when needed.
// gPending is a pointer to the caller's pending flag; gTimerMsg is the message emitted on expiry.
func (s *Selection) UpdateCopyMode(
	msg tea.KeyMsg,
	lines LinesProvider,
	vp *viewport.Model,
	gPending *bool,
	gTimerMsg tea.Msg,
) (yankText string, cmd tea.Cmd) {
	lineCount := len(lines.Lines())
	switch msg.String() {
	case "esc":
		s.copyMode = false
	case "y":
		yankText = s.YankSelection(lines)
		s.copyMode = false
	case "j", "down":
		if s.copyCursor < lineCount-1 {
			s.copyCursor++
			if s.copyCursor >= vp.YOffset+vp.Height {
				vp.SetYOffset(s.copyCursor - vp.Height + 1)
			}
		}
	case "k", "up":
		if s.copyCursor > 0 {
			s.copyCursor--
			if s.copyCursor < vp.YOffset {
				vp.SetYOffset(s.copyCursor)
			}
		}
	case "G":
		s.copyCursor = lineCount - 1
		vp.GotoBottom()
	case "g":
		if *gPending {
			*gPending = false
			s.copyCursor = 0
			vp.GotoTop()
		} else {
			*gPending = true
			timerMsg := gTimerMsg
			cmd = tea.Tick(gTimeout, func(time.Time) tea.Msg {
				return timerMsg
			})
		}
	}
	return
}

// YankSelection returns the selected lines joined by newlines.
func (s *Selection) YankSelection(lines LinesProvider) string {
	all := lines.Lines()
	if len(all) == 0 {
		return ""
	}
	start, end := s.CopySelectionRange()
	if start < 0 {
		start = 0
	}
	if end >= len(all) {
		end = len(all) - 1
	}
	return strings.Join(all[start:end+1], "\n")
}

// CopySelectionRange returns normalized (start <= end) line indices of the selection.
func (s *Selection) CopySelectionRange() (start, end int) {
	start = s.copyAnchor
	end = s.copyCursor
	if start > end {
		start, end = end, start
	}
	return
}

// StartMouse begins a character-level mouse drag selection.
func (s *Selection) StartMouse(relX, relY, viewportYOffset int) {
	s.copyMode = false
	bufLine := viewportYOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	s.mouseSelecting = true
	s.mouseAnchorLine = bufLine
	s.mouseAnchorCol = col
	s.mouseCurrentLine = bufLine
	s.mouseCurrentCol = col
}

// ExtendMouse updates the selection endpoint during a drag.
func (s *Selection) ExtendMouse(relX, relY, viewportYOffset int) {
	if !s.mouseSelecting {
		return
	}
	bufLine := viewportYOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	s.mouseCurrentLine = bufLine
	s.mouseCurrentCol = col
}

// FinalizeMouse ends a mouse drag and returns the normalized coordinates plus
// whether this was a single-click (no drag). Callers handle text extraction.
func (s *Selection) FinalizeMouse(relX, relY, viewportYOffset int) (startLine, startCol, endLine, endCol int, singleClick bool) {
	bufLine := viewportYOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	s.mouseCurrentLine = bufLine
	s.mouseCurrentCol = col
	s.mouseSelecting = false
	if s.mouseAnchorLine == s.mouseCurrentLine && s.mouseAnchorCol == s.mouseCurrentCol {
		singleClick = true
		return
	}
	startLine, startCol, endLine, endCol = s.NormalizedMouseSelection()
	return
}

// CancelMouse clears mouse selection state without extracting text.
func (s *Selection) CancelMouse() {
	s.mouseSelecting = false
}

// NormalizedMouseSelection returns the mouse selection with start before end.
func (s *Selection) NormalizedMouseSelection() (startLine, startCol, endLine, endCol int) {
	startLine, startCol = s.mouseAnchorLine, s.mouseAnchorCol
	endLine, endCol = s.mouseCurrentLine, s.mouseCurrentCol
	if startLine > endLine || (startLine == endLine && startCol > endCol) {
		startLine, startCol, endLine, endCol = endLine, endCol, startLine, startCol
	}
	return
}
