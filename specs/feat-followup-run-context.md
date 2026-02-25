# Feature: Follow-Up Run Context

## Metadata

type: `feat`
task_id: `followup-run-context`
prompt: `updates to runs should be prefaced with context from the run vs just starting totally fresh with no context. this will prevent repeated work and lookups`

## Feature Description

When a user sends a follow-up prompt to a completed/reviewing run, the agent subprocess starts with zero context about what the run already accomplished. The `executeFollowUp()` method builds a minimal prompt containing only safety constraints, working directory, branch, and the follow-up text. The agent must re-discover the entire codebase, re-read the spec, and figure out what was already built — wasting tokens and time on lookups that were already performed during the original run.

This feature enriches the follow-up prompt with structured context from the run: the original task prompt, which workflow and skills were executed, the spec file path (if one was generated), files that were modified, and the history of previous follow-up prompts. This gives the agent a running start instead of a cold start.

## User Story

As an agtop user sending follow-up instructions to a completed run
I want the agent to have context about what the run already did
So that it doesn't waste time re-discovering the codebase and can immediately act on my follow-up

## Relevant Files

- `internal/engine/executor.go` — `executeFollowUp()` (line 294) builds the follow-up prompt; `executeWorkflow()` (line 424) tracks `specFile` in-memory but never persists it
- `internal/engine/prompt.go` — `PromptContext` struct and `BuildPrompt()` function that already handle enriched context for workflow skills
- `internal/engine/prompt_test.go` — Tests for `BuildPrompt()` including handoff fields
- `internal/run/run.go` — `Run` struct; currently has no `SpecFile` field
- `internal/run/persistence.go` — Session persistence; will automatically serialize new Run fields

### New Files

- None — all changes modify existing files

## Implementation Plan

### Phase 1: Persist spec file path on Run

The spec file path is currently tracked as a local variable in `executeWorkflow()` and lost when the workflow completes. Add a `SpecFile` field to the `Run` struct so it survives across workflow completion and is available for follow-ups. Session persistence serializes Run via JSON, so this field will automatically be saved/restored.

### Phase 2: Build enriched follow-up prompt

Refactor `executeFollowUp()` to assemble a context-rich prompt using the run's stored metadata. Use `BuildPrompt()` with a proper `PromptContext` instead of manually building the prompt string, ensuring consistency with the workflow skill prompt format.

### Phase 3: Gather runtime context

Before launching the follow-up subprocess, gather additional context from the worktree: the list of files modified on the branch (via `git diff --name-only` against the merge base) and the git log of commits made during the run.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add SpecFile field to Run struct

- In `internal/run/run.go`, add `SpecFile string` field to the `Run` struct with JSON tag `json:"spec_file,omitempty"`
- Place it near the existing `Workflow` field for logical grouping

### 2. Persist spec file path during workflow execution

- In `internal/engine/executor.go`, in `executeWorkflow()`, after the existing `specFile = parseSpecFilePath(previousOutput)` line (line 514), add a store update to persist the spec file path on the run:
  ```go
  e.store.Update(runID, func(r *run.Run) {
      r.SpecFile = specFile
  })
  ```

### 3. Refactor executeFollowUp to use BuildPrompt

- In `internal/engine/executor.go`, replace the manual prompt construction in `executeFollowUp()` (lines 314-333) with a call to `BuildPrompt()` using a properly populated `PromptContext`
- Retrieve the run from the store and populate context:
  - `WorkDir` = `r.Worktree`
  - `Branch` = `r.Branch`
  - `UserPrompt` = the follow-up prompt text
  - `SafetyPatterns` = `e.cfg.Safety.BlockedPatterns`
  - `SpecFile` = `r.SpecFile`
  - `ModifiedFiles` = files changed on the branch (from git diff)
  - `PreviousOutput` = a summary string built from run metadata (see step 4)

### 4. Build run context summary for PreviousOutput

- Create a helper function `buildFollowUpContext(r *run.Run) string` in `executor.go` that assembles a concise context summary:
  - Original task: `r.Prompt`
  - Workflow executed: `r.Workflow` with skill progression (derive from `r.SkillCosts` skill names or `r.SkillTotal`/`r.CurrentSkill`)
  - Previous follow-up prompts: list `r.FollowUpPrompts` so the agent knows what was already requested
- This string is passed as `PreviousOutput` in the `PromptContext`, which `BuildPrompt()` renders under `- Previous skill output:`

### 5. Gather modified files from git

- Before building the prompt in `executeFollowUp()`, run `git -C <worktree> diff --name-only main...HEAD` (or the configured base branch) to get all files modified during the run
- Pass this list as `ModifiedFiles` in the `PromptContext`
- Swallow errors gracefully (e.g., if no commits exist yet, use empty list)

### 6. Use the build skill content for follow-ups

- The current `executeFollowUp()` already fetches the build skill (`e.registry.SkillForRun("build")`) but only uses its timeout and runtime options — it ignores the skill's `Content` (the SKILL.md instructions)
- Pass the build skill to `BuildPrompt()` so the follow-up agent gets the build skill's full instructions, not just a bare prompt
- This means the follow-up agent will know how to read specs, follow implementation patterns, and use the project's conventions

### 7. Add tests

- In `internal/engine/executor_test.go`, add:
  - `TestBuildFollowUpContext_IncludesOriginalPrompt` — verify the original prompt appears in context
  - `TestBuildFollowUpContext_IncludesFollowUpHistory` — verify previous follow-ups are listed
  - `TestBuildFollowUpContext_EmptyFollowUps` — verify no follow-up section when history is empty
- In `internal/engine/prompt_test.go`, the existing `BuildPrompt` tests already cover `SpecFile` and `ModifiedFiles` rendering — no new prompt tests needed

## Testing Strategy

### Unit Tests

- Test `buildFollowUpContext()` with various run states: first follow-up (no history), subsequent follow-ups (with history), runs with/without spec files
- Existing `BuildPrompt` tests cover the prompt assembly

### Edge Cases

- Follow-up on a run that had no spec phase (e.g., `quick-fix` workflow that was later restarted as completed) — `SpecFile` will be empty, which is handled by `BuildPrompt`'s existing empty-field checks
- Follow-up on a run with multiple previous follow-ups — all should be listed
- Follow-up where git commands fail (e.g., worktree was deleted) — errors should be swallowed, context should degrade gracefully to the current minimal behavior

## Risk Assessment

- **Low risk**: All changes are additive. The follow-up prompt gets more context, but the agent can still function if any piece is missing (empty fields are omitted by `BuildPrompt`)
- **Low risk on Run struct change**: Adding `SpecFile` with `omitempty` is backward-compatible with existing session files (JSON unmarshaling ignores unknown fields, and missing fields default to zero values)
- **Medium risk on token budget**: The enriched prompt will be longer. However, the tokens spent on context are far less than the tokens the agent would spend re-discovering the same information. Net savings expected

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
make check          # Parallel lint + test
make test           # Unit tests only
make lint           # go vet only
```

## Open Questions (Unresolved)

1. **Base branch for diff**: The modified files diff uses `main...HEAD`. Should this use a configurable base branch from `agtop.toml`? **Recommendation**: Use `main` as the default. The worktree was created from main, so this is correct for the common case. Add configurability later if needed.

## Sub-Tasks

Single task — no decomposition needed.
