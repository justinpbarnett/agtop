package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpb/agtop/internal/config"
)

type App struct {
	config *config.Config
}

func NewApp(cfg *config.Config) App {
	return App{config: cfg}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		}
	}
	return a, nil
}

func (a App) View() string {
	return "agtop v0.1.0 — press q to quit\n"
}
