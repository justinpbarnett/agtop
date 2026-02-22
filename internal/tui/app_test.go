package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jpb/agtop/internal/config"
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
	if a.modal != nil {
		t.Error("expected modal to be nil initially")
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
	if a.focusedPanel != 0 {
		t.Errorf("expected focus 0 after second tab, got %d", a.focusedPanel)
	}
}

func TestAppHelpToggle(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	a = sendKey(a, "?")
	if a.modal == nil {
		t.Error("expected modal to be non-nil after ?")
	}

	// When modal is open, ? goes to modal.Update which returns CloseModalMsg
	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	a = m.(App)
	if cmd != nil {
		msg := cmd()
		m, _ = a.Update(msg)
		a = m.(App)
	}
	if a.modal != nil {
		t.Error("expected modal to be nil after second ?")
	}
}

func TestAppHelpCloseEsc(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	a = sendKey(a, "?")
	if a.modal == nil {
		t.Error("expected modal open")
	}

	// Esc should close the modal via CloseModalMsg
	m, cmd := a.Update(tea.KeyMsg{Type: tea.KeyEsc})
	a = m.(App)
	if cmd != nil {
		// Process the command
		msg := cmd()
		m, _ = a.Update(msg)
		a = m.(App)
	}
	if a.modal != nil {
		t.Error("expected modal to be nil after Esc")
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

	// Should contain panel content from run list
	if !strings.Contains(view, "#001") {
		t.Error("expected view to contain run #001")
	}
}

func TestAppViewTooSmall(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 30, 5)
	view := a.View()
	if !strings.Contains(view, "too small") {
		t.Error("expected 'too small' message for small terminal")
	}
}

func TestAppKeyRoutingToRunList(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	// Focus is on run list (panel 0), j should move selection
	initial := a.runList.selected
	a = sendKey(a, "j")
	if a.runList.selected != initial+1 {
		t.Errorf("expected selection to move down, got %d", a.runList.selected)
	}
}

func TestAppKeyRoutingToDetail(t *testing.T) {
	a := newTestApp()
	a = sendWindowSize(a, 120, 40)

	// Switch focus to detail panel
	a = sendSpecialKey(a, tea.KeyTab)

	initial := a.detail.activeTab
	a = sendKey(a, "l")
	if a.detail.activeTab != initial+1 {
		t.Errorf("expected tab to advance, got %d", a.detail.activeTab)
	}
}
