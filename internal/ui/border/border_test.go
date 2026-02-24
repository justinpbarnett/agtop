package border

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

// stripAnsi returns the visible character width of a styled string.
func visibleWidth(s string) int {
	return lipgloss.Width(s)
}

func TestRenderKeybind(t *testing.T) {
	kb := Keybind{Key: "e", Label: "dit"}
	got := RenderKeybind(kb)
	// Should contain [e] and dit
	if !strings.Contains(got, "e") || !strings.Contains(got, "dit") {
		t.Errorf("RenderKeybind: got %q, expected key and label", got)
	}
	if w := KeybindWidth(kb); w != 6 {
		t.Errorf("KeybindWidth single char: got %d, want 6", w)
	}

	// Multi-char key: [Esc] close = 2 + 3 + 6 = 11
	kbEsc := Keybind{Key: "Esc", Label: " close"}
	if w := KeybindWidth(kbEsc); w != 11 {
		t.Errorf("KeybindWidth multi-char: got %d, want 11", w)
	}
}

func TestRenderBorderTopNoTitle(t *testing.T) {
	got := RenderBorderTop("", 20, false)
	w := visibleWidth(got)
	if w != 20 {
		t.Errorf("RenderBorderTop no title: width %d, want 20", w)
	}
	if !strings.Contains(got, "╭") || !strings.Contains(got, "╮") {
		t.Error("RenderBorderTop: missing corner chars")
	}
}

func TestRenderBorderTopWithTitle(t *testing.T) {
	got := RenderBorderTop("Runs", 30, true)
	w := visibleWidth(got)
	if w != 30 {
		t.Errorf("RenderBorderTop with title: width %d, want 30", w)
	}
	if !strings.Contains(got, "Runs") {
		t.Error("RenderBorderTop: missing title")
	}
}

func TestRenderBorderTopFocusedVsUnfocused(t *testing.T) {
	focused := RenderBorderTop("Test", 20, true)
	unfocused := RenderBorderTop("Test", 20, false)
	// Both should have same visible width
	if visibleWidth(focused) != visibleWidth(unfocused) {
		t.Error("focused and unfocused border tops should have same width")
	}
	// Both should contain the title and corners
	for _, s := range []string{focused, unfocused} {
		if !strings.Contains(s, "Test") {
			t.Error("expected title in border top")
		}
		if !strings.Contains(s, "╭") || !strings.Contains(s, "╮") {
			t.Error("expected corners in border top")
		}
	}
}

func TestRenderBorderBottomPlain(t *testing.T) {
	got := RenderBorderBottom(nil, 20, false)
	w := visibleWidth(got)
	if w != 20 {
		t.Errorf("RenderBorderBottom plain: width %d, want 20", w)
	}
	if !strings.Contains(got, "╰") || !strings.Contains(got, "╯") {
		t.Error("RenderBorderBottom: missing corner chars")
	}
}

func TestRenderBorderBottomWithKeybinds(t *testing.T) {
	kbs := []Keybind{
		{Key: "e", Label: "dit"},
		{Key: "k", Label: "ill"},
	}
	got := RenderBorderBottom(kbs, 30, true)
	w := visibleWidth(got)
	if w != 30 {
		t.Errorf("RenderBorderBottom with keybinds: width %d, want 30", w)
	}
	if !strings.Contains(got, "e") || !strings.Contains(got, "k") {
		t.Error("RenderBorderBottom: missing keybind keys")
	}
}

func TestRenderBorderBottomUnicodeKeybind(t *testing.T) {
	// ⏎ is a 3-byte UTF-8 char with visual width 1; must not cause overflow.
	kbs := []Keybind{
		{Key: "⏎", Label: " fullscreen"},
	}
	got := RenderBorderBottom(kbs, 24, true)
	w := visibleWidth(got)
	if w != 24 {
		t.Errorf("RenderBorderBottom unicode keybind: width %d, want 24", w)
	}
}

func TestRenderBorderBottomKeybindOverflow(t *testing.T) {
	// Detail-panel keybinds (58 visual chars) in a 24-wide panel — must not overflow.
	kbs := []Keybind{
		{Key: "⏎", Label: " fullscreen"},
		{Key: "j/k", Label: " scroll"},
		{Key: "G", Label: " bottom"},
		{Key: "g", Label: "g top"},
		{Key: "y", Label: "ank"},
	}
	got := RenderBorderBottom(kbs, 24, true)
	w := visibleWidth(got)
	if w != 24 {
		t.Errorf("RenderBorderBottom overflow: width %d, want 24", w)
	}
}

func TestRenderBorderSides(t *testing.T) {
	content := "hello\nworld"
	got := RenderBorderSides(content, 12, false)
	lines := strings.Split(got, "\n")
	if len(lines) != 2 {
		t.Errorf("RenderBorderSides: got %d lines, want 2", len(lines))
	}
	for i, line := range lines {
		w := visibleWidth(line)
		if w != 12 {
			t.Errorf("RenderBorderSides line %d: width %d, want 12", i, w)
		}
	}
}

func TestRenderBorderSidesTruncation(t *testing.T) {
	content := "this is a very long line that should be truncated"
	got := RenderBorderSides(content, 20, false)
	w := visibleWidth(got)
	if w != 20 {
		t.Errorf("RenderBorderSides truncation: width %d, want 20", w)
	}
}

func TestRenderPanel(t *testing.T) {
	content := "line 1\nline 2"
	got := RenderPanel("Title", content, nil, 30, 6, true)
	lines := strings.Split(got, "\n")
	// height=6: 1 top + 4 content + 1 bottom = 6
	if len(lines) != 6 {
		t.Errorf("RenderPanel: got %d lines, want 6", len(lines))
	}
	// All lines should be 30 wide
	for i, line := range lines {
		w := visibleWidth(line)
		if w != 30 {
			t.Errorf("RenderPanel line %d: width %d, want 30", i, w)
		}
	}
}

func TestRenderPanelContentCrop(t *testing.T) {
	// More content lines than innerHeight
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "content line")
	}
	content := strings.Join(lines, "\n")
	got := RenderPanel("", content, nil, 20, 5, false)
	resultLines := strings.Split(got, "\n")
	// height=5: 1 top + 3 content + 1 bottom
	if len(resultLines) != 5 {
		t.Errorf("RenderPanel crop: got %d lines, want 5", len(resultLines))
	}
}

func TestRenderPanelContentPad(t *testing.T) {
	// Fewer content lines than innerHeight
	got := RenderPanel("", "single line", nil, 20, 8, false)
	resultLines := strings.Split(got, "\n")
	// height=8: 1 top + 6 content + 1 bottom
	if len(resultLines) != 8 {
		t.Errorf("RenderPanel pad: got %d lines, want 8", len(resultLines))
	}
}

func TestRenderPanelEmpty(t *testing.T) {
	got := RenderPanel("", "", nil, 20, 4, false)
	lines := strings.Split(got, "\n")
	if len(lines) != 4 {
		t.Errorf("RenderPanel empty: got %d lines, want 4", len(lines))
	}
}
