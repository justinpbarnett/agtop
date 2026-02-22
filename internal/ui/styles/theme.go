package styles

import "github.com/charmbracelet/lipgloss"

// Common reusable styles built from the color tokens.
var (
	TextPrimaryStyle   = lipgloss.NewStyle().Foreground(TextPrimary)
	TextSecondaryStyle = lipgloss.NewStyle().Foreground(TextSecondary)
	TextDimStyle       = lipgloss.NewStyle().Foreground(TextDim)
	TitleStyle         = lipgloss.NewStyle().Foreground(TitleText).Bold(true)
	SelectedRowStyle   = lipgloss.NewStyle().Background(SelectedRowBg)

	// Search highlight: yellow background, black text for matches
	SearchHighlightStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("11")).
				Foreground(lipgloss.Color("0"))
	// Current match: bright orange background, black text
	CurrentMatchStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("208")).
				Foreground(lipgloss.Color("0"))
)
