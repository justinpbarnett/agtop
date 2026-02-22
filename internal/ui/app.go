package ui

import (
	"fmt"
	"log"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/engine"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
	"github.com/justinpbarnett/agtop/internal/ui/layout"
	"github.com/justinpbarnett/agtop/internal/ui/panels"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
)

const (
	panelRunList = 0
	panelLogView = 1
	panelDetail  = 2
	numPanels    = 3
)

type App struct {
	config       *config.Config
	store        *run.Store
	manager      *process.Manager
	registry     *engine.Registry
	width        int
	height       int
	layout       layout.Layout
	focusedPanel int
	runList      panels.RunList
	logView      panels.LogView
	detail       panels.Detail
	statusBar    panels.StatusBar
	helpOverlay  *panels.HelpOverlay
	keys         KeyMap
	ready        bool
}

func NewApp(cfg *config.Config) App {
	store := run.NewStore()

	var mgr *process.Manager
	rt, err := runtime.NewClaudeRuntime()
	if err != nil {
		log.Printf("warning: %v (running with mock data)", err)
	} else {
		mgr = process.NewManager(store, rt, &cfg.Limits)
	}

	reg := engine.NewRegistry(cfg)
	projectRoot := cfg.Project.Root
	if projectRoot == "" || projectRoot == "." {
		projectRoot, _ = os.Getwd()
	}
	if err := reg.Load(projectRoot); err != nil {
		log.Printf("warning: skill registry load: %v", err)
	}

	seedMockData(store)

	rl := panels.NewRunList(store)
	rl.SetFocused(true)
	lv := panels.NewLogView()
	d := panels.NewDetail()

	selected := rl.SelectedRun()
	d.SetRun(selected)
	if selected != nil && mgr != nil {
		lv.SetRun(selected.ID, selected.CurrentSkill, selected.Branch, mgr.Buffer(selected.ID), !selected.IsTerminal())
	}

	return App{
		config:    cfg,
		store:     store,
		manager:   mgr,
		registry:  reg,
		runList:   rl,
		logView:   lv,
		detail:    d,
		statusBar: panels.NewStatusBar(store),
		keys:      DefaultKeyMap(),
	}
}

func (a App) Init() tea.Cmd {
	return listenForChanges(a.store.Changes())
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.layout = layout.Calculate(msg.Width, msg.Height)
		a.propagateSizes()
		return a, nil

	case CloseModalMsg:
		a.helpOverlay = nil
		return a, nil

	case RunStoreUpdatedMsg:
		var cmd tea.Cmd
		a.runList, cmd = a.runList.Update(msg)
		a.syncSelection()
		cmds := []tea.Cmd{cmd, listenForChanges(a.store.Changes())}
		return a, tea.Batch(cmds...)

	case process.LogLineMsg:
		selected := a.runList.SelectedRun()
		if selected != nil && selected.ID == msg.RunID {
			var cmd tea.Cmd
			a.logView, cmd = a.logView.Update(LogLineMsg{RunID: msg.RunID})
			return a, cmd
		}
		return a, nil

	case LogLineMsg:
		var cmd tea.Cmd
		a.logView, cmd = a.logView.Update(msg)
		return a, cmd

	case tea.KeyMsg:
		if a.helpOverlay != nil {
			var cmd tea.Cmd
			*a.helpOverlay, cmd = a.helpOverlay.Update(msg)
			return a, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "q":
			return a, tea.Quit
		case "tab":
			a.focusedPanel = (a.focusedPanel + 1) % numPanels
			a.updateFocusState()
			return a, nil
		case "h", "left":
			// Spatial: in top row, move between run list and log view
			if a.focusedPanel == panelLogView {
				a.focusedPanel = panelRunList
				a.updateFocusState()
			}
			return a, nil
		case "l", "right":
			// Spatial: in top row, move between run list and log view
			if a.focusedPanel == panelRunList {
				a.focusedPanel = panelLogView
				a.updateFocusState()
			}
			return a, nil
		case "?":
			if a.helpOverlay == nil {
				a.helpOverlay = panels.NewHelpOverlay()
			} else {
				a.helpOverlay = nil
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

	if a.layout.TooSmall {
		msg := fmt.Sprintf("Terminal too small (%d×%d)\nMinimum: %d×%d",
			a.width, a.height, layout.MinWidth, layout.MinHeight)
		return lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, msg)
	}

	// Render panels
	runListView := a.runList.View()
	logViewView := a.logView.View()
	detailView := a.detail.View()
	statusBarView := a.statusBar.View()

	// Assemble layout: top row (runlist | logview), bottom row (detail), status bar
	topRow := lipgloss.JoinHorizontal(lipgloss.Top, runListView, logViewView)
	fullLayout := lipgloss.JoinVertical(lipgloss.Left, topRow, detailView, statusBarView)

	if a.helpOverlay != nil {
		modalView := a.helpOverlay.View()
		fullLayout = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(styles.TextDim),
		)
	}

	return fullLayout
}

