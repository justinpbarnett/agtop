package process

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/cost"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
	"github.com/justinpbarnett/agtop/internal/safety"
)

var rehydrateLineRe = regexp.MustCompile(`^\[(\d{2}:\d{2}:\d{2})(?:\s+(\S+))?\]\s*(.*)$`)

type LogLineMsg struct {
	RunID string
}

// CostThresholdMsg is sent when a run breaches a cost or token threshold.
type CostThresholdMsg struct {
	RunID  string
	Reason string
}

// SkillResult captures the outcome of a single skill subprocess execution.
type SkillResult struct {
	ResultText string // Final result text from the stream-json "result" event
	Err        error  // Non-nil if the process exited with error
}

type ManagedProcess struct {
	proc   *runtime.Process
	cancel context.CancelFunc
	runID  string
}

type Manager struct {
	store        *run.Store
	rt           runtime.Runtime
	runtimeName  string
	cfg          *config.LimitsConfig
	tracker      *cost.Tracker
	limiter      *cost.LimitChecker
	safety       *safety.PatternMatcher
	mu           sync.Mutex
	processes    map[string]*ManagedProcess
	buffers      map[string]*RingBuffer
	entryBuffers map[string]*EntryBuffer
	program      *tea.Program
}

func NewManager(store *run.Store, rt runtime.Runtime, runtimeName string, cfg *config.LimitsConfig, tracker *cost.Tracker, limiter *cost.LimitChecker, safetyMatcher *safety.PatternMatcher) *Manager {
	return &Manager{
		store:        store,
		rt:           rt,
		runtimeName:  runtimeName,
		cfg:          cfg,
		tracker:      tracker,
		limiter:      limiter,
		safety:       safetyMatcher,
		processes:    make(map[string]*ManagedProcess),
		buffers:      make(map[string]*RingBuffer),
		entryBuffers: make(map[string]*EntryBuffer),
	}
}

func (m *Manager) newParser(r io.Reader, bufSize int) EventStream {
	if m.runtimeName == "opencode" {
		return NewOpenCodeStreamParser(r, bufSize)
	}
	return NewStreamParser(r, bufSize)
}

func (m *Manager) SetProgram(p *tea.Program) {
	m.mu.Lock()
	m.program = p
	m.mu.Unlock()
}

func (m *Manager) Start(runID string, prompt string, opts runtime.RunOptions) error {
	m.mu.Lock()
	if m.cfg.MaxConcurrentRuns > 0 && len(m.processes) >= m.cfg.MaxConcurrentRuns {
		m.mu.Unlock()
		return fmt.Errorf("concurrency limit reached: %d/%d runs active", len(m.processes), m.cfg.MaxConcurrentRuns)
	}

	if _, exists := m.processes[runID]; exists {
		m.mu.Unlock()
		return fmt.Errorf("run %s already has an active process", runID)
	}
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	proc, err := m.rt.Start(ctx, prompt, opts)
	if err != nil {
		cancel()
		return fmt.Errorf("start process: %w", err)
	}

	buf := NewRingBuffer(10000)
	eb := NewEntryBuffer(5000)

	mp := &ManagedProcess{
		proc:   proc,
		cancel: cancel,
		runID:  runID,
	}

	m.mu.Lock()
	m.processes[runID] = mp
	m.buffers[runID] = buf
	m.entryBuffers[runID] = eb
	m.mu.Unlock()

	m.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRunning
		r.PID = proc.PID
		r.StartedAt = time.Now()
	})

	go m.consumeEvents(runID, mp, buf, eb)

	return nil
}

func (m *Manager) Stop(runID string) error {
	m.mu.Lock()
	mp, ok := m.processes[runID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active process for run %s", runID)
	}

	if err := m.rt.Stop(mp.proc); err != nil {
		return err
	}
	mp.cancel()
	return nil
}

func (m *Manager) Pause(runID string) error {
	m.mu.Lock()
	mp, ok := m.processes[runID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active process for run %s", runID)
	}

	if err := m.rt.Pause(mp.proc); err != nil {
		return err
	}
	m.store.Update(runID, func(r *run.Run) {
		r.State = run.StatePaused
	})
	return nil
}

