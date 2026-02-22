package panels

import (
	"strings"
	"testing"
)

func TestLogViewTitle(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(60, 20)
	lv.SetRun("001", "build", "feat/auth", nil, true)

	view := lv.View()
	if !strings.Contains(view, "Log:") {
		t.Error("expected log title to contain 'Log:'")
	}
	if !strings.Contains(view, "build") {
		t.Error("expected log title to contain skill name")
	}
}

func TestLogViewDefaultContent(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(80, 30)

	view := lv.View()
	if !strings.Contains(view, "route") {
		t.Error("expected default mock log content")
	}
}

func TestLogViewAutoFollowDefault(t *testing.T) {
	lv := NewLogView()
	if !lv.follow {
		t.Error("expected follow to be true by default")
	}
}

func TestLogViewBorderPresent(t *testing.T) {
	lv := NewLogView()
	lv.SetSize(40, 10)
	view := lv.View()

	if !strings.Contains(view, "╭") || !strings.Contains(view, "╰") {
		t.Error("expected border characters in log view")
	}
}

func TestLogViewStreamingCursor(t *testing.T) {
	content := formatLogContent("line one\nline two", true)
	if !strings.Contains(content, "▍") {
		t.Error("expected streaming cursor ▍ for active content")
	}
}

func TestLogViewNoStreamingCursorForTerminal(t *testing.T) {
	content := formatLogContent("line one\nline two", false)
	if strings.Contains(content, "▍") {
		t.Error("expected no streaming cursor for inactive content")
	}
}

func TestLogViewTimestampStyling(t *testing.T) {
	// The formatted content should not contain raw "[HH:MM:SS skill]" brackets
	// since formatLogContent parses them into styled components
	content := formatLogContent("[14:32:01 route] test message", false)
	if strings.Contains(content, "[14:32:01") {
		t.Error("expected timestamp to be extracted from brackets")
	}
	if !strings.Contains(content, "14:32:01") {
		t.Error("expected timestamp value to be present")
	}
	if !strings.Contains(content, "route") {
		t.Error("expected skill name to be present")
	}
}

func TestLogViewUniformIndentation(t *testing.T) {
	// All parsed log lines should have the same indentation level
	content := formatLogContent("[14:32:01 route] message\n[14:32:18 build] Reading src/file.ts...", false)
	lines := strings.Split(content, "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	// Neither line should be indented differently
	routeIndent := len(lines[0]) - len(strings.TrimLeft(lines[0], " "))
	buildIndent := len(lines[1]) - len(strings.TrimLeft(lines[1], " "))
	if routeIndent != buildIndent {
		t.Errorf("expected uniform indentation: route=%d, build=%d", routeIndent, buildIndent)
	}
}
