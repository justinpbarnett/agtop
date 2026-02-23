package text

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Truncate truncates s to maxWidth, appending "…" if truncated.
// ANSI-aware: escape codes are not counted toward visual width and
// will not be broken by the truncation.
func Truncate(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	w := ansi.StringWidth(s)
	if w <= maxWidth {
		return s
	}
	return ansi.Truncate(s, maxWidth, "…")
}

// WrapText wraps s to fit within width columns, returning one string per line.
// Existing newlines are respected. Words wider than width are truncated with …
func WrapText(s string, width int) []string {
	if width <= 0 {
		return []string{s}
	}
	if s == "" {
		return []string{""}
	}
	var result []string
	for _, para := range strings.Split(s, "\n") {
		result = append(result, wrapParagraph(para, width)...)
	}
	return result
}

func wrapParagraph(s string, width int) []string {
	if ansi.StringWidth(s) <= width {
		return []string{s}
	}
	words := strings.Fields(s)
	if len(words) == 0 {
		return []string{""}
	}
	var lines []string
	current := ""
	currentW := 0
	for _, word := range words {
		ww := ansi.StringWidth(word)
		if ww > width {
			word = ansi.Truncate(word, width, "…")
			ww = width
		}
		if current == "" {
			current = word
			currentW = ww
		} else if currentW+1+ww <= width {
			current += " " + word
			currentW += 1 + ww
		} else {
			lines = append(lines, current)
			current = word
			currentW = ww
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}

// PadRight pads s with spaces to exactly width. If s is wider, returns s unchanged.
// ANSI-aware: escape codes are not counted toward visual width.
func PadRight(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
