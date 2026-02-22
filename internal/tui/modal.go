package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type CloseModalMsg struct{}

type Modal struct {
	title   string
	content string
	width   int
	height  int
}

func NewHelpModal() *Modal {
	content := "" +
		"Navigation\n" +
		"  j / k       Move up/down\n" +
		"  G / gg      Jump to bottom/top\n" +
		"  Tab         Cycle panel focus\n" +
		"\n" +
		"Detail Panel\n" +
		"  h / l       Previous/next tab\n" +
		"\n" +
		"Run Actions\n" +
		"  n           New run\n" +
		"  p           Pause\n" +
		"  r           Resume\n" +
		"  c           Cancel\n" +
		"  a           Accept\n" +
		"  x           Reject\n" +
		"\n" +
		"General\n" +
		"  /           Filter runs\n" +
		"  ?           Toggle this help\n" +
		"  q           Quit\n" +
		"  Esc         Close modal"

	return &Modal{
		title:   "Keybindings",
		content: content,
		width:   40,
	}
}

func (m Modal) Update(msg tea.Msg) (Modal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "?", "q":
			return m, func() tea.Msg { return CloseModalMsg{} }
		}
	}
	return m, nil
}

func (m Modal) View() string {
	title := lipgloss.NewStyle().Bold(true).Render(m.title)
	body := title + "\n\n" + m.content
	return ModalStyle.Width(m.width).Render(body)
}
