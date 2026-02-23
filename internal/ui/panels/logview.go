package panels

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

const gTimeout = 300 * time.Millisecond

// Log view tab indices.
const (
	tabLog  = 0
	tabDiff = 1
)

// logLineRe matches log lines like "[14:32:01 route] message"
var logLineRe = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2})\s+(\S+)\]\s*(.*)$`)

// GTimerExpiredMsg is sent when the gg double-tap window expires.
type GTimerExpiredMsg struct{}

type LogView struct {
	viewport    viewport.Model
	width       int
	height      int
	buffer      *process.RingBuffer
	runID       string
	skill       string
	branch      string
	active      bool
	follow      bool
	focused     bool
	gPending    bool
	scrollSpeed int

	// Tab state
	activeTab int
	diffView  DiffView

	// Search state
	searching    bool
	searchInput  textinput.Model
	searchQuery  string
	matchIndices []int
	currentMatch int

	// Copy mode state
	copyMode   bool
	copyAnchor int // buffer line index where selection started
	copyCursor int // buffer line index of current cursor

	// Mouse selection state (character-level)
	mouseSelecting  bool
	mouseAnchorLine int
	mouseAnchorCol  int
	mouseCurrentLine int
	mouseCurrentCol  int
}

func NewLogView() LogView {
	vp := viewport.New(0, 0)
	ti := textinput.New()
	ti.Prompt = "/"
	ti.Placeholder = "Search..."
	ti.CharLimit = 256
	return LogView{viewport: vp, follow: true, searchInput: ti, scrollSpeed: 3, diffView: NewDiffView()}
}

// ActiveTab returns the currently selected tab index.
func (l LogView) ActiveTab() int {
	return l.activeTab
}

func (l LogView) Update(msg tea.Msg) (LogView, tea.Cmd) {
	switch msg := msg.(type) {
	case LogLineMsg:
		if msg.RunID == l.runID && l.buffer != nil {
			atBottom := l.viewport.AtBottom()
			l.viewport.SetContent(l.renderContent())
			if atBottom || l.follow {
				l.viewport.GotoBottom()
			}
			if l.searchQuery != "" {
				l.recomputeMatches()
			}
			return l, nil
		}
	case DiffGTimerExpiredMsg:
		if l.activeTab == tabDiff {
			var cmd tea.Cmd
			l.diffView, cmd = l.diffView.Update(msg)
			return l, cmd
		}
		return l, nil
	case GTimerExpiredMsg:
		l.gPending = false
		return l, nil
	case tea.KeyMsg:
		// On diff tab, delegate keys to diffView
		if l.activeTab == tabDiff {
			switch msg.String() {
			case "h", "left":
				l.activeTab = tabLog
				l.updateDiffFocus()
				return l, nil
			}
			var cmd tea.Cmd
			l.diffView, cmd = l.diffView.Update(msg)
			return l, cmd
		}

		// Log tab: route keys to search input when in search input mode
		if l.searching {
			return l.updateSearch(msg)
		}

		// Copy mode key handling
		if l.copyMode {
			return l.updateCopyMode(msg)
		}

		// Search query active (not typing) — handle n/N navigation
		if l.searchQuery != "" {
			switch msg.String() {
			case "n":
				l.nextMatch()
				return l, nil
			case "N":
				l.prevMatch()
				return l, nil
			case "/":
				l.searching = true
				l.searchInput.SetValue(l.searchQuery)
				l.searchInput.Focus()
				l.resizeViewport()
				return l, textinput.Blink
			}
		}

		switch msg.String() {
		case "l", "right":
			l.activeTab = tabDiff
			l.updateDiffFocus()
			return l, nil
		case "G":
			l.follow = true
			l.viewport.GotoBottom()
			return l, nil
		case "g":
			if l.gPending {
				// Second g — jump to top
				l.gPending = false
				l.follow = false
				l.viewport.GotoTop()
				return l, nil
			}
			// First g — start timer
			l.gPending = true
			l.follow = false
			return l, tea.Tick(gTimeout, func(time.Time) tea.Msg {
				return GTimerExpiredMsg{}
			})
		case "/":
			l.searching = true
			l.follow = false
			l.searchInput.SetValue("")
			l.searchInput.Focus()
			l.resizeViewport()
			return l, textinput.Blink
		case "y":
			l.enterCopyMode()
			return l, nil
		case "j", "down":
			l.follow = false
			step := l.scrollSpeed
			if step <= 0 {
				step = 1
			}
			l.viewport.SetYOffset(l.viewport.YOffset + step)
			return l, nil
		case "k", "up":
			l.follow = false
			step := l.scrollSpeed
			if step <= 0 {
				step = 1
			}
			offset := l.viewport.YOffset - step
			if offset < 0 {
				offset = 0
			}
			l.viewport.SetYOffset(offset)
			return l, nil
		}
	}

	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return l, cmd
}

func (l *LogView) updateSearch(msg tea.KeyMsg) (LogView, tea.Cmd) {
	switch msg.String() {
	case "esc":
		l.searching = false
		l.searchQuery = ""
		l.matchIndices = nil
		l.currentMatch = 0
		l.searchInput.Blur()
		l.resizeViewport()
		l.refreshContent()
		return *l, nil
	case "enter":
		l.searching = false
		l.searchQuery = l.searchInput.Value()
		l.searchInput.Blur()
		l.resizeViewport()
		l.recomputeMatches()
		if len(l.matchIndices) > 0 {
			l.currentMatch = 0
			l.jumpToMatch()
		}
		l.refreshContent()
		return *l, nil
	}

	var cmd tea.Cmd
	l.searchInput, cmd = l.searchInput.Update(msg)
	// Live-update matches as user types
	l.searchQuery = l.searchInput.Value()
	l.recomputeMatches()
	l.refreshContent()
	return *l, cmd
}

func (l LogView) View() string {
	// Build tab-aware title
	logLabel := "Log"
	if l.skill != "" || l.branch != "" {
		parts := []string{}
		if l.skill != "" {
			parts = append(parts, l.skill)
		}
		if l.branch != "" {
			parts = append(parts, l.branch)
		}
		logLabel = fmt.Sprintf("Log: %s", strings.Join(parts, " — "))
	}
	diffLabel := "Diff"

	var title string
	if l.activeTab == tabLog {
		title = "[3] " + styles.TitleStyle.Render(logLabel) +
			styles.TextDimStyle.Render(" │ ") +
			styles.TextDimStyle.Render(diffLabel)
	} else {
		title = "[3] " + styles.TextDimStyle.Render(logLabel) +
			styles.TextDimStyle.Render(" │ ") +
			styles.TitleStyle.Render(diffLabel)
	}

	var keybinds []border.Keybind
	var content string

	if l.activeTab == tabLog {
		if l.focused {
			if l.copyMode {
				keybinds = []border.Keybind{
					{Key: "y", Label: "ank"},
					{Key: "j", Label: "/k select"},
					{Key: "Esc", Label: " cancel"},
				}
			} else {
				keybinds = []border.Keybind{
					{Key: "y", Label: "ank/copy"},
					{Key: "G", Label: "bottom"},
					{Key: "g", Label: "g top"},
					{Key: "/", Label: "search"},
				}
				if !l.viewport.AtBottom() && !l.follow {
					keybinds = append(keybinds, border.Keybind{Key: "↓", Label: " new output"})
				}
			}
		}

		content = l.viewport.View()

		// Append copy mode status, search bar, or match status below the viewport
		if l.copyMode {
			selStart, selEnd := l.copySelectionRange()
			count := selEnd - selStart + 1
			status := styles.TextSecondaryStyle.Render(
				fmt.Sprintf("  VISUAL: %d line(s) selected", count),
			) + styles.TextDimStyle.Render(" (y yank, Esc cancel)")
			content += "\n" + status
		} else if l.searching {
			content += "\n" + l.searchInput.View()
		} else if l.searchQuery != "" {
			total := len(l.matchIndices)
			var status string
			if total == 0 {
				status = styles.TextDimStyle.Render("  No matches")
			} else {
				status = styles.TextSecondaryStyle.Render(
					fmt.Sprintf("  Match %d/%d", l.currentMatch+1, total),
				) + styles.TextDimStyle.Render(" (n/N navigate, / edit, Esc clear)")
			}
			content += "\n" + status
		}
	} else {
		content = l.diffView.Content()
		if l.focused {
			keybinds = l.diffView.Keybinds()
		}
	}

	return border.RenderPanel(title, content, keybinds, l.width, l.height, l.focused)
}

func (l *LogView) SetSize(w, h int) {
	l.width = w
	l.height = h
	l.resizeViewport()
	l.refreshContent()
	// Propagate inner dimensions to diffView
	innerW := w - 2
	innerH := h - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	l.diffView.SetSize(innerW, innerH)
}

func (l *LogView) SetFocused(focused bool) {
	l.focused = focused
	l.updateDiffFocus()
}

func (l *LogView) SetScrollSpeed(speed int) {
	if speed > 0 {
		l.scrollSpeed = speed
	}
}

// ConsumesKeys reports whether the log view is in a mode that should
// consume all key events (search input or active search query navigation).
// Returns false on the diff tab since search doesn't apply there.
func (l LogView) ConsumesKeys() bool {
	if l.activeTab == tabDiff {
		return l.diffView.ConsumesKeys()
	}
	return l.searching || l.searchQuery != "" || l.copyMode
}

func (l *LogView) SetRun(runID, skill, branch string, buf *process.RingBuffer, active bool) {
	l.runID = runID
	l.skill = skill
	l.branch = branch
	l.buffer = buf
	l.active = active
	l.follow = true
	l.searchQuery = ""
	l.matchIndices = nil
	l.searching = false
	l.copyMode = false
	l.mouseSelecting = false
	l.activeTab = tabLog
	l.updateDiffFocus()
	l.refreshContent()
}

// Diff proxy methods — called by the app to pass diff data into the embedded DiffView.

func (l *LogView) SetDiff(diff, stat string)  { l.diffView.SetDiff(diff, stat) }
func (l *LogView) SetDiffLoading()             { l.diffView.SetLoading() }
func (l *LogView) SetDiffError(err string)     { l.diffView.SetError(err) }
func (l *LogView) SetDiffEmpty()               { l.diffView.SetEmpty() }
func (l *LogView) SetDiffNoBranch()            { l.diffView.SetNoBranch() }
func (l *LogView) SetDiffWaiting()             { l.diffView.SetWaiting() }

func (l *LogView) updateDiffFocus() {
	l.diffView.SetFocused(l.focused && l.activeTab == tabDiff)
}

// resizeViewport recalculates the viewport inner dimensions, accounting for
// the search bar when it's visible.
func (l *LogView) resizeViewport() {
	innerW := l.width - 2
	innerH := l.height - 2
	if l.searching || l.searchQuery != "" || l.copyMode {
		innerH-- // Reserve 1 row for search bar / status / copy mode
	}
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	l.viewport.Width = innerW
	l.viewport.Height = innerH
}

// refreshContent re-sets the viewport content from the current buffer,
// ensuring it's wrapped at the current viewport width.
func (l *LogView) refreshContent() {
	l.viewport.SetContent(l.renderContent())
	if l.follow {
		l.viewport.GotoBottom()
	}
}

// renderContentBase builds the styled log content without selection highlighting.
func (l *LogView) renderContentBase() string {
	var raw string
	if l.buffer != nil {
		lines := l.buffer.Lines()
		if len(lines) > 0 {
			raw = strings.Join(lines, "\n")
		} else {
			raw = "Waiting for output..."
		}
	} else {
		raw = "No run selected"
	}
	return formatLogContent(raw, l.active, l.searchQuery, l.matchIndices, l.currentMatch)
}

// renderContent builds the styled log content, including search and selection highlights.
func (l *LogView) renderContent() string {
	content := l.renderContentBase()

	if l.copyMode {
		selStart, selEnd := l.copySelectionRange()
		content = applySelectionHighlight(content, selStart, selEnd)
	} else if l.mouseSelecting {
		sl, sc, el, ec := l.normalizedMouseSelection()
		content = applyCharSelectionHighlight(content, sl, sc, el, ec)
	}

	return content
}

func (l *LogView) recomputeMatches() {
	l.matchIndices = nil
	l.currentMatch = 0
	if l.searchQuery == "" {
		return
	}
	query := strings.ToLower(l.searchQuery)
	var lines []string
	if l.buffer != nil {
		lines = l.buffer.Lines()
	}
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			l.matchIndices = append(l.matchIndices, i)
		}
	}
}

func (l *LogView) nextMatch() {
	if len(l.matchIndices) == 0 {
		return
	}
	l.currentMatch = (l.currentMatch + 1) % len(l.matchIndices)
	l.jumpToMatch()
}

func (l *LogView) prevMatch() {
	if len(l.matchIndices) == 0 {
		return
	}
	l.currentMatch = (l.currentMatch - 1 + len(l.matchIndices)) % len(l.matchIndices)
	l.jumpToMatch()
}

func (l *LogView) jumpToMatch() {
	if len(l.matchIndices) == 0 {
		return
	}
	lineIdx := l.matchIndices[l.currentMatch]
	l.follow = false
	l.viewport.SetYOffset(lineIdx)
	l.refreshContent()
}

func (l *LogView) enterCopyMode() {
	if l.buffer == nil {
		return
	}
	lines := l.buffer.Lines()
	if len(lines) == 0 {
		return
	}
	// Anchor at the center of the visible viewport
	centerLine := l.viewport.YOffset + l.viewport.Height/2
	if centerLine >= len(lines) {
		centerLine = len(lines) - 1
	}
	if centerLine < 0 {
		centerLine = 0
	}
	l.copyMode = true
	l.mouseSelecting = false
	l.copyAnchor = centerLine
	l.copyCursor = centerLine
	l.follow = false
	l.refreshContent()
}

func (l *LogView) updateCopyMode(msg tea.KeyMsg) (LogView, tea.Cmd) {
	lineCount := 0
	if l.buffer != nil {
		lineCount = len(l.buffer.Lines())
	}

	switch msg.String() {
	case "esc":
		l.copyMode = false
		l.refreshContent()
		return *l, nil
	case "y":
		// Yank the selected lines
		text := l.yankSelection()
		l.copyMode = false
		l.refreshContent()
		if text != "" {
			return *l, func() tea.Msg { return YankMsg{Text: text} }
		}
		return *l, nil
	case "j", "down":
		if l.copyCursor < lineCount-1 {
			l.copyCursor++
			// Scroll viewport to keep cursor visible
			if l.copyCursor >= l.viewport.YOffset+l.viewport.Height {
				l.viewport.SetYOffset(l.copyCursor - l.viewport.Height + 1)
			}
			l.refreshContent()
		}
		return *l, nil
	case "k", "up":
		if l.copyCursor > 0 {
			l.copyCursor--
			// Scroll viewport to keep cursor visible
			if l.copyCursor < l.viewport.YOffset {
				l.viewport.SetYOffset(l.copyCursor)
			}
			l.refreshContent()
		}
		return *l, nil
	case "G":
		l.copyCursor = lineCount - 1
		l.viewport.GotoBottom()
		l.refreshContent()
		return *l, nil
	case "g":
		if l.gPending {
			l.gPending = false
			l.copyCursor = 0
			l.viewport.GotoTop()
			l.refreshContent()
			return *l, nil
		}
		l.gPending = true
		return *l, tea.Tick(gTimeout, func(time.Time) tea.Msg {
			return GTimerExpiredMsg{}
		})
	}
	return *l, nil
}

func (l *LogView) yankSelection() string {
	if l.buffer == nil {
		return ""
	}
	lines := l.buffer.Lines()
	if len(lines) == 0 {
		return ""
	}
	start := l.copyAnchor
	end := l.copyCursor
	if start > end {
		start, end = end, start
	}
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	return strings.Join(lines[start:end+1], "\n")
}

func (l *LogView) copySelectionRange() (int, int) {
	start := l.copyAnchor
	end := l.copyCursor
	if start > end {
		start, end = end, start
	}
	return start, end
}

// StartMouseSelection begins a mouse drag selection at the given panel-relative coordinates.
func (l *LogView) StartMouseSelection(relX, relY int) {
	if l.activeTab == tabDiff {
		l.diffView.StartMouseSelection(relX, relY)
		return
	}
	l.copyMode = false
	bufLine := l.viewport.YOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1 // account for left border
	if col < 0 {
		col = 0
	}
	l.mouseSelecting = true
	l.mouseAnchorLine = bufLine
	l.mouseAnchorCol = col
	l.mouseCurrentLine = bufLine
	l.mouseCurrentCol = col
	l.follow = false
	l.refreshContent()
}

// ExtendMouseSelection updates the cursor position during a mouse drag.
func (l *LogView) ExtendMouseSelection(relX, relY int) {
	if l.activeTab == tabDiff {
		l.diffView.ExtendMouseSelection(relX, relY)
		return
	}
	if !l.mouseSelecting {
		return
	}
	bufLine := l.viewport.YOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	l.mouseCurrentLine = bufLine
	l.mouseCurrentCol = col
	l.refreshContent()
}

// FinalizeMouseSelection ends the mouse drag and returns the selected text.
// Returns empty string for single-click (no drag).
func (l *LogView) FinalizeMouseSelection(relX, relY int) string {
	if l.activeTab == tabDiff {
		return l.diffView.FinalizeMouseSelection(relX, relY)
	}
	if !l.mouseSelecting {
		return ""
	}
	l.mouseSelecting = false
	bufLine := l.viewport.YOffset + (relY - 1)
	if bufLine < 0 {
		bufLine = 0
	}
	col := relX - 1
	if col < 0 {
		col = 0
	}
	l.mouseCurrentLine = bufLine
	l.mouseCurrentCol = col

	// Single click (same position) — no copy
	if l.mouseAnchorLine == l.mouseCurrentLine && l.mouseAnchorCol == l.mouseCurrentCol {
		l.refreshContent()
		return ""
	}

	content := l.renderContentBase()
	sl, sc, el, ec := l.normalizedMouseSelection()
	text := extractCharSelection(content, sl, sc, el, ec)
	l.refreshContent()
	return text
}

// CancelMouseSelection clears mouse selection state without copying.
func (l *LogView) CancelMouseSelection() {
	if l.activeTab == tabDiff {
		l.diffView.CancelMouseSelection()
		return
	}
	l.mouseSelecting = false
	l.refreshContent()
}

// normalizedMouseSelection returns the mouse selection with start before end.
func (l *LogView) normalizedMouseSelection() (startLine, startCol, endLine, endCol int) {
	startLine, startCol = l.mouseAnchorLine, l.mouseAnchorCol
	endLine, endCol = l.mouseCurrentLine, l.mouseCurrentCol
	if startLine > endLine || (startLine == endLine && startCol > endCol) {
		startLine, startCol, endLine, endCol = endLine, endCol, startLine, startCol
	}
	return
}

// formatLogContent styles log lines: timestamps in TextDim, tool names in TextSecondary,
// and appends a streaming cursor if active. Supports search highlighting.
func formatLogContent(content string, active bool, searchQuery string, matchIndices []int, currentMatch int) string {
	if content == "" {
		return content
	}

	tsStyle := styles.TextDimStyle
	skillStyle := styles.TextSecondaryStyle
	msgStyle := lipgloss.NewStyle().Foreground(styles.TextPrimary)

	// Build a set of matching line indices for quick lookup
	matchSet := make(map[int]bool, len(matchIndices))
	for _, idx := range matchIndices {
		matchSet[idx] = true
	}
	var currentMatchLine int
	if len(matchIndices) > 0 && currentMatch >= 0 && currentMatch < len(matchIndices) {
		currentMatchLine = matchIndices[currentMatch]
	} else {
		currentMatchLine = -1
	}

	lines := strings.Split(content, "\n")
	styled := make([]string, 0, len(lines))

	for i, line := range lines {
		m := logLineRe.FindStringSubmatch(line)
		if m != nil {
			timestamp := m[1]
			skillName := m[2]
			message := m[3]

			// Apply search highlighting to the message portion
			if searchQuery != "" && matchSet[i] {
				isCurrent := i == currentMatchLine
				message = highlightMatches(message, searchQuery, isCurrent)
				styledLine := tsStyle.Render(timestamp) + " " +
					skillStyle.Render(skillName) + " " + message
				styled = append(styled, styledLine)
			} else {
				styledLine := tsStyle.Render(timestamp) + " " +
					skillStyle.Render(skillName) + " " +
					msgStyle.Render(message)
				styled = append(styled, styledLine)
			}
		} else {
			// Raw lines — pass through without wrapping in extra styles
			// to preserve ANSI sequences from the subprocess
			if searchQuery != "" && matchSet[i] {
				isCurrent := i == currentMatchLine
				styled = append(styled, highlightMatches(line, searchQuery, isCurrent))
			} else {
				styled = append(styled, line)
			}
		}
	}

	result := strings.Join(styled, "\n")

	if active {
		result += " ▍"
	}

	return result
}

// applySelectionHighlight wraps lines within the selection range with SelectionStyle.
func applySelectionHighlight(content string, selStart, selEnd int) string {
	lines := strings.Split(content, "\n")
	for i := selStart; i <= selEnd && i < len(lines); i++ {
		if i >= 0 {
			lines[i] = styles.SelectionStyle.Render(lines[i])
		}
	}
	return strings.Join(lines, "\n")
}

// applyCharSelectionHighlight applies character-level selection highlighting.
// Uses ANSI-aware cutting so styled content is handled correctly.
func applyCharSelectionHighlight(content string, startLine, startCol, endLine, endCol int) string {
	lines := strings.Split(content, "\n")
	for i := range lines {
		if i < startLine || i > endLine {
			continue
		}
		lineWidth := ansi.StringWidth(lines[i])
		if lineWidth == 0 {
			continue
		}

		var sc, ec int
		if i == startLine && i == endLine {
			sc = startCol
			ec = endCol + 1
		} else if i == startLine {
			sc = startCol
			ec = lineWidth
		} else if i == endLine {
			sc = 0
			ec = endCol + 1
		} else {
			sc = 0
			ec = lineWidth
		}

		if sc > lineWidth {
			sc = lineWidth
		}
		if ec > lineWidth {
			ec = lineWidth
		}
		if sc >= ec {
			continue
		}

		before := ansi.Cut(lines[i], 0, sc)
		selected := ansi.Cut(lines[i], sc, ec)
		after := ansi.Cut(lines[i], ec, lineWidth)
		lines[i] = before + styles.SelectionStyle.Render(ansi.Strip(selected)) + after
	}
	return strings.Join(lines, "\n")
}

// extractCharSelection extracts plain text from a character-level selection
// on styled content. Returns the visible text within the selection range.
func extractCharSelection(styledContent string, startLine, startCol, endLine, endCol int) string {
	lines := strings.Split(styledContent, "\n")
	var result []string

	for i := startLine; i <= endLine && i < len(lines); i++ {
		if i < 0 {
			continue
		}
		lineWidth := ansi.StringWidth(lines[i])

		var sc, ec int
		if i == startLine && i == endLine {
			sc = startCol
			ec = endCol + 1
		} else if i == startLine {
			sc = startCol
			ec = lineWidth
		} else if i == endLine {
			sc = 0
			ec = endCol + 1
		} else {
			sc = 0
			ec = lineWidth
		}

		if sc > lineWidth {
			sc = lineWidth
		}
		if ec > lineWidth {
			ec = lineWidth
		}
		if sc >= ec {
			result = append(result, "")
			continue
		}

		extracted := ansi.Cut(lines[i], sc, ec)
		result = append(result, ansi.Strip(extracted))
	}

	return strings.Join(result, "\n")
}

// highlightMatches wraps occurrences of query in line with highlight styling.
// Uses case-insensitive matching with literal string comparison.
func highlightMatches(line, query string, isCurrent bool) string {
	if query == "" {
		return line
	}
	lower := strings.ToLower(line)
	lowerQ := strings.ToLower(query)

	style := styles.SearchHighlightStyle
	if isCurrent {
		style = styles.CurrentMatchStyle
	}

	var b strings.Builder
	start := 0
	qLen := len(lowerQ)
	for {
		idx := strings.Index(lower[start:], lowerQ)
		if idx < 0 {
			b.WriteString(line[start:])
			break
		}
		// Write text before match
		b.WriteString(line[start : start+idx])
		// Write highlighted match (using original case)
		b.WriteString(style.Render(line[start+idx : start+idx+qLen]))
		start += idx + qLen
	}
	return b.String()
}

