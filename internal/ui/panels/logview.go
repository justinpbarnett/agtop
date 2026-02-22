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
		title = "3 " + styles.TitleStyle.Render(logLabel) +
			styles.TextDimStyle.Render(" │ ") +
			styles.TextDimStyle.Render(diffLabel)
	} else {
		title = "3 " + styles.TextDimStyle.Render(logLabel) +
			styles.TextDimStyle.Render(" │ ") +
			styles.TitleStyle.Render(diffLabel)
	}

	var keybinds []border.Keybind
	var content string

	if l.activeTab == tabLog {
		if l.focused {
			keybinds = []border.Keybind{
				{Key: "G", Label: "bottom"},
				{Key: "g", Label: "g top"},
				{Key: "/", Label: "search"},
			}
			if !l.viewport.AtBottom() && !l.follow {
				keybinds = append(keybinds, border.Keybind{Key: "↓", Label: " new output"})
			}
		}

		content = l.viewport.View()

		// Append search bar or match status below the viewport
		if l.searching {
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
	if l.activeTab != tabLog {
		return false
	}
	return l.searching || l.searchQuery != ""
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
	if l.searching || l.searchQuery != "" {
		innerH-- // Reserve 1 row for search bar / status
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

// refreshContent re-sets the viewport content from the current buffer or mock data,
// ensuring it's wrapped at the current viewport width.
func (l *LogView) refreshContent() {
	l.viewport.SetContent(l.renderContent())
	if l.follow {
		l.viewport.GotoBottom()
	}
}

// renderContent builds the styled log content, including search highlights.
func (l *LogView) renderContent() string {
	var raw string
	if l.buffer != nil {
		lines := l.buffer.Lines()
		if len(lines) > 0 {
			raw = strings.Join(lines, "\n")
		} else {
			raw = "Waiting for output..."
		}
	} else {
		raw = mockLogContent()
	}
	return formatLogContent(raw, l.active, l.searchQuery, l.matchIndices, l.currentMatch)
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

func mockLogContent() string {
	lines := []string{
		"[14:32:01 route] Analyzing task: add JWT authentication to API endpoints",
		"[14:32:02 route] Detected: feat — authentication feature",
		"[14:32:03 route] Selected workflow: sdlc (7 skills)",
		"[14:32:05 spec] Reading existing auth patterns in src/middleware/...",
		"[14:32:08 spec] Writing SPEC.md to worktree",
		"[14:32:10 spec] Spec complete: 3 endpoints, 2 middleware functions",
		"[14:32:12 decompose] Analyzing spec for parallel opportunities",
		"[14:32:14 decompose] Task graph: 4 nodes, 2 parallel groups",
		"[14:32:15 decompose] Group A: auth middleware, token utils",
		"[14:32:15 decompose] Group B: login endpoint, refresh endpoint",
		"[14:32:18 build] Reading src/middleware/auth.ts...",
		"[14:32:20 build] Creating src/middleware/jwt.ts",
		"[14:32:22 build] Writing JWT validation middleware",
		"[14:32:25 build] Reading src/routes/index.ts...",
		"[14:32:27 build] Creating src/routes/auth.ts",
		"[14:32:30 build] Writing POST /auth/login handler",
		"[14:32:33 build] Writing POST /auth/refresh handler",
		"[14:32:35 build] Editing src/routes/index.ts — adding auth routes",
		"[14:32:38 build] Creating src/utils/token.ts",
		"[14:32:40 build] Writing token generation and verification utils",
		"[14:32:42 build] Editing package.json — adding jsonwebtoken dependency",
		"[14:32:45 test] Running npm test...",
		"[14:32:48 test] PASS src/middleware/jwt.test.ts (4 tests)",
		"[14:32:50 test] PASS src/routes/auth.test.ts (6 tests)",
		"[14:32:51 test] PASS src/utils/token.test.ts (3 tests)",
		"[14:32:52 test] All 13 tests passed",
		"[14:32:54 review] Checking code quality...",
		"[14:32:56 review] No issues found. 7 files changed, +342 -12",
		"[14:32:58 document] Generating API documentation...",
		"[14:33:00 document] Written to docs/auth-api.md",
	}
	return strings.Join(lines, "\n")
}
