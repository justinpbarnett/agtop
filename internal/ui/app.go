package ui

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/cost"
	"github.com/justinpbarnett/agtop/internal/engine"
	"github.com/justinpbarnett/agtop/skills"
	gitpkg "github.com/justinpbarnett/agtop/internal/git"
	"github.com/justinpbarnett/agtop/internal/jira"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
	"github.com/justinpbarnett/agtop/internal/safety"
	"github.com/justinpbarnett/agtop/internal/server"
	"github.com/justinpbarnett/agtop/internal/ui/clipboard"
	"github.com/justinpbarnett/agtop/internal/ui/layout"
	"github.com/justinpbarnett/agtop/internal/ui/panels"
	"github.com/justinpbarnett/agtop/internal/ui/styles"
	"github.com/justinpbarnett/agtop/internal/update"
)

const (
	panelRunList = 0
	panelLogView = 1
	panelDetail  = 2
	numPanels    = 3
)

// TickMsg is sent every second to refresh elapsed time displays.
type TickMsg struct{}

// InitResultMsg carries the result of running agtop init asynchronously.
type InitResultMsg struct {
	Err error
}

// StartRunMsg triggers the executor to create and start a new run.
type StartRunMsg struct {
	Prompt   string
	Workflow string
	Model    string
}

type App struct {
	config         *config.Config
	store          *run.Store
	manager        *process.Manager
	registry       *engine.Registry
	executor       *engine.Executor
	pipeline       *engine.Pipeline
	worktrees      *gitpkg.WorktreeManager
	devServers     *server.DevServerManager
	diffGen        *gitpkg.DiffGenerator
	persistence    *run.Persistence
	pidWatchCancel func()
	width          int
	height         int
	layout         layout.Layout
	focusedPanel   int
	runList        panels.RunList
	logView        panels.LogView
	detail         panels.Detail
	statusBar      panels.StatusBar
	helpOverlay    *panels.HelpOverlay
	newRunModal    *panels.NewRunModal
	followUpModal  *panels.FollowUpModal
	runPickerModal *panels.RunPickerModal
	initPrompt     *panels.InitPrompt
	keys            KeyMap
	ready           bool
	lastSyncedRunID string
	updateRepo      string
	fullscreenPanel int // -1 = normal layout, panelDetail/panelLogView = fullscreen
	runStates       map[string]run.State // tracks previous run states to detect transitions
}

