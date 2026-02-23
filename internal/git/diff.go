package git

import (
	"fmt"
	"os/exec"
	"strings"
)

type DiffGenerator struct {
	repoRoot string
}

func NewDiffGenerator(repoRoot string) *DiffGenerator {
	return &DiffGenerator{repoRoot: repoRoot}
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
	base := d.mergeBase(worktreeDir)
	cmd := exec.Command("git", "diff", "--color=never", base)
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func (d *DiffGenerator) DiffStat(worktreeDir string) (string, error) {
	base := d.mergeBase(worktreeDir)
	cmd := exec.Command("git", "diff", "--color=never", "--stat", base)
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}
	return string(out), nil
}
