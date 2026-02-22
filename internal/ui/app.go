package ui

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/cost"
	"github.com/justinpbarnett/agtop/internal/engine"
	gitpkg "github.com/justinpbarnett/agtop/internal/git"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
	"github.com/justinpbarnett/agtop/internal/safety"
	"github.com/justinpbarnett/agtop/internal/server"
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

// StartRunMsg triggers the executor to create and start a new run.
type StartRunMsg struct {
	Prompt   string
	Workflow string
}

type App struct {
	config       *config.Config
	store        *run.Store
	manager      *process.Manager
	registry     *engine.Registry
	executor     *engine.Executor
	worktrees    *gitpkg.WorktreeManager
	devServers   *server.DevServerManager
	diffGen      *gitpkg.DiffGenerator
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

	tracker := cost.NewTracker()
	limiter := &cost.LimitChecker{
		MaxTokensPerRun: cfg.Limits.MaxTokensPerRun,
		MaxCostPerRun:   cfg.Limits.MaxCostPerRun,
	}

	var safetyMatcher *safety.PatternMatcher
	safetyEngine, safetyErr := safety.NewHookEngine(cfg.Safety)
	if safetyErr != nil {
		log.Printf("warning: %v", safetyErr)
	}
	if safetyEngine != nil {
		safetyMatcher = safetyEngine.Matcher()
	}

	var mgr *process.Manager
	rt, err := runtime.NewClaudeRuntime()
	if err != nil {
		log.Printf("warning: %v (running with mock data)", err)
	} else {
		mgr = process.NewManager(store, rt, &cfg.Limits, tracker, limiter, safetyMatcher)
	}

	reg := engine.NewRegistry(cfg)
	projectRoot := cfg.Project.Root
	if projectRoot == "" || projectRoot == "." {
		projectRoot, _ = os.Getwd()
	}
	if err := reg.Load(projectRoot); err != nil {
		log.Printf("warning: skill registry load: %v", err)
	}

	var exec *engine.Executor
	if mgr != nil {
		exec = engine.NewExecutor(store, mgr, reg, cfg)
	}

	wt := gitpkg.NewWorktreeManager(projectRoot)
	dg := gitpkg.NewDiffGenerator(projectRoot)
	ds := server.NewDevServerManager(cfg.Project.DevServer)

	seedMockData(store)

	rl := panels.NewRunList(store)
	rl.SetFocused(true)
	lv := panels.NewLogView()
	if cfg.UI.LogScrollSpeed > 0 {
		lv.SetScrollSpeed(cfg.UI.LogScrollSpeed)
	}
	d := panels.NewDetail()

	selected := rl.SelectedRun()
	d.SetRun(selected)
	if selected != nil && mgr != nil {
		lv.SetRun(selected.ID, selected.CurrentSkill, selected.Branch, mgr.Buffer(selected.ID), !selected.IsTerminal())
	}

	return App{
		config:     cfg,
		store:      store,
		manager:    mgr,
		registry:   reg,
		executor:   exec,
		worktrees:  wt,
		diffGen:    dg,
		devServers: ds,
		runList:    rl,
		logView:    lv,
		detail:     d,
		statusBar:  panels.NewStatusBar(store),
		keys:       DefaultKeyMap(),
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

	case DiffResultMsg:
		selected := a.runList.SelectedRun()
		if selected == nil || selected.ID != msg.RunID {
			return a, nil
		}
		if msg.Err != nil {
			a.detail.SetDiffError(msg.Err.Error())
		} else {
			a.detail.SetDiff(msg.Diff, msg.DiffStat)
		}
		return a, nil

	case panels.DiffGTimerExpiredMsg:
		var cmd tea.Cmd
		a.detail, cmd = a.detail.Update(msg)
		return a, cmd

	case RunStoreUpdatedMsg:
		var cmd tea.Cmd
		a.runList, cmd = a.runList.Update(msg)
		diffCmd := a.syncSelection()
		a.autoStartDevServers()
		cmds := []tea.Cmd{cmd, diffCmd, listenForChanges(a.store.Changes())}
		return a, tea.Batch(cmds...)

	case StartRunMsg:
		if a.executor != nil {
			newRun := &run.Run{
				Workflow:  msg.Workflow,
				State:     run.StateQueued,
				CreatedAt: time.Now(),
			}
			runID := a.store.Add(newRun)

			wtPath, branch, err := a.worktrees.Create(runID)
			if err != nil {
				a.store.Update(runID, func(r *run.Run) {
					r.State = run.StateFailed
					r.Error = fmt.Sprintf("worktree create: %v", err)
				})
				return a, nil
			}

			a.store.Update(runID, func(r *run.Run) {
				r.Worktree = wtPath
				r.Branch = branch
			})

			a.executor.Execute(runID, msg.Workflow, msg.Prompt)
		}
		return a, nil

	case process.CostThresholdMsg:
		a.statusBar.SetFlash(msg.Reason)
		return a, flashClearCmd()

	case ClearFlashMsg:
		a.statusBar.ClearFlash()
		return a, nil

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

	case panels.GTimerExpiredMsg:
		var cmd tea.Cmd
		a.logView, cmd = a.logView.Update(msg)
		return a, cmd

	case tea.KeyMsg:
		if a.helpOverlay != nil {
			var cmd tea.Cmd
			*a.helpOverlay, cmd = a.helpOverlay.Update(msg)
			return a, cmd
		}

		// When the log view is in search mode, route keys directly to it
		// so that typing and n/N navigation aren't intercepted by global handlers.
		if a.focusedPanel == panelLogView && a.logView.ConsumesKeys() {
			switch msg.String() {
			case "ctrl+c":
				a.devServers.StopAll()
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.logView, cmd = a.logView.Update(msg)
			return a, cmd
		}

		switch msg.String() {
		case "ctrl+c":
			a.devServers.StopAll()
			return a, tea.Quit
		case "q":
			a.devServers.StopAll()
			return a, tea.Quit
		case "tab":
			a.focusedPanel = (a.focusedPanel + 1) % numPanels
			a.updateFocusState()
			return a, nil
		case "h", "left":
			// Spatial: in top row, move between run list and log view.
			// When the detail panel is focused, delegate to routeKey so
			// the detail panel can use h/l for tab switching.
			if a.focusedPanel == panelDetail {
				return a.routeKey(msg)
			}
			if a.focusedPanel == panelLogView {
				a.focusedPanel = panelRunList
				a.updateFocusState()
			}
			return a, nil
		case "l", "right":
			if a.focusedPanel == panelDetail {
				return a.routeKey(msg)
			}
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
		case "n":
			return a, func() tea.Msg {
				return StartRunMsg{
					Prompt:   "placeholder task",
					Workflow: "build",
				}
			}
		case "a":
			return a.handleAccept()
		case "x":
			return a.handleReject()
		case "d":
			return a.handleDevServerToggle()
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

func (a App) Executor() *engine.Executor {
	return a.executor
}

func (a App) routeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.focusedPanel {
	case panelRunList:
		var cmd tea.Cmd
		a.runList, cmd = a.runList.Update(msg)
		diffCmd := a.syncSelection()
		return a, tea.Batch(cmd, diffCmd)
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

func (a *App) syncSelection() tea.Cmd {
	selected := a.runList.SelectedRun()
	a.detail.SetRun(selected)
	if selected != nil {
		var buf *process.RingBuffer
		if a.manager != nil {
			buf = a.manager.Buffer(selected.ID)
		}
		a.logView.SetRun(selected.ID, selected.CurrentSkill, selected.Branch, buf, !selected.IsTerminal())

		if selected.Branch != "" {
			a.detail.SetDiffLoading()
			return a.fetchDiff(selected.ID, selected.Branch)
		}
		if selected.State == run.StateQueued || selected.State == run.StateRouting {
			a.detail.SetDiffWaiting()
		} else {
			a.detail.SetDiffNoBranch()
		}
	}
	return nil
}

func (a *App) fetchDiff(runID, branch string) tea.Cmd {
	dg := a.diffGen
	return func() tea.Msg {
		diff, err := dg.Diff(branch)
		if err != nil {
			return DiffResultMsg{RunID: runID, Err: err}
		}
		stat, _ := dg.DiffStat(branch)
		return DiffResultMsg{RunID: runID, Diff: diff, DiffStat: stat}
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

func (a App) handleAccept() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		return a, nil
	}
	if selected.State != run.StateCompleted && selected.State != run.StateReviewing {
		return a, nil
	}

	runID := selected.ID
	branch := selected.Branch

	a.store.Update(runID, func(r *run.Run) {
		r.State = run.StateAccepted
	})

	_ = a.devServers.Stop(runID)

	go func() {
		cmd := exec.Command("git", "push", "origin", branch)
		if a.worktrees != nil {
			r, ok := a.store.Get(runID)
			if ok && r.Worktree != "" {
				cmd.Dir = r.Worktree
			}
		}
		_ = cmd.Run()
		_ = a.worktrees.Remove(runID)
	}()

	return a, nil
}

func (a App) handleReject() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		return a, nil
	}
	if selected.State != run.StateCompleted && selected.State != run.StateReviewing {
		return a, nil
	}

	runID := selected.ID

	a.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRejected
	})

	_ = a.devServers.Stop(runID)
	go func() {
		_ = a.worktrees.Remove(runID)
	}()

	return a, nil
}

func (a App) handleDevServerToggle() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		return a, nil
	}
	if selected.State != run.StateCompleted && selected.State != run.StateReviewing {
		return a, nil
	}

	runID := selected.ID

	if a.devServers.Port(runID) > 0 {
		_ = a.devServers.Stop(runID)
		a.store.Update(runID, func(r *run.Run) {
			r.DevServerPort = 0
			r.DevServerURL = ""
		})
	} else if selected.Worktree != "" {
		port, err := a.devServers.Start(runID, selected.Worktree)
		if err != nil {
			a.statusBar.SetFlash(fmt.Sprintf("dev server: %v", err))
			return a, flashClearCmd()
		} else if port > 0 {
			url := fmt.Sprintf("http://localhost:%d", port)
			a.store.Update(runID, func(r *run.Run) {
				r.DevServerPort = port
				r.DevServerURL = url
			})
			a.statusBar.SetFlash(fmt.Sprintf("Dev server: %s", url))
			return a, flashClearCmd()
		}
	}

	return a, nil
}

func (a *App) autoStartDevServers() {
	if a.config.Project.DevServer.Command == "" {
		return
	}

	for _, r := range a.store.List() {
		if (r.State == run.StateCompleted || r.State == run.StateReviewing) &&
			r.Worktree != "" && r.DevServerPort == 0 {
			port, err := a.devServers.Start(r.ID, r.Worktree)
			if err == nil && port > 0 {
				url := fmt.Sprintf("http://localhost:%d", port)
				a.store.Update(r.ID, func(r *run.Run) {
					r.DevServerPort = port
					r.DevServerURL = url
				})
			}
		}
	}
}

func flashClearCmd() tea.Cmd {
	return tea.Tick(panels.FlashDuration(), func(time.Time) tea.Msg {
		return ClearFlashMsg{}
	})
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
