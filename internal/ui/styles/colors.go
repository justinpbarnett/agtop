package styles

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
)

// Semantic colors — AdaptiveColor{Light, Dark}
var (
	BorderFocused   = lipgloss.AdaptiveColor{Light: "#2e5cb8", Dark: "#7aa2f7"}
	BorderUnfocused = lipgloss.AdaptiveColor{Light: "#c0c0c0", Dark: "#3b4261"}
	TitleText       = lipgloss.AdaptiveColor{Light: "#1a1b26", Dark: "#c0caf5"}
	KeybindKey      = lipgloss.AdaptiveColor{Light: "#8a6200", Dark: "#e0af68"}
	KeybindLabel    = lipgloss.AdaptiveColor{Light: "#8890a8", Dark: "#565f89"}
	TextPrimary     = lipgloss.AdaptiveColor{Light: "#1a1b26", Dark: "#c0caf5"}
	TextSecondary   = lipgloss.AdaptiveColor{Light: "#8890a8", Dark: "#565f89"}
	TextDim         = lipgloss.AdaptiveColor{Light: "#b0b0b0", Dark: "#3b4261"}

	StatusRunning = lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#7dcfff"}
	StatusSuccess = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#9ece6a"}
	StatusError   = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f7768e"}
	StatusWarning = lipgloss.AdaptiveColor{Light: "#8a6200", Dark: "#e0af68"}
	StatusPending = lipgloss.AdaptiveColor{Light: "#8890a8", Dark: "#565f89"}

	SelectedRowBg = lipgloss.AdaptiveColor{Light: "#e0e0e0", Dark: "#292e42"}

	DiffAdded   = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#9ece6a"}
	DiffRemoved = lipgloss.AdaptiveColor{Light: "#cf222e", Dark: "#f7768e"}
	DiffHeader  = lipgloss.AdaptiveColor{Light: "#0969da", Dark: "#7dcfff"}
	DiffHunk    = lipgloss.AdaptiveColor{Light: "#8250df", Dark: "#bb9af7"}

	SelectedOption = lipgloss.AdaptiveColor{Light: "#1a7f37", Dark: "#9ece6a"}

	SelectionBg = lipgloss.AdaptiveColor{Light: "#c8d8f0", Dark: "#283457"}
)

// CostColor returns the appropriate color for a cost value per §2.3 thresholds.
func CostColor(cost float64) lipgloss.AdaptiveColor {
	switch {
	case cost >= 5.0:
		return StatusError
	case cost >= 1.0:
		return StatusWarning
	default:
		return TextPrimary
	}
}

// RunStateColor returns the appropriate status color for a run state.
func RunStateColor(state run.State) lipgloss.AdaptiveColor {
	switch state {
	case run.StateRunning, run.StateRouting, run.StateMerging:
		return StatusRunning
	case run.StateCompleted, run.StateAccepted:
		return StatusSuccess
	case run.StateFailed, run.StateRejected:
		return StatusError
	case run.StatePaused, run.StateReviewing:
		return StatusWarning
	case run.StateQueued:
		return StatusPending
	default:
		return TextDim
	}
}
