package panels

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
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
	{key: "M-a", name: "auto", workflow: "auto"},
	{key: "M-b", name: "build", workflow: "build"},
	{key: "M-p", name: "plan", workflow: "plan-build"},
	{key: "M-l", name: "sdlc", workflow: "sdlc"},
	{key: "M-q", name: "quick", workflow: "quick-fix"},
}

var models = []modelOption{
	{key: "M-x", name: "default", model: ""},
	{key: "M-h", name: "haiku", model: "haiku"},
	{key: "M-o", name: "opus", model: "opus"},
	{key: "M-n", name: "sonnet", model: "sonnet"},
}

type NewRunModal struct {
	promptInput    textarea.Model
	workflow       string
	model          string
	width          int
	height         int
	screenW        int
	screenH        int
	textareaHeight int

	// Mouse selection state
	mouseSelecting   bool
	mouseAnchorLine  int
	mouseAnchorCol   int
	mouseCurrentLine int
	mouseCurrentCol  int
}

func NewNewRunModal(screenW, screenH int) *NewRunModal {
	ta := textarea.New()
	ta.Placeholder = "Describe the task..."
	ta.ShowLineNumbers = false
	ta.CharLimit = 0
	ta.Focus()

	m := &NewRunModal{
		promptInput: ta,
		workflow:    "auto",
		model:       "",
	}
	m.SetSize(screenW, screenH)
	return m
}

func (m *NewRunModal) SetSize(screenW, screenH int) {
	m.screenW = screenW
	m.screenH = screenH
	m.width = screenW * 80 / 100
	m.height = screenH * 80 / 100
	if m.width < 40 {
		m.width = 40
	}
	if m.height < 10 {
		m.height = 10
	}

	innerW := m.width - 2
	// inner height = total - 2 (borders) - 3 (blank line + workflow + model)
	m.textareaHeight = m.height - 5
	if m.textareaHeight < 3 {
		m.textareaHeight = 3
	}
	m.promptInput.SetWidth(innerW)
	m.promptInput.SetHeight(m.textareaHeight)
}

func (m *NewRunModal) Init() tea.Cmd {
	return m.promptInput.Focus()
}

func (m *NewRunModal) Update(msg tea.Msg) (*NewRunModal, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		m.mouseSelecting = false
		switch msg.String() {
		case "esc", "ctrl+c":
			return nil, func() tea.Msg { return CloseModalMsg{} }
		case "ctrl+s":
			prompt := strings.TrimSpace(m.promptInput.Value())
			if prompt == "" {
				return m, nil
			}
			p, w, mo := prompt, m.workflow, m.model
			return nil, func() tea.Msg {
				return SubmitNewRunMsg{Prompt: p, Workflow: w, Model: mo}
			}
		case "alt+a":
			m.workflow = "auto"
			return m, nil
		case "alt+b":
			m.workflow = "build"
			return m, nil
		case "alt+p":
			m.workflow = "plan-build"
			return m, nil
		case "alt+l":
			m.workflow = "sdlc"
			return m, nil
		case "alt+q":
			m.workflow = "quick-fix"
			return m, nil
		case "alt+h":
			m.model = "haiku"
			return m, nil
		case "alt+o":
			m.model = "opus"
			return m, nil
		case "alt+n":
			m.model = "sonnet"
			return m, nil
		case "alt+x":
			m.model = ""
			return m, nil
		}
	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

// handleMouse processes mouse events for text selection within the textarea.
func (m *NewRunModal) handleMouse(msg tea.MouseMsg) (*NewRunModal, tea.Cmd) {
	line, col, ok := m.mouseToTextarea(msg.X, msg.Y)

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button != tea.MouseButtonLeft || !ok {
			return m, nil
		}
		m.mouseSelecting = true
		m.mouseAnchorLine = line
		m.mouseAnchorCol = col
		m.mouseCurrentLine = line
		m.mouseCurrentCol = col
		return m, nil

	case tea.MouseActionMotion:
		if !m.mouseSelecting {
			return m, nil
		}
		if ok {
			m.mouseCurrentLine = line
			m.mouseCurrentCol = col
		}
		return m, nil

	case tea.MouseActionRelease:
		if !m.mouseSelecting {
			return m, nil
		}
		if ok {
			m.mouseCurrentLine = line
			m.mouseCurrentCol = col
		}

		// Single click — no selection, just clear
		if m.mouseAnchorLine == m.mouseCurrentLine && m.mouseAnchorCol == m.mouseCurrentCol {
			m.mouseSelecting = false
			return m, nil
		}

		// Extract selected text and copy
		taView := m.promptInput.View()
		sl, sc, el, ec := m.normalizedSelection()
		text := extractCharSelection(taView, sl, sc, el, ec)
		m.mouseSelecting = false
		if text != "" {
			return m, func() tea.Msg { return YankMsg{Text: text} }
		}
		return m, nil
	}
	return m, nil
}

