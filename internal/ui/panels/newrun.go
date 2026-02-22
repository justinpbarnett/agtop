package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

type workflowOption struct {
	key      string
	name     string
	workflow string
}

type modelOption struct {
	key   string
	name  string
	model string
}

var workflows = []workflowOption{
	{key: "^b", name: "build", workflow: "build"},
	{key: "^p", name: "plan", workflow: "plan-build"},
	{key: "^s", name: "sdlc", workflow: "sdlc"},
	{key: "^q", name: "quick", workflow: "quick-fix"},
}

var models = []modelOption{
	{key: "^h", name: "haiku", model: "haiku"},
	{key: "^o", name: "opus", model: "opus"},
	{key: "^n", name: "sonnet", model: "sonnet"},
	{key: "^x", name: "default", model: ""},
}

type NewRunModal struct {
	promptInput textinput.Model
	workflow    string
	model       string
	width       int
	height      int
}

func NewNewRunModal() *NewRunModal {
	ti := textinput.New()
	ti.Placeholder = "Describe the task..."
	ti.CharLimit = 256
	ti.Width = 50
	ti.Focus()

	return &NewRunModal{
		promptInput: ti,
		workflow:    "build",
		model:       "",
		width:       56,
		height:      12,
	}
}

func (m *NewRunModal) Init() tea.Cmd {
	return textinput.Blink
}

func (m *NewRunModal) Update(msg tea.Msg) (*NewRunModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			return nil, func() tea.Msg { return CloseModalMsg{} }
		case "enter":
			prompt := strings.TrimSpace(m.promptInput.Value())
			if prompt == "" {
				return m, nil
			}
			p, w, mo := prompt, m.workflow, m.model
			return nil, func() tea.Msg {
				return SubmitNewRunMsg{Prompt: p, Workflow: w, Model: mo}
			}
		case "ctrl+b":
			m.workflow = "build"
			return m, nil
		case "ctrl+p":
			m.workflow = "plan-build"
			return m, nil
		case "ctrl+s":
			m.workflow = "sdlc"
			return m, nil
		case "ctrl+q":
			m.workflow = "quick-fix"
			return m, nil
		case "ctrl+h":
			m.model = "haiku"
			return m, nil
		case "ctrl+o":
			m.model = "opus"
			return m, nil
		case "ctrl+n":
			m.model = "sonnet"
			return m, nil
		case "ctrl+x":
			m.model = ""
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

func (m *NewRunModal) View() string {
	keyStyle := styles.TextDimStyle
	selectedStyle := styles.SelectedOptionStyle

	var b strings.Builder

	b.WriteString(m.promptInput.View())
	b.WriteString("\n\n")

	// Workflow row
	b.WriteString(styles.TextSecondaryStyle.Render("Workflow  "))
	for i, w := range workflows {
		if i > 0 {
			b.WriteString("  ")
		}
		label := fmt.Sprintf("[%s] %s", w.key, w.name)
		if w.workflow == m.workflow {
			b.WriteString(selectedStyle.Render(label))
		} else {
			b.WriteString(keyStyle.Render(label))
		}
	}
	b.WriteString("\n")

	// Model row
	b.WriteString(styles.TextSecondaryStyle.Render("Model     "))
	for i, mo := range models {
		if i > 0 {
			b.WriteString("  ")
		}
		label := fmt.Sprintf("[%s] %s", mo.key, mo.name)
		if mo.model == m.model {
			b.WriteString(selectedStyle.Render(label))
		} else {
			b.WriteString(keyStyle.Render(label))
		}
	}

	bottomKb := []border.Keybind{
		{Key: "Enter", Label: " submit"},
		{Key: "Esc", Label: " cancel"},
	}
	return border.RenderPanel("New Run", b.String(), bottomKb, m.width, m.height, true)
}

// Workflow returns the currently selected workflow.
func (m *NewRunModal) Workflow() string { return m.workflow }

// Model returns the currently selected model override.
func (m *NewRunModal) Model() string { return m.model }

// PromptValue returns the current text input value.
func (m *NewRunModal) PromptValue() string { return m.promptInput.Value() }
