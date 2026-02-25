package border

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// TestEveryLineWidth renders panels with both ANSI-styled and plain content,
// then verifies every single line is exactly the target width.
func TestEveryLineWidth(t *testing.T) {
	// Raw ANSI escapes to simulate real terminal rendering
	dim := "\033[38;2;59;66;97m"
	sec := "\033[38;2;86;95;137m"
	pri := "\033[38;2;192;202;245m"
	rst := "\033[0m"

	tests := []struct {
		name    string
		content string
		width   int
		height  int
		focused bool
		kbs     []Keybind
	}{
		{
			name:    "plain content",
			content: "line one\nline two\nline three",
			width:   48, height: 8, focused: false, kbs: nil,
		},
		{
			name:    "styled content short lines",
			content: dim + "14:32" + rst + " " + sec + "route" + rst + " " + pri + "hello" + rst,
			width:   72, height: 8, focused: true, kbs: nil,
		},
		{
			name: "styled content long lines that need truncation",
			content: dim + "14:32:01" + rst + " " + sec + "route" + rst + " " + pri +
				"Analyzing task: add JWT authentication to API endpoints that is very long" + rst,
			width: 50, height: 5, focused: true, kbs: nil,
		},
		{
			name:    "empty content",
			content: "",
			width:   40, height: 6, focused: false, kbs: nil,
		},
		{
			name:    "with keybinds",
			content: "test",
			width:   40, height: 5, focused: true,
			kbs: []Keybind{{Key: "e", Label: "dit"}, {Key: "k", Label: "ill"}},
		},
		{
			name:    "exact width content",
			content: strings.Repeat("x", 38), // exactly innerWidth for width=40
			width:   40, height: 5, focused: false, kbs: nil,
		},
		{
			name: "mixed styled and plain",
			content: dim + "timestamp" + rst + " plain text\n" +
				"  " + sec + "indented" + rst + " " + pri + "styled content here" + rst + "\n" +
				"just plain\n" +
				dim + strings.Repeat("a", 100) + rst, // very long styled line
			width: 60, height: 10, focused: true, kbs: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			panel := RenderPanel("Title", tc.content, tc.kbs, tc.width, tc.height, tc.focused)
			lines := strings.Split(panel, "\n")

			if len(lines) != tc.height {
				t.Errorf("line count: got %d, want %d", len(lines), tc.height)
			}

			for i, line := range lines {
				w := lipgloss.Width(line)
				if w != tc.width {
					t.Errorf("line %d: width=%d, want %d (off by %+d) content=%q",
						i, w, tc.width, w-tc.width, line)
				}
			}
		})
	}
}

// TestJoinHorizontalPanels verifies two panels joined horizontally
// produce correct total width on every line.
func TestJoinHorizontalPanels(t *testing.T) {
	dim := "\033[38;2;59;66;97m"
	sec := "\033[38;2;86;95;137m"
	pri := "\033[38;2;192;202;245m"
	rst := "\033[0m"

	leftWidth := 48
	rightWidth := 72
	height := 15

	// Left panel: plain content
	leftContent := "● build    feat/add-auth    25m $0.42\n◐ build    fix/nav-bug      18m $0.08"
	leftPanel := RenderPanel("Runs (1 active)", leftContent, nil, leftWidth, height, true)

	// Right panel: styled content
	var styledLines []string
	for i := 0; i < 20; i++ {
		styledLines = append(styledLines,
			dim+"14:32:01"+rst+" "+sec+"route"+rst+" "+pri+"Some log message number "+rst)
	}
	rightContent := strings.Join(styledLines, "\n")
	rightPanel := RenderPanel("Log: build — feat/auth", rightContent, nil, rightWidth, height, false)

	// Verify each panel independently
	for i, line := range strings.Split(leftPanel, "\n") {
		if w := lipgloss.Width(line); w != leftWidth {
			t.Errorf("left panel line %d: width=%d want=%d", i, w, leftWidth)
		}
	}
	for i, line := range strings.Split(rightPanel, "\n") {
		if w := lipgloss.Width(line); w != rightWidth {
			t.Errorf("right panel line %d: width=%d want=%d", i, w, rightWidth)
		}
	}

	// Join and verify
	joined := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightPanel)
	totalWidth := leftWidth + rightWidth
	for i, line := range strings.Split(joined, "\n") {
		w := lipgloss.Width(line)
		if w != totalWidth {
			t.Errorf("joined line %d: width=%d want=%d (off by %+d)",
				i, w, totalWidth, w-totalWidth)
		}
	}
}
