package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
)

var (
	BorderColor  = lipgloss.Color("240")
	ActiveColor  = lipgloss.Color("62")
	RunningColor = lipgloss.Color("63")
	PausedColor  = lipgloss.Color("220")
	SuccessColor = lipgloss.Color("34")
	FailedColor  = lipgloss.Color("196")
	ReviewColor  = lipgloss.Color("170")
	DimColor     = lipgloss.Color("243")
	TextColor    = lipgloss.Color("252")
	BgDark       = lipgloss.Color("236")
)

func PanelStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor).
		Width(width).
		Height(height)
}

func ActivePanelStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ActiveColor).
		Width(width).
		Height(height)
}

var TabStyle = lipgloss.NewStyle().
	Foreground(DimColor).
	Padding(0, 1)

var ActiveTabStyle = lipgloss.NewStyle().
	Foreground(TextColor).
	Bold(true).
	Underline(true).
	Padding(0, 1)

var StatusBarStyle = lipgloss.NewStyle().
	Background(BgDark).
	Foreground(TextColor).
	Padding(0, 1)

var ModalStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(ActiveColor).
	Padding(1, 2)

var SelectedStyle = lipgloss.NewStyle().
	Background(lipgloss.Color("237")).
	Bold(true)

var DimStyle = lipgloss.NewStyle().
	Foreground(DimColor)

func RunStateStyle(state run.State) lipgloss.Style {
	var color lipgloss.Color
	switch state {
	case run.StateRunning, run.StateRouting:
		color = RunningColor
	case run.StatePaused:
		color = PausedColor
	case run.StateCompleted, run.StateAccepted:
		color = SuccessColor
	case run.StateFailed, run.StateRejected:
		color = FailedColor
	case run.StateReviewing:
		color = ReviewColor
	default:
		color = DimColor
	}
	return lipgloss.NewStyle().Foreground(color)
}
