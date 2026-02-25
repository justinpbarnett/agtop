package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/cost"
	"github.com/justinpbarnett/agtop/internal/jira"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
)

type Executor struct {
	store        *run.Store
	manager      *process.Manager
	registry     *Registry
	cfg          *config.Config
	limiter      *cost.LimitChecker
	jiraExpander *jira.Expander
	mu           sync.Mutex
	active       map[string]context.CancelFunc
	wg           sync.WaitGroup
	shuttingDown bool
}

// SetJIRAExpander configures the JIRA expander used to expand issue keys in
// prompts before workflow execution. Pass nil to disable expansion.
func (e *Executor) SetJIRAExpander(exp *jira.Expander) {
	e.jiraExpander = exp
}

func NewExecutor(store *run.Store, manager *process.Manager, registry *Registry, cfg *config.Config) *Executor {
	return &Executor{
		store:    store,
		manager:  manager,
		registry: registry,
		cfg:      cfg,
		limiter:  &cost.LimitChecker{},
		active:   make(map[string]context.CancelFunc),
	}
}

// resolveSkills returns the skill list for a workflow name. "auto" maps to
// the built-in route skill; everything else is resolved from config.
func (e *Executor) resolveSkills(workflow string) ([]string, error) {
	if workflow == "auto" {
		return []string{"route"}, nil
	}
	return ResolveWorkflow(e.cfg, workflow)
}

// spawnWorker registers a cancel function for runID, starts a tracked goroutine
// that calls fn(ctx), and removes the cancel registration when fn returns.
func (e *Executor) spawnWorker(runID string, fn func(context.Context)) {
	ctx, cancel := context.WithCancel(context.Background())

	e.mu.Lock()
	e.active[runID] = cancel
	e.mu.Unlock()

	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.active, runID)
			e.mu.Unlock()
		}()
		fn(ctx)
	}()
}

// expandJIRA expands a JIRA issue key in the prompt. If the prompt contains a
// recognized key and expansion succeeds, the run's Prompt and TaskID are updated
// in the store and the expanded prompt is returned. On error or no match, the
// original prompt is returned unchanged.
func (e *Executor) expandJIRA(runID string, prompt string) string {
	if e.jiraExpander == nil {
		return prompt
	}
	expanded, taskID, err := e.jiraExpander.Expand(prompt)
	if err != nil {
		e.logToBuffer(runID, "", fmt.Sprintf("JIRA expansion failed: %v", err))
		return prompt
	}
	if taskID == "" {
		return prompt
	}
	e.logToBuffer(runID, "", fmt.Sprintf("Expanded JIRA issue %s", taskID))
	e.store.Update(runID, func(r *run.Run) {
		r.Prompt = expanded
		r.TaskID = taskID
	})
	return expanded
}

// Execute runs a workflow for the given run. It spawns a goroutine
// and communicates progress via the run store. Call Cancel() to stop.
func (e *Executor) Execute(runID string, workflowName string, userPrompt string) {
	userPrompt = e.expandJIRA(runID, userPrompt)
	// quick-fix is a built-in mode: send the user prompt directly to the
	// model (no skill wrapping) and commit afterward.
	if workflowName == "quick-fix" {
		e.store.Update(runID, func(r *run.Run) {
			r.SkillTotal = 1
			r.Workflow = workflowName
			r.State = run.StateRunning
			r.StartedAt = time.Now()
		})
		e.spawnWorker(runID, func(ctx context.Context) {
			e.executeQuickFix(ctx, runID, userPrompt)
		})
		return
	}

	skills, err := e.resolveSkills(workflowName)
	if err != nil {
		e.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = err.Error()
			r.CompletedAt = time.Now()
		})
		return
	}

	e.store.Update(runID, func(r *run.Run) {
		r.SkillTotal = len(skills)
		r.Workflow = workflowName
		r.State = run.StateRunning
		r.StartedAt = time.Now()
	})

	e.spawnWorker(runID, func(ctx context.Context) {
		e.executeWorkflow(ctx, runID, skills, userPrompt)
	})
}

