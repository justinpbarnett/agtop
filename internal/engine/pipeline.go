package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	gitpkg "github.com/justinpbarnett/agtop/internal/git"
	"github.com/justinpbarnett/agtop/internal/run"
)

// Pipeline orchestrates the accept-to-merge flow: rebase, resolve conflicts,
// push, create PR, poll checks, fix failures, and merge.
type Pipeline struct {
	executor *Executor
	store    *run.Store
	cfg      *config.MergeConfig
	repoRoot string
}

func NewPipeline(executor *Executor, store *run.Store, cfg *config.MergeConfig, repoRoot string) *Pipeline {
	return &Pipeline{
		executor: executor,
		store:    store,
		cfg:      cfg,
		repoRoot: repoRoot,
	}
}

// Run executes the full merge pipeline for a run. It is intended to be called
// in a goroutine. On success the run transitions to StateAccepted; on failure
// the run transitions to StateFailed with an error message.
func (p *Pipeline) Run(ctx context.Context, runID string) {
	r, ok := p.store.Get(runID)
	if !ok {
		return
	}

	worktree := r.Worktree
	branch := r.Branch

	target, err := p.resolveTarget(worktree)
	if err != nil {
		p.fail(runID, fmt.Sprintf("resolve target branch: %v", err))
		return
	}

	// Stage 1: Rebase
	p.setMergeStatus(runID, "rebasing")
	if err := p.rebase(ctx, runID, worktree, target); err != nil {
		p.fail(runID, fmt.Sprintf("rebase failed: %v", err))
		return
	}

	// Stage 2: Push
	p.setMergeStatus(runID, "pushing")
	if err := p.push(worktree, branch); err != nil {
		p.fail(runID, fmt.Sprintf("push failed: %v", err))
		return
	}

	// Stage 3: Create PR (skip if one already exists from a previous attempt)
	prURL := r.PRURL
	if prURL == "" {
		p.setMergeStatus(runID, "pr-created")
		prURL, err = p.createPR(worktree, branch, target, r.Prompt)
		if err != nil {
			p.fail(runID, fmt.Sprintf("create PR failed: %v", err))
			return
		}
		p.store.Update(runID, func(r *run.Run) {
			r.PRURL = prURL
		})
	} else {
		p.setMergeStatus(runID, "pr-created")
	}

	// Stage 4: Poll checks and fix failures
	fixAttempts := p.cfg.FixAttempts
	if fixAttempts <= 0 {
		fixAttempts = 3
	}

	for attempt := 0; attempt <= fixAttempts; attempt++ {
		p.setMergeStatus(runID, "checks-pending")
		passed, failures, err := p.pollChecks(ctx, worktree, branch)
		if err != nil {
			p.fail(runID, fmt.Sprintf("poll checks: %v", err))
			return
		}

		if passed {
			break
		}

		if attempt == fixAttempts {
			p.fail(runID, fmt.Sprintf("checks still failing after %d fix attempts: %s", fixAttempts, failures))
			return
		}

		// Fix failures
		p.setMergeStatus(runID, "fixing")
		if err := p.fixFailures(ctx, runID, worktree, failures); err != nil {
			p.fail(runID, fmt.Sprintf("fix attempt %d failed: %v", attempt+1, err))
			return
		}

		// Push fixes
		p.setMergeStatus(runID, "pushing")
		if err := p.push(worktree, branch); err != nil {
			p.fail(runID, fmt.Sprintf("push after fix failed: %v", err))
			return
		}
	}

	// Stage 5: Merge
	p.setMergeStatus(runID, "merging")
	if err := p.merge(worktree, prURL); err != nil {
		p.fail(runID, fmt.Sprintf("merge failed: %v", err))
		return
	}

	// Success
	p.store.Update(runID, func(r *run.Run) {
		r.State = run.StateAccepted
		r.MergeStatus = "merged"
		r.CompletedAt = time.Now()
	})
}

