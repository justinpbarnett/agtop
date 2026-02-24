package border

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

// Keybind represents a single keybind hint: [e]dit, [k]ill, etc.
type Keybind struct {
	Key   string // The key character, e.g. "e"
	Label string // The label after the key, e.g. "dit"
}

// RenderKeybind renders a single keybind: [e]dit with Key in KeybindKey color (bold), label in KeybindLabel.
func RenderKeybind(kb Keybind) string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.KeybindKey).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(styles.KeybindLabel)
	return keyStyle.Render("["+kb.Key+"]") + labelStyle.Render(kb.Label)
}

// KeybindWidth returns the display width of a rendered keybind (without ANSI).
// Uses lipgloss.Width so multi-byte Unicode keys are measured correctly.
func KeybindWidth(kb Keybind) int {
	return lipgloss.Width(RenderKeybind(kb))
}
