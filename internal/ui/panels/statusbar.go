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

var statusSpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Version is set via -ldflags at build time. Falls back to "dev".
var Version = "dev"

// FlashDuration returns how long the status bar flash is shown.
func FlashDuration() time.Duration { return flashDurationVal }

// FlashLevel controls the icon and color of a status bar flash message.
type FlashLevel int

const (
	FlashInfo    FlashLevel = iota // blue ●
	FlashSuccess                   // green ✓
	FlashWarning                   // yellow ⚠
	FlashError                     // red ✗
)

type StatusBar struct {
	width      int
	store      *run.Store
	flash      string
	flashLevel FlashLevel
	flashUntil time.Time
	tickStep   int
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
	appName := "agtop " + Version
	if running > 0 {
		frame := statusSpinnerFrames[s.tickStep%len(statusSpinnerFrames)]
		spinner := lipgloss.NewStyle().Foreground(styles.StatusRunning).Render(frame)
		appName = spinner + " " + appName
	}
	version := styles.TextSecondaryStyle.Render(appName)

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
		var icon string
		var color lipgloss.TerminalColor
		switch s.flashLevel {
		case FlashSuccess:
			icon, color = "✓", styles.StatusSuccess
		case FlashError:
			icon, color = "✗", styles.StatusError
		case FlashWarning:
			icon, color = "⚠", styles.StatusWarning
		default: // FlashInfo
			icon, color = "●", styles.StatusRunning
		}
		flashStr := lipgloss.NewStyle().Foreground(color).Bold(true).Render(icon + " " + s.flash)
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
	s.SetFlashWithLevel(msg, FlashInfo)
}

func (s *StatusBar) SetFlashWithLevel(msg string, level FlashLevel) {
	s.flash = msg
	s.flashLevel = level
	s.flashUntil = time.Now().Add(flashDurationVal)
}

func (s *StatusBar) ClearFlash() {
	s.flash = ""
	s.flashLevel = FlashInfo
	s.flashUntil = time.Time{}
}

func (s *StatusBar) SetSize(w int) {
	s.width = w
}

// Tick advances the animation frame for the status bar spinner.
func (s *StatusBar) Tick() {
	s.tickStep++
}
