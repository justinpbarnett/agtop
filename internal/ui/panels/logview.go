package panels

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

// logLineRe matches log lines like "[14:32:01 route] message"
var logLineRe = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2})\s+(\S+)\]\s*(.*)$`)

type LogView struct {
	viewport viewport.Model
	width    int
	height   int
	buffer   *process.RingBuffer
	runID    string
	skill    string
	branch   string
	active   bool
	follow   bool
	focused  bool
}

func NewLogView() LogView {
	vp := viewport.New(0, 0)
	return LogView{viewport: vp, follow: true}
}

func (l LogView) Update(msg tea.Msg) (LogView, tea.Cmd) {
	switch msg := msg.(type) {
	case LogLineMsg:
		if msg.RunID == l.runID && l.buffer != nil {
			atBottom := l.viewport.AtBottom()
			l.viewport.SetContent(formatLogContent(strings.Join(l.buffer.Lines(), "\n"), l.active))
			if atBottom || l.follow {
				l.viewport.GotoBottom()
			}
			return l, nil
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "G":
			l.follow = true
			l.viewport.GotoBottom()
			return l, nil
		case "g":
			l.follow = false
		case "j", "k", "up", "down":
			l.follow = false
		}
	}

	var cmd tea.Cmd
	l.viewport, cmd = l.viewport.Update(msg)
	return l, cmd
}

func (l LogView) View() string {
	title := "Log"
	if l.skill != "" || l.branch != "" {
		parts := []string{}
		if l.skill != "" {
			parts = append(parts, l.skill)
		}
		if l.branch != "" {
			parts = append(parts, l.branch)
		}
		title = fmt.Sprintf("Log: %s", strings.Join(parts, " — "))
	}

	var keybinds []border.Keybind
	if l.focused {
		keybinds = []border.Keybind{
			{Key: "G", Label: "bottom"},
			{Key: "g", Label: "g top"},
		}
		// Show new output indicator when scrolled up
		if !l.viewport.AtBottom() && !l.follow {
			keybinds = append(keybinds, border.Keybind{Key: "↓", Label: " new output"})
		}
	}

	content := l.viewport.View()
	return border.RenderPanel(title, content, keybinds, l.width, l.height, l.focused)
}

func (l *LogView) SetSize(w, h int) {
	l.width = w
	l.height = h
	innerW := w - 2
	innerH := h - 2
	if innerW < 0 {
		innerW = 0
	}
	if innerH < 0 {
		innerH = 0
	}
	l.viewport.Width = innerW
	l.viewport.Height = innerH
	// Re-set content so the viewport wraps at the correct width.
	// Without this, content set before SetSize is wrapped at width 0.
	l.refreshContent()
}

func (l *LogView) SetFocused(focused bool) {
	l.focused = focused
}

func (l *LogView) SetRun(runID, skill, branch string, buf *process.RingBuffer, active bool) {
	l.runID = runID
	l.skill = skill
	l.branch = branch
	l.buffer = buf
	l.active = active
	l.follow = true
	l.refreshContent()
}

// refreshContent re-sets the viewport content from the current buffer or mock data,
// ensuring it's wrapped at the current viewport width.
func (l *LogView) refreshContent() {
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
	l.viewport.SetContent(formatLogContent(raw, l.active))
	if l.follow {
		l.viewport.GotoBottom()
	}
}

// formatLogContent styles log lines: timestamps in TextDim, tool names in TextSecondary,
// tool use lines indented, and appends a streaming cursor if active.
func formatLogContent(content string, active bool) string {
	if content == "" {
		return content
	}

	tsStyle := styles.TextDimStyle
	skillStyle := styles.TextSecondaryStyle

	lines := strings.Split(content, "\n")
	styled := make([]string, 0, len(lines))

	for _, line := range lines {
		m := logLineRe.FindStringSubmatch(line)
		if m != nil {
			timestamp := m[1]
			skillName := m[2]
			message := m[3]

			styledLine := tsStyle.Render(timestamp) + " " +
				skillStyle.Render(skillName) + " " +
				lipgloss.NewStyle().Foreground(styles.TextPrimary).Render(message)

			styled = append(styled, styledLine)
		} else {
			styled = append(styled, line)
		}
	}

	result := strings.Join(styled, "\n")

	// Append streaming cursor for active runs
	if active {
		result += " ▍"
	}

	return result
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
