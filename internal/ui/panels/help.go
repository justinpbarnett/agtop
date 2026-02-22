package panels

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

type HelpOverlay struct {
	width  int
	height int
}

func NewHelpOverlay() *HelpOverlay {
	return &HelpOverlay{
		width:  44,
		height: 22,
	}
}

func (h HelpOverlay) Update(msg tea.Msg) (HelpOverlay, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "?", "q":
			return h, func() tea.Msg { return CloseModalMsg{} }
		}
	}
	return h, nil
}

func (h HelpOverlay) View() string {
	keyStyle := lipgloss.NewStyle().Foreground(styles.KeybindKey).Bold(true)
	descStyle := styles.TextPrimaryStyle
	sectionStyle := styles.TitleStyle

	kv := func(key, desc string) string {
		return "  " + keyStyle.Render(key) + "  " + descStyle.Render(desc)
	}

	var b strings.Builder
	b.WriteString(sectionStyle.Render("Navigation") + "\n")
	b.WriteString(kv("j/k", "Move up/down") + "\n")
	b.WriteString(kv("G/gg", "Jump to bottom/top") + "\n")
	b.WriteString(kv("h/l", "Switch panels (top row)") + "\n")
	b.WriteString(kv("Tab", "Cycle panel focus") + "\n")
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Actions") + "\n")
	b.WriteString(kv("n", "New run") + "\n")
	b.WriteString(kv("p", "Pause") + "\n")
	b.WriteString(kv("r", "Resume / Retry") + "\n")
	b.WriteString(kv("c", "Cancel") + "\n")
	b.WriteString(kv("a", "Accept") + "\n")
	b.WriteString(kv("x", "Reject") + "\n")
	b.WriteString("\n")
	b.WriteString(sectionStyle.Render("Global") + "\n")
	b.WriteString(kv("/", "Filter runs") + "\n")
	b.WriteString(kv("?", "Toggle this help") + "\n")
	b.WriteString(kv("q", "Quit") + "\n")
	b.WriteString(kv("Esc", "Close modal"))

	bottomKb := []border.Keybind{{Key: "?", Label: " close"}, {Key: "Esc", Label: " close"}}
	return border.RenderPanel("Keybinds", b.String(), bottomKb, h.width, h.height, true)
}