func (m *Manager) Resume(runID string) error {
	m.mu.Lock()
	mp, ok := m.processes[runID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active process for run %s", runID)
	}

	if err := m.rt.Resume(mp.proc); err != nil {
		return err
	}
	m.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRunning
	})
	return nil
}

func (m *Manager) Kill(runID string) error {
	m.mu.Lock()
	mp, ok := m.processes[runID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active process for run %s", runID)
	}

	if mp.proc.Cmd != nil && mp.proc.Cmd.Process != nil {
		_ = mp.proc.Cmd.Process.Signal(syscall.SIGKILL)
	}
	mp.cancel()
	return nil
}

func (m *Manager) Buffer(runID string) *RingBuffer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.buffers[runID]
}

func (m *Manager) EntryBuffer(runID string) *EntryBuffer {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.entryBuffers[runID]
}

// InjectBuffer creates a ring buffer pre-populated with log lines.
// Used to restore log history for rehydrated runs.
func (m *Manager) InjectBuffer(runID string, lines []string) {
	buf := NewRingBuffer(10000)
	eb := NewEntryBuffer(5000)
	for _, line := range lines {
		buf.Append(line)
		if entry := lineToEntry(line); entry != nil {
			eb.Append(entry)
		}
	}
	m.mu.Lock()
	m.buffers[runID] = buf
	m.entryBuffers[runID] = eb
	m.mu.Unlock()
}

func (m *Manager) RemoveBuffer(runID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.buffers, runID)
	delete(m.entryBuffers, runID)
}

func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.processes)
}

// Tracker returns the cost tracker for external queries (e.g., UI).
func (m *Manager) Tracker() *cost.Tracker {
	return m.tracker
}

// StartSkill launches a skill subprocess and returns a channel that receives
// the result when the process exits. Unlike Start(), it does NOT set the run's
// terminal state (Completed/Failed) on exit — the caller (executor) manages state.
// It still logs to the ring buffer and tracks tokens/cost.
func (m *Manager) StartSkill(runID string, prompt string, opts runtime.RunOptions) (<-chan SkillResult, error) {
	m.mu.Lock()
	if _, exists := m.processes[runID]; exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("run %s already has an active process", runID)
	}
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())
	proc, err := m.rt.Start(ctx, prompt, opts)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("start process: %w", err)
	}

	buf := m.buffers[runID]
	if buf == nil {
		buf = NewRingBuffer(10000)
	}
	eb := m.entryBuffers[runID]
	if eb == nil {
		eb = NewEntryBuffer(5000)
	}

	mp := &ManagedProcess{
		proc:   proc,
		cancel: cancel,
		runID:  runID,
	}

	resultCh := make(chan SkillResult, 1)

	m.mu.Lock()
	m.processes[runID] = mp
	m.buffers[runID] = buf
	m.entryBuffers[runID] = eb
	m.mu.Unlock()

	m.store.Update(runID, func(r *run.Run) {
		r.PID = proc.PID
	})

	go m.consumeSkillEvents(runID, mp, buf, eb, resultCh)

	return resultCh, nil
}