func NewApp(cfg *config.Config) App {
	store := run.NewStore()

	tracker := cost.NewTracker()
	maxCostPerRun := cfg.Limits.MaxCostPerRun
	if cfg.Runtime.Default == "claude" && cfg.Runtime.Claude.Subscription {
		maxCostPerRun = 0 // subscription billing — disable cost threshold
	}
	limiter := &cost.LimitChecker{
		MaxTokensPerRun: cfg.Limits.MaxTokensPerRun,
		MaxCostPerRun:   maxCostPerRun,
	}

	var safetyMatcher *safety.PatternMatcher
	safetyEngine, safetyErr := safety.NewHookEngine(cfg.Safety)
	if safetyErr != nil {
		log.Printf("warning: %v", safetyErr)
	}
	if safetyEngine != nil {
		safetyMatcher = safetyEngine.Matcher()
	}

	projectRoot := cfg.Project.Root
	if projectRoot == "" || projectRoot == "." {
		projectRoot, _ = os.Getwd()
	}

	// Session persistence: create early so we have sessionsDir for the manager
	var persist *run.Persistence
	var sessionsDir string
	persist, err := run.NewPersistence(projectRoot)
	if err != nil {
		log.Printf("warning: session persistence: %v", err)
	}
	if persist != nil {
		sessionsDir = persist.SessionsDir()
	}

	var mgr *process.Manager
	rt, rtName, rtErr := runtime.NewRuntime(&cfg.Runtime)
	if rtErr != nil {
		log.Printf("warning: %v (starting without process management)", rtErr)
	} else {
		mgr = process.NewManager(store, rt, rtName, sessionsDir, &cfg.Limits, tracker, limiter, safetyMatcher)
	}

	reg := engine.NewRegistry(cfg)
	if err := reg.Load(projectRoot, skills.FS); err != nil {
		log.Printf("warning: skill registry load: %v", err)
	}

	var exec *engine.Executor
	if mgr != nil {
		exec = engine.NewExecutor(store, mgr, reg, cfg)
	}

	var pl *engine.Pipeline
	if exec != nil {
		pl = engine.NewPipeline(exec, store, &cfg.Merge, projectRoot)
	}

	wt := gitpkg.NewWorktreeManagerAt(projectRoot, cfg.Project.WorktreePath)
	dg := gitpkg.NewDiffGenerator(projectRoot)
	ds := server.NewDevServerManager(cfg.Project.DevServer)

	// Session persistence: rehydrate previous runs
	var pidWatchCancel func()
	if persist != nil {
		cb := run.RehydrateCallbacks{}
		if mgr != nil {
			cb.InjectBuffer = mgr.InjectBuffer
			cb.RecordCost = func(runID string, sc cost.SkillCost) {
				tracker.Record(runID, sc)
			}
			cb.Reconnect = mgr.Reconnect
			cb.ReplayLogFile = mgr.ReplayLogFile
		}
		rehydrateResult, cancel, rehydrateErr := persist.RehydrateWithWatcher(store, cb)
		if rehydrateErr != nil {
			log.Printf("warning: rehydrate sessions: %v", rehydrateErr)
		}
		pidWatchCancel = cancel
		if rehydrateResult.Count > 0 {
			log.Printf("rehydrated %d runs from session", rehydrateResult.Count)
		}

		// Resume workflows for reconnected runs
		if exec != nil {
			for _, id := range rehydrateResult.ReconnectedIDs {
				r, ok := store.Get(id)
				if ok && !r.IsTerminal() && r.Workflow != "" {
					log.Printf("resuming workflow for reconnected run %s", id)
					exec.ResumeReconnected(id, r.Prompt)
				}
			}
		}

		// Bind auto-save with log file path getter
		persist.BindStore(store, func(runID string) []string {
			if mgr == nil {
				return nil
			}
			buf := mgr.Buffer(runID)
			if buf == nil {
				return nil
			}
			return buf.Tail(1000)
		}, func(runID string) (string, string) {
			if mgr == nil {
				return "", ""
			}
			return mgr.LogFilePaths(runID)
		})
	}

	// JIRA integration: create expander if configured
	var jiraExp *jira.Expander
	if cfg.Integrations.Jira != nil {
		jiraClient, jiraErr := jira.NewClientFromConfig(cfg.Integrations.Jira)
		if jiraErr != nil {
			log.Printf("warning: %v (JIRA expansion disabled)", jiraErr)
		} else {
			jiraExp = jira.NewExpander(jiraClient, cfg.Integrations.Jira.ProjectKey)
		}
	}
	if exec != nil && jiraExp != nil {
		exec.SetJIRAExpander(jiraExp)
	}

	// Pre-seed run states from any rehydrated runs so we don't flash for
	// failures that already existed before this session started.
	runStates := make(map[string]run.State)
	for _, r := range store.List() {
		runStates[r.ID] = r.State
	}

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
		lv.SetRun(selected.ID, selected.CurrentSkill, selected.Branch, mgr.Buffer(selected.ID), mgr.EntryBuffer(selected.ID), !selected.IsTerminal())
	}

	app := App{
		config:          cfg,
		store:           store,
		manager:         mgr,
		registry:        reg,
		executor:        exec,
		pipeline:        pl,
		worktrees:       wt,
		diffGen:         dg,
		devServers:      ds,
		persistence:     persist,
		pidWatchCancel:  pidWatchCancel,
		runList:         rl,
		logView:         lv,
		detail:          d,
		statusBar:       panels.NewStatusBar(store),
		keys:            DefaultKeyMap(),
		fullscreenPanel: -1,
		runStates:       runStates,
	}

	if !config.LocalConfigExists() {
		app.initPrompt = panels.NewInitPrompt()
	}

	if cfg.Update.AutoCheck {
		app.updateRepo = cfg.Update.Repo
	}

	return app
}

