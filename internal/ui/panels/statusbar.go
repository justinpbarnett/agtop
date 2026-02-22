package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/ui/text"
)

type StatusBar struct {
	width int
	store *run.Store
}

func NewStatusBar(store *run.Store) StatusBar {
	return StatusBar{store: store}
}

func (s StatusBar) View() string {
	runs := s.store.List()

	var running, queued, done int
	for _, r := range runs {
		switch r.State {
		case run.StateRunning, run.StateRouting:
			running++
		case run.StateQueued, run.StatePaused:
			queued++
		case run.StateCompleted, run.StateAccepted, run.StateFailed, run.StateRejected:
			done++
		}
	}

	totalCost := s.store.TotalCost()
	sep := styles.TextDimStyle.Render(" │ ")

	// Build sections
	version := styles.TextSecondaryStyle.Render("agtop v0.1.0")

	counts := fmt.Sprintf("%s %s %s",
		lipgloss.NewStyle().Foreground(styles.StatusRunning).Render(fmt.Sprintf("%d running", running)),
		lipgloss.NewStyle().Foreground(styles.StatusPending).Render(fmt.Sprintf("%d queued", queued)),
		lipgloss.NewStyle().Foreground(styles.StatusSuccess).Render(fmt.Sprintf("%d done", done)),
	)

	costStr := lipgloss.NewStyle().Foreground(styles.CostColor(totalCost)).Render(
		fmt.Sprintf("Total: %s", text.FormatCost(totalCost)),
	)

	helpHint := styles.TextSecondaryStyle.Render("?:help")

	left := " " + version + sep + counts + sep + costStr
	right := helpHint + " "

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := s.width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func (s *StatusBar) SetSize(w int) {
	s.width = w
}
