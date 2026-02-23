package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/justinpbarnett/agtop/internal/cost"
)

func tempPersistence(t *testing.T) *Persistence {
	t.Helper()
	dir := t.TempDir()
	return &Persistence{
		sessionsDir: dir,
		lastSave:    make(map[string]time.Time),
		lastState:   make(map[string]State),
	}
}

func TestPersistenceSaveAndLoad(t *testing.T) {
	p := tempPersistence(t)

	now := time.Now().Truncate(time.Second)
	r := Run{
		ID:           "001",
		Prompt:       "add auth",
		Branch:       "agtop/001",
		Worktree:     "/tmp/wt/001",
		Workflow:     "sdlc",
		State:        StateCompleted,
		SkillIndex:   3,
		SkillTotal:   7,
		Tokens:       12400,
		TokensIn:     8200,
		TokensOut:    4200,
		Cost:         0.42,
		CreatedAt:    now.Add(-30 * time.Minute),
		StartedAt:    now.Add(-25 * time.Minute),
		CurrentSkill: "build",
		Model:        "sonnet",
		Command:      "claude -p test",
		Error:        "",
		PID:          0,
		SkillCosts: []cost.SkillCost{
			{SkillName: "build", InputTokens: 5000, OutputTokens: 2000, TotalTokens: 7000, CostUSD: 0.25, StartedAt: now.Add(-20 * time.Minute), CompletedAt: now.Add(-10 * time.Minute)},
		},
		DevServerPort: 3142,
		DevServerURL:  "http://localhost:3142",
	}

	logTail := []string{"line 1", "line 2", "line 3"}

	if err := p.Save(r, logTail, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	sessions, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}

	sf := sessions[0]
	if sf.Version != sessionVersion {
		t.Errorf("version: got %d, want %d", sf.Version, sessionVersion)
	}
	if sf.Run.ID != "001" {
		t.Errorf("ID: got %s, want 001", sf.Run.ID)
	}
	if sf.Run.Prompt != "add auth" {
		t.Errorf("Prompt: got %s, want 'add auth'", sf.Run.Prompt)
	}
	if sf.Run.Branch != "agtop/001" {
		t.Errorf("Branch: got %s, want agtop/001", sf.Run.Branch)
	}
	if sf.Run.State != StateCompleted {
		t.Errorf("State: got %s, want %s", sf.Run.State, StateCompleted)
	}
	if sf.Run.SkillIndex != 3 {
		t.Errorf("SkillIndex: got %d, want 3", sf.Run.SkillIndex)
	}
	if sf.Run.Tokens != 12400 {
		t.Errorf("Tokens: got %d, want 12400", sf.Run.Tokens)
	}
	if sf.Run.Cost != 0.42 {
		t.Errorf("Cost: got %f, want 0.42", sf.Run.Cost)
	}
	if len(sf.Run.SkillCosts) != 1 {
		t.Fatalf("SkillCosts: got %d entries, want 1", len(sf.Run.SkillCosts))
	}
	if sf.Run.SkillCosts[0].SkillName != "build" {
		t.Errorf("SkillCosts[0].SkillName: got %s, want build", sf.Run.SkillCosts[0].SkillName)
	}
	if sf.Run.DevServerPort != 3142 {
		t.Errorf("DevServerPort: got %d, want 3142", sf.Run.DevServerPort)
	}
	if len(sf.LogTail) != 3 {
		t.Errorf("LogTail: got %d lines, want 3", len(sf.LogTail))
	}
	if sf.LogTail[0] != "line 1" {
		t.Errorf("LogTail[0]: got %s, want 'line 1'", sf.LogTail[0])
	}
	if !sf.Run.CreatedAt.Equal(now.Add(-30 * time.Minute)) {
		t.Errorf("CreatedAt: got %v, want %v", sf.Run.CreatedAt, now.Add(-30*time.Minute))
	}
}

