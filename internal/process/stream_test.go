package process

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"
)

func collectEvents(t *testing.T, parser *StreamParser, ctx context.Context) []StreamEvent {
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

func TestParseTextEvent(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Hello, world!"}]}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText, got %s", events[0].Type)
	}
	if events[0].Text != "Hello, world!" {
		t.Errorf("expected 'Hello, world!', got %q", events[0].Text)
	}
}

func TestParseToolUseEvent(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/tmp/test.go"}}]}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

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

func TestParseResultEvent(t *testing.T) {
	input := `{"type":"result","result":"done","usage":{"input_tokens":1234,"output_tokens":567},"total_cost_usd":0.042}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventResult {
		t.Errorf("expected EventResult, got %s", events[0].Type)
	}
	if events[0].Usage == nil {
		t.Fatal("expected non-nil Usage")
	}
	if events[0].Usage.InputTokens != 1234 {
		t.Errorf("expected 1234 input tokens, got %d", events[0].Usage.InputTokens)
	}
	if events[0].Usage.OutputTokens != 567 {
		t.Errorf("expected 567 output tokens, got %d", events[0].Usage.OutputTokens)
	}
	if events[0].Usage.TotalTokens != 1801 {
		t.Errorf("expected 1801 total tokens, got %d", events[0].Usage.TotalTokens)
	}
	if events[0].Usage.CostUSD != 0.042 {
		t.Errorf("expected 0.042 cost, got %f", events[0].Usage.CostUSD)
	}
}

func TestParseMultipleContentBlocks(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"Let me read that file."},{"type":"tool_use","name":"Read","input":{"file_path":"main.go"}}]}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected first event EventText, got %s", events[0].Type)
	}
	if events[1].Type != EventToolUse {
		t.Errorf("expected second event EventToolUse, got %s", events[1].Type)
	}
}

func TestParseMalformedLine(t *testing.T) {
	input := "this is not json\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw, got %s", events[0].Type)
	}
	if events[0].Text != "this is not json" {
		t.Errorf("expected raw text preserved, got %q", events[0].Text)
	}
}

func TestParseEmptyLine(t *testing.T) {
	input := "\n\n" + `{"type":"assistant","message":{"content":[{"type":"text","text":"hi"}]}}` + "\n\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event (empty lines skipped), got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText, got %s", events[0].Type)
	}
}

func TestParseContextCancellation(t *testing.T) {
	// Create a reader that blocks
	r, w := makeBlockingPipe()
	defer w.Close()

	parser := NewStreamParser(r, 10)
	ctx, cancel := context.WithCancel(context.Background())

	// Write one line then cancel
	w.Write([]byte(`{"type":"assistant","message":{"content":[{"type":"text","text":"first"}]}}` + "\n"))

	go parser.Parse(ctx)

	// Read the first event
	select {
	case event := <-parser.Events():
		if event.Type != EventText {
			t.Errorf("expected EventText, got %s", event.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for first event")
	}

	// Cancel and verify parser stops
	cancel()
	w.Close()

	select {
	case <-parser.Done():
		// Parser finished
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for parser to stop after cancellation")
	}
}

func TestParseLargeLine(t *testing.T) {
	// Generate a large text content near the 1MB limit
	largeText := strings.Repeat("x", 500000)
	input := `{"type":"assistant","message":{"content":[{"type":"text","text":"` + largeText + `"}]}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("expected EventText, got %s", events[0].Type)
	}
	if len(events[0].Text) != 500000 {
		t.Errorf("expected text length 500000, got %d", len(events[0].Text))
	}
}

func TestParseMultipleMessages(t *testing.T) {
	input := strings.Join([]string{
		`{"type":"assistant","message":{"content":[{"type":"text","text":"thinking..."}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls"}}]}}`,
		`{"type":"assistant","message":{"content":[{"type":"text","text":"done"}]}}`,
		`{"type":"result","result":"completed","usage":{"input_tokens":100,"output_tokens":50},"total_cost_usd":0.01}`,
	}, "\n") + "\n"

	parser := NewStreamParser(strings.NewReader(input), 10)
	events := collectEvents(t, parser, context.Background())

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].Type != EventText {
		t.Errorf("event 0: expected EventText, got %s", events[0].Type)
	}
	if events[1].Type != EventToolUse {
		t.Errorf("event 1: expected EventToolUse, got %s", events[1].Type)
	}
	if events[2].Type != EventText {
		t.Errorf("event 2: expected EventText, got %s", events[2].Type)
	}
	if events[3].Type != EventResult {
		t.Errorf("event 3: expected EventResult, got %s", events[3].Type)
	}
}