func (a App) Init() tea.Cmd {
	cmds := []tea.Cmd{listenForChanges(a.store.Changes()), tickCmd(), animTickCmd()}
	if a.updateRepo != "" {
		cmds = append(cmds, checkForUpdateCmd(a.updateRepo))
	}
	return tea.Batch(cmds...)
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.ready = true
		a.layout = layout.Calculate(msg.Width, msg.Height)
		a.propagateSizes()
		if a.newRunModal != nil {
			a.newRunModal.SetSize(msg.Width, msg.Height)
		}
		if a.followUpModal != nil {
			a.followUpModal.SetSize(msg.Width, msg.Height)
		}
		return a, nil

	case CloseModalMsg:
		a.helpOverlay = nil
		a.newRunModal = nil
		a.followUpModal = nil
		a.runPickerModal = nil
		a.initPrompt = nil
		return a, nil

	case SelectRunMsg:
		a.runPickerModal = nil
		a.runList.SelectByID(msg.RunID)
		return a, a.syncSelection()

	case InitAcceptedMsg:
		a.initPrompt = nil
		a.statusBar.SetFlashWithLevel("Running agtop init...", panels.FlashInfo)
		return a, tea.Batch(runInitCmd(), flashClearCmd())

	case InitResultMsg:
		if msg.Err != nil {
			a.statusBar.SetFlashWithLevel(fmt.Sprintf("Init failed: %v", msg.Err), panels.FlashError)
		} else {
			a.statusBar.SetFlashWithLevel("agtop init complete", panels.FlashSuccess)
		}
		return a, flashClearCmd()

	case DiffResultMsg:
		selected := a.runList.SelectedRun()
		if selected == nil || selected.ID != msg.RunID {
			return a, nil
		}
		if msg.Err != nil {
			a.logView.SetDiffError(msg.Err.Error())
		} else {
			a.logView.SetDiff(msg.Diff, msg.DiffStat)
		}
		return a, nil


	case TickMsg:
		return a, tickCmd()

	case panels.AnimTickMsg:
		a.statusBar.Tick()
		var logCmd, listCmd tea.Cmd
		a.logView, logCmd = a.logView.Update(msg)
		a.runList, listCmd = a.runList.Update(msg)
		return a, tea.Batch(animTickCmd(), logCmd, listCmd)

	case RunStoreUpdatedMsg:
		var cmd tea.Cmd
		a.runList, cmd = a.runList.Update(msg)
		diffCmd := a.syncSelection()
		a.autoStartDevServers()
		cmds := []tea.Cmd{cmd, diffCmd, listenForChanges(a.store.Changes())}
		// Detect runs that newly transitioned to failed and surface a flash.
		for _, r := range a.store.List() {
			prev, seen := a.runStates[r.ID]
			if r.State == run.StateFailed && seen && prev != run.StateFailed {
				errMsg := r.Error
				if errMsg == "" {
					errMsg = "unknown error"
				}
				a.statusBar.SetFlashWithLevel(fmt.Sprintf("Run %s failed: %s", r.ID, errMsg), panels.FlashError)
				cmds = append(cmds, flashClearCmd())
			}
			a.runStates[r.ID] = r.State
		}
		return a, tea.Batch(cmds...)

	case SubmitNewRunMsg:
		return a, func() tea.Msg {
			prompt := msg.Prompt
			if len(msg.Images) > 0 {
				var sb strings.Builder
				sb.WriteString(prompt)
				sb.WriteString("\n\nAttached images:\n")
				for _, img := range msg.Images {
					sb.WriteString("- ")
					sb.WriteString(img)
					sb.WriteString("\n")
				}
				prompt = sb.String()
			}
			return StartRunMsg{
				Prompt:   prompt,
				Workflow: msg.Workflow,
				Model:    msg.Model,
			}
		}

	case StartRunMsg:
		if a.executor != nil {
			newRun := &run.Run{
				Workflow:  msg.Workflow,
				Prompt:    msg.Prompt,
				State:     run.StateQueued,
				CreatedAt: time.Now(),
			}
			if msg.Model != "" {
				newRun.Model = msg.Model
			}
			runID := a.store.Add(newRun)

			if len(a.config.Repos) > 0 {
				result, err := a.worktrees.CreateMulti(runID, a.config.Repos)
				if err != nil {
					a.store.Update(runID, func(r *run.Run) {
						r.State = run.StateFailed
						r.Error = fmt.Sprintf("worktree create: %v", err)
					})
					return a, nil
				}
				a.store.Update(runID, func(r *run.Run) {
					r.Worktree = result.RootPath
					r.Branch = result.Branch
					r.SubWorktrees = make([]run.SubWorktreeInfo, len(result.SubWorktrees))
					for i, sw := range result.SubWorktrees {
						r.SubWorktrees[i] = run.SubWorktreeInfo{
							Name:     sw.Name,
							Path:     sw.Path,
							RepoRoot: sw.RepoRoot,
						}
					}
				})
			} else {
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
			}

			a.executor.Execute(runID, msg.Workflow, msg.Prompt)
		}
		return a, nil

	case SubmitFollowUpMsg:
		if a.executor != nil {
			if err := a.executor.FollowUp(msg.RunID, msg.Prompt); err != nil {
				a.statusBar.SetFlashWithLevel(fmt.Sprintf("follow-up: %v", err), panels.FlashError)
				return a, flashClearCmd()
			}
		}
		return a, nil

	case UpdateAppliedMsg:
		a.statusBar.SetFlashWithLevel(
			fmt.Sprintf("Updated to v%s — please restart agtop", msg.Version),
			panels.FlashInfo,
		)
		return a, nil

	case UpdateAvailableMsg:
		a.statusBar.SetFlashWithLevel(
			fmt.Sprintf("Update available: v%s — run \"agtop update\"", msg.Version),
			panels.FlashInfo,
		)
		return a, tea.Tick(10*time.Second, func(time.Time) tea.Msg {
			return ClearFlashMsg{}
		})

	case process.CostThresholdMsg:
		a.statusBar.SetFlashWithLevel(msg.Reason, panels.FlashWarning)
		return a, flashClearCmd()

	case ClearFlashMsg:
		a.statusBar.ClearFlash()
		return a, nil

	case panels.FullscreenMsg:
		a.fullscreenPanel = msg.Panel
		a.propagateSizes()
		return a, nil

	case panels.ExitFullscreenMsg:
		a.fullscreenPanel = -1
		a.propagateSizes()
		return a, nil

	case YankMsg:
		if msg.Text != "" {
			if err := clipboard.Write(msg.Text); err != nil {
				a.statusBar.SetFlashWithLevel(fmt.Sprintf("Copy failed: %v", err), panels.FlashError)
			} else {
				a.statusBar.SetFlashWithLevel("Copied to clipboard", panels.FlashSuccess)
			}
			return a, flashClearCmd()
		}
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
		var logCmd, detailCmd tea.Cmd
		a.logView, logCmd = a.logView.Update(msg)
		a.detail, detailCmd = a.detail.Update(msg)
		return a, tea.Batch(logCmd, detailCmd)

	case tea.MouseMsg:
		if a.newRunModal != nil {
			var cmd tea.Cmd
			a.newRunModal, cmd = a.newRunModal.Update(msg)
			return a, cmd
		}
		return a.handleMouse(msg)

	case tea.KeyMsg:
		if a.initPrompt != nil {
			var cmd tea.Cmd
			*a.initPrompt, cmd = a.initPrompt.Update(msg)
			return a, cmd
		}

		if a.helpOverlay != nil {
			var cmd tea.Cmd
			*a.helpOverlay, cmd = a.helpOverlay.Update(msg)
			return a, cmd
		}

		if a.followUpModal != nil {
			var cmd tea.Cmd
			a.followUpModal, cmd = a.followUpModal.Update(msg)
			return a, cmd
		}

		if a.newRunModal != nil {
			var cmd tea.Cmd
			a.newRunModal, cmd = a.newRunModal.Update(msg)
			return a, cmd
		}

		if a.runPickerModal != nil {
			var cmd tea.Cmd
			a.runPickerModal, cmd = a.runPickerModal.Update(msg)
			return a, cmd
		}

		// When the log view is in search mode, route keys directly to it
		// so that typing and n/N navigation aren't intercepted by global handlers.
		if a.focusedPanel == panelLogView && a.logView.ConsumesKeys() {
			switch msg.String() {
			case "ctrl+c":
				a.cleanup()
				return a, tea.Quit
			}
			var cmd tea.Cmd
			a.logView, cmd = a.logView.Update(msg)
			return a, cmd
		}

		switch msg.String() {
		case "esc":
			if a.fullscreenPanel >= 0 {
				a.fullscreenPanel = -1
				a.propagateSizes()
				return a, nil
			}
		case "ctrl+c":
			a.cleanup()
			return a, tea.Quit
		case "q":
			a.cleanup()
			return a, tea.Quit
		case "tab":
			a.focusedPanel = (a.focusedPanel + 1) % numPanels
			a.updateFocusState()
			return a, nil
		case "1":
			a.focusedPanel = panelRunList
			a.updateFocusState()
			return a, nil
		case "2":
			a.focusedPanel = panelDetail
			a.updateFocusState()
			return a, nil
		case "3":
			a.focusedPanel = panelLogView
			a.updateFocusState()
			return a, nil
		case "h", "left":
			if a.focusedPanel == panelLogView {
				if a.logView.ActiveTab() > 0 {
					// Switch tab within logview
					return a.routeKey(msg)
				}
				// On log tab — spatial nav to run list
				a.focusedPanel = panelRunList
				a.updateFocusState()
			}
			return a, nil
		case "l", "right":
			if a.focusedPanel == panelLogView {
				// Delegate to logview for tab switching (no-ops on last tab)
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
			a.newRunModal = panels.NewNewRunModal(a.width, a.height)
			return a, a.newRunModal.Init()
		case "a":
			return a.handleAccept()
		case "x":
			return a.handleReject()
		case " ":
			if a.focusedPanel == panelLogView {
				return a.routeKey(msg)
			}
			return a.handleTogglePause()
		case "r":
			return a.handleRestart()
		case "c":
			return a.handleCancel()
		case "d":
			return a.handleDelete()
		case "D":
			return a.handleDevServerToggle()
		case "u":
			return a.handleFollowUp()
		case "enter":
			if a.focusedPanel == panelRunList && !a.runList.FilterActive() {
				return a.handleRunPicker()
			}
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

	// Assemble layout
	var fullLayout string
	switch a.fullscreenPanel {
	case panelDetail:
		fullLayout = lipgloss.JoinVertical(lipgloss.Left, detailView, statusBarView)
	case panelLogView:
		fullLayout = lipgloss.JoinVertical(lipgloss.Left, logViewView, statusBarView)
	default:
		leftCol := lipgloss.JoinVertical(lipgloss.Left, runListView, detailView)
		mainArea := lipgloss.JoinHorizontal(lipgloss.Top, leftCol, logViewView)
		fullLayout = lipgloss.JoinVertical(lipgloss.Left, mainArea, statusBarView)
	}

	if a.initPrompt != nil {
		modalView := a.initPrompt.View()
		fullLayout = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(styles.TextDim),
		)
	}

	if a.helpOverlay != nil {
		modalView := a.helpOverlay.View()
		fullLayout = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(styles.TextDim),
		)
	}

	if a.newRunModal != nil {
		modalView := a.newRunModal.View()
		fullLayout = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(styles.TextDim),
		)
	}

	if a.followUpModal != nil {
		modalView := a.followUpModal.View()
		fullLayout = lipgloss.Place(a.width, a.height,
			lipgloss.Center, lipgloss.Center, modalView,
			lipgloss.WithWhitespaceChars(" "),
			lipgloss.WithWhitespaceForeground(styles.TextDim),
		)
	}

	if a.runPickerModal != nil {
		modalView := a.runPickerModal.View()
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

// cleanup gracefully shuts down the executor and disconnects from running
// processes (without killing them), then saves state and stops dev servers.
func (a App) cleanup() {
	// 1. Shut down executor — cancels workflow goroutines, waits for drain
	if a.executor != nil {
		a.executor.Shutdown()
	}

	// 2. Set disconnecting flag so consume* goroutines preserve PID/state
	if a.manager != nil {
		a.manager.SetDisconnecting()
	}

	// 3. Cancel FollowReader contexts
	if a.manager != nil {
		a.manager.DisconnectAll()
	}

	// 4. Final synchronous save of all non-terminal runs with PIDs intact
	if a.persistence != nil && a.manager != nil {
		a.persistence.FinalSave(a.store, a.manager.LogFilePaths)
	}

	// 5. Stop dev servers and PID watchers
	a.devServers.StopAll()
	if a.pidWatchCancel != nil {
		a.pidWatchCancel()
	}
}

func (a App) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
			relX, relY, ok := a.mouseInLogView(msg.X, msg.Y)
			if ok {
				_, _ = relX, relY
				var cmd tea.Cmd
				a.logView, cmd = a.logView.Update(msg)
				return a, cmd
			}
			return a, nil
		}
		if msg.Button == tea.MouseButtonLeft {
			relX, relY, ok := a.mouseInLogView(msg.X, msg.Y)
			if ok {
				a.logView.StartMouseSelection(relX, relY)
			} else {
				a.logView.CancelMouseSelection()
			}
			return a, nil
		}

	case tea.MouseActionMotion:
		relX, relY, ok := a.mouseInLogView(msg.X, msg.Y)
		if ok {
			a.logView.ExtendMouseSelection(relX, relY)
		}
		return a, nil

	case tea.MouseActionRelease:
		relX, relY, ok := a.mouseInLogView(msg.X, msg.Y)
		if ok {
			text := a.logView.FinalizeMouseSelection(relX, relY)
			if text != "" {
				if err := clipboard.Write(text); err != nil {
					a.statusBar.SetFlashWithLevel(fmt.Sprintf("Copy failed: %v", err), panels.FlashError)
				} else {
					a.statusBar.SetFlashWithLevel("Copied to clipboard", panels.FlashSuccess)
				}
				return a, flashClearCmd()
			}
		} else {
			a.logView.CancelMouseSelection()
		}
		return a, nil
	}

	return a, nil
}

