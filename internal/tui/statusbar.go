package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type StatusBar struct {
	width       int
	totalRuns   int
	activeRuns  int
	totalTokens int
	totalCost   float64
}

func NewStatusBar() StatusBar {
	return StatusBar{
		totalRuns:   4,
		activeRuns:  2,
		totalTokens: 69400,
		totalCost:   2.04,
	}
}

func (s StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	return s, nil
}

func (s StatusBar) View() string {
	left := fmt.Sprintf("Runs: %d (%d active) │ Tokens: %s │ Cost: $%.2f",
		s.totalRuns, s.activeRuns, formatTokens(s.totalTokens), s.totalCost)
	right := "j/k:navigate  h/l:tabs  Tab:focus  ?:help  q:quit"

	gap := s.width - len(left) - len(right) - 2
	if gap < 1 {
		gap = 1
	}

	bar := left + strings.Repeat(" ", gap) + right

	return StatusBarStyle.Width(s.width).Render(bar)
}

func (s *StatusBar) SetSize(w int) {
	s.width = w
}
