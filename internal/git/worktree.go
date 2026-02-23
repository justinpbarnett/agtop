package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

type WorktreeInfo struct {
	Path   string
	Branch string
	HEAD   string
}

type MergeOptions struct {
	GoldenUpdateCommand string
}

type MergeResult struct {
	GoldenFilesResolved []string
}

type WorktreeManager struct {
	repoRoot    string
	worktreeDir string
	mu          sync.Mutex
}

func NewWorktreeManager(repoRoot string) *WorktreeManager {
	return &WorktreeManager{
		repoRoot:    repoRoot,
		worktreeDir: filepath.Join(repoRoot, ".agtop", "worktrees"),
	}
}

func (w *WorktreeManager) Create(runID string) (string, string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(w.worktreeDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create worktree dir: %w", err)
	}

	branch := "agtop/" + runID
	wtPath := filepath.Join(w.worktreeDir, runID)

	cmd := exec.Command("git", "worktree", "add", wtPath, "-b", branch)
	cmd.Dir = w.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git worktree add: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return wtPath, branch, nil
}

func (w *WorktreeManager) Remove(runID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	wtPath := filepath.Join(w.worktreeDir, runID)

	cmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
	cmd.Dir = w.repoRoot
	_ = cmd.Run() // ignore error if already removed

	branch := "agtop/" + runID
	branchCmd := exec.Command("git", "branch", "-D", branch)
	branchCmd.Dir = w.repoRoot
	_ = branchCmd.Run() // ignore error if branch already gone

	return nil
}

func (w *WorktreeManager) Merge(runID string) error {
	_, err := w.MergeWithOptions(runID, MergeOptions{})
	return err
}

func (w *WorktreeManager) MergeWithOptions(runID string, opts MergeOptions) (MergeResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var result MergeResult
	branch := "agtop/" + runID
	cmd := exec.Command("git", "merge", branch)
	cmd.Dir = w.repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return result, nil
	}

	// Merge failed — check for golden file conflicts we can auto-resolve
	resolved, remaining, resolveErr := w.resolveGoldenConflicts()
	if resolveErr != nil || len(resolved) == 0 {
		// No golden files resolved or error — abort as before
		abort := exec.Command("git", "merge", "--abort")
		abort.Dir = w.repoRoot
		_ = abort.Run()
		return result, fmt.Errorf("merge %s: %s: %w", branch, strings.TrimSpace(string(out)), err)
	}

	if len(remaining) > 0 {
		// Non-golden conflicts remain — abort
		abort := exec.Command("git", "merge", "--abort")
		abort.Dir = w.repoRoot
		_ = abort.Run()
		return result, fmt.Errorf("merge %s: %s: %w", branch, strings.TrimSpace(string(out)), err)
	}

	// All conflicts were golden files — complete the merge
	commit := exec.Command("git", "commit", "--no-edit")
	commit.Dir = w.repoRoot
	if commitOut, commitErr := commit.CombinedOutput(); commitErr != nil {
		abort := exec.Command("git", "merge", "--abort")
		abort.Dir = w.repoRoot
		_ = abort.Run()
		return result, fmt.Errorf("merge commit after golden resolution: %s: %w", strings.TrimSpace(string(commitOut)), commitErr)
	}

	result.GoldenFilesResolved = resolved

	// Run golden update command if configured and golden files were resolved
	if opts.GoldenUpdateCommand != "" {
		_ = w.runGoldenUpdateLocked(opts.GoldenUpdateCommand)
	}

	return result, nil
}

// resolveGoldenConflicts detects conflicted files in the working tree and
// auto-resolves any .golden test snapshot files by taking the incoming version.
// Returns the list of resolved golden files and the list of remaining unresolved files.
func (w *WorktreeManager) resolveGoldenConflicts() (resolved, remaining []string, err error) {
	conflicted, err := w.conflictedFiles()
	if err != nil {
		return nil, nil, err
	}

	var golden, other []string
	for _, f := range conflicted {
		if IsGoldenFile(f) {
			golden = append(golden, f)
		} else {
			other = append(other, f)
		}
	}

	if len(golden) == 0 {
		return nil, conflicted, nil
	}

	// Auto-resolve golden files by taking the incoming branch version
	for _, f := range golden {
		checkout := exec.Command("git", "checkout", "--theirs", f)
		checkout.Dir = w.repoRoot
		if out, err := checkout.CombinedOutput(); err != nil {
			return nil, conflicted, fmt.Errorf("checkout --theirs %s: %s: %w", f, strings.TrimSpace(string(out)), err)
		}
		add := exec.Command("git", "add", f)
		add.Dir = w.repoRoot
		if out, err := add.CombinedOutput(); err != nil {
			return nil, conflicted, fmt.Errorf("git add %s: %s: %w", f, strings.TrimSpace(string(out)), err)
		}
	}

	return golden, other, nil
}

// conflictedFiles returns the list of files with unresolved merge conflicts.
func (w *WorktreeManager) conflictedFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = w.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --diff-filter=U: %w", err)
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// IsGoldenFile returns true if the file path matches the golden test snapshot pattern.
func IsGoldenFile(path string) bool {
	if !strings.HasSuffix(path, ".golden") {
		return false
	}
	return strings.Contains(path, "/testdata/") || strings.HasPrefix(path, "testdata/")
}

// RunGoldenUpdate runs a command to regenerate golden test files, stages any
// changes, and amends the last commit if there are modifications.
func (w *WorktreeManager) RunGoldenUpdate(command string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.runGoldenUpdateLocked(command)
}

func (w *WorktreeManager) runGoldenUpdateLocked(command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = w.repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("golden update command: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Check if golden update produced changes
	status := exec.Command("git", "status", "--porcelain")
	status.Dir = w.repoRoot
	out, err := status.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return nil // no changes
	}

	// Stage and amend
	add := exec.Command("git", "add", "-A")
	add.Dir = w.repoRoot
	if _, err := add.CombinedOutput(); err != nil {
		return fmt.Errorf("git add after golden update: %w", err)
	}

	amend := exec.Command("git", "commit", "--amend", "--no-edit")
	amend.Dir = w.repoRoot
	if out, err := amend.CombinedOutput(); err != nil {
		return fmt.Errorf("amend after golden update: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
}

func (w *WorktreeManager) List() ([]WorktreeInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = w.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list: %w", err)
	}

	var all []WorktreeInfo
	var current WorktreeInfo

	scanner := bufio.NewScanner(strings.NewReader(string(out)))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "worktree "):
			current = WorktreeInfo{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			current.HEAD = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "":
			if current.Path != "" {
				all = append(all, current)
				current = WorktreeInfo{}
			}
		}
	}
	if current.Path != "" {
		all = append(all, current)
	}

	// Filter to only agtop-managed worktrees
	var filtered []WorktreeInfo
	for _, wt := range all {
		if strings.HasPrefix(wt.Path, w.worktreeDir) {
			filtered = append(filtered, wt)
		}
	}
	return filtered, nil
}

func (w *WorktreeManager) Exists(runID string) bool {
	wtPath := filepath.Join(w.worktreeDir, runID)
	_, err := os.Stat(wtPath)
	return err == nil
}
