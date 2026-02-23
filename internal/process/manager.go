package process

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
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

// ErrDisconnected is returned when a process event loop exits because the TUI
// is shutting down (DisconnectAll was called). The executor uses this to
// distinguish a graceful disconnect from a real process failure.
var ErrDisconnected = fmt.Errorf("disconnected: TUI shutting down")

// SkillResult captures the outcome of a single skill subprocess execution.
type SkillResult struct {
	ResultText string // Final result text from the stream-json "result" event
	Err        error  // Non-nil if the process exited with error
}

type ManagedProcess struct {
	proc   *runtime.Process // nil for reconnected processes
	cancel context.CancelFunc
	runID  string
	pid    int // always set — used for signal-based control of reconnected processes
}

type Manager struct {
	store         *run.Store
	rt            runtime.Runtime
	runtimeName   string
	sessionsDir   string
	cfg           *config.LimitsConfig
	tracker       *cost.Tracker
	limiter       *cost.LimitChecker
	safety        *safety.PatternMatcher
	mu            sync.Mutex
	disconnecting bool
	processes     map[string]*ManagedProcess
	buffers       map[string]*RingBuffer
	entryBuffers  map[string]*EntryBuffer
	logFiles      map[string]*LogFiles
	program       *tea.Program
}

func NewManager(store *run.Store, rt runtime.Runtime, runtimeName string, sessionsDir string, cfg *config.LimitsConfig, tracker *cost.Tracker, limiter *cost.LimitChecker, safetyMatcher *safety.PatternMatcher) *Manager {
	return &Manager{
		store:        store,
		rt:           rt,
		runtimeName:  runtimeName,
		sessionsDir:  sessionsDir,
		cfg:          cfg,
		tracker:      tracker,
		limiter:      limiter,
		safety:       safetyMatcher,
		processes:    make(map[string]*ManagedProcess),
		buffers:      make(map[string]*RingBuffer),
		entryBuffers: make(map[string]*EntryBuffer),
		logFiles:     make(map[string]*LogFiles),
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

	// Create log files for persistent output
	var lf *LogFiles
	if m.sessionsDir != "" {
		var err error
		lf, err = CreateLogFiles(m.sessionsDir, runID)
		if err != nil {
			log.Printf("warning: create log files for %s: %v (falling back to pipes)", runID, err)
		} else {
			opts.StdoutFile = lf.StdoutWriter()
			opts.StderrFile = lf.StderrWriter()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	proc, err := m.rt.Start(ctx, prompt, opts)
	if err != nil {
		cancel()
		if lf != nil {
			lf.Close()
		}
		return fmt.Errorf("start process: %w", err)
	}

	// Determine stdout/stderr readers: use FollowReaders on log files, or pipes
	var stdoutReader io.Reader
	var stderrReader io.Reader
	if lf != nil {
		stdoutR, err := os.Open(lf.StdoutPath())
		if err != nil {
			cancel()
			lf.Close()
			return fmt.Errorf("open stdout log for reading: %w", err)
		}
		stderrR, err := os.Open(lf.StderrPath())
		if err != nil {
			stdoutR.Close()
			cancel()
			lf.Close()
			return fmt.Errorf("open stderr log for reading: %w", err)
		}
		stdoutReader = NewFollowReader(ctx, stdoutR)
		stderrReader = NewFollowReader(ctx, stderrR)
	} else {
		stdoutReader = proc.Stdout
		stderrReader = proc.Stderr
	}

	buf := NewRingBuffer(10000)
	eb := NewEntryBuffer(5000)

	mp := &ManagedProcess{
		proc:   proc,
		cancel: cancel,
		runID:  runID,
		pid:    proc.PID,
	}

	m.mu.Lock()
	m.processes[runID] = mp
	m.buffers[runID] = buf
	m.entryBuffers[runID] = eb
	if lf != nil {
		m.logFiles[runID] = lf
	}
	m.mu.Unlock()

	m.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRunning
		r.PID = proc.PID
		r.StartedAt = time.Now()
	})

	go m.consumeEvents(runID, mp, buf, eb, stdoutReader, stderrReader, proc.Done)

	return nil
}

func (m *Manager) Stop(runID string) error {
	m.mu.Lock()
	mp, ok := m.processes[runID]
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("no active process for run %s", runID)
	}

	if mp.proc != nil {
		if err := m.rt.Stop(mp.proc); err != nil {
			return err
		}
	} else if mp.pid > 0 {
		_ = syscall.Kill(mp.pid, syscall.SIGTERM)
		go func() {
			time.Sleep(5 * time.Second)
			_ = syscall.Kill(mp.pid, syscall.SIGKILL)
		}()
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

	if mp.proc != nil {
		if err := m.rt.Pause(mp.proc); err != nil {
			return err
		}
	} else if mp.pid > 0 {
		_ = syscall.Kill(mp.pid, syscall.SIGSTOP)
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

	if mp.proc != nil {
		if err := m.rt.Resume(mp.proc); err != nil {
			return err
		}
	} else if mp.pid > 0 {
		_ = syscall.Kill(mp.pid, syscall.SIGCONT)
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

	if mp.proc != nil && mp.proc.Cmd != nil && mp.proc.Cmd.Process != nil {
		_ = mp.proc.Cmd.Process.Signal(syscall.SIGKILL)
	} else if mp.pid > 0 {
		_ = syscall.Kill(mp.pid, syscall.SIGKILL)
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
// Used to restore log history for rehydrated runs that have no log files
// (backward compatibility with old sessions).
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
	if lf, ok := m.logFiles[runID]; ok {
		lf.Close()
		delete(m.logFiles, runID)
	}
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

// LogFilePaths returns the stdout and stderr log file paths for a run,
// or empty strings if the run has no log files.
func (m *Manager) LogFilePaths(runID string) (stdoutPath, stderrPath string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if lf, ok := m.logFiles[runID]; ok {
		return lf.StdoutPath(), lf.StderrPath()
	}
	return "", ""
}

// Reconnect attaches to a running process by tailing its log files.
// Used during rehydration to reconnect to processes that survived agtop exit.
func (m *Manager) Reconnect(runID string, pid int, stdoutPath string, stderrPath string) {
	ctx, cancel := context.WithCancel(context.Background())

	stdoutF, err := os.Open(stdoutPath)
	if err != nil {
		log.Printf("warning: reconnect %s: open stdout: %v", runID, err)
		cancel()
		m.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = fmt.Sprintf("reconnect failed: open stdout log: %v", err)
			r.PID = 0
		})
		return
	}
	stderrF, err := os.Open(stderrPath)
	if err != nil {
		log.Printf("warning: reconnect %s: open stderr: %v", runID, err)
		stdoutF.Close()
		cancel()
		m.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = fmt.Sprintf("reconnect failed: open stderr log: %v", err)
			r.PID = 0
		})
		return
	}

	stdoutReader := NewFollowReader(ctx, stdoutF)
	stderrReader := NewFollowReader(ctx, stderrF)

	buf := NewRingBuffer(10000)
	eb := NewEntryBuffer(5000)

	mp := &ManagedProcess{
		proc:   nil, // reconnected — no exec.Cmd
		cancel: cancel,
		runID:  runID,
		pid:    pid,
	}

	m.mu.Lock()
	m.processes[runID] = mp
	m.buffers[runID] = buf
	m.entryBuffers[runID] = eb
	m.mu.Unlock()

	// Monitor PID: when the process exits, cancel the FollowReaders
	doneCh := make(chan error, 1)
	go func() {
		for {
			time.Sleep(2 * time.Second)
			if !run.IsProcessAlive(pid) {
				doneCh <- fmt.Errorf("process exited while agtop was not running")
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	go m.consumeEvents(runID, mp, buf, eb, stdoutReader, stderrReader, doneCh)
}

// ReplayLogFile reads an entire log file into buffers without following.
// Used for rehydrating completed/failed runs that have log files on disk.
func (m *Manager) ReplayLogFile(runID string, stdoutPath string, stderrPath string) {
	buf := NewRingBuffer(10000)
	eb := NewEntryBuffer(5000)

	// Replay stdout (stream-json events)
	if stdoutPath != "" {
		if data, err := os.ReadFile(stdoutPath); err == nil {
			parser := m.newParser(strings.NewReader(string(data)), 256)
			go parser.Parse(context.Background())

			skillName := func() string {
				r, ok := m.store.Get(runID)
				if !ok {
					return ""
				}
				return r.CurrentSkill
			}

			for event := range parser.Events() {
				ts := time.Now().Format("15:04:05")
				skill := skillName()
				logLine, entry := m.formatEvent(event, ts, skill, nil, runID, buf)
				if logLine != "" {
					buf.Append(logLine)
					if entry != nil {
						eb.Append(entry)
					}
				}
			}
		}
	}

	// Replay stderr
	if stderrPath != "" {
		if data, err := os.ReadFile(stderrPath); err == nil {
			scanner := bufio.NewScanner(strings.NewReader(string(data)))
			for scanner.Scan() {
				line := scanner.Text()
				ts := time.Now().Format("15:04:05")
				formatted := fmt.Sprintf("[%s] %s", ts, line)
				buf.Append(formatted)
				eb.Append(NewLogEntry(ts, "", EventRaw, line))
			}
		}
	}

	m.mu.Lock()
	m.buffers[runID] = buf
	m.entryBuffers[runID] = eb
	m.mu.Unlock()
}

// SetDisconnecting marks the manager as shutting down. When set,
// consumeSkillEvents and consumeEvents will not zero PIDs or update
// run state, preserving the run for reconnection on next startup.
func (m *Manager) SetDisconnecting() {
	m.mu.Lock()
	m.disconnecting = true
	m.mu.Unlock()
}

func (m *Manager) isDisconnecting() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.disconnecting
}

// DisconnectAll cancels all FollowReader contexts and closes log file handles
// without killing any processes. Called when the TUI exits.
func (m *Manager) DisconnectAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, mp := range m.processes {
		mp.cancel()
	}
	for _, lf := range m.logFiles {
		lf.Close()
	}
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

	// Create log files for persistent output
	var lf *LogFiles
	if m.sessionsDir != "" {
		var err error
		lf, err = CreateLogFiles(m.sessionsDir, runID)
		if err != nil {
			log.Printf("warning: create log files for %s: %v (falling back to pipes)", runID, err)
		} else {
			opts.StdoutFile = lf.StdoutWriter()
			opts.StderrFile = lf.StderrWriter()
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	proc, err := m.rt.Start(ctx, prompt, opts)
	if err != nil {
		cancel()
		if lf != nil {
			lf.Close()
		}
		return nil, fmt.Errorf("start process: %w", err)
	}

	// Determine stdout/stderr readers
	var stdoutReader io.Reader
	var stderrReader io.Reader
	if lf != nil {
		stdoutR, err := os.Open(lf.StdoutPath())
		if err != nil {
			cancel()
			lf.Close()
			return nil, fmt.Errorf("open stdout log for reading: %w", err)
		}
		stderrR, err := os.Open(lf.StderrPath())
		if err != nil {
			stdoutR.Close()
			cancel()
			lf.Close()
			return nil, fmt.Errorf("open stderr log for reading: %w", err)
		}
		stdoutReader = NewFollowReader(ctx, stdoutR)
		stderrReader = NewFollowReader(ctx, stderrR)
	} else {
		stdoutReader = proc.Stdout
		stderrReader = proc.Stderr
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
		pid:    proc.PID,
	}

	resultCh := make(chan SkillResult, 1)

	m.mu.Lock()
	m.processes[runID] = mp
	m.buffers[runID] = buf
	m.entryBuffers[runID] = eb
	if lf != nil {
		m.logFiles[runID] = lf
	}
	m.mu.Unlock()

	m.store.Update(runID, func(r *run.Run) {
		r.PID = proc.PID
	})

	go m.consumeSkillEvents(runID, mp, buf, eb, stdoutReader, stderrReader, proc.Done, resultCh)

	return resultCh, nil
}

func (m *Manager) consumeSkillEvents(runID string, mp *ManagedProcess, buf *RingBuffer, eb *EntryBuffer, stdout io.Reader, stderr io.Reader, done <-chan error, resultCh chan<- SkillResult) {
	defer close(resultCh)

	var resultText string

	// When the process exits, cancel the FollowReader context so the
	// stream parser drains and the event loop below unblocks.
	var exitErr error
	exitDone := make(chan struct{})
	go func() {
		exitErr = <-done
		close(exitDone)
		mp.cancel()
	}()

	skillName := func() string {
		r, ok := m.store.Get(runID)
		if !ok {
			return ""
		}
		return r.CurrentSkill
	}

	parser := m.newParser(stdout, 256)
	go parser.Parse(context.Background())

	go func() {
		scanner := bufio.NewScanner(stderr)
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

		if event.Type == EventResult {
			resultText = event.Text
		}

		logLine, entry := m.formatEvent(event, ts, skill, buf, runID, buf)
		if logLine != "" {
			buf.Append(logLine)
			if entry != nil {
				eb.Append(entry)
			}
			m.sendLogLine(runID)
		}
	}

	<-exitDone

	// If the TUI is shutting down, preserve PID and process entry so the
	// session file saves the live state for reconnection on restart.
	if m.isDisconnecting() {
		resultCh <- SkillResult{ResultText: resultText, Err: ErrDisconnected}
		return
	}

	m.store.Update(runID, func(r *run.Run) {
		r.PID = 0
	})

	m.mu.Lock()
	delete(m.processes, runID)
	m.mu.Unlock()

	resultCh <- SkillResult{ResultText: resultText, Err: exitErr}
}

func (m *Manager) consumeEvents(runID string, mp *ManagedProcess, buf *RingBuffer, eb *EntryBuffer, stdout io.Reader, stderr io.Reader, done <-chan error) {
	// When the process exits, cancel the FollowReader context so the
	// stream parser drains and the event loop below unblocks.
	var exitErr error
	exitDone := make(chan struct{})
	go func() {
		exitErr = <-done
		close(exitDone)
		mp.cancel()
	}()

	skillName := func() string {
		r, ok := m.store.Get(runID)
		if !ok {
			return ""
		}
		return r.CurrentSkill
	}

	parser := m.newParser(stdout, 256)
	go parser.Parse(context.Background())

	go func() {
		scanner := bufio.NewScanner(stderr)
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

		logLine, entry := m.formatEvent(event, ts, skill, buf, runID, buf)
		if logLine != "" {
			buf.Append(logLine)
			if entry != nil {
				eb.Append(entry)
			}
			m.sendLogLine(runID)
		}
	}

	<-exitDone

	// If the TUI is shutting down, preserve PID and process entry so the
	// session file saves the live state for reconnection on restart.
	if m.isDisconnecting() {
		return
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

// formatEvent converts a StreamEvent into a formatted log line and a LogEntry.
// The safetyBuf is used for tool safety checks (pass the ring buffer).
func (m *Manager) formatEvent(event StreamEvent, ts string, skill string, safetyBuf *RingBuffer, runID string, buf *RingBuffer) (string, *LogEntry) {
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
		if safetyBuf != nil {
			m.checkToolSafety(event.ToolName, event.ToolInput, ts, skill, safetyBuf, runID)
		}
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
		entry = InterpretRawEvent(ts, skill, event.Text)
	}

	return logLine, entry
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
