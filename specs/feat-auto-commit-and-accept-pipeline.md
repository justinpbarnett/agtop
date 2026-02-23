# Feature: Auto-Commit After Steps & Accept-to-Merge Pipeline

## Metadata

type: `feat`
task_id: `auto-commit-and-accept-pipeline`
prompt: `1) after each step in the run, the commit skill should be run to save progress to the current worktree. 2) pressing a for accept should automatically create a pr, auto-resolve any merge conflicts, wait for pr checks to pass (fix failing ones), and accept and merge the pr into either the default branch or the configured working branch in the config yaml file.`

## Feature Description

Two related workflow automation features:

1. **Inter-step auto-commit**: After each skill completes in a workflow, automatically run the `commit` skill to create atomic commits in the run's worktree. This preserves incremental progress and produces a clean commit history per skill.

2. **Accept-to-merge pipeline**: When the user presses `a` to accept, instead of just pushing the branch, execute a full merge pipeline: rebase onto the target branch, resolve merge conflicts (using the agent), create a PR via `gh`, poll for CI checks, fix any failures (using the agent), and merge the PR once green.

## User Story

As a developer using agtop to orchestrate agent workflows
I want completed runs to auto-commit after each step and merge automatically on accept
So that I get clean commit history and zero-touch delivery from accept to merged PR

## Relevant Files

- `internal/engine/executor.go` — Workflow execution loop where inter-step commit hook will be added (line 134-244)
- `internal/engine/registry.go` — Skill registry, used to resolve the `commit` skill (lines 165-175)
- `internal/ui/app.go` — `handleAccept()` at lines 643-678, the entry point for the accept pipeline
- `internal/run/run.go` — Run struct with state machine, needs new states/fields for pipeline progress
- `internal/config/config.go` — Config structs, needs new `merge` section for target branch and pipeline settings
- `internal/git/worktree.go` — Worktree management, rebase operations will happen here
- `internal/process/manager.go` — Subprocess management for spawning agent fix passes
- `internal/server/devserver.go` — Dev server cleanup during accept (no changes needed)
- `agtop.example.yaml` — Example config, needs `merge` section
- `skills/commit/SKILL.md` — The commit skill that will be invoked between steps

### New Files

- `internal/engine/pipeline.go` — Accept-to-merge pipeline orchestration (rebase, PR, poll, fix, merge)

## Implementation Plan

### Phase 1: Inter-Step Auto-Commit

Add a post-step hook in the executor's workflow loop that runs the `commit` skill after each non-commit skill completes successfully. The commit skill already exists — it just needs to be invoked with the worktree as its working directory.

Key design decisions:
- The commit skill runs as a lightweight sub-invocation, not counted in `SkillIndex`/`SkillTotal`
- If the commit skill fails, log the error but don't fail the run — it's best-effort
- Skip auto-commit after skills that don't modify files: `route`, `decompose`, `review`
- Skip auto-commit if the current skill _is_ `commit` (avoid recursion)
- Accumulate commit skill tokens/cost into the run total but don't show it as a separate skill step in the UI

### Phase 2: Config for Merge Target

Add a `merge` config section to control the accept pipeline:

```yaml
merge:
  target_branch: main        # branch to merge into (default: repo default branch)
  auto_merge: true            # enable accept-to-merge pipeline (default: false)
  fix_attempts: 3             # max CI fix attempts before giving up (default: 3)
  poll_interval: 30           # seconds between check status polls (default: 30)
  poll_timeout: 600           # max seconds to wait for checks (default: 600)
```

### Phase 3: Accept-to-Merge Pipeline

Replace the current `handleAccept()` logic (push + remove worktree) with a multi-stage pipeline:

1. **Rebase** — Rebase the run's branch onto the target branch in the worktree
2. **Conflict resolution** — If rebase has conflicts, invoke the agent to resolve them, then `git rebase --continue`
3. **Push** — Force-push the rebased branch to origin
4. **Create PR** — Use `gh pr create` targeting the configured branch
5. **Poll checks** — Poll `gh pr checks` until all pass or timeout
6. **Fix failures** — If checks fail, invoke the agent to fix, commit, push, and re-poll (up to `fix_attempts`)
7. **Merge** — `gh pr merge --squash` (or merge strategy from config)
8. **Cleanup** — Remove worktree and branch

The pipeline runs asynchronously. The run transitions through new sub-states so the UI can show progress. If `auto_merge` is false, fall back to the current behavior (push only).

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add merge config