func (a App) Manager() *process.Manager {
	return a.manager
}

func (a App) Registry() *engine.Registry {
	return a.registry
}

func (a App) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.focusedPanel {
	case panelRunList:
		var cmd tea.Cmd
		a.runList, cmd = a.runList.Update(msg)
		a.syncSelection()
		return a, cmd
	case panelLogView:
		var cmd tea.Cmd
		a.logView, cmd = a.logView.Update(msg)
		return a, cmd
	case panelDetail:
		var cmd tea.Cmd
		a.detail, cmd = a.detail.Update(msg)
		return a, cmd
	}
	return a, nil
}

func (a *App) syncSelection() {
	selected := a.runList.SelectedRun()
	a.detail.SetRun(selected)
	if selected != nil {
		var buf *process.RingBuffer
		if a.manager != nil {
			buf = a.manager.Buffer(selected.ID)
		}
		a.logView.SetRun(selected.ID, selected.CurrentSkill, selected.Branch, buf, !selected.IsTerminal())
	}
}

func (a *App) propagateSizes() {
	l := a.layout
	a.runList.SetSize(l.RunListWidth, l.RunListHeight)
	a.logView.SetSize(l.LogViewWidth, l.LogViewHeight)
	a.detail.SetSize(l.DetailWidth, l.DetailHeight)
	a.statusBar.SetSize(l.StatusBarWidth)
}

func (a *App) updateFocusState() {
	a.runList.SetFocused(a.focusedPanel == panelRunList)
	a.logView.SetFocused(a.focusedPanel == panelLogView)
	a.detail.SetFocused(a.focusedPanel == panelDetail)
}

func listenForChanges(ch <-chan struct{}) tea.Cmd {
	return func() tea.Msg {
		<-ch
		return RunStoreUpdatedMsg{}
	}
}

func seedMockData(store *run.Store) {
	now := time.Now()
	store.Add(&run.Run{
		Branch:       "feat/add-auth",
		Workflow:     "sdlc",
		State:        run.StateRunning,
		SkillIndex:   3,
		SkillTotal:   7,
		Tokens:       12400,
		TokensIn:     8200,
		TokensOut:    4200,
		Cost:         0.42,
		CreatedAt:    now.Add(-30 * time.Minute),
		StartedAt:    now.Add(-25 * time.Minute),
		CurrentSkill: "build",
		Model:        "claude-sonnet-4-5",
		Command:      `claude -p "add JWT auth" --output-format stream-json`,
	})
	store.Add(&run.Run{
		Branch:       "fix/nav-bug",
		Workflow:     "quick-fix",
		State:        run.StatePaused,
		SkillIndex:   1,
		SkillTotal:   3,
		Tokens:       3100,
		TokensIn:     2100,
		TokensOut:    1000,
		Cost:         0.08,
		CreatedAt:    now.Add(-20 * time.Minute),
		StartedAt:    now.Add(-18 * time.Minute),
		CurrentSkill: "build",
		Model:        "claude-sonnet-4-5",
	})
	store.Add(&run.Run{
		Branch:       "feat/dashboard",
		Workflow:     "plan-build",
		State:        run.StateReviewing,
		SkillIndex:   3,
		SkillTotal:   3,
		Tokens:       45200,
		TokensIn:     32000,
		TokensOut:    13200,
		Cost:         1.23,
		CreatedAt:    now.Add(-60 * time.Minute),
		StartedAt:    now.Add(-55 * time.Minute),
		Model:        "claude-sonnet-4-5",
	})
	store.Add(&run.Run{
		Branch:       "fix/css-overflow",
		Workflow:     "build",
		State:        run.StateFailed,
		SkillIndex:   2,
		SkillTotal:   3,
		Tokens:       8700,
		TokensIn:     6200,
		TokensOut:    2500,
		Cost:         0.31,
		CreatedAt:    now.Add(-10 * time.Minute),
		StartedAt:    now.Add(-8 * time.Minute),
		Model:        "claude-sonnet-4-5",
		Error:        "build skill timed out",
	})
}
