package panels

import (
	"strings"
	"testing"

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
