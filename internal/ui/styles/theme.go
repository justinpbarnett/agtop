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

	DiffAddedStyle   = lipgloss.NewStyle().Foreground(DiffAdded)
	DiffRemovedStyle = lipgloss.NewStyle().Foreground(DiffRemoved)
	DiffHeaderStyle  = lipgloss.NewStyle().Foreground(DiffHeader).Bold(true)
	DiffHunkStyle    = lipgloss.NewStyle().Foreground(DiffHunk)

	SelectedOptionStyle = lipgloss.NewStyle().Foreground(SelectedOption).Bold(true)

	SelectionStyle = lipgloss.NewStyle().Background(SelectionBg)

	// Log entry styles
	LogEntryCursorStyle  = lipgloss.NewStyle().Foreground(TitleText).Bold(true)
	LogEntryDetailStyle  = lipgloss.NewStyle().Foreground(TextSecondary)
)