// Cancel stops execution of a run. Safe to call if the run isn't active.
func (e *Executor) Cancel(runID string) {
	e.mu.Lock()
	cancel, ok := e.active[runID]
	e.mu.Unlock()
	if ok {
		cancel()
	}
}

// Shutdown gracefully stops all active workflow goroutines. It sets a flag
// so that executeWorkflow exits without marking runs as failed, then cancels
// all contexts and waits for goroutines to drain (with a timeout).
func (e *Executor) Shutdown() {
	e.mu.Lock()
	e.shuttingDown = true
	for _, cancel := range e.active {
		cancel()
	}
	e.mu.Unlock()

	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
}

func (e *Executor) isShuttingDown() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.shuttingDown
}

// Resume restarts a failed or paused run from its last incomplete skill.
func (e *Executor) Resume(runID string, userPrompt string) error {
	r, ok := e.store.Get(runID)
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	if r.State != run.StateFailed && r.State != run.StatePaused {
		return fmt.Errorf("run %s is %s, not resumable", runID, r.State)
	}

	e.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRunning
		r.Error = ""
		if r.StartedAt.IsZero() {
			r.StartedAt = time.Now()
		}
	})

	if r.Workflow == "quick-fix" {
		e.spawnWorker(runID, func(ctx context.Context) {
			e.executeQuickFix(ctx, runID, userPrompt)
		})
		return nil
	}

	skills, err := e.resolveSkills(r.Workflow)
	if err != nil {
		return err
	}

	// Start from the failed skill (SkillIndex is 1-based)
	startIdx := r.SkillIndex - 1
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(skills) {
		startIdx = 0
	}
	remainingSkills := skills[startIdx:]

	e.spawnWorker(runID, func(ctx context.Context) {
		e.executeWorkflow(ctx, runID, remainingSkills, userPrompt)
	})

	return nil
}

// ResumeReconnected restarts workflow execution for a run that was reconnected
// after a TUI restart. Unlike Resume(), it does not change run state — the run
// is already running with a live process attached via Reconnect().
func (e *Executor) ResumeReconnected(runID string, userPrompt string) {
	r, ok := e.store.Get(runID)
	if !ok {
		return
	}

	if r.Workflow == "quick-fix" {
		e.spawnWorker(runID, func(ctx context.Context) {
			e.executeQuickFix(ctx, runID, userPrompt)
		})
		return
	}

	skills, err := e.resolveSkills(r.Workflow)
	if err != nil {
		return
	}

	// Start from the current skill (SkillIndex is 1-based)
	startIdx := r.SkillIndex - 1
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx >= len(skills) {
		startIdx = 0
	}
	remainingSkills := skills[startIdx:]

	e.spawnWorker(runID, func(ctx context.Context) {
		e.executeWorkflow(ctx, runID, remainingSkills, userPrompt)
	})
}

// FollowUp sends a follow-up prompt to a completed run, reusing its worktree.
func (e *Executor) FollowUp(runID, followUpPrompt string) error {
	r, ok := e.store.Get(runID)
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	if r.State != run.StateCompleted && r.State != run.StateReviewing {
		return fmt.Errorf("run %s is %s, not eligible for follow-up", runID, r.State)
	}

	e.store.Update(runID, func(r *run.Run) {
		r.FollowUpPrompts = append(r.FollowUpPrompts, followUpPrompt)
		r.State = run.StateRunning
		r.CompletedAt = time.Time{}
		r.Error = ""
	})

	e.spawnWorker(runID, func(ctx context.Context) {
		e.executeFollowUp(ctx, runID, followUpPrompt)
	})

	return nil
}

