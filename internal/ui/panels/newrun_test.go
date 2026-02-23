package panels

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewRunModalDefaults(t *testing.T) {
	m := NewNewRunModal(120, 40)
	if m.Workflow() != "auto" {
		t.Errorf("default workflow = %q, want %q", m.Workflow(), "auto")
	}
	if m.Model() != "" {
		t.Errorf("default model = %q, want empty", m.Model())
	}
	if m.PromptValue() != "" {
		t.Errorf("default prompt = %q, want empty", m.PromptValue())
	}
}

func TestNewRunModalWorkflowCycle(t *testing.T) {
	m := NewNewRunModal(120, 40)
	if m.Workflow() != "auto" {
		t.Fatalf("expected initial workflow 'auto', got %q", m.Workflow())
	}

	// Each press of alt+w advances to the next workflow, wrapping around
	expected := []string{"build", "plan-build", "sdlc", "quick-fix", "auto"}
	for _, want := range expected {
		m, _ = m.Update(newRunKeyMsg("alt+w"))
		if m == nil {
			t.Fatal("modal was unexpectedly dismissed")
		}
		if m.Workflow() != want {
			t.Errorf("workflow = %q, want %q", m.Workflow(), want)
		}
	}
}

func TestNewRunModalModelCycle(t *testing.T) {
	m := NewNewRunModal(120, 40)
	if m.Model() != "" {
		t.Fatalf("expected initial model '', got %q", m.Model())
	}

	// Each press of alt+m advances to the next model, wrapping around
	expected := []string{"haiku", "opus", "sonnet", ""}
	for _, want := range expected {
		m, _ = m.Update(newRunKeyMsg("alt+m"))
		if m == nil {
			t.Fatal("modal was unexpectedly dismissed")
		}
		if m.Model() != want {
			t.Errorf("model = %q, want %q", m.Model(), want)
		}
	}
}

func TestNewRunModalSubmitDefaultAuto(t *testing.T) {
	m := NewNewRunModal(120, 40)

	// Type a prompt without changing workflow
	for _, ch := range "do the thing" {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{ch}})
		if m == nil {
			t.Fatal("modal dismissed during typing")
		}
	}

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
	if sub.Workflow != "auto" {
		t.Errorf("workflow = %q, want %q", sub.Workflow, "auto")
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

	// Cycle workflow to sdlc: auto -> build -> plan-build -> sdlc
	for i := 0; i < 3; i++ {
		m, _ = m.Update(newRunKeyMsg("alt+w"))
		if m == nil {
			t.Fatal("modal dismissed on workflow cycle")
		}
	}

	// Cycle model to opus: default -> haiku -> opus
	for i := 0; i < 2; i++ {
		m, _ = m.Update(newRunKeyMsg("alt+m"))
		if m == nil {
			t.Fatal("modal dismissed on model cycle")
		}
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
	for _, key := range []string{"esc", "ctrl+c"} {
		t.Run(key, func(t *testing.T) {
			m := NewNewRunModal(120, 40)

			result, cmd := m.Update(newRunKeyMsg(key))
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
		})
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
	return containsStr(plain, sub)
}

func containsStr(s, sub string) bool {
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

// newRunKeyMsg creates a tea.KeyMsg from a key string.
func newRunKeyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "ctrl+s":
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case "alt+w":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}, Alt: true}
	case "alt+m":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}, Alt: true}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