func TestParseUserEventContentBlocks(t *testing.T) {
	input := `{"type":"user","message":{"content":[{"type":"text","text":"implement the feature"}]}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventUser {
		t.Errorf("expected EventUser, got %s", events[0].Type)
	}
	if events[0].Text != "implement the feature" {
		t.Errorf("expected 'implement the feature', got %q", events[0].Text)
	}
}

func TestParseUserEventStringContent(t *testing.T) {
	input := `{"type":"user","message":{"content":"fix the bug"}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventUser {
		t.Errorf("expected EventUser, got %s", events[0].Type)
	}
	if events[0].Text != "fix the bug" {
		t.Errorf("expected 'fix the bug', got %q", events[0].Text)
	}
}

func TestParseUserEventEmptyContent(t *testing.T) {
	// User message with no extractable content should not emit an event
	input := `{"type":"user","message":{}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 0 {
		t.Fatalf("expected 0 events for empty user message, got %d", len(events))
	}
}

func TestParseRateLimitAllowedDropped(t *testing.T) {
	input := `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed","resetsAt":1771858800,"rateLimitType":"five_hour","overageStatus":"rejected","overageDisabledReason":"org_level_disabled","isUsingOverage":false},"uuid":"abc","session_id":"def"}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 0 {
		t.Fatalf("expected 0 events for allowed rate limit, got %d: %+v", len(events), events)
	}
}

func TestParseRateLimitRejected(t *testing.T) {
	input := `{"type":"rate_limit_event","rate_limit_info":{"status":"rejected","rateLimitType":"five_hour"}}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event for rejected rate limit, got %d", len(events))
	}
	if events[0].Type != EventError {
		t.Errorf("expected EventError, got %s", events[0].Type)
	}
	if !strings.Contains(events[0].Text, "five_hour") {
		t.Errorf("expected rate limit type in message, got %q", events[0].Text)
	}
	if !strings.Contains(events[0].Text, "rejected") {
		t.Errorf("expected status in message, got %q", events[0].Text)
	}
}

func TestParseUnknownType(t *testing.T) {
	input := `{"type":"unknown_event","data":"something"}` + "\n"
	parser := NewStreamParser(strings.NewReader(input), 10)

	events := collectEvents(t, parser, context.Background())

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != EventRaw {
		t.Errorf("expected EventRaw for unknown type, got %s", events[0].Type)
	}
}

// makeBlockingPipe creates an io.Reader/io.WriteCloser pair for testing.
type writeCloser struct {
	ch     chan []byte
	closed chan struct{}
}

func (w *writeCloser) Write(p []byte) (int, error) {
	buf := make([]byte, len(p))
	copy(buf, p)
	w.ch <- buf
	return len(p), nil
}

func (w *writeCloser) Close() error {
	select {
	case <-w.closed:
	default:
		close(w.closed)
	}
	return nil
}

type blockingReader struct {
	w   *writeCloser
	buf []byte
}

func (r *blockingReader) Read(p []byte) (int, error) {
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		r.buf = r.buf[n:]
		return n, nil
	}

	select {
	case data := <-r.w.ch:
		n := copy(p, data)
		if n < len(data) {
			r.buf = data[n:]
		}
		return n, nil
	case <-r.w.closed:
		return 0, io.EOF
	}
}

func makeBlockingPipe() (*blockingReader, *writeCloser) {
	w := &writeCloser{
		ch:     make(chan []byte, 10),
		closed: make(chan struct{}),
	}
	r := &blockingReader{w: w}
	return r, w
}
