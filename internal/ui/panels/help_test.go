package panels

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestHelpContent(t *testing.T) {
	h := NewHelpOverlay()
	view := h.View()

	if !strings.Contains(view, "Navigation") {
		t.Error("expected 'Navigation' section")
	}
	if !strings.Contains(view, "Actions") {
		t.Error("expected 'Actions' section")
	}
	if !strings.Contains(view, "Global") {
		t.Error("expected 'Global' section")
	}
}

func TestHelpBorder(t *testing.T) {
	h := NewHelpOverlay()
	view := h.View()

	if !strings.Contains(view, "Keybinds") {
		t.Error("expected 'Keybinds' title in border")
	}
	if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
		t.Error("expected border characters")
	}
}

func TestHelpCloseEsc(t *testing.T) {
	h := NewHelpOverlay()
	_, cmd := h.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("expected command on Esc")
	}
	msg := cmd()
	if _, ok := msg.(CloseModalMsg); !ok {
		t.Error("expected CloseModalMsg on Esc")
	}
}

func TestHelpCloseQuestion(t *testing.T) {
	h := NewHelpOverlay()
	_, cmd := h.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
	if cmd == nil {
		t.Fatal("expected command on ?")
	}
	msg := cmd()
	if _, ok := msg.(CloseModalMsg); !ok {
		t.Error("expected CloseModalMsg on ?")
	}
}
