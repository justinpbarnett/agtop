package panels

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	cliputil "github.com/justinpbarnett/agtop/internal/ui/clipboard"
	"github.com/justinpbarnett/agtop/internal/ui/border"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

// pastedImageMsg is sent when an image is read from clipboard and saved to a temp file.
type pastedImageMsg struct{ path string }

// pastedTextMsg is sent when text is read from the clipboard.
type pastedTextMsg struct{ text string }

// pasteCmd reads the clipboard: images take priority over text.
func pasteCmd() tea.Cmd {
	return func() tea.Msg {
		data, _, err := cliputil.ReadImage()
		if err == nil && len(data) > 0 {
			if path, err := saveTempImage(data); err == nil {
				return pastedImageMsg{path: path}
			}
		}
		text, err := cliputil.ReadText()
		if err == nil && text != "" {
			return pastedTextMsg{text: text}
		}
		return nil
	}
}

// saveTempImage writes PNG data to a temp file and returns its path.
func saveTempImage(data []byte) (string, error) {
	f, err := os.CreateTemp("", "agtop-image-*.png")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

type workflowOption struct {
	name     string
	workflow string
}

type modelOption struct {
	name  string
	model string
}

var workflows = []workflowOption{
	{name: "auto", workflow: "auto"},
	{name: "build", workflow: "build"},
	{name: "plan", workflow: "plan-build"},
	{name: "sdlc", workflow: "sdlc"},
	{name: "quick", workflow: "quick-fix"},
}

var models = []modelOption{
	{name: "default", model: ""},
	{name: "haiku", model: "haiku"},
	{name: "opus", model: "opus"},
	{name: "sonnet", model: "sonnet"},
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
	attachedImages []string // paths to temp image files

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

func (m *NewRunModal) cleanupImages() {
	for _, path := range m.attachedImages {
		os.Remove(path)
	}
	m.attachedImages = nil
}

func (m *NewRunModal) Update(msg tea.Msg) (*NewRunModal, tea.Cmd) {
	switch msg := msg.(type) {
	case pastedImageMsg:
		m.attachedImages = append(m.attachedImages, msg.path)
		return m, nil
	case pastedTextMsg:
		m.promptInput.InsertString(msg.text)
		return m, nil
	case tea.KeyMsg:
		m.mouseSelecting = false
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cleanupImages()
			return nil, func() tea.Msg { return CloseModalMsg{} }
		case "ctrl+s":
			prompt := strings.TrimSpace(m.promptInput.Value())
			if prompt == "" {
				return m, nil
			}
			p, w, mo := prompt, m.workflow, m.model
			imgs := make([]string, len(m.attachedImages))
			copy(imgs, m.attachedImages)
			m.attachedImages = nil // images are handed off; don't clean up
			return nil, func() tea.Msg {
				return SubmitNewRunMsg{Prompt: p, Workflow: w, Model: mo, Images: imgs}
			}
		case "ctrl+v":
			return m, pasteCmd()
		case "alt+w":
			for i, w := range workflows {
				if w.workflow == m.workflow {
					m.workflow = workflows[(i+1)%len(workflows)].workflow
					return m, nil
				}
			}
			m.workflow = workflows[0].workflow
			return m, nil
		case "alt+m":
			for i, mo := range models {
				if mo.model == m.model {
					m.model = models[(i+1)%len(models)].model
					return m, nil
				}
			}
			m.model = models[0].model
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
	b.WriteString("\n")

	// Image indicator line (occupies the blank line between textarea and workflow)
	if n := len(m.attachedImages); n > 0 {
		noun := "image"
		if n > 1 {
			noun = "images"
		}
		b.WriteString(styles.TextSecondaryStyle.Render("Images    "))
		b.WriteString(styles.SelectedOptionStyle.Render(fmt.Sprintf("%d %s attached", n, noun)))
	}
	b.WriteString("\n")

	// Workflow row
	b.WriteString(styles.TextSecondaryStyle.Render("Workflow "))
	b.WriteString(keyStyle.Render("[M-w]"))
	b.WriteString("  ")
	for i, w := range workflows {
		if i > 0 {
			b.WriteString("  ")
		}
		if w.workflow == m.workflow {
			b.WriteString(selectedStyle.Render(w.name))
		} else {
			b.WriteString(keyStyle.Render(w.name))
		}
	}
	b.WriteString("\n")

	// Model row
	b.WriteString(styles.TextSecondaryStyle.Render("Model    "))
	b.WriteString(keyStyle.Render("[M-m]"))
	b.WriteString("  ")
	for i, mo := range models {
		if i > 0 {
			b.WriteString("  ")
		}
		if mo.model == m.model {
			b.WriteString(selectedStyle.Render(mo.name))
		} else {
			b.WriteString(keyStyle.Render(mo.name))
		}
	}

	bottomKb := []border.Keybind{
		{Key: "^S", Label: " submit"},
		{Key: "Esc", Label: " cancel"},
		{Key: "^V", Label: " paste/img"},
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
