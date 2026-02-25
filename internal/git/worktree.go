package git

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/justinpbarnett/agtop/internal/config"
)

func resolveWorktreeDir(repoRoot, worktreePath string) string {
	if worktreePath == "" {
		return filepath.Join(repoRoot, ".agtop", "worktrees")
	}
	if strings.HasPrefix(worktreePath, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			worktreePath = filepath.Join(home, worktreePath[2:])
		}
	}
	if filepath.IsAbs(worktreePath) {
		return worktreePath
	}
	return filepath.Join(repoRoot, worktreePath)
}

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
	return NewWorktreeManagerAt(repoRoot, "")
}

func NewWorktreeManagerAt(repoRoot, worktreePath string) *WorktreeManager {
	return &WorktreeManager{
		repoRoot:    repoRoot,
		worktreeDir: resolveWorktreeDir(repoRoot, worktreePath),
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

	// Rebase worktree branch onto current main before merging
	wtPath := filepath.Join(w.worktreeDir, runID)
	if _, err := os.Stat(wtPath); err == nil {
		goldenResolved, err := w.rebaseOntoMain(wtPath)
		if err != nil {
			return result, fmt.Errorf("pre-merge rebase: %w", err)
		}
		result.GoldenFilesResolved = goldenResolved
	}

	// Stash any uncommitted changes so merge doesn't fail on dirty working tree
	stashed, stashErr := w.stashIfDirty()
	if stashErr != nil {
		return result, fmt.Errorf("pre-merge stash: %w", stashErr)
	}
	defer func() {
		if stashed {
			w.unstash()
		}
	}()

	// Now merge (should be fast-forward after rebase)
	cmd := exec.Command("git", "merge", branch)
	cmd.Dir = w.repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		// Run golden update command if golden files were resolved during rebase
		if opts.GoldenUpdateCommand != "" && len(result.GoldenFilesResolved) > 0 {
			_ = w.runGoldenUpdateLocked(opts.GoldenUpdateCommand)
		}
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

// stashIfDirty stashes uncommitted changes in the main repo if present.
// Returns true if changes were stashed. Must be called with w.mu held.
func (w *WorktreeManager) stashIfDirty() (bool, error) {
	status := exec.Command("git", "status", "--porcelain")
	status.Dir = w.repoRoot
	out, err := status.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, nil
	}

	stash := exec.Command("git", "stash", "push", "-m", "agtop: auto-stash before merge")
	stash.Dir = w.repoRoot
	if out, err := stash.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git stash push: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return true, nil
}

// unstash pops the most recent stash entry. If the pop conflicts (because
// merged files overlap with stashed changes), reset to the clean merge state
// and drop the stash — the accepted merge takes priority. Must be called with
// w.mu held.
func (w *WorktreeManager) unstash() {
	pop := exec.Command("git", "stash", "pop")
	pop.Dir = w.repoRoot
	if err := pop.Run(); err != nil {
		// Pop conflicted — restore to the clean merge state from HEAD
		reset := exec.Command("git", "checkout", "HEAD", "--", ".")
		reset.Dir = w.repoRoot
		_ = reset.Run()

		drop := exec.Command("git", "stash", "drop")
		drop.Dir = w.repoRoot
		_ = drop.Run()
	}
}

// rebaseOntoMain rebases the worktree branch onto current main HEAD.
// Returns the list of golden files that were auto-resolved during rebase.
// Must be called with w.mu held.
func (w *WorktreeManager) rebaseOntoMain(wtPath string) ([]string, error) {
	rebase := exec.Command("git", "rebase", "main")
	rebase.Dir = wtPath
	out, err := rebase.CombinedOutput()
	if err == nil {
		return nil, nil
	}

	// Rebase conflicted — try to resolve golden files
	resolved, remaining, _ := resolveGoldenConflictsInDir(wtPath)
	if len(resolved) > 0 && len(remaining) == 0 {
		// All conflicts were golden — continue rebase
		cont := exec.Command("git", "rebase", "--continue")
		cont.Dir = wtPath
		cont.Env = append(cont.Environ(), "GIT_EDITOR=true")
		if contErr := cont.Run(); contErr == nil {
			return resolved, nil
		}
	}

	// Non-golden conflicts or resolution failed — abort
	abort := exec.Command("git", "rebase", "--abort")
	abort.Dir = wtPath
	_ = abort.Run()
	return nil, fmt.Errorf("rebase onto main: %s: %w", strings.TrimSpace(string(out)), err)
}

// resolveGoldenConflicts detects conflicted files in the working tree and
// auto-resolves any .golden test snapshot files by taking the incoming version.
// Returns the list of resolved golden files and the list of remaining unresolved files.
func (w *WorktreeManager) resolveGoldenConflicts() (resolved, remaining []string, err error) {
	return resolveGoldenConflictsInDir(w.repoRoot)
}

// resolveGoldenConflictsInDir detects conflicted files in the given directory and
// auto-resolves any .golden test snapshot files by taking the incoming version.
func resolveGoldenConflictsInDir(dir string) (resolved, remaining []string, err error) {
	conflicted, err := conflictedFilesInDir(dir)
	if err != nil {
		return nil, nil, err
	}
	if len(conflicted) == 0 {
		return nil, nil, nil
	}
	resolved, remaining = ResolveGoldenConflictsFromList(dir, conflicted)
	return resolved, remaining, nil
}

// conflictedFilesInDir returns the list of files with unresolved conflicts in the given directory.
func conflictedFilesInDir(dir string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
	cmd.Dir = dir
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

// ResolveGoldenConflictsFromList auto-resolves golden files in the conflict
// list by checking out the incoming branch version. Non-golden files and any
// golden files that fail to resolve are returned in remaining.
func ResolveGoldenConflictsFromList(dir string, files []string) (resolved, remaining []string) {
	for _, f := range files {
		if !IsGoldenFile(f) {
			remaining = append(remaining, f)
			continue
		}
		checkout := exec.Command("git", "checkout", "--theirs", f)
		checkout.Dir = dir
		if err := checkout.Run(); err != nil {
			remaining = append(remaining, f)
			continue
		}
		add := exec.Command("git", "add", f)
		add.Dir = dir
		if err := add.Run(); err != nil {
			remaining = append(remaining, f)
			continue
		}
		resolved = append(resolved, f)
	}
	return
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

	// Filter to only agtop-managed worktrees.
	// Resolve symlinks so paths from "git worktree list" (which may return
	// canonical paths like /private/var/... on macOS) match w.worktreeDir.
	resolvedWorktreeDir := w.worktreeDir
	if r, err := filepath.EvalSymlinks(w.worktreeDir); err == nil {
		resolvedWorktreeDir = r
	}
	var filtered []WorktreeInfo
	for _, wt := range all {
		if strings.HasPrefix(wt.Path, w.worktreeDir) || strings.HasPrefix(wt.Path, resolvedWorktreeDir) {
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

// --- Multi-repo support ---

type SubRepoWorktree struct {
	Name     string
	Path     string
	Branch   string
	RepoRoot string
}

type MultiWorktreeResult struct {
	RootPath     string
	Branch       string
	SubWorktrees []SubRepoWorktree
}

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	return cmd.Run() == nil
}

func (w *WorktreeManager) CreateMulti(runID string, repos []config.RepoConfig) (*MultiWorktreeResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := os.MkdirAll(w.worktreeDir, 0o755); err != nil {
		return nil, fmt.Errorf("create worktree dir: %w", err)
	}

	branch := "agtop/" + runID
	rootPath := filepath.Join(w.worktreeDir, runID)

	var created []SubRepoWorktree
	for _, repo := range repos {
		subGitRoot := filepath.Join(w.repoRoot, repo.Path)
		if !isGitRepo(subGitRoot) {
			w.rollbackMulti(created)
			return nil, fmt.Errorf("sub-repo %q at %s is not a git repository", repo.Name, subGitRoot)
		}

		wtPath := filepath.Join(rootPath, repo.Path)
		if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
			w.rollbackMulti(created)
			return nil, fmt.Errorf("create parent dir for %s: %w", repo.Name, err)
		}

		cmd := exec.Command("git", "worktree", "add", wtPath, "-b", branch)
		cmd.Dir = subGitRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			w.rollbackMulti(created)
			return nil, fmt.Errorf("git worktree add for %s: %s: %w", repo.Name, strings.TrimSpace(string(out)), err)
		}

		created = append(created, SubRepoWorktree{
			Name:     repo.Name,
			Path:     wtPath,
			Branch:   branch,
			RepoRoot: subGitRoot,
		})
	}

	return &MultiWorktreeResult{
		RootPath:     rootPath,
		Branch:       branch,
		SubWorktrees: created,
	}, nil
}

func (w *WorktreeManager) rollbackMulti(created []SubRepoWorktree) {
	for _, sub := range created {
		rm := exec.Command("git", "worktree", "remove", sub.Path, "--force")
		rm.Dir = sub.RepoRoot
		_ = rm.Run()

		br := exec.Command("git", "branch", "-D", sub.Branch)
		br.Dir = sub.RepoRoot
		_ = br.Run()
	}
}

func (w *WorktreeManager) RemoveMulti(runID string, repos []config.RepoConfig) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	branch := "agtop/" + runID
	rootPath := filepath.Join(w.worktreeDir, runID)

	for _, repo := range repos {
		subGitRoot := filepath.Join(w.repoRoot, repo.Path)
		wtPath := filepath.Join(rootPath, repo.Path)

		rm := exec.Command("git", "worktree", "remove", wtPath, "--force")
		rm.Dir = subGitRoot
		_ = rm.Run()

		br := exec.Command("git", "branch", "-D", branch)
		br.Dir = subGitRoot
		_ = br.Run()
	}

	_ = os.RemoveAll(rootPath)
	return nil
}

func (w *WorktreeManager) MergeMulti(runID string, repos []config.RepoConfig, opts MergeOptions) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	branch := "agtop/" + runID
	rootPath := filepath.Join(w.worktreeDir, runID)
	var mergeErrors []string

	for _, repo := range repos {
		subGitRoot := filepath.Join(w.repoRoot, repo.Path)
		wtPath := filepath.Join(rootPath, repo.Path)

		if _, err := os.Stat(wtPath); err == nil {
			rebase := exec.Command("git", "rebase", "main")
			rebase.Dir = wtPath
			if out, err := rebase.CombinedOutput(); err != nil {
				abort := exec.Command("git", "rebase", "--abort")
				abort.Dir = wtPath
				_ = abort.Run()
				mergeErrors = append(mergeErrors, fmt.Sprintf("%s: rebase failed: %s", repo.Name, strings.TrimSpace(string(out))))
				continue
			}
		}

		stashed := stashIfDirtyIn(subGitRoot)

		cmd := exec.Command("git", "merge", branch)
		cmd.Dir = subGitRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			abort := exec.Command("git", "merge", "--abort")
			abort.Dir = subGitRoot
			_ = abort.Run()
			if stashed {
				unstashIn(subGitRoot)
			}
			mergeErrors = append(mergeErrors, fmt.Sprintf("%s: merge failed: %s", repo.Name, strings.TrimSpace(string(out))))
			continue
		}

		if opts.GoldenUpdateCommand != "" {
			_ = runGoldenUpdateIn(subGitRoot, opts.GoldenUpdateCommand)
		}

		if stashed {
			unstashIn(subGitRoot)
		}
	}

	if len(mergeErrors) > 0 {
		return fmt.Errorf("multi-repo merge errors:\n  %s", strings.Join(mergeErrors, "\n  "))
	}
	return nil
}

