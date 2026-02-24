package panels

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/ui/text"
)

// RunPickerModal shows a compact dropdown of active (non-terminal) runs,
// allowing the user to quickly switch focus to any concurrent run.
type RunPickerModal struct {
	runs     []run.Run
	selected int
	width    int
	height   int
}

// NewRunPickerModal creates a new run picker populated with the given runs.
func NewRunPickerModal(runs []run.Run, screenW, screenH int) *RunPickerModal {
	m := &RunPickerModal{
		runs: runs,
	}
	m.computeSize(screenW, screenH)
	return m
}

func (m *RunPickerModal) computeSize(screenW, _ int) {
	m.width = screenW * 60 / 100
	if m.width < 44 {
		m.width = 44
	}
	if m.width > 72 {
		m.width = 72
	}

	rows := len(m.runs)
	if rows == 0 {
		rows = 1
	}
	if rows > 10 {
		rows = 10
	}
	// 2 borders + 1 header row + run rows
	m.height = 2 + 1 + rows
}

func (m *RunPickerModal) Update(msg tea.Msg) (*RunPickerModal, tea.Cmd) {
	msg2, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch msg2.String() {
	case "esc", "ctrl+c":
		return nil, func() tea.Msg { return CloseModalMsg{} }
	case "j", "down":
		if m.selected < len(m.runs)-1 {
			m.selected++
		}
	case "k", "up":
		if m.selected > 0 {
			m.selected--
		}
	case "enter":
		if len(m.runs) > 0 {
			runID := m.runs[m.selected].ID
			return nil, func() tea.Msg { return SelectRunMsg{RunID: runID} }
		}
		return nil, func() tea.Msg { return CloseModalMsg{} }
	}
	return m, nil
}

func (m *RunPickerModal) View() string {
	innerWidth := m.width - 2
	var b strings.Builder

	// Column header
	header := fmt.Sprintf("%*s %*s  %-*s %*s",
		colIconW, "",
		colIDW, "ID",
		colStateW, "STATE",
		colTimeW, "TIME",
	)
	b.WriteString(styles.TextSecondaryStyle.Render(text.Truncate(header, innerWidth)))
	b.WriteString("\n")

	if len(m.runs) == 0 {
		b.WriteString(styles.TextDimStyle.Render("No active runs."))
	} else {
		for i, rn := range m.runs {
			elapsed := text.FormatElapsed(rn.ElapsedTime())
			statusIcon := rn.StatusIcon()

			var line string
			if i == m.selected {
				plainLine := fmt.Sprintf("%s %*s  %-*s %*s",
					text.PadRight(statusIcon, colIconW),
					colIDW, rn.ID,
					colStateW, text.Truncate(string(rn.State), colStateW),
					colTimeW, elapsed,
				)
				plainLine = text.Truncate(plainLine, innerWidth)
				line = styles.SelectedRowStyle.Width(innerWidth).Render(plainLine)
			} else {
				icon := lipgloss.NewStyle().Foreground(styles.RunStateColor(rn.State)).Render(
					text.PadRight(statusIcon, colIconW),
				)
				line = fmt.Sprintf("%s %*s  %-*s %*s",
					icon,
					colIDW, rn.ID,
					colStateW, text.Truncate(string(rn.State), colStateW),
					colTimeW, elapsed,
				)
				line = text.Truncate(line, innerWidth)
			}
			b.WriteString(line)
			if i < len(m.runs)-1 {
				b.WriteString("\n")
			}
		}
	}

	keybinds := []border.Keybind{
		{Key: "↵", Label: " select"},
		{Key: "j/k", Label: " navigate"},
		{Key: "Esc", Label: " cancel"},
	}
	return border.RenderPanel("Active Runs", b.String(), keybinds, m.width, m.height, true)
}
