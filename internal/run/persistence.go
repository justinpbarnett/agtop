package run

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/justinpbarnett/agtop/internal/cost"
)

const (
	sessionVersion = 1
	maxLogTail     = 1000
	debouncePeriod = 500 * time.Millisecond
)

type SessionFile struct {
	Version       int       `json:"version"`
	Run           Run       `json:"run"`
	LogTail       []string  `json:"log_tail"`
	StdoutLogPath string    `json:"stdout_log_path,omitempty"`
	StderrLogPath string    `json:"stderr_log_path,omitempty"`
	SavedAt       time.Time `json:"saved_at"`
}

type Persistence struct {
	sessionsDir string
	mu          sync.Mutex
	lastSave    map[string]time.Time
	lastState   map[string]State
}

func NewPersistence(projectRoot string) (*Persistence, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root: %w", err)
	}

	h := fnv.New32a()
	h.Write([]byte(absRoot))
	projectHash := fmt.Sprintf("%08x", h.Sum32())

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	sessionsDir := filepath.Join(homeDir, ".agtop", "sessions", projectHash)
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		return nil, fmt.Errorf("create sessions dir: %w", err)
	}

	return &Persistence{
		sessionsDir: sessionsDir,
		lastSave:    make(map[string]time.Time),
		lastState:   make(map[string]State),
	}, nil
}

// SessionsDir returns the sessions directory path (used by cleanup).
func (p *Persistence) SessionsDir() string {
	return p.sessionsDir
}

func (p *Persistence) Save(r Run, logTail []string, stdoutLogPath, stderrLogPath string) error {
	if r.ID == "" {
		return nil
	}

	if len(logTail) > maxLogTail {
		logTail = logTail[len(logTail)-maxLogTail:]
	}

	sf := SessionFile{
		Version:       sessionVersion,
		Run:           r,
		LogTail:       logTail,
		StdoutLogPath: stdoutLogPath,
		StderrLogPath: stderrLogPath,
		SavedAt:       time.Now(),
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal session: %w", err)
	}
	data = append(data, '\n')

	target := filepath.Join(p.sessionsDir, r.ID+".json")
	tmp := target + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp session file: %w", err)
	}

	if err := os.Rename(tmp, target); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename session file: %w", err)
	}

	return nil
}

func (p *Persistence) Load() ([]SessionFile, error) {
	entries, err := os.ReadDir(p.sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read sessions dir: %w", err)
	}

	var sessions []SessionFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if strings.HasSuffix(entry.Name(), ".tmp") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(p.sessionsDir, entry.Name()))
		if err != nil {
			log.Printf("warning: read session file %s: %v", entry.Name(), err)
			continue
		}

		var sf SessionFile
		if err := json.Unmarshal(data, &sf); err != nil {
			log.Printf("warning: parse session file %s: %v", entry.Name(), err)
			continue
		}

		if sf.Version != sessionVersion {
			log.Printf("warning: session file %s has version %d, expected %d, skipping", entry.Name(), sf.Version, sessionVersion)
			continue
		}

		if sf.Run.ID == "" {
			log.Printf("warning: session file %s has empty run ID, skipping", entry.Name())
			continue
		}

		sessions = append(sessions, sf)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].Run.CreatedAt.Before(sessions[j].Run.CreatedAt)
	})

	return sessions, nil
}

func (p *Persistence) Remove(runID string) error {
	target := filepath.Join(p.sessionsDir, runID+".json")
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove session file: %w", err)
	}
	return nil
}

// BindStore subscribes to store changes and auto-saves runs with debouncing.
// getLogTail returns recent log lines for a run.
// getLogPaths returns the stdout/stderr log file paths for a run (may be nil for backward compat).
func (p *Persistence) BindStore(store *Store, getLogTail func(runID string) []string, getLogPaths func(runID string) (string, string)) {
	store.Subscribe(func() {
		runs := store.List()
		for _, r := range runs {
			// Skip parallel sub-task IDs (contain ':')
			if strings.Contains(r.ID, ":") {
				continue
			}

			p.mu.Lock()
			lastTime := p.lastSave[r.ID]
			lastState := p.lastState[r.ID]
			now := time.Now()

			isTerminal := r.IsTerminal()
			stateChanged := r.State != lastState

			// Always save on terminal state or state change; debounce otherwise
			if !isTerminal && !stateChanged && now.Sub(lastTime) < debouncePeriod {
				p.mu.Unlock()
				continue
			}

			p.lastSave[r.ID] = now
			p.lastState[r.ID] = r.State
			p.mu.Unlock()

			var logTail []string
			if getLogTail != nil {
				logTail = getLogTail(r.ID)
			}

			var stdoutPath, stderrPath string
			if getLogPaths != nil {
				stdoutPath, stderrPath = getLogPaths(r.ID)
			}

			if err := p.Save(r, logTail, stdoutPath, stderrPath); err != nil {
				log.Printf("warning: save session %s: %v", r.ID, err)
			}
		}
	})
}