func stashIfDirtyIn(dir string) bool {
	status := exec.Command("git", "status", "--porcelain")
	status.Dir = dir
	out, err := status.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return false
	}
	stash := exec.Command("git", "stash", "push", "-m", "agtop: auto-stash before merge")
	stash.Dir = dir
	return stash.Run() == nil
}

func unstashIn(dir string) {
	pop := exec.Command("git", "stash", "pop")
	pop.Dir = dir
	if err := pop.Run(); err != nil {
		reset := exec.Command("git", "checkout", "HEAD", "--", ".")
		reset.Dir = dir
		_ = reset.Run()

		drop := exec.Command("git", "stash", "drop")
		drop.Dir = dir
		_ = drop.Run()
	}
}

func runGoldenUpdateIn(dir, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	if _, err := cmd.CombinedOutput(); err != nil {
		return err
	}

	status := exec.Command("git", "status", "--porcelain")
	status.Dir = dir
	out, err := status.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return nil
	}

	add := exec.Command("git", "add", "-A")
	add.Dir = dir
	if _, err := add.CombinedOutput(); err != nil {
		return err
	}

	amend := exec.Command("git", "commit", "--amend", "--no-edit")
	amend.Dir = dir
	_, err = amend.CombinedOutput()
	return err
}

func (w *WorktreeManager) ListMulti(repos []config.RepoConfig) ([]WorktreeInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	seen := make(map[string]bool)
	var result []WorktreeInfo

	for _, repo := range repos {
		subGitRoot := filepath.Join(w.repoRoot, repo.Path)

		cmd := exec.Command("git", "worktree", "list", "--porcelain")
		cmd.Dir = subGitRoot
		out, err := cmd.Output()
		if err != nil {
			continue
		}

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
				if current.Path != "" && strings.HasPrefix(current.Path, w.worktreeDir) {
					if !seen[current.Branch] {
						seen[current.Branch] = true
						result = append(result, current)
					}
				}
				current = WorktreeInfo{}
			}
		}
		if current.Path != "" && strings.HasPrefix(current.Path, w.worktreeDir) {
			if !seen[current.Branch] {
				seen[current.Branch] = true
				result = append(result, current)
			}
		}
	}

	return result, nil
}

func (w *WorktreeManager) ExistsMulti(runID string, repos []config.RepoConfig) bool {
	rootPath := filepath.Join(w.worktreeDir, runID)
	for _, repo := range repos {
		wtPath := filepath.Join(rootPath, repo.Path)
		if _, err := os.Stat(wtPath); err != nil {
			return false
		}
	}
	return true
}
