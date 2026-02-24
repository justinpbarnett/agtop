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

// MergeConflictError is returned when a merge fails due to non-golden file
// conflicts that require manual or AI-assisted resolution. The caller can
// type-assert this error to retrieve the list of conflicted files.
type MergeConflictError struct {
	Branch          string
	ConflictedFiles []string
	Output          string
}

func (e *MergeConflictError) Error() string {
	return fmt.Sprintf("merge %s: %d conflicted files: %s", e.Branch, len(e.ConflictedFiles), e.Output)
}

type WorktreeManager struct {
	repoRoot    string
	worktreeDir string
	repos       []string // git repo roots (len>1 = multi-repo mode)
	projectRoot string   // parent directory in multi-repo mode
	mu          sync.Mutex
}

// DiscoverRepos detects git repositories for a project directory.
// If projectRoot itself is a git repo, returns [projectRoot].
// Otherwise, scans immediate subdirectories for .git dirs.
func DiscoverRepos(projectRoot string) ([]string, error) {
	// Check if projectRoot itself is a git repo
	if _, err := os.Stat(filepath.Join(projectRoot, ".git")); err == nil {
		return []string{projectRoot}, nil
	}

	// Scan immediate subdirectories for git repos
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("scan for repos: %w", err)
	}

	var repos []string
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		subdir := filepath.Join(projectRoot, entry.Name())
		if _, err := os.Stat(filepath.Join(subdir, ".git")); err == nil {
			repos = append(repos, subdir)
		}
	}
	return repos, nil
}

// NewWorktreeManager creates a WorktreeManager for a single git repo.
func NewWorktreeManager(repoRoot string) *WorktreeManager {
	return &WorktreeManager{
		repoRoot:    repoRoot,
		worktreeDir: filepath.Join(repoRoot, ".agtop", "worktrees"),
		repos:       []string{repoRoot},
		projectRoot: repoRoot,
	}
}

// NewMultiRepoWorktreeManager creates a WorktreeManager for multiple git repos
// under a shared project root. In single-repo mode (len(repos)==1), behavior
// is identical to NewWorktreeManager.
func NewMultiRepoWorktreeManager(projectRoot string, repos []string) *WorktreeManager {
	if len(repos) == 1 {
		return NewWorktreeManager(repos[0])
	}
	return &WorktreeManager{
		repoRoot:    repos[0], // primary repo for backward compat
		worktreeDir: filepath.Join(projectRoot, ".agtop", "worktrees"),
		repos:       repos,
		projectRoot: projectRoot,
	}
}

// IsMultiRepo returns true if this manager handles multiple repositories.
func (w *WorktreeManager) IsMultiRepo() bool {
	return len(w.repos) > 1
}

// Repos returns the list of managed repository roots.
func (w *WorktreeManager) Repos() []string {
	return w.repos
}

// ProjectRoot returns the project root directory (parent of all repos in multi-repo mode).
func (w *WorktreeManager) ProjectRoot() string {
	return w.projectRoot
}

func (w *WorktreeManager) Create(runID string) (string, string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.IsMultiRepo() {
		return w.createSingle(runID)
	}
	return w.createMulti(runID)
}

// createSingle creates a worktree for a single-repo setup (original behavior).
func (w *WorktreeManager) createSingle(runID string) (string, string, error) {
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

// createMulti creates worktrees in each sub-repo under a composite directory
// that mirrors the original project structure.
func (w *WorktreeManager) createMulti(runID string) (string, string, error) {
	compositeRoot := filepath.Join(w.worktreeDir, runID)
	if err := os.MkdirAll(compositeRoot, 0o755); err != nil {
		return "", "", fmt.Errorf("create composite worktree dir: %w", err)
	}

	branch := "agtop/" + runID
	var created []string

	for _, repo := range w.repos {
		relPath, err := filepath.Rel(w.projectRoot, repo)
		if err != nil {
			w.rollbackCreate(created, runID)
			return "", "", fmt.Errorf("relative path for %s: %w", repo, err)
		}

		wtPath := filepath.Join(compositeRoot, relPath)
		if err := os.MkdirAll(filepath.Dir(wtPath), 0o755); err != nil {
			w.rollbackCreate(created, runID)
			return "", "", fmt.Errorf("create parent dir for %s: %w", relPath, err)
		}

		cmd := exec.Command("git", "worktree", "add", wtPath, "-b", branch)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			w.rollbackCreate(created, runID)
			return "", "", fmt.Errorf("git worktree add in %s: %s: %w", relPath, strings.TrimSpace(string(out)), err)
		}
		created = append(created, repo)
	}

	return compositeRoot, branch, nil
}

