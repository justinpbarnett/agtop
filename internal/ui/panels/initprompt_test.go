package panels

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestOnboarding(runtimes ...string) *OnboardingModal {
	m := &OnboardingModal{
		runtimes: runtimes,
		width:    54,
		height:   12,
	}
	if len(runtimes) == 1 {
		m.step = 1
	}
	return m
}

func TestOnboardingRuntimeSelectNavigation(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")

	if m.SelectedRuntime() != "claude" {
		t.Fatalf("expected claude, got %s", m.SelectedRuntime())
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	if updated.SelectedRuntime() != "opencode" {
		t.Fatalf("after j: expected opencode, got %s", updated.SelectedRuntime())
	}

	updated, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if updated.SelectedRuntime() != "claude" {
		t.Fatalf("after k: expected claude, got %s", updated.SelectedRuntime())
	}
}

func TestOnboardingRuntimeSelectWrap(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	if updated.SelectedRuntime() != "opencode" {
		t.Fatalf("wrap up: expected opencode, got %s", updated.SelectedRuntime())
	}
}

func TestOnboardingEnterAdvancesToConfirm(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if updated.step != 1 {
		t.Fatal("expected step 1 after Enter")
	}
}

func TestOnboardingConfirmEmitsInitAccepted(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")
	m.step = 1

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on Enter in confirm step")
	}
	msg := cmd()
	accepted, ok := msg.(InitAcceptedMsg)
	if !ok {
		t.Fatalf("expected InitAcceptedMsg, got %T", msg)
	}
	if accepted.Runtime != "claude" {
		t.Fatalf("expected runtime claude, got %s", accepted.Runtime)
	}
}

func TestOnboardingConfirmEmitsSelectedRuntime(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")
	m.selected = 1
	m.step = 1

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("expected command on y")
	}
	msg := cmd()
	accepted, ok := msg.(InitAcceptedMsg)
	if !ok {
		t.Fatalf("expected InitAcceptedMsg, got %T", msg)
	}
	if accepted.Runtime != "opencode" {
		t.Fatalf("expected runtime opencode, got %s", accepted.Runtime)
	}
}

func TestOnboardingEscOnStep0Closes(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command on Esc")
	}
	msg := cmd()
	if _, ok := msg.(CloseModalMsg); !ok {
		t.Fatalf("expected CloseModalMsg, got %T", msg)
	}
}

func TestOnboardingEscOnStep1GoesBack(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")
	m.step = 1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Fatal("expected no command on Esc from confirm with multiple runtimes")
	}
	if updated.step != 0 {
		t.Fatal("expected step 0 after Esc on confirm")
	}
}

func TestOnboardingBackspaceOnStep1GoesBack(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")
	m.step = 1

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if cmd != nil {
		t.Fatal("expected no command on Backspace from confirm with multiple runtimes")
	}
	if updated.step != 0 {
		t.Fatal("expected step 0 after Backspace on confirm")
	}
}

func TestOnboardingSingleRuntimeStartsOnConfirm(t *testing.T) {
	m := newTestOnboarding("opencode")

	if m.step != 1 {
		t.Fatal("expected step 1 with single runtime")
	}
	if m.SelectedRuntime() != "opencode" {
		t.Fatalf("expected opencode, got %s", m.SelectedRuntime())
	}
}

func TestOnboardingSingleRuntimeEscCloses(t *testing.T) {
	m := newTestOnboarding("opencode")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command on Esc with single runtime")
	}
	msg := cmd()
	if _, ok := msg.(CloseModalMsg); !ok {
		t.Fatalf("expected CloseModalMsg, got %T", msg)
	}
}

func TestOnboardingNoRuntimesView(t *testing.T) {
	m := newTestOnboarding()

	view := m.View()
	if !strings.Contains(view, "No AI runtime found") {
		t.Error("expected 'No AI runtime found' in view")
	}
}

func TestOnboardingRuntimeSelectView(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")

	view := m.View()
	if !strings.Contains(view, "Setup agtop") {
		t.Error("expected title 'Setup agtop'")
	}
	if !strings.Contains(view, "claude") {
		t.Error("expected 'claude' in view")
	}
	if !strings.Contains(view, "opencode") {
		t.Error("expected 'opencode' in view")
	}
}

func TestOnboardingConfirmView(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")
	m.step = 1

	view := m.View()
	if !strings.Contains(view, "Setup agtop") {
		t.Error("expected title 'Setup agtop'")
	}
	if !strings.Contains(view, "agtop.toml") {
		t.Error("expected 'agtop.toml' in view")
	}
	if !strings.Contains(view, ".claude/settings.json") {
		t.Error("expected '.claude/settings.json' in view for claude runtime")
	}
}

func TestOnboardingConfirmViewOpenCode(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")
	m.selected = 1
	m.step = 1

	view := m.View()
	if !strings.Contains(view, "opencode.json") {
		t.Error("expected 'opencode.json' in view for opencode runtime")
	}
}

func TestOnboardingIgnoresUnrelatedKeys(t *testing.T) {
	m := newTestOnboarding("claude", "opencode")

	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Error("expected no command on unrelated key")
	}
}