func TestPersistenceSaveAtomic(t *testing.T) {
	p := tempPersistence(t)

	r := Run{ID: "001", State: StateRunning, CreatedAt: time.Now()}
	if err := p.Save(r, nil, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Verify no .tmp files remain
	entries, _ := os.ReadDir(p.sessionsDir)
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("found leftover temp file: %s", e.Name())
		}
	}

	// Verify the file is valid JSON
	data, err := os.ReadFile(filepath.Join(p.sessionsDir, "001.json"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var sf SessionFile
	if err := json.Unmarshal(data, &sf); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sf.Run.ID != "001" {
		t.Errorf("expected ID 001, got %s", sf.Run.ID)
	}
}

func TestPersistenceLoadSkipsCorruptFiles(t *testing.T) {
	p := tempPersistence(t)

	// Write a valid session
	r := Run{ID: "001", State: StateCompleted, CreatedAt: time.Now()}
	if err := p.Save(r, nil, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// Write a corrupt file
	corruptPath := filepath.Join(p.sessionsDir, "002.json")
	os.WriteFile(corruptPath, []byte("not json{{{"), 0o644)

	sessions, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session (corrupt skipped), got %d", len(sessions))
	}
	if sessions[0].Run.ID != "001" {
		t.Errorf("expected ID 001, got %s", sessions[0].Run.ID)
	}
}

func TestPersistenceLoadSkipsBadVersion(t *testing.T) {
	p := tempPersistence(t)

	// Write a session with wrong version
	sf := SessionFile{Version: 99, Run: Run{ID: "001", State: StateCompleted}, SavedAt: time.Now()}
	data, _ := json.Marshal(sf)
	os.WriteFile(filepath.Join(p.sessionsDir, "001.json"), data, 0o644)

	sessions, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (bad version), got %d", len(sessions))
	}
}

func TestPersistenceLoadSkipsEmptyID(t *testing.T) {
	p := tempPersistence(t)

	sf := SessionFile{Version: 1, Run: Run{ID: "", State: StateCompleted}, SavedAt: time.Now()}
	data, _ := json.Marshal(sf)
	os.WriteFile(filepath.Join(p.sessionsDir, "bad.json"), data, 0o644)

	sessions, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (empty ID), got %d", len(sessions))
	}
}

func TestPersistenceRehydrateTerminalRuns(t *testing.T) {
	p := tempPersistence(t)
	store := NewStore()

	now := time.Now()
	for _, state := range []State{StateCompleted, StateFailed, StateAccepted, StateRejected} {
		r := Run{
			ID:        string(state),
			State:     state,
			Tokens:    1000,
			Cost:      0.50,
			CreatedAt: now,
		}
		if err := p.Save(r, nil, "", ""); err != nil {
			t.Fatalf("Save %s: %v", state, err)
		}
	}

	count, _, err := p.Rehydrate(store, RehydrateCallbacks{})
	if err != nil {
		t.Fatalf("Rehydrate: %v", err)
	}
	if count != 4 {
		t.Errorf("expected 4 rehydrated, got %d", count)
	}
	if store.Count() != 4 {
		t.Errorf("expected 4 in store, got %d", store.Count())
	}

	for _, state := range []State{StateCompleted, StateFailed, StateAccepted, StateRejected} {
		r, ok := store.Get(string(state))
		if !ok {
			t.Errorf("run %s not found in store", state)
			continue
		}
		if r.State != state {
			t.Errorf("run %s: expected state %s, got %s", state, state, r.State)
		}
	}
}