// mouseInLogView tests whether absolute screen coordinates fall within the
// log view panel. Returns panel-relative coordinates and true if inside.
// LogView occupies X=[RunListWidth, RunListWidth+LogViewWidth), Y=[0, LogViewHeight).
func (a App) mouseInLogView(x, y int) (relX, relY int, ok bool) {
	l := a.layout
	if x >= l.RunListWidth && x < l.RunListWidth+l.LogViewWidth &&
		y >= 0 && y < l.LogViewHeight {
		return x - l.RunListWidth, y, true
	}
	return 0, 0, false
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
		var eb *process.EntryBuffer
		if a.manager != nil {
			buf = a.manager.Buffer(selected.ID)
			eb = a.manager.EntryBuffer(selected.ID)
		}

		if selected.ID == a.lastSyncedRunID {
			// Same run — update metadata without resetting tab/search/cursor state
			a.logView.UpdateRunMeta(selected.CurrentSkill, selected.Branch, buf, eb, !selected.IsTerminal())
		} else {
			// Different run — full reset
			a.logView.SetRun(selected.ID, selected.CurrentSkill, selected.Branch, buf, eb, !selected.IsTerminal())
			a.lastSyncedRunID = selected.ID
		}

		if selected.Worktree != "" {
			a.logView.SetDiffLoading()
			return a.fetchDiff(selected.ID, selected.Worktree, selected.SubWorktrees)
		}
		if selected.State == run.StateQueued || selected.State == run.StateRouting {
			a.logView.SetDiffWaiting()
		} else {
			a.logView.SetDiffNoBranch()
		}
	} else {
		a.logView.SetRun("", "", "", nil, nil, false)
		a.lastSyncedRunID = ""
		a.logView.SetDiffEmpty()
	}
	return nil
}