func (m *Manager) consumeSkillEvents(runID string, mp *ManagedProcess, buf *RingBuffer, eb *EntryBuffer, resultCh chan<- SkillResult) {
	defer close(resultCh)

	var resultText string

	skillName := func() string {
		r, ok := m.store.Get(runID)
		if !ok {
			return ""
		}
		return r.CurrentSkill
	}

	parser := m.newParser(mp.proc.Stdout, 256)
	go parser.Parse(context.Background())

	go func() {
		scanner := bufio.NewScanner(mp.proc.Stderr)
		for scanner.Scan() {
			line := scanner.Text()
			ts := time.Now().Format("15:04:05")
			skill := skillName()
			var formatted string
			if skill != "" {
				formatted = fmt.Sprintf("[%s %s] %s", ts, skill, line)
			} else {
				formatted = fmt.Sprintf("[%s] %s", ts, line)
			}
			buf.Append(formatted)
			eb.Append(NewLogEntry(ts, skill, EventRaw, line))
			m.sendLogLine(runID)
		}
	}()

	for event := range parser.Events() {
		ts := time.Now().Format("15:04:05")
		skill := skillName()

		var logLine string
		var entry *LogEntry
		switch event.Type {
		case EventText:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventText, event.Text)
		case EventToolUse:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] Tool: %s", ts, skill, event.ToolName)
			} else {
				logLine = fmt.Sprintf("[%s] Tool: %s", ts, event.ToolName)
			}
			m.checkToolSafety(event.ToolName, event.ToolInput, ts, skill, buf, runID)
			summary := ToolUseSummary(event.ToolName, event.ToolInput)
			entry = &LogEntry{
				Timestamp: ts,
				Skill:     skill,
				Type:      EventToolUse,
				Summary:   summary,
				Detail:    FormatJSON(event.ToolInput),
				Complete:  true,
			}
		case EventToolResult:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] Result: %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] Result: %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventToolResult, event.Text)
		case EventResult:
			resultText = event.Text

			if event.Usage != nil {
				m.recordUsage(runID, skill, event.Usage, ts, buf)

				if skill != "" {
					logLine = fmt.Sprintf("[%s %s] Completed — %d tokens, $%.4f", ts, skill, event.Usage.TotalTokens, event.Usage.CostUSD)
				} else {
					logLine = fmt.Sprintf("[%s] Completed — %d tokens, $%.4f", ts, event.Usage.TotalTokens, event.Usage.CostUSD)
				}
				entry = NewLogEntry(ts, skill, EventResult, fmt.Sprintf("Completed — %d tokens, $%.4f", event.Usage.TotalTokens, event.Usage.CostUSD))
			}
		case EventUser:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] User: %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] User: %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventUser, event.Text)
		case EventError:
			if m.limiter != nil && m.limiter.IsRateLimit(event.Text) {
				if skill != "" {
					logLine = fmt.Sprintf("[%s %s] RATE LIMITED: %s", ts, skill, event.Text)
				} else {
					logLine = fmt.Sprintf("[%s] RATE LIMITED: %s", ts, event.Text)
				}
				// entry stays nil — hidden from structured entry view
			} else {
				if skill != "" {
					logLine = fmt.Sprintf("[%s %s] ERROR: %s", ts, skill, event.Text)
				} else {
					logLine = fmt.Sprintf("[%s] ERROR: %s", ts, event.Text)
				}
				entry = NewLogEntry(ts, skill, EventError, event.Text)
			}
		case EventRaw:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventRaw, event.Text)
		}

		if logLine != "" {
			buf.Append(logLine)
			if entry != nil {
				eb.Append(entry)
			}
			m.sendLogLine(runID)
		}
	}

	var exitErr error
	select {
	case exitErr = <-mp.proc.Done:
	default:
		exitErr = <-mp.proc.Done
	}

	m.store.Update(runID, func(r *run.Run) {
		r.PID = 0
	})

	m.mu.Lock()
	delete(m.processes, runID)
	m.mu.Unlock()

	resultCh <- SkillResult{ResultText: resultText, Err: exitErr}
}

