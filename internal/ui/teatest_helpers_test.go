package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/justinpbarnett/agtop/internal/config"
)

const waitDuration = 3 * time.Second

// appAdapter wraps the App (value receiver model) into a model that
// suppresses Init() side effects (store listener, tick timer) so the
// teatest program doesn't block forever on channel reads.
type appAdapter struct {
	app App
}

func newTestAppAdapter(tb testing.TB) *appAdapter {
	tb.Helper()
	cfg := config.DefaultConfig()
	cfg.Project.Root = tb.(*testing.T).TempDir() // isolate from real session data
	a := NewApp(&cfg)
	a.initPrompt = nil // dismiss init prompt for tests
	return &appAdapter{app: a}
}

func (a *appAdapter) Init() tea.Cmd {
	// Skip the real Init() which blocks on store.Changes() channel.
	return nil
}

func (a *appAdapter) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m, cmd := a.app.Update(msg)
	a.app = m.(App)
	return a, cmd
}

func (a *appAdapter) View() string {
	return a.app.View()
}

// waitForContains waits until the output contains the given substring.
func waitForContains(tb testing.TB, tm *teatest.TestModel, substr string) {
	tb.Helper()
	teatest.WaitFor(
		tb,
		tm.Output(),
		func(bts []byte) bool { return bytesContains(bts, []byte(substr)) },
		teatest.WithDuration(waitDuration),
	)
}

func bytesContains(haystack, needle []byte) bool {
	if len(needle) == 0 || len(haystack) < len(needle) {
		return false
	}
	for i := 0; i <= len(haystack)-len(needle); i++ {
		found := true
		for j := range needle {
			if haystack[i+j] != needle[j] {
				found = false
				break
			}
		}
		if found {
			return true
		}
	}
	return false
}

func containsStr(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && strings.Contains(s, substr)
}
