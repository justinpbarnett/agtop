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

func TestOCParseTextEvent(t *testing.T) {
	input := `{"type":"message.part.updated","properties":{"part":{"type":"text","text":"Hello from OpenCode"}}}` + "\n"
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

func TestOCParseTextContentField(t *testing.T) {
	input := `{"type":"message.part.updated","properties":{"part":{"type":"text","content":"via content field"}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Text != "via content field" {
		t.Errorf("expected 'via content field', got %q", events[0].Text)
	}
}

func TestOCParseToolInvocation(t *testing.T) {
	input := `{"type":"message.part.updated","properties":{"part":{"type":"tool-invocation","toolName":"Read","input":{"file_path":"main.go"}}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventToolUse {
		t.Errorf("expected EventToolUse, got %s", events[0].Type)
	}
	if events[0].ToolName != "Read" {
		t.Errorf("expected tool name 'Read', got %q", events[0].ToolName)
	}
	if events[0].ToolInput == "" {
		t.Error("expected non-empty tool input")
	}
}

func TestOCParseToolResult(t *testing.T) {
	input := `{"type":"message.part.updated","properties":{"part":{"type":"tool-result","text":"file contents here"}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventToolResult {
		t.Errorf("expected EventToolResult, got %s", events[0].Type)
	}
	if events[0].Text != "file contents here" {
		t.Errorf("expected 'file contents here', got %q", events[0].Text)
	}
}

func TestOCParseReasoning(t *testing.T) {
	input := `{"type":"message.part.updated","properties":{"part":{"type":"reasoning","text":"thinking about this..."}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText for reasoning, got %s", events[0].Type)
	}
	if events[0].Text != "thinking about this..." {
		t.Errorf("expected 'thinking about this...', got %q", events[0].Text)
	}
}

func TestOCParseMessageUpdatedWithUsage(t *testing.T) {
	input := `{"type":"message.updated","properties":{"message":{"role":"assistant","content":"done","usage":{"input_tokens":1000,"output_tokens":500,"total_cost":0.03}}}}` + "\n"
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
	if events[0].Usage.InputTokens != 1000 {
		t.Errorf("expected 1000 input tokens, got %d", events[0].Usage.InputTokens)
	}
	if events[0].Usage.OutputTokens != 500 {
		t.Errorf("expected 500 output tokens, got %d", events[0].Usage.OutputTokens)
	}
	if events[0].Usage.TotalTokens != 1500 {
		t.Errorf("expected 1500 total tokens, got %d", events[0].Usage.TotalTokens)
	}
	if events[0].Usage.CostUSD != 0.03 {
		t.Errorf("expected 0.03 cost, got %f", events[0].Usage.CostUSD)
	}
	if events[0].Text != "done" {
		t.Errorf("expected text 'done', got %q", events[0].Text)
	}
}

func TestOCParseSessionUpdatedWithUsage(t *testing.T) {
	input := `{"type":"session.updated","properties":{"session":{"status":"completed","usage":{"input_tokens":2000,"output_tokens":800,"total_cost":0.05}}}}` + "\n"
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
	if events[0].Usage.TotalTokens != 2800 {
		t.Errorf("expected 2800 total tokens, got %d", events[0].Usage.TotalTokens)
	}
	if events[0].Usage.CostUSD != 0.05 {
		t.Errorf("expected 0.05 cost, got %f", events[0].Usage.CostUSD)
	}
}

func TestOCParseSessionUpdatedNoUsage(t *testing.T) {
	input := `{"type":"session.updated","properties":{"session":{"status":"running"}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	// No usage data means no event emitted for session updates
	if len(events) != 0 {
		t.Fatalf("expected 0 events for session update without usage, got %d", len(events))
	}
}

func TestOCParseErrorEvent(t *testing.T) {
	input := `{"type":"error","properties":{"error":"rate limit exceeded"}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventError {
		t.Errorf("expected EventError, got %s", events[0].Type)
	}
	if events[0].Text != "rate limit exceeded" {
		t.Errorf("expected 'rate limit exceeded', got %q", events[0].Text)
	}
}

func TestOCParseErrorEventMessageField(t *testing.T) {
	input := `{"type":"error","properties":{"message":"something went wrong"}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Text != "something went wrong" {
		t.Errorf("expected 'something went wrong', got %q", events[0].Text)
	}
}

func TestOCParseUnknownEventType(t *testing.T) {
	input := `{"type":"some.unknown.event","properties":{"foo":"bar"}}` + "\n"
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

func TestOCParseEmptyLines(t *testing.T) {
	input := "\n\n" + `{"type":"message.part.updated","properties":{"part":{"type":"text","text":"hi"}}}` + "\n\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event (empty lines skipped), got %d", len(events))
	}
}

func TestOCParseMultipleEvents(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"message.part.updated","properties":{"part":{"type":"text","text":"thinking..."}}}`,
		`{"type":"message.part.updated","properties":{"part":{"type":"tool-invocation","toolName":"Bash","input":{"command":"ls"}}}}`,
		`{"type":"message.part.updated","properties":{"part":{"type":"tool-result","text":"file list"}}}`,
		`{"type":"message.updated","properties":{"message":{"role":"assistant","content":"done","usage":{"input_tokens":100,"output_tokens":50,"total_cost":0.01}}}}`,
	}, "\n") + "\n"

	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)
	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("event 0: expected EventText, got %s", events[0].Type)
	}
	if events[1].Type != EventToolUse {
		t.Errorf("event 1: expected EventToolUse, got %s", events[1].Type)
	}
	if events[2].Type != EventToolResult {
		t.Errorf("event 2: expected EventToolResult, got %s", events[2].Type)
	}
	if events[3].Type != EventResult {
		t.Errorf("event 3: expected EventResult, got %s", events[3].Type)
	}
}

func TestOCParseMalformedProperties(t *testing.T) {
	// Valid JSON with type but malformed properties for that type
	input := `{"type":"message.part.updated","properties":"not an object"}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw for malformed properties, got %s", events[0].Type)
	}
}

func TestOCParseMessageUpdatedNoUsage(t *testing.T) {
	input := `{"type":"message.updated","properties":{"message":{"role":"assistant","content":"intermediate"}}}` + "\n"
	parser := NewOpenCodeStreamParser(strings.NewReader(input), 10)

	events := collectOCEvents(t, parser, context.Background())

	// No usage means no event emitted
	if len(events) != 0 {
		t.Fatalf("expected 0 events for message update without usage, got %d", len(events))
	}
}