// RehydrateCallbacks holds optional callbacks invoked during rehydration.
type RehydrateCallbacks struct {
	// InjectBuffer restores log lines for a rehydrated run (backward compat — no log files).
	InjectBuffer func(runID string, lines []string)
	// RecordCost replays a skill cost entry into the cost tracker.
	RecordCost func(runID string, sc SkillCost)
	// Reconnect tails log files for a live process. Called instead of InjectBuffer when log files exist.
	Reconnect func(runID string, pid int, stdoutPath, stderrPath string)
	// ReplayLogFile reads log files for a dead/terminal process. Called instead of InjectBuffer when log files exist.
	ReplayLogFile func(runID string, stdoutPath, stderrPath string)
}

// SkillCost is re-exported here to avoid a circular import in the callback
// signature. Callers pass cost.SkillCost values; see the adapter in app.go.
type SkillCost = cost.SkillCost

// Rehydrate loads session files and restores them into the store.
// Returns the number of rehydrated runs and the IDs of runs with live PIDs.
func (p *Persistence) Rehydrate(store *Store, cb RehydrateCallbacks) (int, []string, error) {
	sessions, err := p.Load()
	if err != nil {
		return 0, nil, err
	}

	if len(sessions) == 0 {
		return 0, nil, nil
	}

	var watchIDs []string
	maxID := 0

	for _, sf := range sessions {
		r := sf.Run
		hasLogFiles := sf.StdoutLogPath != "" && sf.StderrLogPath != ""

		if !r.IsTerminal() {
			if r.PID > 0 && IsProcessAlive(r.PID) {
				if hasLogFiles && cb.Reconnect != nil {
					// Live process with log files: reconnect via file tailing
					store.Add(&r)
					cb.Reconnect(r.ID, r.PID, sf.StdoutLogPath, sf.StderrLogPath)
				} else {
					// Live process without log files: legacy PID watcher
					store.Add(&r)
					watchIDs = append(watchIDs, r.ID)
					if cb.InjectBuffer != nil && len(sf.LogTail) > 0 {
						cb.InjectBuffer(r.ID, sf.LogTail)
					}
				}
			} else {
				r.State = StateFailed
				r.Error = "process no longer running (agtop restarted)"
				r.PID = 0
				store.Add(&r)
				if hasLogFiles && cb.ReplayLogFile != nil {
					cb.ReplayLogFile(r.ID, sf.StdoutLogPath, sf.StderrLogPath)
				} else if cb.InjectBuffer != nil && len(sf.LogTail) > 0 {
					cb.InjectBuffer(r.ID, sf.LogTail)
				}
			}
		} else {
			store.Add(&r)
			if hasLogFiles && cb.ReplayLogFile != nil {
				cb.ReplayLogFile(r.ID, sf.StdoutLogPath, sf.StderrLogPath)
			} else if cb.InjectBuffer != nil && len(sf.LogTail) > 0 {
				cb.InjectBuffer(r.ID, sf.LogTail)
			}
		}

		if cb.RecordCost != nil {
			for _, sc := range r.SkillCosts {
				cb.RecordCost(r.ID, sc)
			}
		}

		// Track max numeric ID for counter restoration
		if id, err := strconv.Atoi(r.ID); err == nil && id > maxID {
			maxID = id
		}
	}

	if maxID > 0 {
		store.SetNextID(maxID)
	}

	// Initialize debounce state for rehydrated runs
	p.mu.Lock()
	now := time.Now()
	for _, sf := range sessions {
		p.lastSave[sf.Run.ID] = now
		p.lastState[sf.Run.ID] = sf.Run.State
	}
	p.mu.Unlock()

	return len(sessions), watchIDs, nil
}

// RehydrateWithWatcher loads sessions and starts a PID watcher for live processes.
// Returns the count of rehydrated runs and a cancel function for the watcher.
func (p *Persistence) RehydrateWithWatcher(store *Store, cb RehydrateCallbacks) (int, context.CancelFunc, error) {
	count, watchIDs, err := p.Rehydrate(store, cb)
	if err != nil {
		return 0, func() {}, err
	}

	var cancel context.CancelFunc
	if len(watchIDs) > 0 {
		cancel = WatchPIDs(watchIDs, store, 5*time.Second)
	} else {
		cancel = func() {}
	}

	return count, cancel, nil
}

// WatchPIDs monitors a set of run IDs whose PIDs were alive at rehydration time.
// When a PID dies, the run is marked as failed. Returns a cancel function.
func WatchPIDs(runIDs []string, store *Store, interval time.Duration) context.CancelFunc {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		remaining := make([]string, len(runIDs))
		copy(remaining, runIDs)

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				var stillAlive []string
				for _, id := range remaining {
					r, ok := store.Get(id)
					if !ok || r.IsTerminal() {
						continue
					}
					if r.PID > 0 && IsProcessAlive(r.PID) {
						stillAlive = append(stillAlive, id)
					} else {
						store.Update(id, func(r *Run) {
							r.State = StateFailed
							r.Error = "process exited while agtop was not running"
							r.PID = 0
						})
					}
				}
				remaining = stillAlive
				if len(remaining) == 0 {
					return
				}
			}
		}
	}()

	return cancel
}

// IsProcessAlive checks if a process with the given PID exists.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil
}

// ProjectHash computes the FNV-32a hash of an absolute project root path.
// Exported for use by the cleanup subcommand.
func ProjectHash(projectRoot string) (string, error) {
	absRoot, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	h := fnv.New32a()
	h.Write([]byte(absRoot))
	return fmt.Sprintf("%08x", h.Sum32()), nil
}