func (a *App) fetchDiff(runID, worktreeDir string, subWorktrees []run.SubWorktreeInfo) tea.Cmd {
	dg := a.diffGen
	return func() tea.Msg {
		if len(subWorktrees) > 0 {
			var allDiff, allStat strings.Builder
			for _, sw := range subWorktrees {
				diff, err := dg.Diff(sw.Path)
				if err != nil {
					continue
				}
				if diff != "" {
					fmt.Fprintf(&allDiff, "── %s ──\n%s\n", sw.Name, diff)
				}
				stat, _ := dg.DiffStat(sw.Path)
				if stat != "" {
					fmt.Fprintf(&allStat, "── %s ──\n%s\n", sw.Name, stat)
				}
			}
			return DiffResultMsg{RunID: runID, Diff: allDiff.String(), DiffStat: allStat.String()}
		}

		diff, err := dg.Diff(worktreeDir)
		if err != nil {
			return DiffResultMsg{RunID: runID, Err: err}
		}
		stat, _ := dg.DiffStat(worktreeDir)
		return DiffResultMsg{RunID: runID, Diff: diff, DiffStat: stat}
	}
}

func (a *App) propagateSizes() {
	l := a.layout
	if a.fullscreenPanel == panelDetail {
		a.detail.SetSize(a.width, a.height-1)
		a.runList.SetSize(l.RunListWidth, l.RunListHeight)
		a.logView.SetSize(l.LogViewWidth, l.LogViewHeight)
	} else if a.fullscreenPanel == panelLogView {
		a.logView.SetSize(a.width, a.height-1)
		a.runList.SetSize(l.RunListWidth, l.RunListHeight)
		a.detail.SetSize(l.DetailWidth, l.DetailHeight)
	} else {
		a.runList.SetSize(l.RunListWidth, l.RunListHeight)
		a.logView.SetSize(l.LogViewWidth, l.LogViewHeight)
		a.detail.SetSize(l.DetailWidth, l.DetailHeight)
	}
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
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}

	// Allow re-accept of failed merge pipelines
	if selected.State != run.StateCompleted && selected.State != run.StateReviewing &&
		!(selected.State == run.StateFailed && selected.MergeStatus != "") {
		a.statusBar.SetFlashWithLevel(fmt.Sprintf("Cannot accept: run is %s", selected.State), panels.FlashError)
		return a, flashClearCmd()
	}

	runID := selected.ID

	_ = a.devServers.Stop(runID)

	// Auto-merge pipeline
	if a.config.Merge.AutoMerge && a.pipeline != nil {
		a.store.Update(runID, func(r *run.Run) {
			r.State = run.StateMerging
			r.MergeStatus = "starting"
			r.Error = ""
		})
		go func() {
			ctx := context.Background()
			a.pipeline.Run(ctx, runID)
			r, ok := a.store.Get(runID)
			if ok && r.State == run.StateAccepted {
				a.cleanupRun(runID)
			}
		}()
		a.statusBar.SetFlashWithLevel("Merge pipeline started", panels.FlashSuccess)
		return a, flashClearCmd()
	}

	// Legacy flow: merge locally then clean up
	a.store.Update(runID, func(r *run.Run) { r.State = run.StateAccepted })

	repos := a.config.Repos
	goldenCmd := a.config.Merge.GoldenUpdateCommand
	go func() {
		if len(repos) > 0 {
			if err := a.worktrees.MergeMulti(runID, repos, gitpkg.MergeOptions{
				GoldenUpdateCommand: goldenCmd,
			}); err != nil {
				a.store.Update(runID, func(r *run.Run) {
					r.State = run.StateFailed
					r.Error = fmt.Sprintf("merge failed: %v", err)
				})
				return
			}
		} else {
			_, err := a.worktrees.MergeWithOptions(runID, gitpkg.MergeOptions{
				GoldenUpdateCommand: goldenCmd,
			})
			if err != nil {
				a.store.Update(runID, func(r *run.Run) {
					r.State = run.StateFailed
					r.Error = fmt.Sprintf("merge failed: %v", err)
				})
				return
			}
		}
		a.cleanupRun(runID)
	}()

	a.statusBar.SetFlashWithLevel("Merging and cleaning up...", panels.FlashSuccess)
	return a, flashClearCmd()
}

