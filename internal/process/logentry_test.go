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

func TestLineToEntryRateLimit(t *testing.T) {
	e := lineToEntry("[14:32:01 build] RATE LIMITED: 429 Too Many Requests")
	if e != nil {
		t.Errorf("expected nil for rate limit entry, got %+v", e)
	}
}

func TestLineToEntryUser(t *testing.T) {
	e := lineToEntry("[14:32:01 build] User: implement the feature")
	if e.Type != EventUser {
		t.Errorf("expected EventUser, got %v", e.Type)
	}
	if e.Summary != "User: implement the feature" {
		t.Errorf("unexpected summary: %q", e.Summary)
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

func TestNewLogEntryUser(t *testing.T) {
	e := NewLogEntry("14:32:01", "build", EventUser, "implement the feature")
	if e.Summary != "User: implement the feature" {
		t.Errorf("unexpected summary: %q", e.Summary)
	}
	if e.Type != EventUser {
		t.Errorf("expected EventUser, got %v", e.Type)
	}
}

func TestToolUseSummaryWebSearch(t *testing.T) {
	s := ToolUseSummary("WebSearch", `{"query":"golang word wrap"}`)
	if s != "Tool: WebSearch — golang word wrap" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryWebFetch(t *testing.T) {
	s := ToolUseSummary("WebFetch", `{"url":"https://example.com/docs"}`)
	if s != "Tool: WebFetch — https://example.com/docs" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestToolUseSummaryTask(t *testing.T) {
	s := ToolUseSummary("Task", `{"description":"explore codebase"}`)
	if s != "Tool: Task — explore codebase" {
		t.Errorf("unexpected: %q", s)
	}
}

func TestFormatJSONValid(t *testing.T) {
	input := `{"file_path":"src/main.go","old_string":"foo"}`
	got := FormatJSON(input)
	if !strings.Contains(got, "\n") {
		t.Errorf("expected multi-line output, got %q", got)
	}
	if !strings.Contains(got, "  ") {
		t.Errorf("expected indentation, got %q", got)
	}
	if !strings.Contains(got, "file_path") {
		t.Error("expected key to be preserved")
	}
}

func TestFormatJSONInvalid(t *testing.T) {
	input := "not json at all"
	got := FormatJSON(input)
	if got != input {
		t.Errorf("expected input unchanged, got %q", got)
	}
}

func TestFormatJSONEmpty(t *testing.T) {
	got := FormatJSON("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestWordWrapShortLine(t *testing.T) {
	input := "hello world"
	got := WordWrap(input, 80)
	if got != input {
		t.Errorf("short line should be unchanged, got %q", got)
	}
}

func TestWordWrapLongLine(t *testing.T) {
	input := "the quick brown fox jumps over the lazy dog and then some more words"
	got := WordWrap(input, 30)
	lines := strings.Split(got, "\n")
	for i, line := range lines {
		if len(line) > 30 {
			t.Errorf("line %d exceeds width: %q (%d chars)", i, line, len(line))
		}
	}
	if len(lines) < 2 {
		t.Error("expected multiple lines after wrapping")
	}
}

func TestWordWrapPreservesNewlines(t *testing.T) {
	input := "line one\nline two\nline three"
	got := WordWrap(input, 80)
	if got != input {
		t.Errorf("existing newlines should be preserved, got %q", got)
	}
}

func TestWordWrapNoSpaces(t *testing.T) {
	input := strings.Repeat("x", 50)
	got := WordWrap(input, 20)
	lines := strings.Split(got, "\n")
	if len(lines) < 2 {
		t.Error("expected word-less line to be broken at width")
	}
}

func TestWordWrapZeroWidth(t *testing.T) {
	input := "hello"
	got := WordWrap(input, 0)
	if got != input {
		t.Errorf("zero width should return input unchanged, got %q", got)
	}
}

func TestInterpretRawEventSystemInit(t *testing.T) {
	raw := `{"type":"system","subtype":"init","model":"claude-haiku-4-5-20251001","permissionMode":"acceptEdits","tools":["Task","Bash","Read","Edit","Write"],"claude_code_version":"2.1.50"}`
	e := InterpretRawEvent("14:32:01", "build", raw)
	if !strings.Contains(e.Summary, "Session init") {
		t.Errorf("expected 'Session init' in summary, got %q", e.Summary)
	}
	if !strings.Contains(e.Summary, "claude-haiku-4-5-20251001") {
		t.Errorf("expected model in summary, got %q", e.Summary)
	}
	if !strings.Contains(e.Summary, "acceptEdits") {
		t.Errorf("expected permissionMode in summary, got %q", e.Summary)
	}
	if !strings.Contains(e.Summary, "5 tools") {
		t.Errorf("expected tool count in summary, got %q", e.Summary)
	}
	if !strings.Contains(e.Summary, "v2.1.50") {
		t.Errorf("expected version in summary, got %q", e.Summary)
	}
	// Detail should be structured human-readable format, not raw JSON
	if !strings.Contains(e.Detail, "Tools (5):") {
		t.Errorf("expected 'Tools (5):' in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "Task, Bash, Read, Edit, Write") {
		t.Errorf("expected tool list in detail, got %q", e.Detail)
	}
}

func TestInterpretRawEventSystemInitFull(t *testing.T) {
	raw := `{"type":"system","subtype":"init","cwd":"/tmp/worktrees/006","session_id":"07d89954-001b","tools":["Task","Bash"],"mcp_servers":[{"name":"github","status":"failed"}],"model":"claude-sonnet-4-6","permissionMode":"acceptEdits","claude_code_version":"2.1.50","agents":["Bash","Explore"],"skills":["commit","spec"],"plugins":[{"name":"github","path":"/home/.claude/plugins/github"}]}`
	e := InterpretRawEvent("09:06:49", "build", raw)

	// Check structured detail sections
	if !strings.Contains(e.Detail, "v2.1.50 · claude-sonnet-4-6 · acceptEdits") {
		t.Errorf("expected header in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "Session: 07d89954-001b") {
		t.Errorf("expected session ID in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "CWD: /tmp/worktrees/006") {
		t.Errorf("expected CWD in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "Tools (2): Task, Bash") {
		t.Errorf("expected tools in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "Agents (2): Bash, Explore") {
		t.Errorf("expected agents in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "Skills (2): commit, spec") {
		t.Errorf("expected skills in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "Plugins (1): github") {
		t.Errorf("expected plugins in detail, got %q", e.Detail)
	}
	if !strings.Contains(e.Detail, "MCP: github (failed)") {
		t.Errorf("expected MCP servers in detail, got %q", e.Detail)
	}
}

func TestInterpretRawEventUnknownJSON(t *testing.T) {
	raw := `{"type":"something_new","data":"hello"}`
	e := InterpretRawEvent("14:32:01", "", raw)
	if e.Summary != "[something_new]" {
		t.Errorf("expected [something_new], got %q", e.Summary)
	}
	if !strings.Contains(e.Detail, "\n") {
		t.Error("expected formatted JSON in detail")
	}
}

func TestInterpretRawEventUnknownJSONWithSubtype(t *testing.T) {
	raw := `{"type":"foo","subtype":"bar"}`
	e := InterpretRawEvent("14:32:01", "", raw)
	if e.Summary != "[foo/bar]" {
		t.Errorf("expected [foo/bar], got %q", e.Summary)
	}
}

func TestInterpretRawEventPlainText(t *testing.T) {
	e := InterpretRawEvent("14:32:01", "build", "just plain text")
	if e.Summary != "just plain text" {
		t.Errorf("expected plain text summary, got %q", e.Summary)
	}
	if e.Type != EventRaw {
		t.Errorf("expected EventRaw, got %v", e.Type)
	}
}

func TestInterpretRawEventSystemUnknownSubtype(t *testing.T) {
	raw := `{"type":"system","subtype":"shutdown"}`
	e := InterpretRawEvent("14:32:01", "", raw)
	if e.Summary != "[system/shutdown]" {
		t.Errorf("expected [system/shutdown], got %q", e.Summary)
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
