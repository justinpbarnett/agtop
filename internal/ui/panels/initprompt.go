package panels

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

type InitPrompt struct {
	width  int
	height int
}

func NewInitPrompt() *InitPrompt {
	return &InitPrompt{
		width:  50,
		height: 7,
	}
}

func (p InitPrompt) Update(msg tea.Msg) (InitPrompt, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter", "y", "Y":
			return p, func() tea.Msg { return InitAcceptedMsg{} }
		case "esc", "n", "N":
			return p, func() tea.Msg { return CloseModalMsg{} }
		}
	}
	return p, nil
}

func (p InitPrompt) View() string {
	body := styles.TextPrimaryStyle.Render("No agtop.toml found in this directory.") + "\n" +
		"\n" +
		styles.TextPrimaryStyle.Render("Run agtop init to set up hooks and config?")

	bottomKb := []border.Keybind{
		{Key: "Enter", Label: " yes"},
		{Key: "Esc", Label: " no"},
	}
	return border.RenderPanel("Initialize Project", body, bottomKb, p.width, p.height, true)
}