func (a App) handleReject() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}
	if selected.State != run.StateCompleted && selected.State != run.StateReviewing {
		a.statusBar.SetFlashWithLevel(fmt.Sprintf("Cannot reject: run is %s", selected.State), panels.FlashError)
		return a, flashClearCmd()
	}

	runID := selected.ID

	a.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRejected
	})

	_ = a.devServers.Stop(runID)
	worktrees := a.worktrees
	repos := a.config.Repos
	store := a.store
	go func() {
		if len(repos) > 0 {
			_ = worktrees.RemoveMulti(runID, repos)
		} else {
			_ = worktrees.Remove(runID)
		}
		store.Update(runID, func(r *run.Run) { r.Worktree = "" })
	}()

	return a, nil
}

func (a App) handleDevServerToggle() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}
	if selected.State != run.StateCompleted && selected.State != run.StateReviewing {
		a.statusBar.SetFlashWithLevel(fmt.Sprintf("Dev server: run is %s", selected.State), panels.FlashWarning)
		return a, flashClearCmd()
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
			a.statusBar.SetFlashWithLevel(fmt.Sprintf("dev server: %v", err), panels.FlashError)
			return a, flashClearCmd()
		} else if port > 0 {
			url := fmt.Sprintf("http://localhost:%d", port)
			a.store.Update(runID, func(r *run.Run) {
				r.DevServerPort = port
				r.DevServerURL = url
			})
			a.statusBar.SetFlashWithLevel(fmt.Sprintf("Dev server: %s", url), panels.FlashSuccess)
			return a, flashClearCmd()
		}
	}

	return a, nil
}

