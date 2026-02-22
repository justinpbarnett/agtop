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
