package panels

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/justinpbarnett/agtop/internal/process"
)

func TestLogViewEmptySnapshot(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	lv.SetFocused(true)

	tm := teatest.NewTestModel(t, wrapLogView(&lv), teatest.WithInitialTermSize(80, 30))
	waitForContains(t, tm, "Log")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(lv.View()))
}

func TestLogViewWithContentSnapshot(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	lv.SetFocused(true)

	buf := process.NewRingBuffer(1000)
	buf.Append("[14:32:01 route] Analyzing task requirements...")
	buf.Append("[14:32:03 route] Selected workflow: build")
	buf.Append("[14:32:04 build] Starting build skill...")
	buf.Append("[14:32:05 build] Reading project files")

	lv.SetRun("001", "build", "feat/add-auth", buf, nil, true)

	tm := teatest.NewTestModel(t, wrapLogView(&lv), teatest.WithInitialTermSize(80, 30))
	waitForContains(t, tm, "build")
	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
	teatest.RequireEqualOutput(t, []byte(lv.View()))
}

func TestLogViewTabSwitchFlow(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)
	lv.SetFocused(true)

	buf := process.NewRingBuffer(1000)
	buf.Append("[14:32:01 build] Working...")
	lv.SetRun("001", "build", "feat/test", buf, nil, false)

	tm := teatest.NewTestModel(t, wrapLogView(&lv), teatest.WithInitialTermSize(80, 30))
	waitForContains(t, tm, "Log")

	// Start on log tab (0), switch to diff tab (1)
	if lv.activeTab != tabLog {
		t.Fatalf("expected initial tab to be log, got %d", lv.activeTab)
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("l")})
	time.Sleep(100 * time.Millisecond)

	if lv.activeTab != tabDiff {
		t.Errorf("expected diff tab after l, got %d", lv.activeTab)
	}

	// Switch back to log tab
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	time.Sleep(100 * time.Millisecond)

	if lv.activeTab != tabLog {
		t.Errorf("expected log tab after h, got %d", lv.activeTab)
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestLogViewNavigationFlow(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 10)
	lv.SetFocused(true)

	buf := process.NewRingBuffer(1000)
	for i := 0; i < 50; i++ {
		buf.Append("[14:32:01 build] Log line content for testing scrolling behavior")
	}
	lv.SetRun("001", "build", "feat/test", buf, nil, false)

	tm := teatest.NewTestModel(t, wrapLogView(&lv), teatest.WithInitialTermSize(80, 10))
	waitForContains(t, tm, "Log")

	// Navigate down several times
	for i := 0; i < 5; i++ {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	}
	time.Sleep(100 * time.Millisecond)

	// Jump to top with gg
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	time.Sleep(100 * time.Millisecond)

	if lv.follow {
		t.Error("expected follow to be false after manual navigation")
	}

	// Jump to bottom with G
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("G")})
	time.Sleep(100 * time.Millisecond)

	if !lv.follow {
		t.Error("expected follow to be true after G")
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}

func TestLogViewSearchFlow(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 20)
	lv.SetFocused(true)

	buf := process.NewRingBuffer(1000)
	buf.Append("[14:32:01 build] Starting build process")
	buf.Append("[14:32:02 build] Compiling source files")
	buf.Append("[14:32:03 test] Running test suite")
	buf.Append("[14:32:04 build] Build complete")
	buf.Append("[14:32:05 test] Tests passed")
	lv.SetRun("001", "build", "feat/test", buf, nil, false)

	tm := teatest.NewTestModel(t, wrapLogView(&lv), teatest.WithInitialTermSize(80, 20))
	waitForContains(t, tm, "Log")

	// Activate search
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	time.Sleep(50 * time.Millisecond)

	if !lv.searching {
		t.Fatal("expected searching to be true after /")
	}

	// Type search query
	for _, c := range "test" {
		tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{c}})
	}
	time.Sleep(100 * time.Millisecond)

	// Confirm search
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})
	time.Sleep(100 * time.Millisecond)

	if lv.searching {
		t.Error("expected searching to be false after Enter")
	}
	if lv.searchQuery != "test" {
		t.Errorf("expected searchQuery 'test', got %q", lv.searchQuery)
	}
	if len(lv.matchIndices) != 2 {
		t.Errorf("expected 2 matches for 'test', got %d", len(lv.matchIndices))
	}

	// Navigate matches with n/N
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")})
	time.Sleep(50 * time.Millisecond)

	if lv.currentMatch != 1 {
		t.Errorf("expected currentMatch 1 after n, got %d", lv.currentMatch)
	}

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("N")})
	time.Sleep(50 * time.Millisecond)

	if lv.currentMatch != 0 {
		t.Errorf("expected currentMatch 0 after N, got %d", lv.currentMatch)
	}

	tm.Send(tea.QuitMsg{})
	tm.FinalModel(t, teatest.WithFinalTimeout(waitDuration))
}
