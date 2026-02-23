package process

import (
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

// EntryBuffer is a thread-safe circular buffer of LogEntry pointers.
type EntryBuffer struct {
	mu       sync.RWMutex
	entries  []*LogEntry
	capacity int
	head     int
	count    int
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
