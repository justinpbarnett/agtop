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

func (d *DiffGenerator) Diff(branch string) (string, error) {
	cmd := exec.Command("git", "diff", "main..."+branch)
	cmd.Dir = d.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff: %w", err)
	}
	return string(out), nil
}

func (d *DiffGenerator) DiffStat(branch string) (string, error) {
	cmd := exec.Command("git", "diff", "--stat", "main..."+branch)
	cmd.Dir = d.repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git diff --stat: %w", err)
	}
	return string(out), nil
}