func TestPersistenceRehydrateDeadProcess(t *testing.T) {
	p := tempPersistence(t)
	store := NewStore()

	// Use a PID that almost certainly doesn't exist
	r := Run{
		ID:        "001",
		State:     StateRunning,
		PID:       999999999,
		CreatedAt: time.Now(),
	}
	if err := p.Save(r, nil, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	count, _, err := p.Rehydrate(store, RehydrateCallbacks{})
	if err != nil {
		t.Fatalf("Rehydrate: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	got, ok := store.Get("001")
	if !ok {
		t.Fatal("run not found")
	}
	if got.State != StateFailed {
		t.Errorf("expected state %s, got %s", StateFailed, got.State)
	}
	if got.PID != 0 {
		t.Errorf("expected PID 0, got %d", got.PID)
	}
	if got.Error == "" {
		t.Error("expected non-empty error message")
	}
}

func TestPersistenceRehydrateNonTerminalZeroPID(t *testing.T) {
	p := tempPersistence(t)
	store := NewStore()

	r := Run{
		ID:        "001",
		State:     StatePaused,
		PID:       0,
		CreatedAt: time.Now(),
	}
	if err := p.Save(r, nil, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	p.Rehydrate(store, RehydrateCallbacks{})

	got, _ := store.Get("001")
	if got.State != StateFailed {
		t.Errorf("expected state %s, got %s", StateFailed, got.State)
	}
}

func TestPersistenceRehydrateRestoresNextID(t *testing.T) {
	p := tempPersistence(t)
	store := NewStore()

	now := time.Now()
	for _, id := range []string{"003", "007", "012"} {
		r := Run{ID: id, State: StateCompleted, CreatedAt: now}
		if err := p.Save(r, nil, "", ""); err != nil {
			t.Fatalf("Save %s: %v", id, err)
		}
	}

	p.Rehydrate(store, RehydrateCallbacks{})

	// Next ID should be 013
	newID := store.Add(&Run{State: StateQueued})
	if newID != "013" {
		t.Errorf("expected next ID 013, got %s", newID)
	}
}

func TestPersistenceRehydrateRestoresLogTail(t *testing.T) {
	p := tempPersistence(t)
	store := NewStore()

	lines := make([]string, 500)
	for i := range lines {
		lines[i] = "log line"
	}

	r := Run{ID: "001", State: StateCompleted, CreatedAt: time.Now()}
	if err := p.Save(r, lines, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var gotRunID string
	var gotLines []string
	p.Rehydrate(store, RehydrateCallbacks{
		InjectBuffer: func(runID string, l []string) {
			gotRunID = runID
			gotLines = l
		},
	})

	if gotRunID != "001" {
		t.Errorf("expected runID 001, got %s", gotRunID)
	}
	if len(gotLines) != 500 {
		t.Errorf("expected 500 lines, got %d", len(gotLines))
	}
}

func TestPersistenceRemove(t *testing.T) {
	p := tempPersistence(t)

	r := Run{ID: "001", State: StateCompleted, CreatedAt: time.Now()}
	if err := p.Save(r, nil, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := p.Remove("001"); err != nil {
		t.Fatalf("Remove: %v", err)
	}

	sessions, _ := p.Load()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions after remove, got %d", len(sessions))
	}
}

func TestPersistenceRemoveNonexistent(t *testing.T) {
	p := tempPersistence(t)
	if err := p.Remove("nonexistent"); err != nil {
		t.Errorf("Remove nonexistent should not error, got: %v", err)
	}
}

func TestPersistenceProjectHash(t *testing.T) {
	hash1, err := ProjectHash("/home/user/project-a")
	if err != nil {
		t.Fatalf("ProjectHash: %v", err)
	}
	hash2, err := ProjectHash("/home/user/project-b")
	if err != nil {
		t.Fatalf("ProjectHash: %v", err)
	}

	if hash1 == hash2 {
		t.Error("expected different hashes for different paths")
	}

	// Same path gives same hash
	hash1again, _ := ProjectHash("/home/user/project-a")
	if hash1 != hash1again {
		t.Error("expected same hash for same path")
	}

	if len(hash1) != 8 {
		t.Errorf("expected 8-char hash, got %d chars: %s", len(hash1), hash1)
	}
}

func TestPersistenceSessionFileVersion(t *testing.T) {
	p := tempPersistence(t)

	r := Run{ID: "001", State: StateCompleted, CreatedAt: time.Now()}
	if err := p.Save(r, nil, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(p.sessionsDir, "001.json"))
	var sf SessionFile
	json.Unmarshal(data, &sf)

	if sf.Version != 1 {
		t.Errorf("expected version 1, got %d", sf.Version)
	}
}

func TestPersistenceLogTailTruncation(t *testing.T) {
	p := tempPersistence(t)

	lines := make([]string, 2000)
	for i := range lines {
		lines[i] = "line"
	}

	r := Run{ID: "001", State: StateCompleted, CreatedAt: time.Now()}
	if err := p.Save(r, lines, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	sessions, _ := p.Load()
	if len(sessions) != 1 {
		t.Fatalf("expected 1 session, got %d", len(sessions))
	}
	if len(sessions[0].LogTail) != maxLogTail {
		t.Errorf("expected %d log lines (truncated), got %d", maxLogTail, len(sessions[0].LogTail))
	}
}

func TestPersistenceLoadSortsByCreatedAt(t *testing.T) {
	p := tempPersistence(t)

	now := time.Now()
	// Save in non-chronological order
	p.Save(Run{ID: "002", State: StateCompleted, CreatedAt: now.Add(-10 * time.Minute)}, nil, "", "")
	p.Save(Run{ID: "001", State: StateCompleted, CreatedAt: now.Add(-30 * time.Minute)}, nil, "", "")
	p.Save(Run{ID: "003", State: StateCompleted, CreatedAt: now.Add(-5 * time.Minute)}, nil, "", "")

	sessions, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3, got %d", len(sessions))
	}
	// Should be sorted oldest first
	if sessions[0].Run.ID != "001" {
		t.Errorf("expected first to be 001 (oldest), got %s", sessions[0].Run.ID)
	}
	if sessions[1].Run.ID != "002" {
		t.Errorf("expected second to be 002, got %s", sessions[1].Run.ID)
	}
	if sessions[2].Run.ID != "003" {
		t.Errorf("expected third to be 003 (newest), got %s", sessions[2].Run.ID)
	}
}

func TestPersistenceEmptyDirectory(t *testing.T) {
	p := tempPersistence(t)

	sessions, err := p.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}

	store := NewStore()
	count, _, err := p.Rehydrate(store, RehydrateCallbacks{})
	if err != nil {
		t.Fatalf("Rehydrate: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 rehydrated, got %d", count)
	}
}

func TestPersistenceSaveEmptyID(t *testing.T) {
	p := tempPersistence(t)

	err := p.Save(Run{ID: ""}, nil, "", "")
	if err != nil {
		t.Errorf("Save with empty ID should return nil, got: %v", err)
	}

	sessions, _ := p.Load()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions (empty ID skipped), got %d", len(sessions))
	}
}

func TestPersistenceLoadSkipsTmpFiles(t *testing.T) {
	p := tempPersistence(t)

	// Write a valid session
	p.Save(Run{ID: "001", State: StateCompleted, CreatedAt: time.Now()}, nil, "", "")

	// Write a .tmp file that looks like a session
	tmpPath := filepath.Join(p.sessionsDir, "002.json.tmp")
	sf := SessionFile{Version: 1, Run: Run{ID: "002", State: StateCompleted}}
	data, _ := json.Marshal(sf)
	os.WriteFile(tmpPath, data, 0o644)

	sessions, _ := p.Load()
	if len(sessions) != 1 {
		t.Errorf("expected 1 session (.tmp skipped), got %d", len(sessions))
	}
}

func TestPersistenceRehydrateRestoresCostTracker(t *testing.T) {
	p := tempPersistence(t)
	store := NewStore()

	now := time.Now()
	r := Run{
		ID:        "001",
		State:     StateCompleted,
		CreatedAt: now,
		SkillCosts: []cost.SkillCost{
			{SkillName: "spec", TotalTokens: 5000, CostUSD: 0.20, CompletedAt: now},
			{SkillName: "build", TotalTokens: 8000, CostUSD: 0.35, CompletedAt: now},
		},
	}
	if err := p.Save(r, nil, "", ""); err != nil {
		t.Fatalf("Save: %v", err)
	}

	var recorded []cost.SkillCost
	p.Rehydrate(store, RehydrateCallbacks{
		RecordCost: func(runID string, sc cost.SkillCost) {
			if runID != "001" {
				t.Errorf("expected runID 001, got %s", runID)
			}
			recorded = append(recorded, sc)
		},
	})

	if len(recorded) != 2 {
		t.Fatalf("expected 2 cost records, got %d", len(recorded))
	}
	if recorded[0].SkillName != "spec" {
		t.Errorf("expected first skill 'spec', got %s", recorded[0].SkillName)
	}
	if recorded[1].SkillName != "build" {
		t.Errorf("expected second skill 'build', got %s", recorded[1].SkillName)
	}
}

func TestStoreSetNextID(t *testing.T) {
	s := NewStore()

	s.SetNextID(10)
	id := s.Add(&Run{State: StateQueued})
	if id != "011" {
		t.Errorf("expected 011, got %s", id)
	}

	// SetNextID should not regress
	s.SetNextID(5)
	id2 := s.Add(&Run{State: StateQueued})
	if id2 != "012" {
		t.Errorf("expected 012 (no regression), got %s", id2)
	}
}
