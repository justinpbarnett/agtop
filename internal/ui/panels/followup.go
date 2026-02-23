package panels

import (
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

// SubmitFollowUpMsg is sent when the user confirms the follow-up modal.
type SubmitFollowUpMsg struct {
	RunID  string
	Prompt string
}

type FollowUpModal struct {
	runID          string
	originalPrompt string
	promptInput    textarea.Model
	width          int
	height         int
	textareaHeight int
}

func NewFollowUpModal(runID, originalPrompt string, screenW, screenH int) *FollowUpModal {
	ta := textarea.New()
	ta.Placeholder = "Follow-up instructions..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.Focus()

	m := &FollowUpModal{
		runID:          runID,
		originalPrompt: originalPrompt,
		promptInput:    ta,
	}
	m.SetSize(screenW, screenH)
	return m
}

func (m *FollowUpModal) SetSize(screenW, screenH int) {
	m.width = screenW * 80 / 100
	m.height = screenH * 80 / 100
	if m.width < 40 {
		m.width = 40
	}
	if m.height < 10 {
		m.height = 10
	}

	innerW := m.width - 2
	// inner height = total - 2 (borders) - 3 (blank line + context line + blank)
	m.textareaHeight = m.height - 5
	if m.textareaHeight < 3 {
		m.textareaHeight = 3
	}
	m.promptInput.SetWidth(innerW)
	m.promptInput.SetHeight(m.textareaHeight)
}

func (m *FollowUpModal) Init() tea.Cmd {
	return m.promptInput.Focus()
}

func (m *FollowUpModal) Update(msg tea.Msg) (*FollowUpModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return nil, func() tea.Msg { return CloseModalMsg{} }
		case "ctrl+s":
			prompt := strings.TrimSpace(m.promptInput.Value())
			if prompt == "" {
				return m, nil
			}
			rid, p := m.runID, prompt
			return nil, func() tea.Msg {
				return SubmitFollowUpMsg{RunID: rid, Prompt: p}
			}
		}
	}

	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

func (m *FollowUpModal) View() string {
	var b strings.Builder

	// Context line showing what run this follows up on
	orig := m.originalPrompt
	if len(orig) > 55 {
		orig = orig[:52] + "..."
	}
	b.WriteString(styles.TextSecondaryStyle.Render("Run "))
	b.WriteString(styles.TextDimStyle.Render(m.runID))
	b.WriteString(styles.TextSecondaryStyle.Render(" | "))
	b.WriteString(styles.TextDimStyle.Render(orig))
	b.WriteString("\n\n")

	b.WriteString(m.promptInput.View())

	bottomKb := []border.Keybind{
		{Key: "^S", Label: " submit"},
		{Key: "Esc", Label: " cancel"},
	}
	return border.RenderPanel("Follow Up", b.String(), bottomKb, m.width, m.height, true)
}
