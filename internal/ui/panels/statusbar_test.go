package panels

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/justinpbarnett/agtop/internal/run"
)

func TestStatusBarCounts(t *testing.T) {
	s := testStore()
	sb := NewStatusBar(s)
	sb.SetSize(120)

	view := sb.View()
	if !strings.Contains(view, "running") {
		t.Error("expected 'running' count in status bar")
	}
	if !strings.Contains(view, "queued") {
		t.Error("expected 'queued' count in status bar")
	}
	if !strings.Contains(view, "done") {
		t.Error("expected 'done' count in status bar")
	}
}

func TestStatusBarCost(t *testing.T) {
	s := testStore()
	sb := NewStatusBar(s)
	sb.SetSize(120)

	view := sb.View()
	if !strings.Contains(view, "Total:") {
		t.Error("expected 'Total:' in status bar")
	}
	if !strings.Contains(view, "$") {
		t.Error("expected cost in status bar")
	}
}

func TestStatusBarHelpHint(t *testing.T) {
	s := run.NewStore()
	sb := NewStatusBar(s)
	sb.SetSize(80)

	view := sb.View()
	if !strings.Contains(view, "?:help") {
		t.Error("expected '?:help' hint in status bar")
	}
}

func TestStatusBarVersion(t *testing.T) {
	s := run.NewStore()
	sb := NewStatusBar(s)
	sb.SetSize(80)

	view := sb.View()
	if !strings.Contains(view, "agtop") {
		t.Error("expected 'agtop' in status bar")
	}
}

func TestStatusBarFlashNoOverflow(t *testing.T) {
	s := run.NewStore()
	sb := NewStatusBar(s)
	sb.SetSize(80)
	sb.SetFlashWithLevel("unparseable workflow: some very long error message that exceeds terminal width", FlashError)

	view := sb.View()
	if w := lipgloss.Width(view); w > 80 {
		t.Errorf("status bar width %d exceeds terminal width 80", w)
	}
}

func TestStatusBarFlashFitsShowsFull(t *testing.T) {
	s := run.NewStore()
	sb := NewStatusBar(s)
	sb.SetSize(160)
	sb.SetFlash("short")

	view := sb.View()
	if !strings.Contains(view, "short") {
		t.Error("expected full flash message to appear when it fits")
	}
	if w := lipgloss.Width(view); w > 160 {
		t.Errorf("status bar width %d exceeds terminal width 160", w)
	}
}

func TestStatusBarFlashTruncatedWithEllipsis(t *testing.T) {
	s := run.NewStore()
	sb := NewStatusBar(s)
	sb.SetSize(80)
	sb.SetFlashWithLevel("this is a very long error message that will definitely be truncated at 80 cols", FlashError)

	view := sb.View()
	if w := lipgloss.Width(view); w > 80 {
		t.Errorf("status bar width %d exceeds terminal width 80", w)
	}
	if !strings.Contains(view, "…") {
		t.Error("expected ellipsis when flash message is truncated")
	}
}

func TestStatusBarNoFlashNoOverflow(t *testing.T) {
	s := run.NewStore()
	sb := NewStatusBar(s)
	sb.SetSize(80)

	view := sb.View()
	if w := lipgloss.Width(view); w > 80 {
		t.Errorf("status bar width %d exceeds terminal width 80 (no flash)", w)
	}
}
