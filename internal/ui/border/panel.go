package border

import (
	"strings"
)

// RenderPanel assembles a complete bordered panel:
//
//	top border (with title)
//	content lines (with side borders)
//	bottom border (with keybinds if focused)
//
// Content is padded/cropped to exactly fill height-2 rows x width-2 cols.
func RenderPanel(title string, content string, keybinds []Keybind,
	width, height int, focused bool) string {

	if height < 2 || width < 2 {
		return ""
	}

	innerHeight := height - 2
	innerWidth := width - 2

	// Split content into lines, pad or crop to innerHeight
	var lines []string
	if content != "" {
		lines = strings.Split(content, "\n")
	}
	// Crop if too many lines
	if len(lines) > innerHeight {
		lines = lines[:innerHeight]
	}
	// Pad with empty lines if too few
	for len(lines) < innerHeight {
		lines = append(lines, strings.Repeat(" ", innerWidth))
	}

	paddedContent := strings.Join(lines, "\n")

	top := RenderBorderTop(title, width, focused)
	middle := RenderBorderSides(paddedContent, width, focused)
	bottom := RenderBorderBottom(keybinds, width, focused)

	return top + "\n" + middle + "\n" + bottom
}