// rollbackCreate removes worktrees from repos that were successfully created
// during a failed multi-repo Create call.
func (w *WorktreeManager) rollbackCreate(createdRepos []string, runID string) {
	compositeRoot := filepath.Join(w.worktreeDir, runID)
	branch := "agtop/" + runID

	for _, repo := range createdRepos {
		relPath, _ := filepath.Rel(w.projectRoot, repo)
		wtPath := filepath.Join(compositeRoot, relPath)

		rm := exec.Command("git", "worktree", "remove", wtPath, "--force")
		rm.Dir = repo
		_ = rm.Run()

		br := exec.Command("git", "branch", "-D", branch)
		br.Dir = repo
		_ = br.Run()
	}
	os.RemoveAll(compositeRoot)
}

func (w *WorktreeManager) Remove(runID string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	branch := "agtop/" + runID

	if !w.IsMultiRepo() {
		wtPath := filepath.Join(w.worktreeDir, runID)

		cmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
		cmd.Dir = w.repoRoot
		_ = cmd.Run()

		branchCmd := exec.Command("git", "branch", "-D", branch)
		branchCmd.Dir = w.repoRoot
		_ = branchCmd.Run()

		return nil
	}

	// Multi-repo: remove worktree + branch in each repo
	compositeRoot := filepath.Join(w.worktreeDir, runID)
	for _, repo := range w.repos {
		relPath, _ := filepath.Rel(w.projectRoot, repo)
		wtPath := filepath.Join(compositeRoot, relPath)

		cmd := exec.Command("git", "worktree", "remove", wtPath, "--force")
		cmd.Dir = repo
		_ = cmd.Run()

		branchCmd := exec.Command("git", "branch", "-D", branch)
		branchCmd.Dir = repo
		_ = branchCmd.Run()
	}
	os.RemoveAll(compositeRoot)

	return nil
}

func (w *WorktreeManager) Merge(runID string) error {
	_, err := w.MergeWithOptions(runID, MergeOptions{})
	return err
}

func (w *WorktreeManager) MergeWithOptions(runID string, opts MergeOptions) (MergeResult, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.IsMultiRepo() {
		return w.mergeMulti(runID, opts)
	}
	return w.mergeSingle(runID, opts, w.repoRoot)
}

// mergeSingle performs the merge for a single repo (original behavior).
func (w *WorktreeManager) mergeSingle(runID string, opts MergeOptions, repoRoot string) (MergeResult, error) {
	var result MergeResult
	branch := "agtop/" + runID

	// Determine the worktree path
	var wtPath string
	if w.IsMultiRepo() {
		relPath, _ := filepath.Rel(w.projectRoot, repoRoot)
		wtPath = filepath.Join(w.worktreeDir, runID, relPath)
	} else {
		wtPath = filepath.Join(w.worktreeDir, runID)
	}

	if _, err := os.Stat(wtPath); err == nil {
		goldenResolved, err := w.rebaseOntoMainInRepo(wtPath, repoRoot)
		if err != nil {
			return result, fmt.Errorf("pre-merge rebase: %w", err)
		}
		result.GoldenFilesResolved = goldenResolved
	}

	stashed, stashErr := stashIfDirtyInDir(repoRoot)
	if stashErr != nil {
		return result, fmt.Errorf("pre-merge stash: %w", stashErr)
	}
	defer func() {
		if stashed {
			unstashInDir(repoRoot)
		}
	}()

	cmd := exec.Command("git", "merge", branch)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		if opts.GoldenUpdateCommand != "" && len(result.GoldenFilesResolved) > 0 {
			_ = runGoldenUpdateInDir(repoRoot, opts.GoldenUpdateCommand)
		}
		return result, nil
	}

	allConflicted, _ := conflictedFilesInDir(repoRoot)

	resolved, remaining, resolveErr := resolveGoldenConflictsInDir(repoRoot)
	if resolveErr != nil || len(resolved) == 0 {
		abort := exec.Command("git", "merge", "--abort")
		abort.Dir = repoRoot
		_ = abort.Run()

		if len(allConflicted) > 0 {
			return result, &MergeConflictError{
				Branch:          branch,
				ConflictedFiles: allConflicted,
				Output:          strings.TrimSpace(string(out)),
			}
		}
		return result, fmt.Errorf("merge %s: %s: %w", branch, strings.TrimSpace(string(out)), err)
	}

	if len(remaining) > 0 {
		abort := exec.Command("git", "merge", "--abort")
		abort.Dir = repoRoot
		_ = abort.Run()
		return result, &MergeConflictError{
			Branch:          branch,
			ConflictedFiles: remaining,
			Output:          strings.TrimSpace(string(out)),
		}
	}

	commit := exec.Command("git", "commit", "--no-edit")
	commit.Dir = repoRoot
	if commitOut, commitErr := commit.CombinedOutput(); commitErr != nil {
		abort := exec.Command("git", "merge", "--abort")
		abort.Dir = repoRoot
		_ = abort.Run()
		return result, fmt.Errorf("merge commit after golden resolution: %s: %w", strings.TrimSpace(string(commitOut)), commitErr)
	}

	result.GoldenFilesResolved = resolved

	if opts.GoldenUpdateCommand != "" {
		_ = runGoldenUpdateInDir(repoRoot, opts.GoldenUpdateCommand)
	}

	return result, nil
}

