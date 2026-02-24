package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type DiffGenerator struct {
	repoRoot    string
	repos       []string
	projectRoot string
}

func NewDiffGenerator(repoRoot string) *DiffGenerator {
	return &DiffGenerator{
		repoRoot:    repoRoot,
		repos:       []string{repoRoot},
		projectRoot: repoRoot,
	}
}

// NewMultiRepoDiffGenerator creates a DiffGenerator for multiple repos.
func NewMultiRepoDiffGenerator(projectRoot string, repos []string) *DiffGenerator {
	if len(repos) == 1 {
		return NewDiffGenerator(repos[0])
	}
	return &DiffGenerator{
		repoRoot:    repos[0],
		repos:       repos,
		projectRoot: projectRoot,
	}
}

func (d *DiffGenerator) mergeBase(worktreeDir string) string {
	cmd := exec.Command("git", "merge-base", "HEAD", "main")
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "main"
	}
	return strings.TrimSpace(string(out))
}

func (d *DiffGenerator) Diff(worktreeDir string) (string, error) {
	if len(d.repos) <= 1 {
		return d.diffSingleDir(worktreeDir)
	}
	return d.diffMulti(worktreeDir)
}

func (d *DiffGenerator) DiffStat(worktreeDir string) (string, error) {
	if len(d.repos) <= 1 {
		return d.diffStatSingleDir(worktreeDir)
	}
	return d.diffStatMulti(worktreeDir)
}

func (d *DiffGenerator) diffSingleDir(worktreeDir string) (string, error) {
	base := d.mergeBase(worktreeDir)
	cmd := exec.Command("git", "diff", "--color=never", base)
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func (d *DiffGenerator) diffStatSingleDir(worktreeDir string) (string, error) {
	base := d.mergeBase(worktreeDir)
	cmd := exec.Command("git", "diff", "--color=never", "--stat", base)
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}
	return string(out), nil
}

// diffMulti generates diffs across all sub-repo worktrees with headers.
func (d *DiffGenerator) diffMulti(compositeWorktreeDir string) (string, error) {
	var b strings.Builder
	for _, repo := range d.repos {
		relPath, _ := filepath.Rel(d.projectRoot, repo)
		subWt := filepath.Join(compositeWorktreeDir, relPath)

		diff, err := d.diffSingleDir(subWt)
		if err != nil {
			continue // skip repos with errors
		}
		if diff == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("=== %s ===\n", relPath))
		b.WriteString(diff)
		b.WriteString("\n")
	}
	return b.String(), nil
}

// diffStatMulti generates diff stats across all sub-repo worktrees.
func (d *DiffGenerator) diffStatMulti(compositeWorktreeDir string) (string, error) {
	var b strings.Builder
	for _, repo := range d.repos {
		relPath, _ := filepath.Rel(d.projectRoot, repo)
		subWt := filepath.Join(compositeWorktreeDir, relPath)

		stat, err := d.diffStatSingleDir(subWt)
		if err != nil {
			continue
		}
		if stat == "" {
			continue
		}
		b.WriteString(fmt.Sprintf("=== %s ===\n", relPath))
		b.WriteString(stat)
		b.WriteString("\n")
	}
	return b.String(), nil
}
