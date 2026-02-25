package process

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
)

// OpenCode parser handles two event formats that appear in the same stream:
//
// 1. Claude Code stream-json format (from the outer Claude Code process):
//    {"type":"assistant","message":{"content":[...],"usage":{...}}}
//    {"type":"user","message":{"content":[...]}}
//    {"type":"result","subtype":"success","total_cost_usd":0.01,...}
//    {"type":"rate_limit_event","rate_limit_info":{...}}
//    {"type":"system","subtype":"init",...}
//
// 2. OpenCode v1.2.14 flat format (from the OpenCode runtime):
//    {"type":"text","timestamp":...,"part":{"type":"text","text":"..."}}
//    {"type":"tool_use","timestamp":...,"part":{"type":"tool","tool":"Read","state":{...}}}
//    {"type":"step_finish","timestamp":...,"part":{"cost":0.002,"tokens":{...}}}

// --- Shared envelope ---

type ocEvent struct {
	Type      string          `json:"type"`
	Subtype   string          `json:"subtype,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
	SessionID string          `json:"sessionID,omitempty"`
	Part      json.RawMessage `json:"part,omitempty"`
	Error     json.RawMessage `json:"error,omitempty"`
	// Claude Code fields
	Message  json.RawMessage `json:"message,omitempty"`
	Result   string          `json:"result,omitempty"`
	CostUSD  float64         `json:"total_cost_usd,omitempty"`
	Usage    json.RawMessage `json:"usage,omitempty"`
	IsError  bool            `json:"is_error,omitempty"`
}

// --- OpenCode part types ---

type ocPart struct {
	Type   string       `json:"type"`
	Text   string       `json:"text,omitempty"`
	Tool   string       `json:"tool,omitempty"`
	State  *ocToolState `json:"state,omitempty"`
	Cost   float64      `json:"cost,omitempty"`
	Tokens *ocTokens    `json:"tokens,omitempty"`
	Reason string       `json:"reason,omitempty"`
}

type ocToolState struct {
	Status string          `json:"status"`
	Input  json.RawMessage `json:"input,omitempty"`
	Output string          `json:"output,omitempty"`
	Error  string          `json:"error,omitempty"`
	Title  string          `json:"title,omitempty"`
}

type ocTokens struct {
	Total     int      `json:"total"`
	Input     int      `json:"input"`
	Output    int      `json:"output"`
	Reasoning int      `json:"reasoning,omitempty"`
	Cache     *ocCache `json:"cache,omitempty"`
}

type ocCache struct {
	Read  int `json:"read"`
	Write int `json:"write"`
}

type ocError struct {
	Name string `json:"name,omitempty"`
	Data struct {
		Message    string `json:"message,omitempty"`
		StatusCode int    `json:"statusCode,omitempty"`
	} `json:"data,omitempty"`
}

// --- Claude Code message types ---

type ccContentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

type ccMessage struct {
	Content []ccContentBlock `json:"content"`
}

type ccUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type ccRateLimitInfo struct {
	Status        string  `json:"status"`
	RateLimitType string  `json:"rateLimitType"`
	Utilization   float64 `json:"utilization"`
}

// OpenCodeStreamParser translates events from both Claude Code and OpenCode
// formats into StreamEvent values.
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
		// --- OpenCode format events ---

		case "text":
			var part ocPart
			if err := json.Unmarshal(ev.Part, &part); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			p.send(ctx, StreamEvent{Type: EventText, Text: part.Text})

		case "reasoning":
			var part ocPart
			if err := json.Unmarshal(ev.Part, &part); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			p.send(ctx, StreamEvent{Type: EventText, Text: part.Text})

		case "tool_use":
			var part ocPart
			if err := json.Unmarshal(ev.Part, &part); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			toolInput := ""
			if part.State != nil && len(part.State.Input) > 0 {
				toolInput = string(part.State.Input)
			}
			p.send(ctx, StreamEvent{
				Type:      EventToolUse,
				ToolName:  part.Tool,
				ToolInput: toolInput,
			})
			if part.State != nil && part.State.Output != "" {
				p.send(ctx, StreamEvent{
					Type: EventToolResult,
					Text: part.State.Output,
				})
			}

		case "step_start":
			// Lifecycle marker — skip silently

		case "step_finish":
			var part ocPart
			if err := json.Unmarshal(ev.Part, &part); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			var usage *UsageData
			if part.Tokens != nil {
				total := part.Tokens.Total
				if total == 0 {
					total = part.Tokens.Input + part.Tokens.Output + part.Tokens.Reasoning
				}
				usage = &UsageData{
					InputTokens:  part.Tokens.Input,
					OutputTokens: part.Tokens.Output,
					TotalTokens:  total,
					CostUSD:      part.Cost,
				}
			}
			p.send(ctx, StreamEvent{
				Type:  EventResult,
				Usage: usage,
			})

		case "error":
			var errEv ocError
			if err := json.Unmarshal(ev.Error, &errEv); err != nil {
				p.send(ctx, StreamEvent{Type: EventError, Text: line})
				continue
			}
			errText := errEv.Data.Message
			if errText == "" {
				errText = errEv.Name
			}
			p.send(ctx, StreamEvent{Type: EventError, Text: errText})

		// --- Claude Code format events ---

		case "assistant":
			var msg ccMessage
			if err := json.Unmarshal(ev.Message, &msg); err != nil {
				p.send(ctx, StreamEvent{Type: EventRaw, Text: line})
				continue
			}
			for _, block := range msg.Content {
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

		case "user":
			text := p.extractUserContent(line, ev)
			if text != "" {
				p.send(ctx, StreamEvent{Type: EventUser, Text: text})
			}

		case "result":
			event := StreamEvent{Type: EventResult, Text: ev.Result}
			if len(ev.Usage) > 0 {
				var usage ccUsage
				if json.Unmarshal(ev.Usage, &usage) == nil {
					total := usage.InputTokens + usage.OutputTokens
					event.Usage = &UsageData{
						InputTokens:  usage.InputTokens,
						OutputTokens: usage.OutputTokens,
						TotalTokens:  total,
						CostUSD:      ev.CostUSD,
					}
				}
			}
			p.send(ctx, event)

		case "rate_limit_event":
			var rl struct {
				Info ccRateLimitInfo `json:"rate_limit_info"`
			}
			if json.Unmarshal([]byte(line), &rl) == nil {
				if rl.Info.Status == "allowed" {
					continue
				}
				p.send(ctx, StreamEvent{
					Type: EventError,
					Text: "Rate limited (" + rl.Info.RateLimitType + "): " + rl.Info.Status,
				})
			}

		case "system":
			p.send(ctx, StreamEvent{Type: EventRaw, Text: line})

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

func (p *OpenCodeStreamParser) extractUserContent(rawLine string, ev ocEvent) string {
	if len(ev.Message) > 0 {
		var msg ccMessage
		if json.Unmarshal(ev.Message, &msg) == nil {
			for _, block := range msg.Content {
				if block.Type == "text" && block.Text != "" {
					return block.Text
				}
			}
		}
		// Content might be a plain string
		var raw struct {
			Content string `json:"content"`
		}
		if json.Unmarshal(ev.Message, &raw) == nil && raw.Content != "" {
			return raw.Content
		}
	}
	return ""
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
