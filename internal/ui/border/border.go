package border

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

// Border characters
const (
	cornerTL = "╭"
	cornerTR = "╮"
	cornerBL = "╰"
	cornerBR = "╯"
	horizBar = "─"
	vertBar  = "│"
)

func borderColor(focused bool) lipgloss.AdaptiveColor {
	if focused {
		return styles.BorderFocused
	}
	return styles.BorderUnfocused
}

// RenderBorderTop renders: ╭─ Title ────────────╮
// Title is bold TitleText (focused) or TextSecondary (unfocused).
func RenderBorderTop(title string, width int, focused bool) string {
	if width < 2 {
		return ""
	}
	bc := borderColor(focused)
	bs := lipgloss.NewStyle().Foreground(bc)

	var ts lipgloss.Style
	if focused {
		ts = styles.TitleStyle
	} else {
		ts = styles.TextSecondaryStyle.Bold(true)
	}

	innerWidth := width - 2 // corners
	if title == "" {
		return bs.Render(cornerTL + strings.Repeat(horizBar, innerWidth) + cornerTR)
	}

	// ╭─ Title ─...─╮
	// "─ " + title + " " = title visual width + 3
	titleRendered := ts.Render(title)
	titleVisualWidth := lipgloss.Width(titleRendered)
	// prefix: "─ "
	prefixWidth := 2
	// suffix: " " then fill
	suffixPadWidth := 1
	usedWidth := prefixWidth + titleVisualWidth + suffixPadWidth
	fillWidth := innerWidth - usedWidth
	if fillWidth < 0 {
		fillWidth = 0
	}

	return bs.Render(cornerTL+horizBar+" ") +
		titleRendered +
		bs.Render(" "+strings.Repeat(horizBar, fillWidth)+cornerTR)
}

// RenderBorderBottom renders the bottom border.
// If focused and keybinds provided: ╰─ [e]dit  [k]ill ──╯
// Otherwise: ╰────────────────────╯
func RenderBorderBottom(keybinds []Keybind, width int, focused bool) string {
	if width < 2 {
		return ""
	}
	bc := borderColor(focused)
	bs := lipgloss.NewStyle().Foreground(bc)

	innerWidth := width - 2

	if !focused || len(keybinds) == 0 {
		return bs.Render(cornerBL + strings.Repeat(horizBar, innerWidth) + cornerBR)
	}

	// ╰─ [e]dit  [k]ill ─...─╯
	// "─ " prefix (2) + keybinds + " " suffix pad (1) must fit within innerWidth.
	// Build incrementally so keybinds that overflow the panel are simply dropped.
	prefixWidth := 2
	suffixPadWidth := 1
	maxKbWidth := innerWidth - prefixWidth - suffixPadWidth
	if maxKbWidth < 0 {
		maxKbWidth = 0
	}

	var kbParts []string
	usedWidth := 0
	for _, kb := range keybinds {
		rendered := RenderKeybind(kb)
		kbW := lipgloss.Width(rendered)
		sepW := 0
		if len(kbParts) > 0 {
			sepW = 2 // "  " separator
		}
		if usedWidth+sepW+kbW > maxKbWidth {
			break
		}
		kbParts = append(kbParts, rendered)
		usedWidth += sepW + kbW
	}

	kbStr := strings.Join(kbParts, "  ")
	fillWidth := maxKbWidth - usedWidth
	if fillWidth < 0 {
		fillWidth = 0
	}

	return bs.Render(cornerBL+horizBar+" ") +
		kbStr +
		bs.Render(" "+strings.Repeat(horizBar, fillWidth)+cornerBR)
}

// RenderBorderSides wraps content lines with │ on each side.
// Each line is truncated/padded to innerWidth (width - 2).
// Uses lipgloss.Width() for ANSI-aware width measurement so styled
// content is handled correctly.
func RenderBorderSides(content string, width int, focused bool) string {
	if width < 2 {
		return content
	}
	bc := borderColor(focused)
	bs := lipgloss.NewStyle().Foreground(bc)
	truncator := lipgloss.NewStyle().MaxWidth(width - 2)

	innerWidth := width - 2
	lines := strings.Split(content, "\n")
	var result []string
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > innerWidth {
			line = truncator.Render(line)
			w = lipgloss.Width(line)
		}
		if w < innerWidth {
			line += strings.Repeat(" ", innerWidth-w)
		}
		result = append(result, bs.Render(vertBar)+line+bs.Render(vertBar))
	}
	return strings.Join(result, "\n")
}