func (e *Executor) executeFollowUp(ctx context.Context, runID string, followUpPrompt string) {
	skill, opts, ok := e.registry.SkillForRun("build")
	if !ok {
		e.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = "build skill not found"
			r.CompletedAt = time.Now()
		})
		return
	}

	r, _ := e.store.Get(runID)
	opts.WorkDir = r.Worktree

	e.store.Update(runID, func(r *run.Run) {
		r.SkillIndex = 1
		r.SkillTotal = 1
		r.CurrentSkill = "quick-fix"
	})

	var b strings.Builder
	if len(e.cfg.Safety.BlockedPatterns) > 0 {
		b.WriteString("## Safety Constraints\n\n")
		b.WriteString("You MUST NOT execute any of the following command patterns under any circumstances:\n")
		for _, p := range e.cfg.Safety.BlockedPatterns {
			b.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		b.WriteString("\nIf a task requires any of these operations, STOP and report that the operation is blocked by safety policy.\n\n")
	}
	b.WriteString("## Context\n")
	if r.Worktree != "" {
		b.WriteString("\n- Working directory: ")
		b.WriteString(r.Worktree)
	}
	if r.Branch != "" {
		b.WriteString("\n- Branch: ")
		b.WriteString(r.Branch)
	}
	b.WriteString("\n\n## Task\n\n")
	b.WriteString(followUpPrompt)

	_, err := e.runSkill(ctx, runID, b.String(), opts, skill.Timeout)
	if err != nil {
		if errors.Is(err, process.ErrDisconnected) || e.isShuttingDown() {
			return
		}
		e.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = fmt.Sprintf("follow-up failed: %v", err)
			r.CompletedAt = time.Now()
		})
		return
	}

	e.commitAfterStep(ctx, runID)

	e.store.Update(runID, func(r *run.Run) {
		r.State = run.StateCompleted
		r.CurrentSkill = ""
		r.CompletedAt = time.Now()
	})
}

// executeQuickFix sends the user prompt directly to the model without skill
// wrapping, then commits. Used for trivial changes where the build skill's
// spec-parsing and plan-following overhead isn't needed.
func (e *Executor) executeQuickFix(ctx context.Context, runID string, userPrompt string) {
	skill, opts, ok := e.registry.SkillForRun("build")
	if !ok {
		e.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = "build skill not found"
			r.CompletedAt = time.Now()
		})
		return
	}

	r, _ := e.store.Get(runID)
	opts.WorkDir = r.Worktree

	e.store.Update(runID, func(r *run.Run) {
		r.SkillIndex = 1
		r.SkillTotal = 1
		r.CurrentSkill = "quick-fix"
	})

	// Build a minimal prompt: safety + context + user task, no skill content.
	var b strings.Builder
	if len(e.cfg.Safety.BlockedPatterns) > 0 {
		b.WriteString("## Safety Constraints\n\n")
		b.WriteString("You MUST NOT execute any of the following command patterns under any circumstances:\n")
		for _, p := range e.cfg.Safety.BlockedPatterns {
			b.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		b.WriteString("\nIf a task requires any of these operations, STOP and report that the operation is blocked by safety policy.\n\n")
	}
	b.WriteString("## Context\n")
	if r.Worktree != "" {
		b.WriteString("\n- Working directory: ")
		b.WriteString(r.Worktree)
	}
	if r.Branch != "" {
		b.WriteString("\n- Branch: ")
		b.WriteString(r.Branch)
	}
	b.WriteString("\n\n## Task\n\n")
	b.WriteString(userPrompt)

	_, err := e.runSkill(ctx, runID, b.String(), opts, skill.Timeout)
	if err != nil {
		if errors.Is(err, process.ErrDisconnected) || e.isShuttingDown() {
			return
		}
		e.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = fmt.Sprintf("quick-fix failed: %v", err)
			r.CompletedAt = time.Now()
		})
		return
	}

	e.commitAfterStep(ctx, runID)

	e.store.Update(runID, func(r *run.Run) {
		r.State = run.StateCompleted
		r.CurrentSkill = ""
		r.CompletedAt = time.Now()
	})
}