func (p *Pipeline) resolveTarget(worktree string) (string, error) {
	if p.cfg.TargetBranch != "" {
		return p.cfg.TargetBranch, nil
	}

	// Detect default branch from remote
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = worktree
	out, err := cmd.Output()
	if err != nil {
		// Fallback: try common default branch names
		for _, name := range []string{"main", "master"} {
			check := exec.Command("git", "rev-parse", "--verify", "origin/"+name)
			check.Dir = worktree
			if check.Run() == nil {
				return name, nil
			}
		}
		return "", fmt.Errorf("cannot detect default branch: %v", err)
	}

	ref := strings.TrimSpace(string(out))
	// refs/remotes/origin/main → main
	parts := strings.Split(ref, "/")
	return parts[len(parts)-1], nil
}

func (p *Pipeline) rebase(ctx context.Context, runID, worktree, target string) error {
	// Fetch latest
	fetch := exec.Command("git", "fetch", "origin", target)
	fetch.Dir = worktree
	if out, err := fetch.CombinedOutput(); err != nil {
		return fmt.Errorf("fetch: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Attempt rebase
	rebase := exec.Command("git", "rebase", "origin/"+target)
	rebase.Dir = worktree
	if out, err := rebase.CombinedOutput(); err != nil {
		output := string(out)
		if !strings.Contains(output, "CONFLICT") && !strings.Contains(output, "conflict") {
			// Not a conflict — abort and return error
			abort := exec.Command("git", "rebase", "--abort")
			abort.Dir = worktree
			_ = abort.Run()
			return fmt.Errorf("rebase: %s: %w", strings.TrimSpace(output), err)
		}

		// Conflict detected — resolve with agent
		if err := p.resolveConflicts(ctx, runID, worktree); err != nil {
			abort := exec.Command("git", "rebase", "--abort")
			abort.Dir = worktree
			_ = abort.Run()
			return fmt.Errorf("conflict resolution: %w", err)
		}
	}

	return nil
}

func (p *Pipeline) resolveConflicts(ctx context.Context, runID, worktree string) error {
	p.setMergeStatus(runID, "resolving-conflicts")

	for attempt := 0; attempt < 3; attempt++ {
		// Get list of conflicted files
		cmd := exec.Command("git", "diff", "--name-only", "--diff-filter=U")
		cmd.Dir = worktree
		out, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(out)) == "" {
			// No conflicts remaining — try to continue
			cont := exec.Command("git", "rebase", "--continue")
			cont.Dir = worktree
			cont.Env = append(cont.Environ(), "GIT_EDITOR=true")
			if err := cont.Run(); err == nil {
				return nil
			}
			// May need another round
			continue
		}

		// Auto-resolve golden test snapshot files (binary — agents can't resolve them)
		files := strings.Split(strings.TrimSpace(string(out)), "\n")
		var nonGolden []string
		for _, f := range files {
			if gitpkg.IsGoldenFile(f) {
				checkout := exec.Command("git", "checkout", "--theirs", f)
				checkout.Dir = worktree
				if err := checkout.Run(); err != nil {
					nonGolden = append(nonGolden, f)
					continue
				}
				add := exec.Command("git", "add", f)
				add.Dir = worktree
				if err := add.Run(); err != nil {
					nonGolden = append(nonGolden, f)
					continue
				}
			} else {
				nonGolden = append(nonGolden, f)
			}
		}

		// If only golden files conflicted, continue rebase without invoking agent
		if len(nonGolden) == 0 {
			cont := exec.Command("git", "rebase", "--continue")
			cont.Dir = worktree
			cont.Env = append(cont.Environ(), "GIT_EDITOR=true")
			if err := cont.Run(); err == nil {
				p.runGoldenUpdate(worktree)
				return nil
			}
			continue
		}

		conflictedFiles := strings.Join(nonGolden, "\n")

		// Invoke the build skill to resolve remaining (non-golden) conflicts
		skill, opts, ok := p.executor.registry.SkillForRun("build")
		if !ok {
			return fmt.Errorf("build skill not found for conflict resolution")
		}
		opts.WorkDir = worktree

		r, _ := p.store.Get(runID)
		prompt := BuildPrompt(skill, PromptContext{
			WorkDir: worktree,
			Branch:  r.Branch,
			UserPrompt: fmt.Sprintf(
				"The following files have merge conflicts that need to be resolved:\n\n%s\n\n"+
					"Open each conflicted file, resolve the conflict markers (<<<<<<< ======= >>>>>>>), "+
					"keeping the correct code for both sides. After resolving, stage the files with git add.",
				conflictedFiles,
			),
		})

		_, err = p.executor.runSkill(ctx, runID, prompt, opts, skill.Timeout)
		if err != nil {
			return fmt.Errorf("agent conflict resolution (attempt %d): %w", attempt+1, err)
		}

		// Stage resolved files and continue rebase
		add := exec.Command("git", "add", "-A")
		add.Dir = worktree
		_ = add.Run()

		cont := exec.Command("git", "rebase", "--continue")
		cont.Dir = worktree
		cont.Env = append(cont.Environ(), "GIT_EDITOR=true")
		if err := cont.Run(); err == nil {
			return nil
		}
		// More conflicts from next commit — loop again
	}

	return fmt.Errorf("could not resolve conflicts after 3 attempts")
}

// runGoldenUpdate runs the configured golden update command in the given directory
// after a golden-only conflict resolution. If it produces changes, they are committed.
func (p *Pipeline) runGoldenUpdate(worktree string) {
	if p.cfg.GoldenUpdateCommand == "" {
		return
	}

	cmd := exec.Command("sh", "-c", p.cfg.GoldenUpdateCommand)
	cmd.Dir = worktree
	if _, err := cmd.CombinedOutput(); err != nil {
		log.Printf("warning: golden update command failed: %v", err)
		return
	}

	status := exec.Command("git", "status", "--porcelain")
	status.Dir = worktree
	out, err := status.Output()
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return
	}

	add := exec.Command("git", "add", "-A")
	add.Dir = worktree
	if err := add.Run(); err != nil {
		log.Printf("warning: git add after golden update: %v", err)
		return
	}

	commit := exec.Command("git", "commit", "-m", "chore: regenerate golden test files")
	commit.Dir = worktree
	if err := commit.Run(); err != nil {
		log.Printf("warning: commit after golden update: %v", err)
	}
}

