package process

import (
	"context"
	"strings"
	"testing"
	"time"
)

func collectOCEvents(t *testing.T, parser *OpenCodeStreamParser, ctx context.Context) []StreamEvent {
	t.Helper()
	go parser.Parse(ctx)

	var events []StreamEvent
	timeout := time.After(2 * time.Second)
	for {
		select {
		case event, ok := <-parser.Events():
			if !ok {
				return events
			}
			events = append(events, event)
		case <-timeout:
			t.Fatal("timeout waiting for parser to finish")
			return events
		}
	}
}

// --- OpenCode format tests ---

func TestOCParseTextEvent(t *testing.T) {
	input := `{"type":"text","timestamp":1772046627730,"sessionID":"ses_abc","part":{"type":"text","text":"Hello from OpenCode","time":{"start":1772046627726,"end":1772046627726}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText, got %s", events[0].Type)
	}
	if events[0].Text != "Hello from OpenCode" {
		t.Errorf("expected 'Hello from OpenCode', got %q", events[0].Text)
	}
}

func TestOCParseReasoningEvent(t *testing.T) {
	input := `{"type":"reasoning","timestamp":1700000000,"sessionID":"ses_abc","part":{"type":"reasoning","text":"Let me think about this..."}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText for reasoning, got %s", events[0].Type)
	}
	if events[0].Text != "Let me think about this..." {
		t.Errorf("expected reasoning text, got %q", events[0].Text)
	}
}

