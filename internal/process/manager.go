package process

import (
	"bufio"
	"context"
	"fmt"
	"sync"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
)

type LogLineMsg struct {
	RunID string
}

type ManagedProcess struct {
	proc   *runtime.Process
	cancel context.CancelFunc
	runID  string
}

type Manager struct {
	store     *run.Store
	rt        runtime.Runtime
	cfg       *config.LimitsConfig
	mu        sync.Mutex
	processes map[string]*ManagedProcess
	buffers   map[string]*RingBuffer
	program   *tea.Program
}

func NewManager(store *run.Store, rt runtime.Runtime, cfg *config.LimitsConfig) *Manager {
	return &Manager{
		store:     store,
		rt:        rt,
		cfg:       cfg,
		processes: make(map[string]*ManagedProcess),
		buffers:   make(map[string]*RingBuffer),
	}
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

	mp := &ManagedProcess{
		proc:   proc,
		cancel: cancel,
		runID:  runID,
	}

	m.mu.Lock()
	m.processes[runID] = mp
	m.buffers[runID] = buf
	m.mu.Unlock()

	m.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRunning
		r.PID = proc.PID
		r.StartedAt = time.Now()
	})

	go m.consumeEvents(runID, mp, buf)

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

func (m *Manager) ActiveCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.processes)
}

func (m *Manager) consumeEvents(runID string, mp *ManagedProcess, buf *RingBuffer) {
	// Get current skill name for log prefix
	skillName := func() string {
		r, ok := m.store.Get(runID)
		if !ok {
			return ""
		}
		return r.CurrentSkill
	}

	// Create stream parser on stdout
	parser := NewStreamParser(mp.proc.Stdout, 256)
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
			m.sendLogLine(runID)
		}
	}()

	for event := range parser.Events() {
		ts := time.Now().Format("15:04:05")
		skill := skillName()

		var logLine string
		switch event.Type {
		case EventText:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] %s", ts, event.Text)
			}
		case EventToolUse:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] Tool: %s", ts, skill, event.ToolName)
			} else {
				logLine = fmt.Sprintf("[%s] Tool: %s", ts, event.ToolName)
			}
		case EventToolResult:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] Result: %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] Result: %s", ts, event.Text)
			}
		case EventResult:
			if event.Usage != nil {
				m.store.Update(runID, func(r *run.Run) {
					r.Tokens += event.Usage.TotalTokens
					r.Cost += event.Usage.CostUSD
				})

				// Check cost/token thresholds
				if m.cfg.MaxCostPerRun > 0 || m.cfg.MaxTokensPerRun > 0 {
					r, ok := m.store.Get(runID)
					if ok {
						if m.cfg.MaxCostPerRun > 0 && r.Cost >= m.cfg.MaxCostPerRun {
							warning := fmt.Sprintf("[%s] WARNING: Cost threshold exceeded ($%.2f >= $%.2f), pausing run", ts, r.Cost, m.cfg.MaxCostPerRun)
							buf.Append(warning)
							_ = m.Pause(runID)
						}
						if m.cfg.MaxTokensPerRun > 0 && r.Tokens >= m.cfg.MaxTokensPerRun {
							warning := fmt.Sprintf("[%s] WARNING: Token threshold exceeded (%d >= %d), pausing run", ts, r.Tokens, m.cfg.MaxTokensPerRun)
							buf.Append(warning)
							_ = m.Pause(runID)
						}
					}
				}

				if skill != "" {
					logLine = fmt.Sprintf("[%s %s] Completed — %d tokens, $%.4f", ts, skill, event.Usage.TotalTokens, event.Usage.CostUSD)
				} else {
					logLine = fmt.Sprintf("[%s] Completed — %d tokens, $%.4f", ts, event.Usage.TotalTokens, event.Usage.CostUSD)
				}
			}
		case EventError:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] ERROR: %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] ERROR: %s", ts, event.Text)
			}
		case EventRaw:
			if skill != "" {
				logLine = fmt.Sprintf("[%s %s] %s", ts, skill, event.Text)
			} else {
				logLine = fmt.Sprintf("[%s] %s", ts, event.Text)
			}
		}

		if logLine != "" {
			buf.Append(logLine)
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

func (m *Manager) sendLogLine(runID string) {
	m.mu.Lock()
	p := m.program
	m.mu.Unlock()
	if p != nil {
		p.Send(LogLineMsg{RunID: runID})
	}
}
