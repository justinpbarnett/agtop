package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/run"
)

type StatusBar struct {
	width int
	store *run.Store
}

func NewStatusBar(store *run.Store) StatusBar {
	return StatusBar{store: store}
}

func (s StatusBar) Update(msg tea.Msg) (StatusBar, tea.Cmd) {
	return s, nil
}

func (s StatusBar) View() string {
	left := fmt.Sprintf("Runs: %d (%d active) │ Tokens: %s │ Cost: $%.2f",
		s.store.Count(), s.store.ActiveRuns(), formatTokens(s.store.TotalTokens()), s.store.TotalCost())
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