func (p *Pipeline) push(worktree, branch string) error {
	cmd := exec.Command("git", "push", "origin", branch, "--force-with-lease")
	cmd.Dir = worktree
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (p *Pipeline) createPR(worktree, branch, target, prompt string) (string, error) {
	// Build a short title from the prompt
	title := prompt
	if len(title) > 65 {
		title = title[:62] + "..."
	}
	// Remove newlines for the title
	title = strings.ReplaceAll(title, "\n", " ")

	body := fmt.Sprintf("## Summary\n\n%s\n\n---\nCreated by agtop", prompt)

	cmd := exec.Command("gh", "pr", "create",
		"--base", target,
		"--head", branch,
		"--title", title,
		"--body", body,
	)
	cmd.Dir = worktree
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}

	prURL := strings.TrimSpace(string(out))
	return prURL, nil
}

func (p *Pipeline) pollChecks(ctx context.Context, worktree, branch string) (passed bool, failures string, err error) {
	pollInterval := p.cfg.PollInterval
	if pollInterval <= 0 {
		pollInterval = 30
	}
	pollTimeout := p.cfg.PollTimeout
	if pollTimeout <= 0 {
		pollTimeout = 600
	}

	deadline := time.After(time.Duration(pollTimeout) * time.Second)
	ticker := time.NewTicker(time.Duration(pollInterval) * time.Second)
	defer ticker.Stop()

	// Initial wait before first poll
	select {
	case <-ctx.Done():
		return false, "", ctx.Err()
	case <-time.After(10 * time.Second):
	}

	for {
		cmd := exec.Command("gh", "pr", "checks", branch, "--json", "name,state,conclusion")
		cmd.Dir = worktree
		out, err := cmd.Output()
		if err != nil {
			// If no checks exist, consider it passed
			errOut := string(out)
			if strings.Contains(errOut, "no checks") || strings.Contains(err.Error(), "exit status 1") {
				return true, "", nil
			}
			return false, "", fmt.Errorf("gh pr checks: %w", err)
		}

		allPassed, pending, failed := parseCheckResults(string(out))
		if allPassed {
			return true, "", nil
		}
		if len(failed) > 0 {
			return false, strings.Join(failed, ", "), nil
		}

		// Still pending
		_ = pending

		select {
		case <-ctx.Done():
			return false, "", ctx.Err()
		case <-deadline:
			return false, "timeout waiting for checks", fmt.Errorf("checks did not complete within %ds", pollTimeout)
		case <-ticker.C:
			continue
		}
	}
}

