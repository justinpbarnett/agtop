package panels

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/ui/text"
)

const flashDurationVal = 5 * time.Second

// Version is set via -ldflags at build time. Falls back to "dev".
var Version = "dev"

// FlashDuration returns how long the status bar flash is shown.
func FlashDuration() time.Duration { return flashDurationVal }

type StatusBar struct {
	width      int
	store      *run.Store
	flash      string
	flashUntil time.Time
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
	version := styles.TextSecondaryStyle.Render("agtop " + Version)

	counts := fmt.Sprintf("%s %s %s",
		lipgloss.NewStyle().Foreground(styles.StatusRunning).Render(fmt.Sprintf("%d running", running)),
		lipgloss.NewStyle().Foreground(styles.StatusPending).Render(fmt.Sprintf("%d queued", queued)),
		lipgloss.NewStyle().Foreground(styles.StatusSuccess).Render(fmt.Sprintf("%d done", done)),
	)

	totalTokens := s.store.TotalTokens()
	tokensStr := styles.TextSecondaryStyle.Render(
		fmt.Sprintf("Tokens: %s", text.FormatTokens(totalTokens)),
	)

	costStr := lipgloss.NewStyle().Foreground(styles.CostColor(totalCost)).Render(
		fmt.Sprintf("Total: %s", text.FormatCost(totalCost)),
	)

	helpHint := styles.TextSecondaryStyle.Render("?:help")

	left := " " + version + sep + counts + sep + tokensStr + sep + costStr

	if s.flash != "" && time.Now().Before(s.flashUntil) {
		flashStr := lipgloss.NewStyle().Foreground(styles.StatusError).Bold(true).Render("⚠ " + s.flash)
		left += sep + flashStr
	}

	right := helpHint + " "

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)
	gap := s.width - leftWidth - rightWidth
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

func (s *StatusBar) SetFlash(msg string) {
	s.flash = msg
	s.flashUntil = time.Now().Add(flashDurationVal)
}

func (s *StatusBar) ClearFlash() {
	s.flash = ""
	s.flashUntil = time.Time{}
}

func (s *StatusBar) SetSize(w int) {
	s.width = w
}
