package process

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

type StreamEventType string

const (
	EventText       StreamEventType = "text"
	EventToolUse    StreamEventType = "tool_use"
	EventToolResult StreamEventType = "tool_result"
	EventResult     StreamEventType = "result"
	EventError      StreamEventType = "error"
	EventRaw        StreamEventType = "raw"
)

type StreamEvent struct {
	Type      StreamEventType
	Text      string
	ToolName  string
	ToolInput string
	Usage     *UsageData
}

type UsageData struct {
	InputTokens  int
	OutputTokens int
	TotalTokens  int
	CostUSD      float64
}

// JSON structures for Claude Code stream-json format.

type streamMessage struct {
	Type    string         `json:"type"`
	Message *streamContent `json:"message,omitempty"`
	Result  string         `json:"result,omitempty"`
	Usage   *streamUsage   `json:"usage,omitempty"`
	CostUSD float64        `json:"total_cost_usd,omitempty"`
}

type streamContent struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type streamUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// EventStream is the interface consumed by the process manager.
type EventStream interface {
	Parse(ctx context.Context)
	Events() <-chan StreamEvent
	Done() <-chan error
}

type StreamParser struct {
	reader io.Reader
	events chan StreamEvent
	done   chan error
}

func NewStreamParser(r io.Reader, bufSize int) *StreamParser {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &StreamParser{
		reader: r,
		events: make(chan StreamEvent, bufSize),
		done:   make(chan error, 1),
	}
}

func (p *StreamParser) Parse(ctx context.Context) {
	defer close(p.events)

	scanner := bufio.NewScanner(p.reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // 1MB max line

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			p.done <- ctx.Err()
			return
		default:
		}

		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg streamMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
			continue
		}

		switch msg.Type {
		case "assistant":
			if msg.Message != nil {
				for _, block := range msg.Message.Content {
					switch block.Type {
					case "text":
						p.send(ctx, StreamEvent{Type: EventText, Text: block.Text})
					case "tool_use":
						p.send(ctx, StreamEvent{
							Type:      EventToolUse,
							ToolName:  block.Name,
							ToolInput: string(block.Input),
						})
					case "tool_result":
						p.send(ctx, StreamEvent{Type: EventToolResult, Text: block.Text})
					}
				}
			}
		case "result":
			event := StreamEvent{Type: EventResult, Text: msg.Result}
			if msg.Usage != nil {
				total := msg.Usage.InputTokens + msg.Usage.OutputTokens
				event.Usage = &UsageData{
					InputTokens:  msg.Usage.InputTokens,
					OutputTokens: msg.Usage.OutputTokens,
					TotalTokens:  total,
					CostUSD:      msg.CostUSD,
				}
			}
			p.send(ctx, event)
		default:
			p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
		}
	}

	if err := scanner.Err(); err != nil {
		p.done <- err
	} else {
		p.done <- nil
	}
}

func (p *StreamParser) send(ctx context.Context, event StreamEvent) {
	select {
	case <-ctx.Done():
	case p.events <- event:
	}
}

func (p *StreamParser) Events() <-chan StreamEvent {
	return p.events
}

func (p *StreamParser) Done() <-chan error {
	return p.done
}
