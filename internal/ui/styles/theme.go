package styles

import "github.com/charmbracelet/lipgloss"

// Common reusable styles built from the color tokens.
var (
	TextPrimaryStyle   = lipgloss.NewStyle().Foreground(TextPrimary)
	TextSecondaryStyle = lipgloss.NewStyle().Foreground(TextSecondary)
	TextDimStyle       = lipgloss.NewStyle().Foreground(TextDim)
	TitleStyle         = lipgloss.NewStyle().Foreground(TitleText).Bold(true)
	SelectedRowStyle   = lipgloss.NewStyle().Background(SelectedRowBg)
)