// mergeMulti merges across all repos with rollback on failure.
func (w *WorktreeManager) mergeMulti(runID string, opts MergeOptions) (MergeResult, error) {
	var combinedResult MergeResult
	var mergedRepos []string

	for _, repo := range w.repos {
		result, err := w.mergeSingle(runID, opts, repo)
		if err != nil {
			// Rollback: abort merge in repos that already merged
			w.rollbackMerge(mergedRepos, runID)
			relPath, _ := filepath.Rel(w.projectRoot, repo)
			return combinedResult, fmt.Errorf("merge in %s: %w", relPath, err)
		}
		combinedResult.GoldenFilesResolved = append(combinedResult.GoldenFilesResolved, result.GoldenFilesResolved...)
		mergedRepos = append(mergedRepos, repo)
	}

	return combinedResult, nil
}

// rollbackMerge reverts merges in repos that were successfully merged during
// a failed multi-repo merge. Uses git reset to undo the merge commit.
func (w *WorktreeManager) rollbackMerge(mergedRepos []string, runID string) {
	for _, repo := range mergedRepos {
		// Reset to the commit before the merge
		reset := exec.Command("git", "reset", "--hard", "HEAD~1")
		reset.Dir = repo
		_ = reset.Run()
	}
}

// stashIfDirtyInDir stashes uncommitted changes in the given directory.
func stashIfDirtyInDir(dir string) (bool, error) {
	status := exec.Command("git", "status", "--porcelain")
	status.Dir = dir
	out, err := status.Output()
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return false, nil
	}

	stash := exec.Command("git", "stash", "push", "-m", "agtop: auto-stash before merge")
	stash.Dir = dir
	if out, err := stash.CombinedOutput(); err != nil {
		return false, fmt.Errorf("git stash push: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return true, nil
}

// unstashInDir pops the most recent stash entry in the given directory.
func unstashInDir(dir string) {
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

// rebaseOntoMainInRepo rebases the worktree branch onto current main HEAD in
// the specified repo. Must be called with w.mu held.
func (w *WorktreeManager) rebaseOntoMainInRepo(wtPath string, repoRoot string) ([]string, error) {
	rebase := exec.Command("git", "rebase", "main")
	rebase.Dir = wtPath
	out, err := rebase.CombinedOutput()
	if err == nil {
		return nil, nil
	}

	// Capture conflicted files before any resolution attempts
	allConflicted, _ := conflictedFilesInDir(wtPath)

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

	// Return structured error if we know what files conflicted
	if len(allConflicted) > 0 {
		branch := ""
		// Extract branch name from worktree path
		head := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		head.Dir = wtPath
		if branchOut, err := head.Output(); err == nil {
			branch = strings.TrimSpace(string(branchOut))
		}
		return nil, &MergeConflictError{
			Branch:          branch,
			ConflictedFiles: allConflicted,
			Output:          strings.TrimSpace(string(out)),
		}
	}
	return nil, fmt.Errorf("rebase onto main: %s: %w", strings.TrimSpace(string(out)), err)
}

// runGoldenUpdateInDir runs a golden update command in the specified directory.
func runGoldenUpdateInDir(dir string, command string) error {
	cmd := exec.Command("sh", "-c", command)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("golden update command: %s: %w", strings.TrimSpace(string(out)), err)
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
		return fmt.Errorf("git add after golden update: %w", err)
	}

	amend := exec.Command("git", "commit", "--amend", "--no-edit")
	amend.Dir = dir
	if out, err := amend.CombinedOutput(); err != nil {
		return fmt.Errorf("amend after golden update: %s: %w", strings.TrimSpace(string(out)), err)
	}

	return nil
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
	return runGoldenUpdateInDir(w.repoRoot, command)
}

func (w *WorktreeManager) List() ([]WorktreeInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	var allFiltered []WorktreeInfo
	seen := make(map[string]bool) // deduplicate by branch in multi-repo mode

	for _, repo := range w.repos {
		cmd := exec.Command("git", "worktree", "list", "--porcelain")
		cmd.Dir = repo
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("git worktree list in %s: %w", repo, err)
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
				if current.Path != "" {
					if strings.HasPrefix(current.Path, w.worktreeDir) && !seen[current.Branch] {
						allFiltered = append(allFiltered, current)
						seen[current.Branch] = true
					}
					current = WorktreeInfo{}
				}
			}
		}
		if current.Path != "" && strings.HasPrefix(current.Path, w.worktreeDir) && !seen[current.Branch] {
			allFiltered = append(allFiltered, current)
			seen[current.Branch] = true
		}
	}

	return allFiltered, nil
}

func (w *WorktreeManager) RepoRoot() string {
	return w.repoRoot
}

func (w *WorktreeManager) Exists(runID string) bool {
	wtPath := filepath.Join(w.worktreeDir, runID)
	_, err := os.Stat(wtPath)
	return err == nil
}
