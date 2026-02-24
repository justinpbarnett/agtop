package process

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

var (
	cwdOnce sync.Once
	cachedCWD string
)

func getCWD() string {
	cwdOnce.Do(func() {
		if d, err := os.Getwd(); err == nil {
			cachedCWD = d
		}
	})
	return cachedCWD
}

// shortenPath converts an absolute path to a relative path from the process
// working directory. Non-absolute or non-local paths are returned unchanged.
func shortenPath(p string) string {
	if !filepath.IsAbs(p) {
		return p
	}
	cwd := getCWD()
	if cwd == "" {
		return p
	}
	rel, err := filepath.Rel(cwd, p)
	if err != nil || strings.HasPrefix(rel, "..") {
		return p
	}
	return rel
}

// toolField extracts a single string field from a JSON tool input.
// Returns "" if the input is invalid JSON or the field is absent/non-string.
func toolField(toolInput, fieldName string) string {
	var m map[string]interface{}
	if json.Unmarshal([]byte(toolInput), &m) != nil {
		return ""
	}
	if v, ok := m[fieldName].(string); ok {
		return v
	}
	return ""
}

// ToolUseSummary produces a readable one-line tool summary.
// For known tools it extracts meaningful context from the JSON input.
func ToolUseSummary(toolName, toolInput string) string {
	if toolInput == "" {
		return "Tool: " + toolName
	}
	switch toolName {
	case "Read", "Edit", "Write":
		if p := toolField(toolInput, "file_path"); p != "" {
			return "Tool: " + toolName + " — " + shortenPath(p)
		}
	case "Glob", "Grep":
		if p := toolField(toolInput, "pattern"); p != "" {
			return "Tool: " + toolName + " — " + p
		}
	case "Bash":
		if cmd := toolField(toolInput, "command"); cmd != "" {
			return "Tool: Bash — " + truncateLine(firstLine(cmd), 60)
		}
	case "WebSearch":
		if q := toolField(toolInput, "query"); q != "" {
			return "Tool: WebSearch — " + truncateLine(q, 60)
		}
	case "WebFetch":
		if u := toolField(toolInput, "url"); u != "" {
			return "Tool: WebFetch — " + truncateLine(u, 60)
		}
	case "Task":
		if desc := toolField(toolInput, "description"); desc != "" {
			return "Tool: Task — " + truncateLine(desc, 60)
		}
	case "TodoWrite", "TaskCreate":
		if subj := toolField(toolInput, "subject"); subj != "" {
			return "Tool: " + toolName + " — " + truncateLine(subj, 50)
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

// InterpretRawEvent creates a LogEntry with a human-readable summary for
// known JSON event types (system, etc.). For non-JSON text or unknown
// types, falls back to the default raw summary. The detail is always
// the pretty-printed JSON so the user can expand to see the full payload.
func InterpretRawEvent(ts, skill, text string) *LogEntry {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') {
		return NewLogEntry(ts, skill, EventRaw, text)
	}

	var header struct {
		Type    string `json:"type"`
		Subtype string `json:"subtype,omitempty"`
	}
	if json.Unmarshal([]byte(trimmed), &header) != nil || header.Type == "" {
		return NewLogEntry(ts, skill, EventRaw, text)
	}

	summary := interpretJSONSummary(trimmed, header.Type, header.Subtype)
	detail := interpretJSONDetail(trimmed, header.Type, header.Subtype)
	return &LogEntry{
		Timestamp: ts,
		Skill:     skill,
		Type:      EventRaw,
		Summary:   summary,
		Detail:    detail,
		Complete:  true,
	}
}

func interpretJSONSummary(raw, eventType, subtype string) string {
	switch eventType {
	case "system":
		return interpretSystemSummary(raw, subtype)
	default:
		label := eventType
		if subtype != "" {
			label += "/" + subtype
		}
		return "[" + label + "]"
	}
}

func interpretSystemSummary(raw, subtype string) string {
	switch subtype {
	case "init":
		var init struct {
			Model          string   `json:"model"`
			PermissionMode string   `json:"permissionMode"`
			Tools          []string `json:"tools"`
			Version        string   `json:"claude_code_version"`
		}
		if json.Unmarshal([]byte(raw), &init) == nil {
			var parts []string
			if init.Version != "" {
				parts = append(parts, "v"+init.Version)
			}
			if init.Model != "" {
				parts = append(parts, init.Model)
			}
			if init.PermissionMode != "" {
				parts = append(parts, init.PermissionMode)
			}
			if len(init.Tools) > 0 {
				parts = append(parts, fmt.Sprintf("%d tools", len(init.Tools)))
			}
			if len(parts) > 0 {
				return "Session init — " + strings.Join(parts, " · ")
			}
		}
		return "[system/init]"
	default:
		if subtype != "" {
			return "[system/" + subtype + "]"
		}
		return "[system]"
	}
}

// interpretJSONDetail returns a human-readable detail string for known
// JSON event types, falling back to pretty-printed JSON for unknown types.
func interpretJSONDetail(raw, eventType, subtype string) string {
	if eventType == "system" && subtype == "init" {
		if d := formatSystemInitDetail(raw); d != "" {
			return d
		}
	}
	return FormatJSON(raw)
}

// formatSystemInitDetail produces a structured, readable detail view for
// system init events instead of dumping raw JSON.
func formatSystemInitDetail(raw string) string {
	var init struct {
		Model          string   `json:"model"`
		PermissionMode string   `json:"permissionMode"`
		Tools          []string `json:"tools"`
		Version        string   `json:"claude_code_version"`
		SessionID      string   `json:"session_id"`
		CWD            string   `json:"cwd"`
		Agents         []string `json:"agents"`
		Skills         []string `json:"skills"`
		MCPServers     []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"mcp_servers"`
		Plugins []struct {
			Name string `json:"name"`
		} `json:"plugins"`
	}
	if json.Unmarshal([]byte(raw), &init) != nil {
		return ""
	}

	var lines []string

	// Header: version · model · permission mode
	var header []string
	if init.Version != "" {
		header = append(header, "v"+init.Version)
	}
	if init.Model != "" {
		header = append(header, init.Model)
	}
	if init.PermissionMode != "" {
		header = append(header, init.PermissionMode)
	}
	if len(header) > 0 {
		lines = append(lines, strings.Join(header, " · "))
	}

	// Session and working directory
	if init.SessionID != "" {
		lines = append(lines, "Session: "+init.SessionID)
	}
	if init.CWD != "" {
		lines = append(lines, "CWD: "+init.CWD)
	}

	// Lists: tools, agents, skills
	if len(init.Tools) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Tools (%d): %s", len(init.Tools), strings.Join(init.Tools, ", ")))
	}
	if len(init.Agents) > 0 {
		lines = append(lines, fmt.Sprintf("Agents (%d): %s", len(init.Agents), strings.Join(init.Agents, ", ")))
	}
	if len(init.Skills) > 0 {
		lines = append(lines, fmt.Sprintf("Skills (%d): %s", len(init.Skills), strings.Join(init.Skills, ", ")))
	}

	// Plugins
	if len(init.Plugins) > 0 {
		names := make([]string, len(init.Plugins))
		for i, p := range init.Plugins {
			names[i] = p.Name
		}
		lines = append(lines, fmt.Sprintf("Plugins (%d): %s", len(init.Plugins), strings.Join(names, ", ")))
	}

	// MCP servers (highlight status)
	if len(init.MCPServers) > 0 {
		var parts []string
		for _, s := range init.MCPServers {
			parts = append(parts, s.Name+" ("+s.Status+")")
		}
		lines = append(lines, "MCP: "+strings.Join(parts, ", "))
	}

	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
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