func (e *Executor) executeWorkflow(ctx context.Context, runID string, skills []string, userPrompt string) {
	var previousOutput string

	for i := 0; i < len(skills); i++ {
		skillName := skills[i]

		// Check if cancelled
		select {
		case <-ctx.Done():
			// If TUI is shutting down, leave state as-is for reconnection
			if e.isShuttingDown() {
				return
			}
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = "cancelled"
				r.CompletedAt = time.Now()
			})
			return
		default:
		}

		// Check if paused — wait for resume
		if !e.waitIfPaused(ctx, runID) {
			return // cancelled while paused
		}

		// Update progress
		e.store.Update(runID, func(r *run.Run) {
			r.SkillIndex = i + 1
			r.CurrentSkill = skillName
			r.State = run.StateRunning
		})

		// Get skill and options
		skill, opts, ok := e.registry.SkillForRun(skillName)
		if !ok {
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = fmt.Sprintf("skill not found: %s", skillName)
				r.CompletedAt = time.Now()
			})
			return
		}

		// Set worktree from run
		r, _ := e.store.Get(runID)
		opts.WorkDir = r.Worktree

		// Build prompt
		pctx := PromptContext{
			WorkDir:        r.Worktree,
			Branch:         r.Branch,
			PreviousOutput: previousOutput,
			UserPrompt:     userPrompt,
			SafetyPatterns: e.cfg.Safety.BlockedPatterns,
		}
		if skillName == "route" {
			pctx.WorkflowNames = workflowNames(e.cfg)
		}
		prompt := BuildPrompt(skill, pctx)

		// Execute skill
		result, err := e.runSkill(ctx, runID, prompt, opts, skill.Timeout)
		if err != nil {
			// If TUI is shutting down, leave state as-is for reconnection
			if errors.Is(err, process.ErrDisconnected) || e.isShuttingDown() {
				return
			}
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = fmt.Sprintf("skill %s failed: %v", skillName, err)
				r.CompletedAt = time.Now()
			})
			return
		}

		previousOutput = result.ResultText

		// Auto-commit after modifying skills
		if !isNonModifyingSkill(skillName) && skillName != "commit" {
			e.commitAfterStep(ctx, runID)
		}

		// Handle route skill: override workflow
		if skillName == "route" {
			resolvedWorkflow := parseRouteResult(previousOutput, workflowNames(e.cfg))
			if resolvedWorkflow == "" {
				e.logToBuffer(runID, "route", "WARNING: could not parse workflow name from route output, falling back to build")
				resolvedWorkflow = "build"
			}

			newSkills, err := ResolveWorkflow(e.cfg, resolvedWorkflow)
			if err != nil {
				e.logToBuffer(runID, "route", fmt.Sprintf("WARNING: workflow %q not found, falling back to build", resolvedWorkflow))
				resolvedWorkflow = "build"
				newSkills, err = ResolveWorkflow(e.cfg, resolvedWorkflow)
				if err != nil {
					e.store.Update(runID, func(r *run.Run) {
						r.State = run.StateFailed
						r.Error = fmt.Sprintf("fallback workflow %q not found: %v", resolvedWorkflow, err)
						r.CompletedAt = time.Now()
					})
					return
				}
			}

			skills = newSkills
			i = -1 // will be incremented to 0 by the for loop
			e.store.Update(runID, func(r *run.Run) {
				r.Workflow = resolvedWorkflow
				r.SkillTotal = len(newSkills)
			})
			continue
		}

		// Handle decompose skill: parallel execution
		if skillName == "decompose" {
			decomposed, err := ParseDecomposeResult(previousOutput)
			if err == nil && len(decomposed.Tasks) > 0 {
				groups := decomposed.GroupByParallel()
				mergedOutput, err := e.executeParallelGroups(ctx, runID, groups, userPrompt, previousOutput)
				if err != nil {
					e.store.Update(runID, func(r *run.Run) {
						r.State = run.StateFailed
						r.Error = fmt.Sprintf("parallel execution failed: %v", err)
						r.CompletedAt = time.Now()
					})
					return
				}
				previousOutput = mergedOutput
			}
		}
	}

	// Workflow complete
	finalState := terminalState(skills, previousOutput)
	e.store.Update(runID, func(r *run.Run) {
		r.State = finalState
		r.CurrentSkill = ""
		r.CompletedAt = time.Now()
	})
}

