package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/process"
)

type LogViewer struct {
	viewport viewport.Model
	width    int
	height   int
	buffer   *process.RingBuffer
	runID    string
	follow   bool
}

func NewLogViewer() LogViewer {
	vp := viewport.New(0, 0)
	vp.SetContent(mockLogContent())
	return LogViewer{viewport: vp, follow: true}
}

func (l LogViewer) Update(msg tea.Msg) (LogViewer, tea.Cmd) {
	switch msg := msg.(type) {
	case LogLineMsg:
		if msg.RunID == l.runID && l.buffer != nil {
			atBottom := l.viewport.AtBottom()
			l.viewport.SetContent(strings.Join(l.buffer.Lines(), "\n"))
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

func (l LogViewer) View() string {
	return l.viewport.View()
}

func (l *LogViewer) SetSize(w, h int) {
	l.width = w
	l.height = h
	l.viewport.Width = w
	l.viewport.Height = h
}

func (l *LogViewer) SetRun(runID string, buf *process.RingBuffer) {
	l.runID = runID
	l.buffer = buf
	l.follow = true
	if buf != nil {
		lines := buf.Lines()
		if len(lines) > 0 {
			l.viewport.SetContent(strings.Join(lines, "\n"))
		} else {
			l.viewport.SetContent("Waiting for output...")
		}
		l.viewport.GotoBottom()
	} else {
		l.viewport.SetContent(mockLogContent())
	}
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
