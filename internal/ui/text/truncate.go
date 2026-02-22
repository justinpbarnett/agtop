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

// PadRight pads s with spaces to exactly width. If s is wider, returns s unchanged.
// ANSI-aware: escape codes are not counted toward visual width.
func PadRight(s string, width int) string {
	w := ansi.StringWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}