// mouseToTextarea converts absolute screen coordinates to textarea-relative
// line and column. Returns false if the click is outside the textarea area.
func (m *NewRunModal) mouseToTextarea(absX, absY int) (line, col int, ok bool) {
	// Modal top-left on screen (centered by lipgloss.Place)
	modalX := (m.screenW - m.width) / 2
	modalY := (m.screenH - m.height) / 2

	// Textarea sits inside the panel border: 1 row down, 1 col in
	taX := modalX + 1
	taY := modalY + 1

	relX := absX - taX
	relY := absY - taY

	if relX < 0 || relY < 0 || relX >= m.width-2 || relY >= m.textareaHeight {
		return 0, 0, false
	}
	return relY, relX, true
}

// normalizedSelection returns the mouse selection with start before end.
func (m *NewRunModal) normalizedSelection() (startLine, startCol, endLine, endCol int) {
	startLine, startCol = m.mouseAnchorLine, m.mouseAnchorCol
	endLine, endCol = m.mouseCurrentLine, m.mouseCurrentCol
	if startLine > endLine || (startLine == endLine && startCol > endCol) {
		startLine, startCol, endLine, endCol = endLine, endCol, startLine, startCol
	}
	return
}

func (m *NewRunModal) View() string {
	keyStyle := styles.TextDimStyle
	selectedStyle := styles.SelectedOptionStyle

	var b strings.Builder

	taView := m.promptInput.View()
	if m.mouseSelecting {
		sl, sc, el, ec := m.normalizedSelection()
		taView = m.applySelectionHighlight(taView, sl, sc, el, ec)
	}
	b.WriteString(taView)
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
		{Key: "^S", Label: " submit"},
		{Key: "Esc", Label: " cancel"},
		{Key: "M-·", Label: " workflow/model"},
	}
	return border.RenderPanel("New Run", b.String(), bottomKb, m.width, m.height, true)
}

// Workflow returns the currently selected workflow.
func (m *NewRunModal) Workflow() string { return m.workflow }

// Model returns the currently selected model override.
func (m *NewRunModal) Model() string { return m.model }

// PromptValue returns the current text input value.
func (m *NewRunModal) PromptValue() string { return m.promptInput.Value() }

// applySelectionHighlight overlays selection styling on rendered textarea content.
func (m *NewRunModal) applySelectionHighlight(content string, startLine, startCol, endLine, endCol int) string {
	lines := strings.Split(content, "\n")
	for i := range lines {
		if i < startLine || i > endLine {
			continue
		}
		lineWidth := ansi.StringWidth(lines[i])
		if lineWidth == 0 {
			continue
		}

		var sc, ec int
		if i == startLine && i == endLine {
			sc = startCol
			ec = endCol + 1
		} else if i == startLine {
			sc = startCol
			ec = lineWidth
		} else if i == endLine {
			sc = 0
			ec = endCol + 1
		} else {
			sc = 0
			ec = lineWidth
		}

		if sc > lineWidth {
			sc = lineWidth
		}
		if ec > lineWidth {
			ec = lineWidth
		}
		if sc >= ec {
			continue
		}

		before := ansi.Cut(lines[i], 0, sc)
		selected := ansi.Cut(lines[i], sc, ec)
		after := ansi.Cut(lines[i], ec, lineWidth)
		lines[i] = before + styles.SelectionStyle.Render(ansi.Strip(selected)) + after
	}
	return strings.Join(lines, "\n")
}