func (m *Manager) consumeEvents(runID string, mp *ManagedProcess, buf *RingBuffer, eb *EntryBuffer) {
	// Get current skill name for log prefix
	skillName := func() string {
		r, ok := m.store.Get(runID)
		if !ok {
			return ""
		}
		return r.CurrentSkill
	}

	// Create stream parser on stdout
	parser := m.newParser(mp.proc.Stdout, 256)
	go parser.Parse(context.Background())

	// Stream stderr as raw events
	go func() {
		scanner := bufio.NewScanner(mp.proc.Stderr)
		for scanner.Scan() {
			line := scanner.Text()
			ts := time.Now().Format("15:04:05")
			skill := skillName()
			var formatted string
			if skill != "" {
				formatted = fmt.Sprintf("[%s %s] %s", ts, skill, line)
			} else {
				formatted = fmt.Sprintf("[%s] %s", ts, line)
			}
			buf.Append(formatted)
			eb.Append(NewLogEntry(ts, skill, EventRaw, line))
			m.sendLogLine(runID)
		}
	}()

	for event := range parser.Events() {
		ts := time.Now().Format("15:04:05")
		skill := skillName()

		var logLine string
		var entry *LogEntry
		switch event.Type {
		case EventText:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventText, event.Text)
		case EventToolUse:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] Tool: %s", ts, skill, event.ToolName)
			} else {
				logLine = fmt.Sprintf("[%s] Tool: %s", ts, event.ToolName)
			}
			m.checkToolSafety(event.ToolName, event.ToolInput, ts, skill, buf, runID)
			summary := ToolUseSummary(event.ToolName, event.ToolInput)
			entry = &LogEntry{
				Timestamp: ts,
				Skill:     skill,
				Type:      EventToolUse,
				Summary:   summary,
				Detail:    FormatJSON(event.ToolInput),
				Complete:  true,
			}
		case EventToolResult:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] Result: %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] Result: %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventToolResult, event.Text)
		case EventResult:
			if event.Usage != nil {
				m.recordUsage(runID, skill, event.Usage, ts, buf)

				if skill != "" {
					logLine = fmt.Sprintf("[%s %s] Completed — %d tokens, $%.4f", ts, skill, event.Usage.TotalTokens, event.Usage.CostUSD)
				} else {
					logLine = fmt.Sprintf("[%s] Completed — %d tokens, $%.4f", ts, event.Usage.TotalTokens, event.Usage.CostUSD)
				}
				entry = NewLogEntry(ts, skill, EventResult, fmt.Sprintf("Completed — %d tokens, $%.4f", event.Usage.TotalTokens, event.Usage.CostUSD))
			}
		case EventUser:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] User: %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] User: %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventUser, event.Text)
		case EventError:
			if m.limiter != nil && m.limiter.IsRateLimit(event.Text) {
				if skill != "" {
					logLine = fmt.Sprintf("[%s %s] RATE LIMITED: %s", ts, skill, event.Text)
				} else {
					logLine = fmt.Sprintf("[%s] RATE LIMITED: %s", ts, event.Text)
				}
				// entry stays nil — hidden from structured entry view
			} else {
				if skill != "" {
					logLine = fmt.Sprintf("[%s %s] ERROR: %s", ts, skill, event.Text)
				} else {
					logLine = fmt.Sprintf("[%s] ERROR: %s", ts, event.Text)
				}
				entry = NewLogEntry(ts, skill, EventError, event.Text)
			}
		case EventRaw:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] %s", ts, event.Text)
			}
			entry = NewLogEntry(ts, skill, EventRaw, event.Text)
		}

		if logLine != "" {
			buf.Append(logLine)
			if entry != nil {
				eb.Append(entry)
			}
			m.sendLogLine(runID)
		}
	}

	// Wait for process to exit
	var exitErr error
	select {
	case exitErr = <-mp.proc.Done:
	default:
		// If parser finished but process hasn't, wait for it
		exitErr = <-mp.proc.Done
	}

	// Update run state based on exit
	m.store.Update(runID, func(r *run.Run) {
		r.PID = 0
		r.CompletedAt = time.Now()
		if exitErr == nil {
			r.State = run.StateCompleted
		} else {
			r.State = run.StateFailed
			r.Error = exitErr.Error()
		}
	})

	// Remove from active processes but keep buffer
	m.mu.Lock()
	delete(m.processes, runID)
	m.mu.Unlock()
}

