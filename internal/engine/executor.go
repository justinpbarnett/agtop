package engine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/process"
	"github.com/justinpbarnett/agtop/internal/run"
	"github.com/justinpbarnett/agtop/internal/runtime"
)

type Executor struct {
	store    *run.Store
	manager  *process.Manager
	registry *Registry
	cfg      *config.Config
	mu       sync.Mutex
	active   map[string]context.CancelFunc
}

func NewExecutor(store *run.Store, manager *process.Manager, registry *Registry, cfg *config.Config) *Executor {
	return &Executor{
		store:    store,
		manager:  manager,
		registry: registry,
		cfg:      cfg,
		active:   make(map[string]context.CancelFunc),
	}
}

// Execute runs a workflow for the given run. It spawns a goroutine
// and communicates progress via the run store. Call Cancel() to stop.
func (e *Executor) Execute(runID string, workflowName string, userPrompt string) {
	skills, err := ResolveWorkflow(e.cfg, workflowName)
	if err != nil {
		e.store.Update(runID, func(r *run.Run) {
			r.State = run.StateFailed
			r.Error = err.Error()
		})
		return
	}

	e.store.Update(runID, func(r *run.Run) {
		r.SkillTotal = len(skills)
		r.Workflow = workflowName
		r.State = run.StateRunning
	})

	ctx, cancel := context.WithCancel(context.Background())

	e.mu.Lock()
	e.active[runID] = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.active, runID)
			e.mu.Unlock()
		}()

		e.executeWorkflow(ctx, runID, skills, userPrompt)
	}()
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

// Resume restarts a failed or paused run from its last incomplete skill.
func (e *Executor) Resume(runID string, userPrompt string) error {
	r, ok := e.store.Get(runID)
	if !ok {
		return fmt.Errorf("run not found: %s", runID)
	}
	if r.State != run.StateFailed && r.State != run.StatePaused {
		return fmt.Errorf("run %s is %s, not resumable", runID, r.State)
	}

	skills, err := ResolveWorkflow(e.cfg, r.Workflow)
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

	e.store.Update(runID, func(r *run.Run) {
		r.State = run.StateRunning
		r.Error = ""
	})

	ctx, cancel := context.WithCancel(context.Background())

	e.mu.Lock()
	e.active[runID] = cancel
	e.mu.Unlock()

	go func() {
		defer func() {
			e.mu.Lock()
			delete(e.active, runID)
			e.mu.Unlock()
		}()

		e.executeWorkflow(ctx, runID, remainingSkills, userPrompt)
	}()

	return nil
}

func (e *Executor) executeWorkflow(ctx context.Context, runID string, skills []string, userPrompt string) {
	var previousOutput string

	for i := 0; i < len(skills); i++ {
		skillName := skills[i]

		// Check if cancelled
		select {
		case <-ctx.Done():
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = "cancelled"
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
			})
			return
		}

		// Set worktree from run
		r, _ := e.store.Get(runID)
		opts.WorkDir = r.Worktree

		// Build prompt
		prompt := BuildPrompt(skill, PromptContext{
			WorkDir:        r.Worktree,
			Branch:         r.Branch,
			PreviousOutput: previousOutput,
			UserPrompt:     userPrompt,
		})

		// Execute skill
		result, err := e.runSkill(ctx, runID, prompt, opts, skill.Timeout)
		if err != nil {
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = fmt.Sprintf("skill %s failed: %v", skillName, err)
			})
			return
		}

		previousOutput = result.ResultText

		// Handle route skill: override workflow
		if skillName == "route" {
			resolvedWorkflow := parseRouteResult(previousOutput)
			if resolvedWorkflow != "" {
				newSkills, err := ResolveWorkflow(e.cfg, resolvedWorkflow)
				if err == nil {
					skills = newSkills
					i = -1 // will be incremented to 0 by the for loop
					e.store.Update(runID, func(r *run.Run) {
						r.Workflow = resolvedWorkflow
						r.SkillTotal = len(newSkills)
					})
					continue
				}
			}
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
					})
					return
				}
				previousOutput = mergedOutput
			}
		}
	}

	// Workflow complete
	finalState := terminalState(skills)
	e.store.Update(runID, func(r *run.Run) {
		r.State = finalState
		r.CurrentSkill = ""
	})
}

func (e *Executor) runSkill(ctx context.Context, runID string, prompt string, opts runtime.RunOptions, timeout int) (process.SkillResult, error) {
	skillCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		skillCtx, cancel = context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()
	}

	ch, err := e.manager.StartSkill(runID, prompt, opts)
	if err != nil {
		return process.SkillResult{}, err
	}

	select {
	case result := <-ch:
		return result, result.Err
	case <-skillCtx.Done():
		_ = e.manager.Stop(runID)
		return process.SkillResult{}, skillCtx.Err()
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
			e.store.Update(runID, func(r *run.Run) {
				r.State = run.StateFailed
				r.Error = "cancelled"
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
			r.Cost += taskRun.Cost
		})
	}

	return result, result.Err
}

// parseRouteResult extracts a workflow name from the route skill's output.
// Tries JSON format first ({"workflow": "name"}), falls back to plain text.
func parseRouteResult(resultText string) string {
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

	// Plain text: take first line, trim whitespace
	lines := strings.SplitN(text, "\n", 2)
	candidate := strings.TrimSpace(lines[0])

	// Basic validation: workflow names are simple identifiers
	for _, c := range candidate {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return ""
		}
	}
	return candidate
}

// terminalState determines the final run state after all skills complete.
func terminalState(skills []string) run.State {
	if len(skills) == 0 {
		return run.StateCompleted
	}
	last := skills[len(skills)-1]
	if last == "review" {
		return run.StateReviewing
	}
	return run.StateCompleted
}
