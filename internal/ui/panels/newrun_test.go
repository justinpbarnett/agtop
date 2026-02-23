package panels

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewRunModalDefaults(t *testing.T) {
	m := NewNewRunModal(120, 40)
	if m.Workflow() != "build" {
		t.Errorf("default workflow = %q, want %q", m.Workflow(), "build")
	}
	if m.Model() != "" {
		t.Errorf("default model = %q, want empty", m.Model())
	}
	if m.PromptValue() != "" {
		t.Errorf("default prompt = %q, want empty", m.PromptValue())
	}
}

func TestNewRunModalWorkflowSelection(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"ctrl+b", "build"},
		{"ctrl+p", "plan-build"},
		{"ctrl+l", "sdlc"},
		{"ctrl+q", "quick-fix"},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := NewNewRunModal(120, 40)
			m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: nil}) // dummy to init
			m, _ = m.Update(newRunKeyMsg(tt.key))
			if m == nil {
				t.Fatal("modal was unexpectedly dismissed")
			}
			if m.Workflow() != tt.expected {
				t.Errorf("workflow = %q, want %q", m.Workflow(), tt.expected)
			}
		})
	}
}

func TestNewRunModalModelOverride(t *testing.T) {
	tests := []struct {
		key      string
		expected string
	}{
		{"ctrl+h", "haiku"},
		{"ctrl+o", "opus"},
		{"ctrl+n", "sonnet"},
		{"ctrl+x", ""},
	}

	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			m := NewNewRunModal(120, 40)
			m, _ = m.Update(newRunKeyMsg(tt.key))
			if m == nil {
				t.Fatal("modal was unexpectedly dismissed")
			}
			if m.Model() != tt.expected {
				t.Errorf("model = %q, want %q", m.Model(), tt.expected)
			}
		})
	}
}

func TestNewRunModalSubmit(t *testing.T) {
	m := NewNewRunModal(120, 40)

	// Type a prompt
	for _, ch := range "fix the bug" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		if m == nil {
			t.Fatal("modal dismissed during typing")
		}
	}

	// Select workflow
	m, _ = m.Update(newRunKeyMsg("ctrl+l"))
	if m == nil {
		t.Fatal("modal dismissed on workflow select")
	}

	// Select model
	m, _ = m.Update(newRunKeyMsg("ctrl+o"))
	if m == nil {
		t.Fatal("modal dismissed on model select")
	}

	// Submit
	result, cmd := m.Update(newRunKeyMsg("ctrl+s"))
	if result != nil {
		t.Error("modal should be nil after submit")
	}
	if cmd == nil {
		t.Fatal("expected a command from submit")
	}

	msg := cmd()
	sub, ok := msg.(SubmitNewRunMsg)
	if !ok {
		t.Fatalf("expected SubmitNewRunMsg, got %T", msg)
	}
	if sub.Prompt != "fix the bug" {
		t.Errorf("prompt = %q, want %q", sub.Prompt, "fix the bug")
	}
	if sub.Workflow != "sdlc" {
		t.Errorf("workflow = %q, want %q", sub.Workflow, "sdlc")
	}
	if sub.Model != "opus" {
		t.Errorf("model = %q, want %q", sub.Model, "opus")
	}
}

func TestNewRunModalEmptyPromptNoSubmit(t *testing.T) {
	m := NewNewRunModal(120, 40)

	// Try to submit with empty prompt
	result, cmd := m.Update(newRunKeyMsg("ctrl+s"))
	if result == nil {
		t.Error("modal should stay open on empty prompt submit")
	}
	if cmd != nil {
		t.Error("should not produce a command on empty prompt submit")
	}
}

func TestNewRunModalCancel(t *testing.T) {
	m := NewNewRunModal(120, 40)

	result, cmd := m.Update(newRunKeyMsg("esc"))
	if result != nil {
		t.Error("modal should be nil after cancel")
	}
	if cmd == nil {
		t.Fatal("expected a command from cancel")
	}

	msg := cmd()
	if _, ok := msg.(CloseModalMsg); !ok {
		t.Errorf("expected CloseModalMsg, got %T", msg)
	}
}

func TestNewRunModalTextInput(t *testing.T) {
	m := NewNewRunModal(120, 40)

	for _, ch := range "hello" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		if m == nil {
			t.Fatal("modal dismissed during typing")
		}
	}

	if m.PromptValue() != "hello" {
		t.Errorf("prompt value = %q, want %q", m.PromptValue(), "hello")
	}
}

func TestNewRunModalView(t *testing.T) {
	m := NewNewRunModal(120, 40)
	view := m.View()

	checks := []string{"New Run", "Workflow", "Model", "^S", "Esc"}
	for _, s := range checks {
		if !containsPlain(view, s) {
			t.Errorf("view missing %q", s)
		}
	}
}

// containsPlain checks if s contains sub, ignoring ANSI escape sequences.
func containsPlain(s, sub string) bool {
	// Strip ANSI for a simple check
	plain := stripAnsi(s)
	return contains(plain, sub)
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && searchString(s, sub)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func stripAnsi(s string) string {
	var result []byte
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we find the terminating letter
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the terminating letter
			}
			i = j
		} else {
			result = append(result, s[i])
			i++
		}
	}
	return string(result)
}

// keyMsg creates a tea.KeyMsg from a key string.
func newRunKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "ctrl+b":
		return tea.KeyMsg{Type: tea.KeyCtrlB}
	case "ctrl+p":
		return tea.KeyMsg{Type: tea.KeyCtrlP}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "ctrl+l":
		return tea.KeyMsg{Type: tea.KeyCtrlL}
	case "ctrl+q":
		return tea.KeyMsg{Type: tea.KeyCtrlQ}
	case "ctrl+h":
		return tea.KeyMsg{Type: tea.KeyCtrlH}
	case "ctrl+o":
		return tea.KeyMsg{Type: tea.KeyCtrlO}
	case "ctrl+n":
		return tea.KeyMsg{Type: tea.KeyCtrlN}
	case "ctrl+x":
		return tea.KeyMsg{Type: tea.KeyCtrlX}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