// parseCheckResults parses gh pr checks JSON output.
// Returns whether all passed, names of pending checks, and names of failed checks.
func parseCheckResults(output string) (allPassed bool, pending []string, failed []string) {
	type check struct {
		Name       string `json:"name"`
		State      string `json:"state"`
		Conclusion string `json:"conclusion"`
	}

	output = strings.TrimSpace(output)
	if output == "" || output == "[]" {
		return true, nil, nil
	}

	var checks []check
	if err := json.Unmarshal([]byte(output), &checks); err != nil {
		return false, nil, []string{"parse error: " + err.Error()}
	}

	if len(checks) == 0 {
		return true, nil, nil
	}

	for _, c := range checks {
		switch {
		case c.State == "PENDING" || c.State == "QUEUED" || c.State == "IN_PROGRESS":
			pending = append(pending, c.Name)
		case c.Conclusion == "FAILURE" || c.Conclusion == "TIMED_OUT" || c.Conclusion == "CANCELLED":
			failed = append(failed, c.Name)
		}
	}

	if len(pending) == 0 && len(failed) == 0 {
		return true, nil, nil
	}
	return false, pending, failed
}

func (p *Pipeline) fixFailures(ctx context.Context, runID, worktree, failures string) error {
	skill, opts, ok := p.executor.registry.SkillForRun("build")
	if !ok {
		return fmt.Errorf("build skill not found")
	}

	r, _ := p.store.Get(runID)
	opts.WorkDir = worktree

	prompt := BuildPrompt(skill, PromptContext{
		WorkDir: worktree,
		Branch:  r.Branch,
		UserPrompt: fmt.Sprintf(
			"The following CI checks have failed on this PR: %s\n\n"+
				"Investigate the failures, fix the issues, and ensure the checks will pass. "+
				"Run the project's test/lint commands locally to verify before finishing.",
			failures,
		),
	})

	_, err := p.executor.runSkill(ctx, runID, prompt, opts, skill.Timeout)
	if err != nil {
		return err
	}

	// Commit the fixes
	p.executor.commitAfterStep(ctx, runID)

	return nil
}

func (p *Pipeline) merge(worktree, prURL string) error {
	strategy := p.cfg.MergeStrategy
	if strategy == "" {
		strategy = "squash"
	}

	cmd := exec.Command("gh", "pr", "merge", prURL, "--"+strategy, "--delete-branch")
	cmd.Dir = worktree
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func (p *Pipeline) setMergeStatus(runID, status string) {
	p.store.Update(runID, func(r *run.Run) {
		r.MergeStatus = status
	})
}

func (p *Pipeline) fail(runID, msg string) {
	log.Printf("merge pipeline failed for run %s: %s", runID, msg)
	p.store.Update(runID, func(r *run.Run) {
		r.State = run.StateFailed
		r.MergeStatus = "failed"
		r.Error = msg
		r.CompletedAt = time.Now()
	})
}
