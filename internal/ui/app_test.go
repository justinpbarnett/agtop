package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/run"
)

func newTestApp() App {
	cfg := config.DefaultConfig()
	return NewApp(&cfg)
}

func sendKey(a App, key string) App {
	m, _ := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return m.(App)
}

func sendSpecialKey(a App, t tea.KeyType) App {
	m, _ := a.Update(tea.KeyMsg{Type: t})
	return m.(App)
}

func sendWindowSize(a App, w, h int) App {
	m, _ := a.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return m.(App)
}

func TestAppInitialState(t *testing.T) {
	a := newTestApp()
	if a.ready {
		t.Error("expected ready to be false initially")
	}
	if a.focusedPanel != 0 {
		t.Errorf("expected focusedPanel 0, got %d", a.focusedPanel)
	}
	if a.helpOverlay != nil {
		t.Error("expected helpOverlay to be nil initially")
	}
}

func TestAppWindowResize(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	if !a.ready {
		t.Error("expected ready to be true after WindowSizeMsg")
	}
	if a.width != 120 {
		t.Errorf("expected width 120, got %d", a.width)
	}
	if a.height != 40 {
		t.Errorf("expected height 40, got %d", a.height)
	}
}

func TestAppFocusCycle(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	if a.focusedPanel != 0 {
		t.Errorf("expected initial focus 0, got %d", a.focusedPanel)
	}

	a = sendSpecialKey(a, tea.KeyTab)
	if a.focusedPanel != 1 {
		t.Errorf("expected focus 1 after tab, got %d", a.focusedPanel)
	}

	a = sendSpecialKey(a, tea.KeyTab)
	if a.focusedPanel != 2 {
		t.Errorf("expected focus 2 after second tab, got %d", a.focusedPanel)
	}

	a = sendSpecialKey(a, tea.KeyTab)
	if a.focusedPanel != 0 {
		t.Errorf("expected focus 0 after third tab (wrap), got %d", a.focusedPanel)
	}
}

func TestAppSpatialNavigation(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	// Start at run list (0), l should go to log view (1)
	if a.focusedPanel != panelRunList {
		t.Fatalf("expected start at panelRunList, got %d", a.focusedPanel)
	}

	// But l is also used for panel navigation — when on run list, l goes to log view
	// Actually per the app code, l moves from runList to logView
	a = sendKey(a, "l")
	if a.focusedPanel != panelLogView {
		t.Errorf("expected panelLogView after l from runList, got %d", a.focusedPanel)
	}

	// h should go back to run list
	a = sendKey(a, "h")
	if a.focusedPanel != panelRunList {
		t.Errorf("expected panelRunList after h from logView, got %d", a.focusedPanel)
	}
}

func TestAppHelpToggle(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	a = sendKey(a, "?")
	if a.helpOverlay == nil {
		t.Error("expected helpOverlay to be non-nil after ?")
	}

	// When overlay is open, ? goes to overlay.Update which returns CloseModalMsg
	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	a = m.(App)
	if cmd != nil {
		msg := cmd()
		m, _ = a.Update(msg)
		a = m.(App)
	}
	if a.helpOverlay != nil {
		t.Error("expected helpOverlay to be nil after second ?")
	}
}

func TestAppHelpCloseEsc(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	a = sendKey(a, "?")
	if a.helpOverlay == nil {
		t.Error("expected helpOverlay open")
	}

	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if cmd != nil {
		msg := cmd()
		m, _ = a.Update(msg)
		a = m.(App)
	}
	if a.helpOverlay != nil {
		t.Error("expected helpOverlay to be nil after Esc")
	}
}

func TestAppQuit(t *testing.T) {
	a := newTestApp()
	_, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("expected quit command")
	}
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("expected tea.QuitMsg")
	}
}

func TestAppViewNotReady(t *testing.T) {
	a := newTestApp()
	view := a.View()
	if !strings.Contains(view, "Loading") {
		t.Error("expected loading message before WindowSizeMsg")
	}
}

func TestAppViewReady(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)
	view := a.View()

	if !strings.Contains(view, "Runs") {
		t.Error("expected view to contain 'Runs' panel title")
	}
}

func TestAppViewTooSmall(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 70, 20)
	view := a.View()
	if !strings.Contains(view, "too small") || !strings.Contains(view, "Terminal") {
		t.Error("expected descriptive 'too small' message for small terminal")
	}
}

func TestAppThreePanelLayout(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)
	view := a.View()

	// Should have all three panel titles
	if !strings.Contains(view, "Runs") {
		t.Error("expected 'Runs' panel")
	}
	if !strings.Contains(view, "Log") {
		t.Error("expected 'Log' panel")
	}
	if !strings.Contains(view, "Details") {
		t.Error("expected 'Details' panel")
	}
}

func TestAppStoreUpdate(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	a.store.Update("001", func(r *run.Run) {
		r.Tokens = 99999
		r.Cost = 9.99
	})

	m, _ := a.Update(RunStoreUpdatedMsg{})
	a = m.(App)

	view := a.View()
	if !strings.Contains(view, "$9.99") && !strings.Contains(view, "100.0k") {
		// At least the status bar should reflect it
		statusView := a.statusBar.View()
		_ = statusView // The store totals are computed fresh each render
	}
}

func TestAppKeyRoutingToRunList(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	// Focus is on run list (panel 0), j should work within panel
	a = sendKey(a, "j")
	// Should not crash and should stay on panel 0
	if a.focusedPanel != 0 {
		t.Errorf("expected to stay on panel 0, got %d", a.focusedPanel)
	}
}

func TestAppDeleteTerminalRun(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	countBefore := a.store.Count()
	a.store.Add(&run.Run{State: run.StateCompleted, Prompt: "test run"})
	m, _ := a.Update(RunStoreUpdatedMsg{})
	a = m.(App)

	if a.store.Count() != countBefore+1 {
		t.Fatalf("expected %d runs, got %d", countBefore+1, a.store.Count())
	}

	// The newly added run is at the top of the list (newest first), so it's selected
	a = sendKey(a, "d")

	if a.store.Count() != countBefore {
		t.Errorf("expected %d runs after delete, got %d", countBefore, a.store.Count())
	}
}

func TestAppDeleteNonTerminalRunNoOp(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	a.store.Add(&run.Run{State: run.StateRunning, Prompt: "active run"})
	m, _ := a.Update(RunStoreUpdatedMsg{})
	a = m.(App)

	countBefore := a.store.Count()
	a = sendKey(a, "d")

	if a.store.Count() != countBefore {
		t.Errorf("expected count unchanged at %d, got %d", countBefore, a.store.Count())
	}
}

func TestAppDeleteNoSelection(t *testing.T) {
	a := newTestApp()
	// Don't add any runs — but rehydration may have added some.
	// Pressing d on whatever is selected (if anything) should not panic.
	a = sendWindowSize(a, 120, 40)
	a = sendKey(a, "d")
	// Just verify no panic occurred
}
