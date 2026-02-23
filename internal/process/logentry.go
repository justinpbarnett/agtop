package process

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// LogEntry represents a single logical log event with a one-line summary
// and optional multi-line detail text for expanded view.
type LogEntry struct {
	Timestamp string
	Skill     string
	Type      StreamEventType
	Summary   string
	Detail    string
	Complete  bool
}

// NewLogEntry creates a LogEntry with an auto-generated summary based on event type.
func NewLogEntry(ts, skill string, eventType StreamEventType, detail string) *LogEntry {
	e := &LogEntry{
		Timestamp: ts,
		Skill:     skill,
		Type:      eventType,
		Detail:    detail,
		Complete:  true,
	}
	switch eventType {
	case EventText:
		e.Summary = truncateLine(firstLine(detail), 80)
	case EventToolUse:
		e.Summary = "Tool: " + detail
	case EventToolResult:
		if len(detail) > 200 {
			e.Summary = fmt.Sprintf("Result: (%d chars)", len(detail))
		} else {
			e.Summary = "Result: " + truncateLine(firstLine(detail), 60)
		}
	case EventResult:
		e.Summary = detail
	case EventError:
		e.Summary = "ERROR: " + truncateLine(firstLine(detail), 60)
	case EventUser:
		e.Summary = "User: " + truncateLine(firstLine(detail), 70)
	default:
		e.Summary = truncateLine(firstLine(detail), 80)
	}
	return e
}

// ToolUseSummary produces a readable one-line tool summary.
// For known tools it extracts meaningful context from the JSON input.
func ToolUseSummary(toolName, toolInput string) string {
	if toolInput == "" {
		return "Tool: " + toolName
	}
	switch toolName {
	case "Read":
		var input struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.FilePath != "" {
			return "Tool: Read — " + input.FilePath
		}
	case "Edit":
		var input struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.FilePath != "" {
			return "Tool: Edit — " + input.FilePath
		}
	case "Write":
		var input struct {
			FilePath string `json:"file_path"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.FilePath != "" {
			return "Tool: Write — " + input.FilePath
		}
	case "Bash":
		var input struct {
			Command string `json:"command"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.Command != "" {
			cmd := truncateLine(firstLine(input.Command), 60)
			return "Tool: Bash — " + cmd
		}
	case "Glob":
		var input struct {
			Pattern string `json:"pattern"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.Pattern != "" {
			return "Tool: Glob — " + input.Pattern
		}
	case "Grep":
		var input struct {
			Pattern string `json:"pattern"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.Pattern != "" {
			return "Tool: Grep — " + input.Pattern
		}
	case "WebSearch":
		var input struct {
			Query string `json:"query"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.Query != "" {
			return "Tool: WebSearch — " + truncateLine(input.Query, 60)
		}
	case "WebFetch":
		var input struct {
			URL string `json:"url"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.URL != "" {
			return "Tool: WebFetch — " + truncateLine(input.URL, 60)
		}
	case "Task":
		var input struct {
			Description string `json:"description"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.Description != "" {
			return "Tool: Task — " + truncateLine(input.Description, 60)
		}
	case "TodoWrite", "TaskCreate":
		var input struct {
			Subject string `json:"subject"`
		}
		if json.Unmarshal([]byte(toolInput), &input) == nil && input.Subject != "" {
			return "Tool: " + toolName + " — " + truncateLine(input.Subject, 50)
		}
	}
	return "Tool: " + toolName
}

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func truncateLine(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// FormatJSON pretty-prints a JSON string with 2-space indentation.
// Returns the input unchanged if it's not valid JSON.
func FormatJSON(s string) string {
	s = strings.TrimSpace(s)
	if s == "" || (s[0] != '{' && s[0] != '[') {
		return s
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s
	}
	return buf.String()
}

// WordWrap wraps text to the given width at word boundaries.
// Preserves existing newlines and only wraps lines that exceed width.
func WordWrap(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	var result []string
	for _, line := range lines {
		if len(line) <= width {
			result = append(result, line)
			continue
		}
		remaining := line
		for len(remaining) > width {
			breakAt := strings.LastIndex(remaining[:width], " ")
			if breakAt <= 0 {
				breakAt = width
			}
			result = append(result, remaining[:breakAt])
			remaining = strings.TrimLeft(remaining[breakAt:], " ")
		}
		if remaining != "" {
			result = append(result, remaining)
		}
	}
	return strings.Join(result, "\n")
}

// EntryBuffer is a thread-safe circular buffer of LogEntry pointers.
type EntryBuffer struct {
	mu       sync.RWMutex
	entries  []*LogEntry
	capacity int
	head     int
	count    int
	evicted  int // total entries evicted due to capacity wrap
}

// NewEntryBuffer creates an EntryBuffer with the given capacity.
func NewEntryBuffer(capacity int) *EntryBuffer {
	if capacity <= 0 {
		capacity = 5000
	}
	return &EntryBuffer{
		entries:  make([]*LogEntry, capacity),
		capacity: capacity,
	}
}

// Append adds a new entry to the buffer.
func (eb *EntryBuffer) Append(entry *LogEntry) {
	eb.mu.Lock()
	eb.entries[eb.head] = entry
	eb.head = (eb.head + 1) % eb.capacity
	if eb.count < eb.capacity {
		eb.count++
	} else {
		eb.evicted++
	}
	eb.mu.Unlock()
}

// UpdateLast calls fn on the most recently appended entry.
func (eb *EntryBuffer) UpdateLast(fn func(*LogEntry)) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if eb.count == 0 {
		return
	}
	idx := (eb.head - 1 + eb.capacity) % eb.capacity
	fn(eb.entries[idx])
}

// Entries returns all entries in chronological order.
func (eb *EntryBuffer) Entries() []*LogEntry {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	if eb.count == 0 {
		return nil
	}
	result := make([]*LogEntry, eb.count)
	if eb.count < eb.capacity {
		copy(result, eb.entries[:eb.count])
	} else {
		n := copy(result, eb.entries[eb.head:])
		copy(result[n:], eb.entries[:eb.head])
	}
	return result
}

// Len returns the number of entries currently in the buffer.
func (eb *EntryBuffer) Len() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.count
}

// TotalEvicted returns the total number of entries evicted due to capacity wrap.
func (eb *EntryBuffer) TotalEvicted() int {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.evicted
}

// Get returns the entry at the given logical index (0 = oldest).
func (eb *EntryBuffer) Get(index int) *LogEntry {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	if index < 0 || index >= eb.count {
		return nil
	}
	var physIdx int
	if eb.count < eb.capacity {
		physIdx = index
	} else {
		physIdx = (eb.head + index) % eb.capacity
	}
	return eb.entries[physIdx]
}
