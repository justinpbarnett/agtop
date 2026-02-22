package tui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jpb/agtop/internal/config"
)

const minWidth = 40
const minHeight = 10

type App struct {
	config       *config.Config
	width        int
	height       int
	focusedPanel int
	runList      RunList
	detail       Detail
	statusBar    StatusBar
	modal        *Modal
	keys         KeyMap
	ready        bool
}

func NewApp(cfg *config.Config) App {
	rl := NewRunList()
	d := NewDetail()
	d.SetRun(rl.SelectedRun())
	return App{
		config:    cfg,
		runList:   rl,
		detail:    d,
		statusBar: NewStatusBar(),
		keys:      DefaultKeyMap(),
	}
}

func (a App) Init() tea.Cmd {
	return nil
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.propagateSizes()
		return a, nil

	case CloseModalMsg:
		a.modal = nil
		return a, nil

	case tea.KeyMsg:
		if a.modal != nil {
			var cmd tea.Cmd
			*a.modal, cmd = a.modal.Update(msg)
			return a, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			return a, tea.Quit
		case "tab":
			a.focusedPanel = (a.focusedPanel + 1) % 2
			return a, nil
		case "?":
			if a.modal == nil {
				a.modal = NewHelpModal()
			} else {
				a.modal = nil
			}
			return a, nil
		}

		return a.routeKey(msg)
	}
	return a, nil
}

func (a App) View() string {
	if !a.ready {
		return lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, "Loading...")
	}

	if a.width < minWidth || a.height < minHeight {
		return lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, "Terminal too small")
	}

	leftWidth, rightWidth, contentHeight := a.panelDimensions()

	var leftStyle, rightStyle lipgloss.Style
	if a.focusedPanel == 0 {
		leftStyle = ActivePanelStyle(leftWidth, contentHeight)
		rightStyle = PanelStyle(rightWidth, contentHeight)
	} else {
		leftStyle = PanelStyle(leftWidth, contentHeight)
		rightStyle = ActivePanelStyle(rightWidth, contentHeight)
	}

	left := leftStyle.Render(a.runList.View())
	right := rightStyle.Render(a.detail.View())

	panels := lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	layout := lipgloss.JoinVertical(lipgloss.Left, panels, a.statusBar.View())

	if a.modal != nil {
		modalView := a.modal.View()
		layout = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
		)
	}

	return layout
}

func (a App) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if a.focusedPanel == 0 {
		var cmd tea.Cmd
		a.runList, cmd = a.runList.Update(msg)
		a.detail.SetRun(a.runList.SelectedRun())
		return a, cmd
	}

	var cmd tea.Cmd
	a.detail, cmd = a.detail.Update(msg)
	return a, cmd
}

func (a *App) propagateSizes() {
	leftWidth, rightWidth, contentHeight := a.panelDimensions()
	a.runList.SetSize(leftWidth, contentHeight)
	a.detail.SetSize(rightWidth, contentHeight)
	a.statusBar.SetSize(a.width)
}

func (a App) panelDimensions() (leftWidth, rightWidth, contentHeight int) {
	leftWidth = a.width*30/100 - 2
	rightWidth = a.width - leftWidth - 6
	contentHeight = a.height - 4
	if leftWidth < 0 {
		leftWidth = 0
	}
	if rightWidth < 0 {
		rightWidth = 0
	}
	if contentHeight < 0 {
		contentHeight = 0
	}
	return
}