func (a App) handleTogglePause() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}

	switch selected.State {
	case run.StateRunning:
		if a.manager == nil {
			return a, nil
		}
		if err := a.manager.Pause(selected.ID); err != nil {
			a.statusBar.SetFlashWithLevel(fmt.Sprintf("Pause: %v", err), panels.FlashError)
			return a, flashClearCmd()
		}
		return a, nil

	case run.StatePaused:
		if a.manager != nil {
			if err := a.manager.Resume(selected.ID); err != nil {
				a.statusBar.SetFlashWithLevel(fmt.Sprintf("Resume: %v", err), panels.FlashError)
				return a, flashClearCmd()
			}
			return a, nil
		}
		return a, nil

	case run.StateFailed:
		if a.executor != nil {
			if err := a.executor.Resume(selected.ID, selected.Prompt); err != nil {
				a.statusBar.SetFlashWithLevel(fmt.Sprintf("Resume: %v", err), panels.FlashError)
				return a, flashClearCmd()
			}
			return a, nil
		}
		return a, nil

	default:
		a.statusBar.SetFlashWithLevel(fmt.Sprintf("Cannot pause/resume: run is %s", selected.State), panels.FlashError)
		return a, flashClearCmd()
	}
}

func (a App) handleRestart() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}
	if !selected.IsTerminal() && selected.State != run.StateReviewing {
		a.statusBar.SetFlashWithLevel(fmt.Sprintf("Cannot restart: run is %s", selected.State), panels.FlashError)
		return a, flashClearCmd()
	}
	if a.executor == nil {
		return a, nil
	}

	prompt := selected.OriginalPrompt
	if prompt == "" {
		prompt = selected.Prompt
	}
	workflow := selected.Workflow
	model := selected.Model
	return a, func() tea.Msg {
		return StartRunMsg{
			Prompt:   prompt,
			Workflow: workflow,
			Model:    model,
		}
	}
}