- In `internal/config/config.go`, add `MergeConfig` struct:
  ```go
  type MergeConfig struct {
      TargetBranch string `yaml:"target_branch"`
      AutoMerge    bool   `yaml:"auto_merge"`
      FixAttempts  int    `yaml:"fix_attempts"`
      PollInterval int    `yaml:"poll_interval"`
      PollTimeout  int    `yaml:"poll_timeout"`
  }
  ```
- Add `Merge MergeConfig` field to the top-level `Config` struct
- Update `agtop.example.yaml` with the new `merge` section and comments

### 2. Add inter-step auto-commit to executor

- In `internal/engine/executor.go`, add a `commitAfterStep` method:
  ```go
  func (e *Executor) commitAfterStep(ctx context.Context, runID string) error
  ```
- This method should:
  - Resolve the `commit` skill from the registry via `e.registry.SkillForRun("commit")`
  - Set `opts.WorkDir` to the run's worktree
  - Call `e.runSkill()` with a prompt like: "Review all uncommitted changes in this worktree and create atomic commits using conventional commit format."
  - Accumulate tokens/cost from the result into the run
  - Return nil on success, error on failure (caller ignores errors)
- In `executeWorkflow()`, after line 199 (`previousOutput = result.ResultText`), insert:
  ```go
  if !isNonModifyingSkill(skillName) && skillName != "commit" {
      if err := e.commitAfterStep(ctx, runID); err != nil {
          // Log but don't fail the workflow
      }
  }
  ```
- Add helper `isNonModifyingSkill(name string) bool` that returns true for `route`, `decompose`, `review`, `document`
- Also add auto-commit after parallel task groups complete in `executeParallelGroups` (after line 333)

### 3. Add pipeline sub-states to Run

- In `internal/run/run.go`, add new state constants for accept pipeline progress:
  ```go
  StateMerging = "merging"  // Accept pipeline in progress
  ```
- Add a `MergeStatus` field to the `Run` struct:
  ```go
  MergeStatus string `json:"merge_status"` // rebasing, pushing, pr-created, checks-pending, fixing, merged, failed
  PRURL       string `json:"pr_url"`       // URL of created PR
  ```
- Update `StatusIcon()` to handle `StateMerging` (use `⟳` or similar)
- Update `IsTerminal()` — `StateMerging` is NOT terminal (pipeline still running)

### 4. Create the accept pipeline

- Create `internal/engine/pipeline.go` with a `Pipeline` struct:
  ```go
  type Pipeline struct {
      executor *Executor
      store    *run.Store
      cfg      *config.MergeConfig
      repoRoot string
  }
  ```
- Implement `func (p *Pipeline) Run(ctx context.Context, runID string) error` with these stages:

  **Stage 1 — Rebase:**
  - Set `MergeStatus = "rebasing"`
  - Determine target: `p.cfg.TargetBranch` if set, otherwise detect via `git symbolic-ref refs/remotes/origin/HEAD`
  - Run `git fetch origin <target>` in worktree
  - Run `git rebase origin/<target>` in worktree
  - If exit code indicates conflicts, go to conflict resolution

  **Stage 2 — Conflict Resolution:**
  - Set `MergeStatus = "resolving-conflicts"`
  - Invoke the `build` skill with a prompt explaining the conflicts and asking to resolve them
  - After resolution, run `git add -A && git rebase --continue` in worktree
  - If still conflicting, retry up to 3 times then fail

  **Stage 3 — Push:**
  - Set `MergeStatus = "pushing"`
  - Run `git push origin <branch> --force-with-lease` in worktree

  **Stage 4 — Create PR:**
  - Set `MergeStatus = "pr-created"`
  - Run `gh pr create --base <target> --head <branch> --title "<conventional title>" --body "<body>"` in worktree
  - Parse PR URL from stdout, store in `run.PRURL`

  **Stage 5 — Poll Checks:**
  - Set `MergeStatus = "checks-pending"`
  - Loop with `poll_interval` sleep:
    - Run `gh pr checks <branch> --json name,state,conclusion`
    - If all pass → proceed to merge
    - If any fail → go to fix stage
    - If timeout → fail with descriptive error

  **Stage 6 — Fix Failures:**
  - Set `MergeStatus = "fixing"`
  - Parse which checks failed from `gh pr checks` output
  - Invoke the `build` skill with a prompt describing the failures and asking to fix them
  - Run auto-commit (reuse `commitAfterStep`)
  - Push again, return to poll stage
  - Track attempts; if `fix_attempts` exceeded, fail

  **Stage 7 — Merge:**
  - Set `MergeStatus = "merged"`
  - Run `gh pr merge <pr-url> --squash --delete-branch`
  - Transition run to `StateAccepted`

  **On any failure:**
  - Set `MergeStatus = "failed"` with error in `run.Error`
  - Transition run to `StateFailed` (or stay in `StateMerging` with error for retry)