func (e *Executor) runSkill(ctx context.Context, runID string, prompt string, opts runtime.RunOptions, timeout int) (process.SkillResult, error) {
	maxRetries := e.cfg.Limits.RateLimitMaxRetries
	backoff := time.Duration(e.cfg.Limits.RateLimitBackoff) * time.Second

	for attempt := 0; ; attempt++ {
		skillCtx := ctx
		var cancel context.CancelFunc
		if timeout > 0 {
			skillCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		}

		ch, err := e.manager.StartSkill(runID, prompt, opts)
		if err != nil {
			if cancel != nil {
				cancel()
			}
			return process.SkillResult{}, err
		}

		var result process.SkillResult
		select {
		case result = <-ch:
		case <-skillCtx.Done():
			_ = e.manager.Stop(runID)
			if cancel != nil {
				cancel()
			}
			// Drain ch so consumeSkillEvents finishes and removes the process
			// from the manager before we return, preventing "already has an
			// active process" errors when the next skill starts immediately.
			for range ch {
			}
			return process.SkillResult{}, skillCtx.Err()
		}
		if cancel != nil {
			cancel()
		}

		if result.Err == nil {
			return result, nil
		}

		// If disconnecting, propagate without marking as failure
		if errors.Is(result.Err, process.ErrDisconnected) {
			return result, result.Err
		}

		// Check if the error is a rate limit and we can retry
		if attempt < maxRetries && e.limiter.IsRateLimit(result.Err.Error()) {
			buf := e.manager.Buffer(runID)
			if buf != nil {
				ts := time.Now().Format("15:04:05")
				buf.Append(fmt.Sprintf("[%s] Rate limited, retrying in %ds (attempt %d/%d)", ts, int(backoff.Seconds()), attempt+1, maxRetries))
			}

			select {
			case <-ctx.Done():
				return process.SkillResult{}, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}

		return result, result.Err
	}
}

// waitIfPaused blocks while the run is paused. Returns false if cancelled.
func (e *Executor) waitIfPaused(ctx context.Context, runID string) bool {
	for {
		r, ok := e.store.Get(runID)
		if !ok || r.State != run.StatePaused {
			return true
		}
		select {
		case <-ctx.Done():
			if e.isShuttingDown() {
				return false
			}
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = "cancelled"
				r.CompletedAt = time.Now()
			})
			return false
		case <-e.store.Changes():
			// State changed, re-check
		}
	}
}

func (e *Executor) executeParallelGroups(ctx context.Context, runID string, groups [][]DecomposeTask, userPrompt string, previousOutput string) (string, error) {
	var allResults []string

	for _, group := range groups {
		groupOutput, err := e.executeParallelGroup(ctx, runID, group, userPrompt, previousOutput)
		if err != nil {
			return "", err
		}
		allResults = append(allResults, groupOutput)

		// Auto-commit after each parallel group completes
		e.commitAfterStep(ctx, runID)
	}

	return strings.Join(allResults, "\n"), nil
}

func (e *Executor) executeParallelGroup(ctx context.Context, runID string, tasks []DecomposeTask, userPrompt string, previousOutput string) (string, error) {
	if len(tasks) == 1 {
		return e.executeSingleTask(ctx, runID, tasks[0], userPrompt, previousOutput)
	}

	var wg sync.WaitGroup
	results := make([]string, len(tasks))
	errs := make([]error, len(tasks))

	for i, task := range tasks {
		wg.Add(1)
		go func(idx int, t DecomposeTask) {
			defer wg.Done()

			skill, opts, ok := e.registry.SkillForRun("build")
			if !ok {
				errs[idx] = fmt.Errorf("build skill not found")
				return
			}

			r, _ := e.store.Get(runID)
			opts.WorkDir = r.Worktree

			taskPrompt := BuildPrompt(skill, PromptContext{
				WorkDir:        r.Worktree,
				Branch:         r.Branch,
				PreviousOutput: previousOutput,
				UserPrompt:     fmt.Sprintf("%s\n\nSub-task: %s", userPrompt, t.Name),
				SafetyPatterns: e.cfg.Safety.BlockedPatterns,
			})

			// For parallel tasks, use composite key so processes don't collide
			taskRunID := fmt.Sprintf("%s:%s", runID, t.Name)

			// Create a temporary entry in the store for the sub-task
			// that mirrors the parent run so logging works
			result, err := e.runParallelSkill(ctx, taskRunID, runID, taskPrompt, opts, skill.Timeout)
			results[idx] = result.ResultText
			errs[idx] = err
		}(i, task)
	}

	wg.Wait()

	for i, err := range errs {
		if err != nil {
			return "", fmt.Errorf("parallel task %q failed: %w", tasks[i].Name, err)
		}
	}

	var merged strings.Builder
	for i, task := range tasks {
		merged.WriteString(fmt.Sprintf("### %s\n%s\n\n", task.Name, results[i]))
	}
	return merged.String(), nil
}