func (a App) handleCancel() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}
	if a.manager == nil {
		return a, nil
	}
	if selected.State != run.StateRunning && selected.State != run.StatePaused && selected.State != run.StateQueued {
		a.statusBar.SetFlashWithLevel(fmt.Sprintf("Cannot cancel: run is %s", selected.State), panels.FlashError)
		return a, flashClearCmd()
	}

	if a.executor != nil {
		a.executor.Cancel(selected.ID)
	}
	if err := a.manager.Stop(selected.ID); err != nil {
		// Process may have already exited
		a.store.Update(selected.ID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = "cancelled"
			r.CompletedAt = time.Now()
		})
	}
	return a, nil
}

func (a App) handleDelete() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}
	if !selected.IsTerminal() {
		a.statusBar.SetFlashWithLevel("Cannot delete: run is still active", panels.FlashError)
		return a, flashClearCmd()
	}

	a.cleanupRun(selected.ID)

	a.statusBar.SetFlashWithLevel(fmt.Sprintf("Deleted run %s", selected.ID), panels.FlashSuccess)
	return a, flashClearCmd()
}

// cleanupRun removes a run and all its associated resources: buffers, dev server,
// store entry, worktree, session file, and log files. Safe to call from goroutines.
func (a App) cleanupRun(runID string) {
	var stdoutLog, stderrLog string
	if a.manager != nil {
		stdoutLog, stderrLog = a.manager.LogFilePaths(runID)
		a.manager.RemoveBuffer(runID)
	}

	_ = a.devServers.Stop(runID)
	a.store.Remove(runID)

	repos := a.config.Repos
	go func() {
		if len(repos) > 0 {
			_ = a.worktrees.RemoveMulti(runID, repos)
		} else {
			_ = a.worktrees.Remove(runID)
		}
		if a.persistence != nil {
			_ = a.persistence.Remove(runID)
		}
		process.RemoveLogFiles(stdoutLog, stderrLog)
	}()
}

func (a App) handleRunPicker() (tea.Model, tea.Cmd) {
	var active []run.Run
	for _, r := range a.store.List() {
		if !r.IsTerminal() {
			active = append(active, r)
		}
	}
	a.runPickerModal = panels.NewRunPickerModal(active, a.width, a.height)
	return a, nil
}

func (a App) handleFollowUp() (tea.Model, tea.Cmd) {
	selected := a.runList.SelectedRun()
	if selected == nil {
		a.statusBar.SetFlashWithLevel("No run selected", panels.FlashWarning)
		return a, flashClearCmd()
	}
	if selected.State != run.StateCompleted && selected.State != run.StateReviewing {
		a.statusBar.SetFlashWithLevel(fmt.Sprintf("Cannot follow up: run is %s", selected.State), panels.FlashError)
		return a, flashClearCmd()
	}

	a.followUpModal = panels.NewFollowUpModal(selected.ID, selected.Prompt, a.width, a.height)
	return a, a.followUpModal.Init()
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

func runInitCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("agtop", "init")
		cmd.Dir, _ = os.Getwd()
		output, err := cmd.CombinedOutput()
		if err != nil {
			return InitResultMsg{Err: fmt.Errorf("%v: %s", err, output)}
		}
		return InitResultMsg{}
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return TickMsg{}
	})
}

func animTickCmd() tea.Cmd {
	return tea.Tick(250*time.Millisecond, func(time.Time) tea.Msg {
		return panels.AnimTickMsg{}
	})
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

func checkForUpdateCmd(repo string) tea.Cmd {
	return func() tea.Msg {
		rel, err := update.CheckForUpdate(panels.Version, repo)
		if err != nil || rel == nil {
			return nil
		}
		applied, err := update.Apply(panels.Version, repo)
		if err != nil || applied == nil {
			return UpdateAvailableMsg{Version: rel.Version}
		}
		return UpdateAppliedMsg{Version: applied.Version}
	}
}

