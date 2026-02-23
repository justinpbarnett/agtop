package git

import (
	"fmt"
	"os/exec"
)

type DiffGenerator struct {
	repoRoot string
}

func NewDiffGenerator(repoRoot string) *DiffGenerator {
	return &DiffGenerator{repoRoot: repoRoot}
}

func (d *DiffGenerator) Diff(worktreeDir string) (string, error) {
	cmd := exec.Command("git", "diff", "--color=never", "main")
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func (d *DiffGenerator) DiffStat(worktreeDir string) (string, error) {
	cmd := exec.Command("git", "diff", "--color=never", "--stat", "main")
	cmd.Dir = worktreeDir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}
	return string(out), nil
}
