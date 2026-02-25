# Refactor: Move JIRA Expansion from UI Layer to Executor

## Metadata

type: `refactor`
task_id: `jira-expansion-to-executor`
prompt: `Move JIRA prompt expansion from the UI message handler (SubmitNewRunMsg in app.go) into the executor layer, so it runs as a tracked pre-step before route (for auto workflows) or before the first skill (for explicit workflows)`

## Refactor Description

JIRA prompt expansion currently lives in the UI layer — the `SubmitNewRunMsg` handler in `internal/ui/app.go:363-394`. It runs inside a `tea.Cmd` goroutine, expands the prompt, then returns a `StartRunMsg` with the expanded text. This works, but has architectural drawbacks:

1. **Wrong layer of responsibility** — prompt transformation is execution logic, not UI logic. The UI should relay the user's input; the executor should prepare it for skills.
2. **Invisible to the run** — expansion happens before the run exists in the store. There's no way to log progress ("Fetching JIRA issue..."), track expansion errors on the run, or see that expansion occurred.
3. **Inconsistent with other pre-skill steps** — the route skill runs inside the executor as a tracked step. JIRA expansion is conceptually the same kind of pre-step but lives in a completely different place.
4. **Not applied on resume/restart** — when a run is restarted via `handleRestart()` (`app.go:1148-1172`), it creates a `StartRunMsg` directly from the stored (already-expanded) prompt, bypassing JIRA expansion. This is fine today because the prompt is already expanded, but it means the architecture depends on the stored prompt being the expanded version rather than the original.

This refactor moves JIRA expansion into `executor.Execute()` so it runs as the first operation before any skill executes — before route for auto workflows, or before the first skill for explicit workflows.

## Current State

### UI Layer (app.go:363-394)

```go
case SubmitNewRunMsg:
    expander := a.jiraExpander
    return a, func() tea.Msg {
        prompt := msg.Prompt
        taskID := ""
        if expander != nil {
            expanded, tid, err := expander.Expand(prompt)
            if err != nil {
                log.Printf("warning: %v", err)
            } else {
                prompt = expanded
                taskID = tid
            }
        }
        // ... image attachment ...
        return StartRunMsg{
            Prompt:   prompt,
            Workflow: msg.Workflow,
            Model:    msg.Model,
            TaskID:   taskID,
        }
    }
```

- JIRA expansion runs in a tea.Cmd before the run is created
- Errors are logged to stderr, not associated with any run
- The expanded prompt is stored as `Run.Prompt` (original is lost)
- `jiraExpander` lives on the `App` struct

### Executor Layer (executor.go:74-110)

```go
func (e *Executor) Execute(runID string, workflowName string, userPrompt string) {
    // quick-fix path
    // ...
    skills, err := e.resolveSkills(workflowName)
    // ...
    e.spawnWorker(runID, func(ctx context.Context) {
        e.executeWorkflow(ctx, runID, skills, userPrompt)
    })
}
```

- No awareness of JIRA expansion
- Receives the already-expanded prompt from the UI

## Target State

### UI Layer (app.go)

- `SubmitNewRunMsg` handler passes the raw user prompt through to `StartRunMsg` without JIRA expansion
- `jiraExpander` field removed from `App` struct
- JIRA client/expander creation stays in `NewApp()` but is passed to the executor

### Executor Layer (executor.go)

- `Executor` gains a `jiraExpander *jira.Expander` field (nil-safe)
- `Execute()` runs JIRA expansion before anything else:
  1. If `jiraExpander` is non-nil, call `Expand(userPrompt)`
  2. If expansion succeeds, update the run's `Prompt` and `TaskID` in the store
  3. Log the expansion to the run's log buffer
  4. Pass the expanded prompt to `executeWorkflow()` or `executeQuickFix()`
- Expansion errors are logged to the run's buffer but don't fail the run (same behavior as today — fall through with original prompt)

## Relevant Files

- `internal/ui/app.go` — Remove JIRA expansion from `SubmitNewRunMsg` handler; remove `jiraExpander` field; pass expander to executor
- `internal/engine/executor.go` — Add `jiraExpander` field; add expansion logic in `Execute()`; update constructor
- `internal/engine/executor_test.go` — Add test for JIRA expansion in executor
- `internal/jira/expander.go` — No changes (existing API is sufficient)

### New Files

None.

## Migration Strategy

1. **Add** the JIRA expander field to `Executor` and update the constructor
2. **Add** expansion logic at the top of `Execute()`, before skills resolve
3. **Remove** expansion from the `SubmitNewRunMsg` handler in `app.go`
4. **Wire** the expander from `NewApp()` → `NewExecutor()`

This is a single atomic change — no intermediate states. The behavior is identical from the user's perspective: JIRA keys in prompts are expanded before any skill runs.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Update Executor constructor to accept JIRA expander

In `internal/engine/executor.go`:

- Add `jiraExpander` field to the `Executor` struct:
  ```go
  jiraExpander *jira.Expander
  ```
  Import `github.com/justinpbarnett/agtop/internal/jira`. The field is a pointer and nil-safe (nil means JIRA not configured).

- Add a `SetJIRAExpander` method (keeps the constructor signature stable and avoids breaking existing callers):
  ```go
  func (e *Executor) SetJIRAExpander(exp *jira.Expander) {
      e.jiraExpander = exp
  }
  ```