func (e *Executor) executeSingleTask(ctx context.Context, runID string, task DecomposeTask, userPrompt string, previousOutput string) (string, error) {
	skill, opts, ok := e.registry.SkillForRun("build")
	if !ok {
		return "", fmt.Errorf("build skill not found")
	}

	r, _ := e.store.Get(runID)
	opts.WorkDir = r.Worktree

	taskPrompt := BuildPrompt(skill, PromptContext{
		WorkDir:        r.Worktree,
		Branch:         r.Branch,
		PreviousOutput: previousOutput,
		UserPrompt:     fmt.Sprintf("%s\n\nSub-task: %s", userPrompt, task.Name),
		SafetyPatterns: e.cfg.Safety.BlockedPatterns,
	})

	result, err := e.runSkill(ctx, runID, taskPrompt, opts, skill.Timeout)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("### %s\n%s\n\n", task.Name, result.ResultText), nil
}

// runParallelSkill runs a skill for a parallel sub-task. It uses a composite
// key for process tracking but accumulates tokens/cost to the parent run.
func (e *Executor) runParallelSkill(ctx context.Context, taskRunID string, parentRunID string, prompt string, opts runtime.RunOptions, timeout int) (process.SkillResult, error) {
	// For parallel tasks we create a temporary run entry so the process manager
	// can track the process. We copy essential fields from the parent.
	parent, ok := e.store.Get(parentRunID)
	if !ok {
		return process.SkillResult{}, fmt.Errorf("parent run not found: %s", parentRunID)
	}

	e.store.Add(&run.Run{
		ID:           taskRunID,
		Branch:       parent.Branch,
		Worktree:     parent.Worktree,
		Workflow:     parent.Workflow,
		State:        run.StateRunning,
		CurrentSkill: "build",
	})

	defer e.store.Remove(taskRunID)

	skillCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		skillCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	ch, err := e.manager.StartSkill(taskRunID, prompt, opts)
	if err != nil {
		return process.SkillResult{}, err
	}

	var result process.SkillResult
	select {
	case result = <-ch:
	case <-skillCtx.Done():
		_ = e.manager.Stop(taskRunID)
		return process.SkillResult{}, skillCtx.Err()
	}

	// Accumulate cost/tokens to parent run
	taskRun, ok := e.store.Get(taskRunID)
	if ok {
		e.store.Update(parentRunID, func(r *run.Run) {
			r.Tokens += taskRun.Tokens
			r.TokensIn += taskRun.TokensIn
			r.TokensOut += taskRun.TokensOut
			r.Cost += taskRun.Cost
			r.SkillCosts = append(r.SkillCosts, taskRun.SkillCosts...)
		})
	}

	return result, result.Err
}

// parseRouteResult extracts a workflow name from the route skill's output.
// Tries JSON format first ({"workflow": "name"}), then scans all lines
// (last to first) for a valid workflow identifier. The knownWorkflows
// slice is used as a fallback to find workflow names embedded in prose.
func parseRouteResult(resultText string, knownWorkflows []string) string {
	text := strings.TrimSpace(resultText)
	if text == "" {
		return ""
	}

	// Try JSON format
	var obj struct {
		Workflow string `json:"workflow"`
	}
	if err := json.Unmarshal([]byte(text), &obj); err == nil && obj.Workflow != "" {
		return obj.Workflow
	}

	// Scan all lines last-to-first for a valid workflow name.
	// The route skill is instructed to output just the name, but models
	// often prepend explanation text. The actual name is typically last.
	lines := strings.Split(text, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(lines[i])
		if candidate == "" {
			continue
		}

		// First try as a standalone name
		candidate = stripWrapping(candidate)
		if isWorkflowName(candidate) {
			return candidate
		}

		// If not a standalone name, try to extract a workflow name from the line
		// by searching for known workflow names (this handles cases like "use build" or "build workflow")
		found := extractWorkflowName(lines[i], knownWorkflows)
		if found != "" {
			return found
		}
	}
	return ""
}

