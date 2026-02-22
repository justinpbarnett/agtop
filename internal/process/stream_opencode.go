package process

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// OpenCode JSON event structures.
// OpenCode's --format json outputs line-delimited JSON with a "type" field
// and a "properties" object. The schema is not fully documented, so the
// parser is defensive — unknown event types become EventRaw.

type ocEvent struct {
	Type       string          `json:"type"`
	Properties json.RawMessage `json:"properties,omitempty"`
}

type ocPartUpdated struct {
	Part ocPart `json:"part"`
}

type ocPart struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ToolName  string          `json:"toolName,omitempty"`
	ToolInput json.RawMessage `json:"input,omitempty"`
	Content   string          `json:"content,omitempty"`
}

type ocMessageUpdated struct {
	Message ocMessage `json:"message"`
}

type ocMessage struct {
	Role    string    `json:"role"`
	Content string    `json:"content,omitempty"`
	Parts   []ocPart  `json:"parts,omitempty"`
	Usage   *ocUsage  `json:"usage,omitempty"`
}

type ocUsage struct {
	InputTokens  int     `json:"input_tokens"`
	OutputTokens int     `json:"output_tokens"`
	TotalCost    float64 `json:"total_cost,omitempty"`
}

type ocSessionUpdated struct {
	Session ocSession `json:"session"`
}

type ocSession struct {
	Status string   `json:"status"`
	Usage  *ocUsage `json:"usage,omitempty"`
}

type ocError struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
}

// OpenCodeStreamParser translates OpenCode's JSON events into StreamEvent values.
type OpenCodeStreamParser struct {
	reader io.Reader
	events chan StreamEvent
	done   chan error
}

func NewOpenCodeStreamParser(r io.Reader, bufSize int) *OpenCodeStreamParser {
	if bufSize <= 0 {
		bufSize = 256
	}
	return &OpenCodeStreamParser{
		reader: r,
		events: make(chan StreamEvent, bufSize),
		done:   make(chan error, 1),
	}
}

func (p *OpenCodeStreamParser) Parse(ctx context.Context) {
	defer close(p.events)

	scanner := bufio.NewScanner(p.reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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

		var ev ocEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
			continue
		}

		switch ev.Type {
		case "message.part.updated":
			var part ocPartUpdated
			if err := json.Unmarshal(ev.Properties, &part); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			switch part.Part.Type {
			case "text":
				text := part.Part.Text
				if text == "" {
					text = part.Part.Content
				}
				p.send(ctx, StreamEvent{Type: EventText, Text: text})
			case "tool-invocation", "tool_use":
				p.send(ctx, StreamEvent{
					Type:      EventToolUse,
					ToolName:  part.Part.ToolName,
					ToolInput: string(part.Part.ToolInput),
				})
			case "tool-result", "tool_result":
				text := part.Part.Text
				if text == "" {
					text = part.Part.Content
				}
				p.send(ctx, StreamEvent{Type: EventToolResult, Text: text})
			case "reasoning", "thinking":
				// Treat reasoning/thinking as text
				text := part.Part.Text
				if text == "" {
					text = part.Part.Content
				}
				p.send(ctx, StreamEvent{Type: EventText, Text: text})
			default:
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
			}

		case "message.updated":
			var msg ocMessageUpdated
			if err := json.Unmarshal(ev.Properties, &msg); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			if msg.Message.Usage != nil {
				u := msg.Message.Usage
				total := u.InputTokens + u.OutputTokens
				p.send(ctx, StreamEvent{
					Type: EventResult,
					Text: msg.Message.Content,
					Usage: &UsageData{
						InputTokens:  u.InputTokens,
						OutputTokens: u.OutputTokens,
						TotalTokens:  total,
						CostUSD:      u.TotalCost,
					},
				})
			}

		case "session.updated":
			var sess ocSessionUpdated
			if err := json.Unmarshal(ev.Properties, &sess); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			if sess.Session.Usage != nil {
				u := sess.Session.Usage
				total := u.InputTokens + u.OutputTokens
				p.send(ctx, StreamEvent{
					Type: EventResult,
					Usage: &UsageData{
						InputTokens:  u.InputTokens,
						OutputTokens: u.OutputTokens,
						TotalTokens:  total,
						CostUSD:      u.TotalCost,
					},
				})
			}

		case "error":
			var errEv ocError
			if err := json.Unmarshal(ev.Properties, &errEv); err != nil {
				p.send(ctx, StreamEvent{Type: EventError, Text: line})
				continue
			}
			errText := errEv.Error
			if errText == "" {
				errText = errEv.Message
			}
			p.send(ctx, StreamEvent{Type: EventError, Text: errText})

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

func (p *OpenCodeStreamParser) send(ctx context.Context, event StreamEvent) {
	select {
	case <-ctx.Done():
	case p.events <- event:
	}
}

func (p *OpenCodeStreamParser) Events() <-chan StreamEvent {
	return p.events
}

func (p *OpenCodeStreamParser) Done() <-chan error {
	return p.done
}