func TestOCParseToolUse(t *testing.T) {
	input := `{"type":"tool_use","timestamp":1772046627957,"sessionID":"ses_abc","part":{"type":"tool","callID":"toolu_abc","tool":"glob","state":{"status":"completed","input":{"pattern":"justfile"},"output":"No files found","title":"","time":{"start":1772046627941,"end":1772046627956}}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 2 {
		t.Fatalf("expected 2 events (tool_use + tool_result), got %d", len(events))
	}
	if events[0].Type != EventToolUse {
		t.Errorf("expected EventToolUse, got %s", events[0].Type)
	}
	if events[0].ToolName != "glob" {
		t.Errorf("expected tool name 'glob', got %q", events[0].ToolName)
	}
	if events[1].Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %s", events[1].Type)
	}
	if events[1].Text != "No files found" {
		t.Errorf("expected 'No files found', got %q", events[1].Text)
	}
}

func TestOCParseToolUseRunning(t *testing.T) {
	input := `{"type":"tool_use","timestamp":1700000000,"sessionID":"ses_abc","part":{"type":"tool","tool":"Read","state":{"status":"running","input":{"file_path":"main.go"}}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event (no output yet), got %d", len(events))
	}
	if events[0].Type != EventToolUse {
		t.Errorf("expected EventToolUse, got %s", events[0].Type)
	}
	if events[0].ToolInput == "" {
		t.Error("expected non-empty tool input")
	}
}

func TestOCParseStepStart(t *testing.T) {
	input := `{"type":"step_start","timestamp":1772046624478,"sessionID":"ses_abc","part":{"id":"prt_abc","type":"step-start","snapshot":"306f4e627654f70fc87e53221355e9de77d035ce"}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 0 {
		t.Fatalf("expected 0 events for step_start, got %d", len(events))
	}
}

func TestOCParseStepFinishWithUsage(t *testing.T) {
	input := `{"type":"step_finish","timestamp":1772046624836,"sessionID":"ses_abc","part":{"type":"step-finish","reason":"tool-calls","cost":0,"tokens":{"total":16985,"input":2,"output":55,"reasoning":0,"cache":{"read":0,"write":16928}}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventResult {
		t.Errorf("expected EventResult, got %s", events[0].Type)
	}
	if events[0].Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if events[0].Usage.InputTokens != 2 {
		t.Errorf("expected 2 input tokens, got %d", events[0].Usage.InputTokens)
	}
	if events[0].Usage.OutputTokens != 55 {
		t.Errorf("expected 55 output tokens, got %d", events[0].Usage.OutputTokens)
	}
	if events[0].Usage.TotalTokens != 16985 {
		t.Errorf("expected 16985 total tokens, got %d", events[0].Usage.TotalTokens)
	}
}

func TestOCParseStepFinishNoTokens(t *testing.T) {
	input := `{"type":"step_finish","timestamp":1700000000,"sessionID":"ses_abc","part":{"reason":"end_turn"}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Usage != nil {
		t.Error("expected nil Usage when no tokens present")
	}
}

func TestOCParseErrorEvent(t *testing.T) {
	input := `{"type":"error","timestamp":1772046621622,"sessionID":"ses_abc","error":{"name":"UnknownError","data":{"message":"Error: Token refresh failed: 500"}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventError {
		t.Errorf("expected EventError, got %s", events[0].Type)
	}
	if events[0].Text != "Error: Token refresh failed: 500" {
		t.Errorf("expected error message, got %q", events[0].Text)
	}
}

func TestOCParseErrorEventNameFallback(t *testing.T) {
	input := `{"type":"error","timestamp":1700000000,"error":{"name":"UnknownError","data":{}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Text != "UnknownError" {
		t.Errorf("expected 'UnknownError', got %q", events[0].Text)
	}
}

// --- Claude Code format tests ---

func TestOCParseAssistantText(t *testing.T) {
	input := `{"type":"assistant","message":{"model":"claude-opus-4-6","id":"msg_abc","type":"message","role":"assistant","content":[{"type":"text","text":"I'll research the codebase."}],"usage":{"input_tokens":3,"output_tokens":11}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText, got %s", events[0].Type)
	}
	if events[0].Text != "I'll research the codebase." {
		t.Errorf("expected assistant text, got %q", events[0].Text)
	}
}

func TestOCParseAssistantToolUse(t *testing.T) {
	input := `{"type":"assistant","message":{"model":"claude-opus-4-6","id":"msg_abc","type":"message","role":"assistant","content":[{"type":"tool_use","id":"toolu_abc","name":"Glob","input":{"pattern":"**/*.go"}}],"usage":{"input_tokens":3,"output_tokens":5}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventToolUse {
		t.Errorf("expected EventToolUse, got %s", events[0].Type)
	}
	if events[0].ToolName != "Glob" {
		t.Errorf("expected tool name 'Glob', got %q", events[0].ToolName)
	}
}

func TestOCParseAssistantSkipsThinking(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"deep thoughts..."},{"type":"text","text":"Here is my answer."}]}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	// thinking blocks are skipped, only text emitted
	if len(events) != 1 {
		t.Fatalf("expected 1 event (thinking skipped), got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText, got %s", events[0].Type)
	}
	if events[0].Text != "Here is my answer." {
		t.Errorf("expected 'Here is my answer.', got %q", events[0].Text)
	}
}

func TestOCParseUserText(t *testing.T) {
	input := `{"type":"user","message":{"role":"user","content":[{"type":"text","text":"Please help me with this."}]},"session_id":"abc"}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventUser {
		t.Errorf("expected EventUser, got %s", events[0].Type)
	}
	if events[0].Text != "Please help me with this." {
		t.Errorf("expected user text, got %q", events[0].Text)
	}
}

func TestOCParseUserToolResult(t *testing.T) {
	// User events with only tool_result content blocks have no text
	input := `{"type":"user","message":{"role":"user","content":[{"tool_use_id":"toolu_abc","type":"tool_result","content":"file contents here"}]},"session_id":"abc"}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	// No text content block, so no event emitted
	if len(events) != 0 {
		t.Fatalf("expected 0 events for user tool_result, got %d", len(events))
	}
}

func TestOCParseResultSuccess(t *testing.T) {
	input := `{"type":"result","subtype":"success","is_error":false,"duration_ms":8859,"result":"plan-build","total_cost_usd":0.010820799999999998,"usage":{"input_tokens":10,"cache_creation_input_tokens":4252,"cache_read_input_tokens":18608,"output_tokens":727}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventResult {
		t.Errorf("expected EventResult, got %s", events[0].Type)
	}
	if events[0].Text != "plan-build" {
		t.Errorf("expected result text 'plan-build', got %q", events[0].Text)
	}
	if events[0].Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if events[0].Usage.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", events[0].Usage.InputTokens)
	}
	if events[0].Usage.OutputTokens != 727 {
		t.Errorf("expected 727 output tokens, got %d", events[0].Usage.OutputTokens)
	}
	if events[0].Usage.CostUSD != 0.010820799999999998 {
		t.Errorf("expected cost ~0.0108, got %f", events[0].Usage.CostUSD)
	}
}

func TestOCParseResultError(t *testing.T) {
	input := `{"type":"result","subtype":"success","is_error":true,"result":"API Error: 500","total_cost_usd":0.928,"usage":{"input_tokens":12,"output_tokens":3250}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventResult {
		t.Errorf("expected EventResult, got %s", events[0].Type)
	}
	if events[0].Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
}

func TestOCParseRateLimitWarning(t *testing.T) {
	input := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed_warning","resetsAt":1772049600,"rateLimitType":"seven_day","utilization":0.85,"isUsingOverage":false,"surpassedThreshold":0.75}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event for rate limit warning, got %d", len(events))
	}
	if events[0].Type != EventError {
		t.Errorf("expected EventError, got %s", events[0].Type)
	}
	if !strings.Contains(events[0].Text, "seven_day") {
		t.Errorf("expected rate limit type in text, got %q", events[0].Text)
	}
}

func TestOCParseRateLimitAllowed(t *testing.T) {
	input := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","rateLimitType":"seven_day"}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 0 {
		t.Fatalf("expected 0 events for allowed rate limit, got %d", len(events))
	}
}

func TestOCParseSystemInit(t *testing.T) {
	input := `{"type":"system","subtype":"init","cwd":"/tmp","model":"claude-haiku-4-5-20251001","claude_code_version":"2.1.56"}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw for system, got %s", events[0].Type)
	}
}

func TestOCParseSystemTaskStarted(t *testing.T) {
	input := `{"type":"system","subtype":"task_started","task_id":"a14ee348","description":"Research branch config"}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw for task_started, got %s", events[0].Type)
	}
}

// --- Edge case tests ---

func TestOCParseUnknownEventType(t *testing.T) {
	input := `{"type":"some_unknown_event","timestamp":1700000000}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw for unknown type, got %s", events[0].Type)
	}
}

func TestOCParseMalformedJSON(t *testing.T) {
	input := "this is not json\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw, got %s", events[0].Type)
	}
}

func TestOCParseMalformedPart(t *testing.T) {
	input := `{"type":"text","timestamp":1700000000,"part":"not an object"}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw for malformed part, got %s", events[0].Type)
	}
}

func TestOCParseEmptyLines(t *testing.T) {
	input := "\n\n" + `{"type":"text","timestamp":1700000000,"part":{"type":"text","text":"hi"}}` + "\n\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event (empty lines skipped), got %d", len(events))
	}
}

func TestOCParseMixedStream(t *testing.T) {
	// Simulates real stdout: Claude Code events followed by OpenCode events
	input := strings.Join([]string{
		`{"type":"system","subtype":"init","model":"claude-opus-4-6","claude_code_version":"2.1.56"}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"Starting work..."}]}}`,
		`{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","rateLimitType":"seven_day"}}`,
		`{"type":"result","subtype":"success","is_error":false,"result":"done","total_cost_usd":0.01,"usage":{"input_tokens":100,"output_tokens":50}}`,
		`{"type":"step_start","timestamp":1772046624478,"sessionID":"ses_abc","part":{"type":"step-start"}}`,
		`{"type":"text","timestamp":1772046627730,"sessionID":"ses_abc","part":{"type":"text","text":"Running tests."}}`,
		`{"type":"tool_use","timestamp":1772046627957,"sessionID":"ses_abc","part":{"type":"tool","tool":"bash","state":{"status":"completed","input":{"command":"make check"},"output":"ok"}}}`,
		`{"type":"step_finish","timestamp":1772046649775,"sessionID":"ses_abc","part":{"type":"step-finish","cost":0,"tokens":{"total":20249,"input":1,"output":115,"reasoning":0,"cache":{"read":19697,"write":436}}}}`,
	}, "\n") + "\n"

	parser := NewOpenCodeStreamParser(strings.NewReader(input), 20)
	events := collectOCEvents(t, parser, context.Background())

	// system → EventRaw
	// assistant text → EventText
	// rate_limit allowed → skipped
	// result → EventResult
	// step_start → skipped
	// text → EventText
	// tool_use (with output) → EventToolUse + EventToolResult
	// step_finish → EventResult
	expected := []StreamEventType{
		EventRaw,       // system
		EventText,      // assistant text
		EventResult,    // result
		EventText,      // opencode text
		EventToolUse,   // opencode tool_use
		EventToolResult, // opencode tool output
		EventResult,    // step_finish
	}

	if len(events) != len(expected) {
		t.Fatalf("expected %d events, got %d", len(expected), len(events))
	}
	for i, exp := range expected {
		if events[i].Type != exp {
			t.Errorf("event %d: expected %s, got %s", i, exp, events[i].Type)
		}
	}
}