### 2. Add JIRA expansion to Execute()

In `internal/engine/executor.go`, at the top of `Execute()`, before the `quick-fix` check:

- Add a `expandJIRA` method:
  ```go
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
  ```

- Call `expandJIRA` at the top of `Execute()`, before the quick-fix branch:
  ```go
  func (e *Executor) Execute(runID string, workflowName string, userPrompt string) {
      userPrompt = e.expandJIRA(runID, userPrompt)
      // ... rest of Execute unchanged
  }
  ```

  This ensures JIRA expansion runs for all workflow types — auto, explicit, and quick-fix.

### 3. Remove JIRA expansion from SubmitNewRunMsg handler

In `internal/ui/app.go`:

- Simplify the `SubmitNewRunMsg` handler. Remove the JIRA expansion logic and pass the raw prompt through:
  ```go
  case SubmitNewRunMsg:
      return a, func() tea.Msg {
          prompt := msg.Prompt
          if len(msg.Images) > 0 {
              var sb strings.Builder
              sb.WriteString(prompt)
              sb.WriteString("\n\nAttached images:\n")
              for _, img := range msg.Images {
                  sb.WriteString("- ")
                  sb.WriteString(img)
                  sb.WriteString("\n")
              }
              prompt = sb.String()
          }
          return StartRunMsg{
              Prompt:   prompt,
              Workflow: msg.Workflow,
              Model:    msg.Model,
          }
      }
  ```

- Remove `TaskID` from `StartRunMsg` — it's now set by the executor directly on the run:
  ```go
  type StartRunMsg struct {
      Prompt   string
      Workflow string
      Model    string
  }
  ```

- In the `StartRunMsg` handler (`app.go:396-448`), remove `TaskID: msg.TaskID` from the `run.Run` initialization.

### 4. Remove jiraExpander from App struct

In `internal/ui/app.go`:

- Remove the `jiraExpander *jira.Expander` field from the `App` struct
- In `NewApp()`, instead of storing the expander on `App`, pass it to the executor:
  ```go
  // Replace:
  //   app.jiraExpander = jiraExp
  // With (after executor creation):
  if exec != nil && jiraExp != nil {
      exec.SetJIRAExpander(jiraExp)
  }
  ```
- Remove the `jiraExpander: jiraExp` line from the `App` struct literal
- The JIRA client/expander creation block (`app.go:204-213`) stays in `NewApp()` — only the destination changes

### 5. Update executor tests

In `internal/engine/executor_test.go`:

- Update `newTestExecutor` to remain compatible (no JIRA expander by default — nil means no expansion, same as today)

- Add a test for JIRA expansion in the executor:
  ```go
  func TestExecuteWithJIRAExpansion(t *testing.T) {
      // 1. Set up a mock JIRA server that returns an issue
      // 2. Create an executor with a JIRA expander
      // 3. Execute a run with a JIRA key as the prompt
      // 4. Verify the run's Prompt is expanded and TaskID is set
  }
  ```

- Add a test verifying that JIRA expansion errors don't fail the run:
  ```go
  func TestExecuteJIRAExpansionErrorFallsThrough(t *testing.T) {
      // 1. Set up a JIRA server that returns 500
      // 2. Execute a run with a JIRA key
      // 3. Verify the run still starts with the original prompt
  }
  ```

## Testing Strategy

**No behavior changes from the user's perspective.** JIRA keys in prompts are expanded before skills run, same as today. The change is purely where in the call stack the expansion occurs.

### Unit Tests

- **Existing JIRA tests** (`internal/jira/expander_test.go`): Unchanged — the `Expander` API doesn't change
- **Existing executor tests** (`internal/engine/executor_test.go`): Unchanged — `newTestExecutor` creates executors without JIRA expanders (nil), same as before
- **New executor tests**: Verify JIRA expansion integrates correctly at the executor layer

### Validation

```bash
# All packages compile
go build ./...

# No vet issues
go vet ./...

# JIRA tests still pass
go test ./internal/jira/... -v

# Executor tests pass (including new tests)
go test ./internal/engine/... -v

# UI package compiles (removed fields/imports)
go build ./internal/ui/...

# All tests pass
go test ./...
```

## Risk Assessment

- **Very low risk**: This is a mechanical code move. The `Expander.Expand()` API is unchanged. The expansion logic is identical — the only difference is it runs inside `Execute()` instead of inside the `SubmitNewRunMsg` tea.Cmd.
- **Nil safety**: The `jiraExpander` field is nil when JIRA is not configured. The `expandJIRA` method nil-checks before calling, same as the current code.
- **Error handling**: Preserved — expansion errors log a warning and fall through with the original prompt, exactly as today.
- **Resume/Restart**: `Resume()` and `ResumeReconnected()` use the stored `Run.Prompt`, which is already expanded. This is correct — re-expansion would be wasteful and potentially produce different results if the JIRA issue changed. No changes needed to resume paths.
- **FollowUp**: `FollowUp()` sends a new prompt that is unlikely to contain a JIRA key (it's a follow-up instruction, not a task description). If it does contain one, it won't be expanded — same as today. This is acceptable; follow-ups are about refinement, not new task intake.

## Open Questions (Unresolved)

None — the refactoring target is clearly defined and the approach is straightforward.

## Sub-Tasks

Single task — no decomposition needed. The change touches two files (`executor.go`, `app.go`) plus tests.