### 5. Wire pipeline into handleAccept

- In `internal/ui/app.go`, modify `handleAccept()`:
  - If `cfg.Merge.AutoMerge` is true:
    - Transition run to `StateMerging` instead of `StateAccepted`
    - Stop dev server (same as current)
    - Launch `pipeline.Run(ctx, runID)` in a goroutine
    - The pipeline will handle state transitions and worktree cleanup
  - If `cfg.Merge.AutoMerge` is false:
    - Keep current behavior (push + remove worktree)
- Add the `Pipeline` as a field on the `App` struct, initialized in `NewApp()`
- Update the `handleAccept()` state guard to also allow `StateMerging` runs to be re-accepted (retry after failure)

### 6. Update UI to show pipeline progress

- In the run list and detail panel, show `MergeStatus` when state is `StateMerging`
- Show the PR URL in the detail panel when available
- Status bar flash messages for each stage transition
- When `MergeStatus = "merged"`, show a success indicator

### 7. Update example config and README

- Add `merge` section to `agtop.example.yaml` with documented defaults
- Add keybinding and pipeline behavior to README.md key bindings table

## Testing Strategy

### Unit Tests

- `internal/engine/executor_test.go` — Test `commitAfterStep` is called for modifying skills and skipped for non-modifying ones
- `internal/engine/pipeline_test.go` — Test each pipeline stage with mocked git/gh commands; test retry logic, timeout, and failure paths
- `internal/config/config_test.go` — Test merge config parsing with defaults

### Edge Cases

- Commit skill not found in registry (skip gracefully, don't block workflow)
- Rebase with no conflicts (skip resolution stage)
- PR checks never appear (some repos have no required checks — detect and skip poll)
- `gh` CLI not installed or not authenticated (fail with actionable error message)
- Run cancelled during pipeline (respect context cancellation at each stage)
- Multiple accept attempts on same run (guard against duplicate pipelines)
- Worktree has no uncommitted changes after a skill (commit skill handles this gracefully)
- Target branch doesn't exist on remote (fail with clear error)
- Network failures during push/PR creation (retry with backoff)

## Risk Assessment

- **Merge conflicts**: Agent-driven resolution may produce incorrect merges. Mitigation: the review skill can catch issues, and CI checks serve as a safety net.
- **CI fix loop**: Agent may not be able to fix CI failures, burning tokens. Mitigation: `fix_attempts` cap with configurable limit.
- **Force push**: `--force-with-lease` is safer than `--force` but still overwrites remote. Mitigation: only used on agtop-managed branches (`agtop/<runID>`), never on shared branches.
- **Token cost**: Auto-commit after every step and CI fix loops add token overhead. Mitigation: commit skill uses `haiku` model (cheap), fix attempts are capped.
- **`gh` dependency**: Pipeline requires GitHub CLI. Mitigation: check for `gh` at app startup and warn if missing; `auto_merge: false` bypasses the dependency entirely.

## Validation Commands

```bash
go build ./...
go test ./...
go vet ./...
```

## Open Questions (Unresolved)

1. **Merge strategy**: Should the PR merge use `--squash`, `--merge`, or `--rebase`? **Recommendation**: Default to `--squash` for clean history, but add a `merge_strategy` config option.

2. **Failed pipeline recovery**: If the pipeline fails mid-way (e.g., after PR creation but before merge), should `a` retry from the failed stage or start over? **Recommendation**: Retry from the failed stage — track the last completed stage in `MergeStatus` and resume from there.

3. **PR description content**: Should the PR body include a summary of what the agent did (skill outputs, token usage)? **Recommendation**: Yes — include a brief summary generated from the run's skill history, similar to what the `document` skill produces.

4. **Commit message style for inter-step commits**: Should auto-commits use a fixed format like `chore(agtop): save progress after <skill>` or let the commit skill decide freely? **Recommendation**: Let the commit skill decide — it already follows conventional commit format and will produce more meaningful messages by reading the actual changes.

## Sub-Tasks

Single task — no decomposition needed. The two features are tightly coupled (auto-commit feeds into the accept pipeline's clean commit history) and share infrastructure (skill invocation, worktree management).
