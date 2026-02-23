package process

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestNewLogEntryTextSummary(t *testing.T) {
	e := NewLogEntry("14:32:01", "build", EventText, "Building the project now")
	if e.Summary != "Building the project now" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestNewLogEntryTextTruncation(t *testing.T) {
	long := strings.Repeat("a", 100)
	e := NewLogEntry("14:32:01", "build", EventText, long)
	// 80 chars + "…" (3 bytes UTF-8) = 83 bytes
	if len(e.Summary) > 83 {
		t.Errorf("summary too long: %d bytes", len(e.Summary))
	}
	if !strings.HasSuffix(e.Summary, "…") {
		t.Error("expected truncated summary to end with …")
	}
}

func TestNewLogEntryTextMultiline(t *testing.T) {
	e := NewLogEntry("14:32:01", "build", EventText, "first line\nsecond line\nthird line")
	if e.Summary != "first line" {
		t.Errorf("expected first line as summary, got %q", e.Summary)
	}
}

func TestNewLogEntryToolUse(t *testing.T) {
	e := NewLogEntry("14:32:01", "build", EventToolUse, "Read")
	if e.Summary != "Tool: Read" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestNewLogEntryToolResultShort(t *testing.T) {
	e := NewLogEntry("14:32:01", "build", EventToolResult, "File contents here")
	if e.Summary != "Result: File contents here" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestNewLogEntryToolResultLong(t *testing.T) {
	long := strings.Repeat("x", 250)
	e := NewLogEntry("14:32:01", "build", EventToolResult, long)
	expected := fmt.Sprintf("Result: (%d chars)", 250)
	if e.Summary != expected {
		t.Errorf("expected %q, got %q", expected, e.Summary)
	}
}

func TestNewLogEntryResult(t *testing.T) {
	e := NewLogEntry("14:32:01", "build", EventResult, "Completed — 1500 tokens, $0.0300")
	if e.Summary != "Completed — 1500 tokens, $0.0300" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestNewLogEntryError(t *testing.T) {
	e := NewLogEntry("14:32:01", "build", EventError, "connection refused")
	if e.Summary != "ERROR: connection refused" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestNewLogEntryRaw(t *testing.T) {
	e := NewLogEntry("14:32:01", "", EventRaw, "some raw output")
	if e.Summary != "some raw output" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestToolUseSummaryRead(t *testing.T) {
	s := ToolUseSummary("Read", `{"file_path":"src/main.go"}`)
	if s != "Tool: Read — src/main.go" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryBash(t *testing.T) {
	s := ToolUseSummary("Bash", `{"command":"git status"}`)
	if s != "Tool: Bash — git status" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryEdit(t *testing.T) {
	s := ToolUseSummary("Edit", `{"file_path":"src/app.go"}`)
	if s != "Tool: Edit — src/app.go" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryGlob(t *testing.T) {
	s := ToolUseSummary("Glob", `{"pattern":"**/*.ts"}`)
	if s != "Tool: Glob — **/*.ts" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryGrep(t *testing.T) {
	s := ToolUseSummary("Grep", `{"pattern":"TODO"}`)
	if s != "Tool: Grep — TODO" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryUnknown(t *testing.T) {
	s := ToolUseSummary("CustomTool", `{"foo":"bar"}`)
	if s != "Tool: CustomTool" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryEmpty(t *testing.T) {
	s := ToolUseSummary("Read", "")
	if s != "Tool: Read" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestEntryBufferAppendAndEntries(t *testing.T) {
	eb := NewEntryBuffer(10)
	eb.Append(&LogEntry{Summary: "a"})
	eb.Append(&LogEntry{Summary: "b"})
	eb.Append(&LogEntry{Summary: "c"})

	entries := eb.Entries()
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
	if entries[0].Summary != "a" || entries[1].Summary != "b" || entries[2].Summary != "c" {
		t.Error("entries not in expected order")
	}
}

func TestEntryBufferCapacityWrap(t *testing.T) {
	eb := NewEntryBuffer(3)
	for i := 0; i < 5; i++ {
		eb.Append(&LogEntry{Summary: fmt.Sprintf("%d", i)})
	}
	if eb.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", eb.Len())
	}
	entries := eb.Entries()
	// Should have entries 2, 3, 4
	if entries[0].Summary != "2" || entries[1].Summary != "3" || entries[2].Summary != "4" {
		t.Errorf("unexpected entries after wrap: %s, %s, %s", entries[0].Summary, entries[1].Summary, entries[2].Summary)
	}
}

func TestEntryBufferGet(t *testing.T) {
	eb := NewEntryBuffer(10)
	eb.Append(&LogEntry{Summary: "first"})
	eb.Append(&LogEntry{Summary: "second"})

	e := eb.Get(0)
	if e == nil || e.Summary != "first" {
		t.Error("Get(0) failed")
	}
	e = eb.Get(1)
	if e == nil || e.Summary != "second" {
		t.Error("Get(1) failed")
	}
	e = eb.Get(2)
	if e != nil {
		t.Error("Get(2) should be nil")
	}
	e = eb.Get(-1)
	if e != nil {
		t.Error("Get(-1) should be nil")
	}
}

func TestEntryBufferGetAfterWrap(t *testing.T) {
	eb := NewEntryBuffer(3)
	for i := 0; i < 5; i++ {
		eb.Append(&LogEntry{Summary: fmt.Sprintf("%d", i)})
	}
	e := eb.Get(0)
	if e == nil || e.Summary != "2" {
		t.Errorf("expected oldest=2, got %v", e)
	}
	e = eb.Get(2)
	if e == nil || e.Summary != "4" {
		t.Errorf("expected newest=4, got %v", e)
	}
}

func TestEntryBufferUpdateLast(t *testing.T) {
	eb := NewEntryBuffer(10)
	eb.Append(&LogEntry{Summary: "initial", Detail: "v1"})
	eb.UpdateLast(func(e *LogEntry) {
		e.Detail = "v2"
		e.Complete = true
	})
	entries := eb.Entries()
	if entries[0].Detail != "v2" {
		t.Errorf("expected v2, got %q", entries[0].Detail)
	}
	if !entries[0].Complete {
		t.Error("expected Complete to be true")
	}
}

func TestEntryBufferUpdateLastEmpty(t *testing.T) {
	eb := NewEntryBuffer(10)
	// Should not panic
	eb.UpdateLast(func(e *LogEntry) {
		e.Summary = "oops"
	})
}

func TestEntryBufferLen(t *testing.T) {
	eb := NewEntryBuffer(10)
	if eb.Len() != 0 {
		t.Error("expected 0")
	}
	eb.Append(&LogEntry{Summary: "a"})
	if eb.Len() != 1 {
		t.Error("expected 1")
	}
}

func TestEntryBufferConcurrency(t *testing.T) {
	eb := NewEntryBuffer(100)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				eb.Append(&LogEntry{Summary: fmt.Sprintf("w%d-%d", id, j)})
			}
		}(i)
	}
	wg.Wait()
	if eb.Len() != 100 {
		t.Errorf("expected 100, got %d", eb.Len())
	}
}

func TestEntryBufferEntriesEmpty(t *testing.T) {
	eb := NewEntryBuffer(10)
	if eb.Entries() != nil {
		t.Error("expected nil for empty buffer")
	}
}

func TestLineToEntryToolUse(t *testing.T) {
	e := lineToEntry("[14:32:01 build] Tool: Read")
	if e.Type != EventToolUse {
		t.Errorf("expected EventToolUse, got %v", e.Type)
	}
	if e.Summary != "Tool: Read" {
		t.Errorf("expected summary 'Tool: Read', got %q", e.Summary)
	}
	if e.Timestamp != "14:32:01" {
		t.Errorf("expected ts '14:32:01', got %q", e.Timestamp)
	}
	if e.Skill != "build" {
		t.Errorf("expected skill 'build', got %q", e.Skill)
	}
}

func TestLineToEntryToolResult(t *testing.T) {
	e := lineToEntry("[14:32:02 build] Result: File contents here")
	if e.Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %v", e.Type)
	}
	if e.Summary != "Result: File contents here" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestLineToEntryError(t *testing.T) {
	e := lineToEntry("[14:32:03 build] ERROR: connection refused")
	if e.Type != EventError {
		t.Errorf("expected EventError, got %v", e.Type)
	}
	if e.Summary != "ERROR: connection refused" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestLineToEntryCompleted(t *testing.T) {
	e := lineToEntry("[14:32:04 build] Completed — 1500 tokens, $0.0300")
	if e.Type != EventResult {
		t.Errorf("expected EventResult, got %v", e.Type)
	}
	if e.Summary != "Completed — 1500 tokens, $0.0300" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestLineToEntryText(t *testing.T) {
	e := lineToEntry("[14:32:05 build] Building the project")
	if e.Type != EventText {
		t.Errorf("expected EventText, got %v", e.Type)
	}
	if e.Summary != "Building the project" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestLineToEntryNoSkill(t *testing.T) {
	e := lineToEntry("[14:32:06] Tool: Bash")
	if e.Type != EventToolUse {
		t.Errorf("expected EventToolUse, got %v", e.Type)
	}
	if e.Skill != "" {
		t.Errorf("expected empty skill, got %q", e.Skill)
	}
}

func TestLineToEntryUnparseable(t *testing.T) {
	e := lineToEntry("raw output with no timestamp")
	if e.Type != EventRaw {
		t.Errorf("expected EventRaw, got %v", e.Type)
	}
	if e.Summary != "raw output with no timestamp" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
}

func TestEntryBufferTotalEvicted(t *testing.T) {
	eb := NewEntryBuffer(3)
	if eb.TotalEvicted() != 0 {
		t.Error("expected 0 evictions initially")
	}
	eb.Append(&LogEntry{Summary: "0"})
	eb.Append(&LogEntry{Summary: "1"})
	eb.Append(&LogEntry{Summary: "2"})
	if eb.TotalEvicted() != 0 {
		t.Error("expected 0 evictions when buffer not yet full")
	}
	eb.Append(&LogEntry{Summary: "3"})
	if eb.TotalEvicted() != 1 {
		t.Errorf("expected 1 eviction, got %d", eb.TotalEvicted())
	}
	eb.Append(&LogEntry{Summary: "4"})
	eb.Append(&LogEntry{Summary: "5"})
	if eb.TotalEvicted() != 3 {
		t.Errorf("expected 3 evictions, got %d", eb.TotalEvicted())
	}
}