// stripWrapping removes backticks, quotes, markdown bold/italic markers, and
// trailing punctuation that LLMs commonly add around a bare workflow name.
// Two passes handle layered wrapping like **word**. (bold + period).
func stripWrapping(s string) string {
	s = strings.Trim(s, "`\"'*")
	s = strings.TrimRight(s, ".,;:!?")
	s = strings.Trim(s, "`\"'*")
	return strings.TrimSpace(s)
}

// isWorkflowName returns true if s looks like a valid workflow identifier
// (alphanumeric, dashes, underscores only, non-empty).
func isWorkflowName(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// extractWorkflowName searches a line for known workflow names and returns the
// first match found. This handles cases where the workflow name is embedded in text,
// e.g., "I recommend the build workflow" or "use quick-fix for this task".
func extractWorkflowName(line string, knownWorkflows []string) string {
	// Only attempt extraction on short lines (≤ 4 words). This handles
	// near-direct references like "use build" or "build workflow" while
	// rejecting full sentences such as "I think you should use the build workflow".
	words := strings.Fields(line)
	if len(words) > 4 {
		return ""
	}

	// Convert line to lowercase for case-insensitive matching
	lowerLine := strings.ToLower(line)

	// Search for each known workflow name in the line
	for _, wf := range knownWorkflows {
		if strings.Contains(lowerLine, strings.ToLower(wf)) {
			return wf
		}
	}
	return ""
}

// logToBuffer writes a timestamped message to the run's log buffer.
func (e *Executor) logToBuffer(runID string, skill string, msg string) {
	buf := e.manager.Buffer(runID)
	if buf == nil {
		return
	}
	ts := time.Now().Format("15:04:05")
	if skill != "" {
		buf.Append(fmt.Sprintf("[%s %s] %s", ts, skill, msg))
	} else {
		buf.Append(fmt.Sprintf("[%s] %s", ts, msg))
	}
}

// isNonModifyingSkill returns true for skills that don't modify files in the worktree.
func isNonModifyingSkill(name string) bool {
	switch name {
	case "route", "decompose":
		return true
	}
	return false
}

// commitAfterStep runs the commit skill to save progress after a workflow step.
// Errors are logged but do not fail the workflow — this is best-effort.
func (e *Executor) commitAfterStep(ctx context.Context, runID string) {
	skill, opts, ok := e.registry.SkillForRun("commit")
	if !ok {
		return
	}

	r, ok := e.store.Get(runID)
	if !ok {
		return
	}
	opts.WorkDir = r.Worktree

	prompt := BuildPrompt(skill, PromptContext{
		WorkDir:    r.Worktree,
		Branch:     r.Branch,
		UserPrompt: "Review all uncommitted changes in this worktree and create atomic commits using conventional commit format. If there are no changes to commit, do nothing.",
	})

	result, err := e.runSkill(ctx, runID, prompt, opts, skill.Timeout)
	if err != nil {
		return
	}

	// Accumulate tokens/cost from the commit skill (already tracked by process manager)
	_ = result
}

func workflowNames(cfg *config.Config) []string {
	names := make([]string, 0, len(cfg.Workflows))
	for name := range cfg.Workflows {
		names = append(names, name)
	}
	return names
}

func terminalState(skills []string, lastOutput string) run.State {
	if len(skills) == 0 {
		return run.StateCompleted
	}
	last := skills[len(skills)-1]
	if last == "review" {
		if reviewPassed(lastOutput) {
			return run.StateCompleted
		}
		return run.StateReviewing
	}
	return run.StateCompleted
}

func reviewPassed(output string) bool {
	text := strings.TrimSpace(output)
	if text == "" {
		return false
	}

	var report struct {
		Success bool `json:"success"`
	}
	if json.Unmarshal([]byte(text), &report) == nil {
		return report.Success
	}

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		if json.Unmarshal([]byte(text[start:end+1]), &report) == nil {
			return report.Success
		}
	}

	return false
}
