package panels

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInitPromptAcceptEnter(t *testing.T) {
	p := NewInitPrompt()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("expected command on Enter")
	}
	msg := cmd()
	if _, ok := msg.(InitAcceptedMsg); !ok {
		t.Errorf("expected InitAcceptedMsg, got %T", msg)
	}
}

func TestInitPromptAcceptY(t *testing.T) {
	p := NewInitPrompt()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")})
	if cmd == nil {
		t.Fatal("expected command on y")
	}
	msg := cmd()
	if _, ok := msg.(InitAcceptedMsg); !ok {
		t.Errorf("expected InitAcceptedMsg, got %T", msg)
	}
}

func TestInitPromptCloseEsc(t *testing.T) {
	p := NewInitPrompt()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command on Esc")
	}
	msg := cmd()
	if _, ok := msg.(CloseModalMsg); !ok {
		t.Errorf("expected CloseModalMsg, got %T", msg)
	}
}

func TestInitPromptCloseN(t *testing.T) {
	p := NewInitPrompt()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	if cmd == nil {
		t.Fatal("expected command on n")
	}
	msg := cmd()
	if _, ok := msg.(CloseModalMsg); !ok {
		t.Errorf("expected CloseModalMsg, got %T", msg)
	}
}

func TestInitPromptIgnoresOtherKeys(t *testing.T) {
	p := NewInitPrompt()
	_, cmd := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	if cmd != nil {
		t.Error("expected no command on unrelated key")
	}
}

func TestInitPromptView(t *testing.T) {
	p := NewInitPrompt()
	view := p.View()

	if !strings.Contains(view, "Initialize Project") {
		t.Error("expected title 'Initialize Project'")
	}
	if !strings.Contains(view, "No agtop.yaml") {
		t.Error("expected body text about missing config")
	}
}