// recordUsage updates run token/cost fields, records to the tracker, and checks thresholds.
func (m *Manager) recordUsage(runID string, skill string, usage *UsageData, ts string, buf *RingBuffer) {
	m.store.Update(runID, func(r *run.Run) {
		r.TokensIn += usage.InputTokens
		r.TokensOut += usage.OutputTokens
		r.Tokens += usage.TotalTokens
		r.Cost += usage.CostUSD
		r.SkillCosts = append(r.SkillCosts, cost.SkillCost{
			SkillName:    skill,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			CostUSD:      usage.CostUSD,
			CompletedAt:  time.Now(),
		})
	})

	if m.tracker != nil {
		m.tracker.Record(runID, cost.SkillCost{
			SkillName:    skill,
			InputTokens:  usage.InputTokens,
			OutputTokens: usage.OutputTokens,
			TotalTokens:  usage.TotalTokens,
			CostUSD:      usage.CostUSD,
			CompletedAt:  time.Now(),
		})
	}

	if m.limiter != nil {
		r, ok := m.store.Get(runID)
		if ok {
			if exceeded, reason := m.limiter.CheckRun(r.Tokens, r.Cost); exceeded {
				warning := fmt.Sprintf("[%s] WARNING: %s, pausing run", ts, reason)
				buf.Append(warning)
				m.sendLogLine(runID)
				_ = m.Pause(runID)
				m.sendCostThreshold(runID, reason)
			}
		}
	}
}

// lineToEntry converts a formatted log line back into a LogEntry.
// Used when rehydrating persisted sessions. Detects event type from
// known prefixes (Tool:, Result:, ERROR:, Completed) so rehydrated
// entries preserve their original type and summary style.
func lineToEntry(line string) *LogEntry {
	parts := rehydrateLineRe.FindStringSubmatch(line)
	if parts == nil {
		return NewLogEntry("", "", EventRaw, line)
	}
	ts := parts[1]
	skill := parts[2]
	msg := parts[3]

	switch {
	case strings.HasPrefix(msg, "Tool: "):
		return &LogEntry{
			Timestamp: ts,
			Skill:     skill,
			Type:      EventToolUse,
			Summary:   msg,
			Detail:    "",
			Complete:  true,
		}
	case strings.HasPrefix(msg, "Result: "):
		detail := strings.TrimPrefix(msg, "Result: ")
		return NewLogEntry(ts, skill, EventToolResult, detail)
	case strings.HasPrefix(msg, "ERROR: "):
		detail := strings.TrimPrefix(msg, "ERROR: ")
		return NewLogEntry(ts, skill, EventError, detail)
	case strings.HasPrefix(msg, "RATE LIMITED: "):
		// Rate limit entries are hidden from structured view during rehydration
		return nil
	case strings.HasPrefix(msg, "User: "):
		detail := strings.TrimPrefix(msg, "User: ")
		return NewLogEntry(ts, skill, EventUser, detail)
	case strings.HasPrefix(msg, "Completed"):
		return NewLogEntry(ts, skill, EventResult, msg)
	default:
		return NewLogEntry(ts, skill, EventText, msg)
	}
}

func (m *Manager) sendLogLine(runID string) {
	m.mu.Lock()
	p := m.program
	m.mu.Unlock()
	if p != nil {
		p.Send(LogLineMsg{RunID: runID})
	}
}

func (m *Manager) sendCostThreshold(runID string, reason string) {
	m.mu.Lock()
	p := m.program
	m.mu.Unlock()
	if p != nil {
		p.Send(CostThresholdMsg{RunID: runID, Reason: reason})
	}
}

// checkToolSafety checks a Bash tool invocation against safety patterns
// and returns a warning log line if blocked. The actual blocking is handled
// by the Claude Code PreToolUse hook — this is informational only.
func (m *Manager) checkToolSafety(toolName string, toolInput string, ts string, skill string, buf *RingBuffer, runID string) {
	if m.safety == nil || toolName != "Bash" || toolInput == "" {
		return
	}
	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(toolInput), &input); err != nil || input.Command == "" {
		return
	}
	if blocked, pattern := m.safety.Check(input.Command); blocked {
		var warning string
		if skill != "" {
			warning = fmt.Sprintf("[%s %s] WARNING: safety pattern matched: %s", ts, skill, pattern)
		} else {
			warning = fmt.Sprintf("[%s] WARNING: safety pattern matched: %s", ts, pattern)
		}
		buf.Append(warning)
		m.sendLogLine(runID)
	}
}
