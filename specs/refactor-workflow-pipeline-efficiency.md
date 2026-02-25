# Refactor: Workflow Pipeline Efficiency

## Metadata

type: `refactor`
task_id: `workflow-pipeline-efficiency`
prompt: `Investigate worktree 005 logs to identify optimization and refactoring opportunities for the plan-build workflow`

## Refactor Description

Analysis of worktree 005's execution reveals three systemic inefficiencies in the workflow pipeline that increase cost, waste tokens, and produce suboptimal commit history. Worktree 005 ran the plan-build workflow (`spec → build → test → review`) and produced 3 commits — one of which was a duplicate (same message, 2 minutes apart). The root causes are:

1. **Expensive LLM-driven commits**: `commitAfterStep()` spawns a full Claude Code process to run `git status` + `git diff` + `git commit`. This costs tokens and API time for what is fundamentally a deterministic operation.
2. **Duplicate commits**: The commit skill has no awareness of prior commits on the branch. When sequential steps touch the same files, it produces near-identical commits (e.g., two "fix: only show expand arrow for expandable log entries" commits 2 minutes apart).
3. **Redundant codebase discovery**: Each skill in the pipeline starts a fresh Claude process and re-reads the same files. The spec skill reads README.md, explores directories, and builds understanding — then the build skill starts from scratch and does the same reads.

## Current State

### Commit-after-step flow (`executor.go:880-905`)

After every modifying skill, `commitAfterStep()` calls `runSkill()` with the commit skill. This:
- Spawns a new Claude Code subprocess
- Sends the full commit skill prompt (SKILL.md + context)
- The LLM reads `git status`, `git diff HEAD`, analyzes changes, groups files, writes commit messages
- Creates commits via `git add <files> && git commit -m "<msg>"`

**Problem**: For a 4-phase workflow, this fires after `build` and `review` — 2 extra Claude invocations just for committing. Each invocation costs tokens for tool calls (Bash for git commands) and LLM reasoning about what changed.

### Output propagation (`prompt.go:60-63`)

Previous skill output is passed as a flat string:
```
- Previous skill output:
<text>
```

For spec→build: the spec skill outputs the spec file path. The build skill must read it from disk.
For build→test: the build skill outputs a diff summary. The test skill ignores it and rediscovers test commands.
For test→review: the test skill outputs JSON. The review skill uses it but still re-reads the spec file from disk.

**Problem**: No structured handoff. Each skill independently discovers the same project information (README.md, Makefile targets, directory structure).

### Commit skill behavior (`skills/commit/SKILL.md`)

The commit skill groups files by "logical concern" using LLM judgment. It has no concept of:
- What the current workflow phase is (build vs review)
- What commits already exist on the branch
- Whether a new commit would duplicate a recent one

**Problem**: When the build step creates `fix: improve X` and the review step makes a small tweak to the same area, the commit skill creates another `fix: improve X` — producing the duplicate seen in worktree 005.

## Target State

### 1. Deterministic commit-after-step

Replace the LLM-driven commit skill invocation in `commitAfterStep()` with a deterministic shell-based approach:

- Run `git status --porcelain` to detect changes
- If no changes, skip (no Claude invocation needed)
- Stage all changes: `git add -A` (safe in a worktree where all changes are from the current skill)
- Generate commit message from workflow context: `<phase-type>: <phase-description>` (e.g., `feat: implement spec` for the build phase, `fix: address review findings` for the review phase)
- Run `git commit -m "<message>"`

This eliminates 2 Claude invocations per plan-build workflow execution.

The existing commit skill remains available as a standalone skill for the `commit` workflow and user-invoked commits. Only the auto-commit between pipeline steps changes.

### 2. Commit deduplication guard

Before creating any auto-commit, check the most recent commit on the branch:
- Run `git log -1 --format=%s` to get the last commit message
- If the new message would be identical, amend instead of creating a duplicate: `git commit --amend --no-edit`

This prevents the duplicate commit pattern observed in worktree 005.

### 3. Structured skill handoff context

Enhance `PromptContext` to carry structured metadata that downstream skills can use without re-reading files:

```go
type PromptContext struct {
    // ... existing fields ...
    SpecFile       string   // Path to the spec file (set after spec skill)
    ModifiedFiles  []string // Files changed by previous skill (from git diff --name-only)
    ProjectType    string   // Detected project type (e.g., "go", "node")
    TestCommand    string   // Discovered test command (e.g., "make check")
}
```

After each skill completes:
- Parse `git diff --name-only HEAD~1` to populate `ModifiedFiles`
- Carry forward `SpecFile` from spec skill output (parse the file path from the result text)
- On first detection, cache `ProjectType` and `TestCommand`

Inject these into the prompt template so downstream skills skip redundant discovery.

## Relevant Files

- `internal/engine/executor.go` — `commitAfterStep()` method (line 880), `executeWorkflow()` loop (line 374), `runSkill()` (line 518)
- `internal/engine/prompt.go` — `PromptContext` struct (line 8), `BuildPrompt()` function (line 31), `skillTaskOverrides` map (line 22)
- `internal/engine/workflow.go` — Workflow resolution (line 16)
- `internal/process/manager.go` — Process lifecycle, `StartSkill()` method
- `skills/commit/SKILL.md` — Commit skill definition (unchanged, stays for standalone use)
- `agtop.toml` — Workflow definitions (lines 73-77)
- `internal/config/config.go` — Config struct with `WorkflowConfig` (line 66)

### New Files

- None — all changes modify existing files

## Migration Strategy

Each optimization is independent and can land in any order:

1. **Deterministic commit** (highest impact) — Replace `commitAfterStep()` internals. The commit skill itself is untouched, so standalone `commit` workflows still work.
2. **Dedup guard** (low effort, high value) — Add a 3-line check before the `git commit` call in the new deterministic path.
3. **Structured handoff** (medium effort) — Extend `PromptContext` and `BuildPrompt()`. Downstream skills benefit automatically.

No behavior changes to the TUI, run store, or process management. The commit skill continues to work for user-invoked commits.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add deterministic commit helper to executor

- In `internal/engine/executor.go`, add a `determinsiticCommit(ctx, runID, phase string) error` method
- Implementation:
  - Get the run from store to find `Worktree` path
  - Run `git -C <worktree> status --porcelain` via `exec.CommandContext`
  - If output is empty, return nil (nothing to commit)
  - Run `git -C <worktree> add -A`
  - Derive commit message from a hardcoded skill-to-type map: `build`→`feat`, `review`→`fix`, `spec`→`docs`, default→`chore`
  - Format: `<type>: <skill-name> changes` (e.g., `feat: build changes`, `fix: review changes`)
  - Run `git -C <worktree> log -1 --format=%s` to check for duplicate
  - If identical to last commit, use `git commit --amend --no-edit` instead
  - Otherwise, `git commit -m "<message>"`
- Add `"os/exec"` to imports if not present

### 2. Replace commitAfterStep with deterministic path

- In `internal/engine/executor.go`, modify `commitAfterStep()` (line 880):
  - Replace the skill-based commit with a call to `deterministicCommit(ctx, runID, currentSkillName)`
  - Remove the `skill, opts, ok := e.registry.SkillForRun("commit")` lookup
  - Remove the `BuildPrompt` and `runSkill` calls
  - Keep the error-swallowing behavior (log errors but don't fail the workflow)
- Update `executeWorkflow()` (line 454) to pass the current skill name to `commitAfterStep()`

### 3. Extend PromptContext with structured handoff fields

- In `internal/engine/prompt.go`, add fields to `PromptContext`:
  - `SpecFile string` — path to the generated spec file
  - `ModifiedFiles []string` — files changed by the previous skill
- In `BuildPrompt()`, after the existing `PreviousOutput` block, add:
  - If `SpecFile` is set: `\n- Spec file: <path>`
  - If `ModifiedFiles` is non-empty: `\n- Files modified by previous step: <comma-joined list>`

### 4. Populate handoff context in executeWorkflow

- In `internal/engine/executor.go`, in the `executeWorkflow()` loop (after `previousOutput = result.ResultText` on line 451):
  - After a successful skill run, populate `pctx.ModifiedFiles` by running `git -C <worktree> diff --name-only HEAD~1 2>/dev/null` (swallow errors for first commit)
  - If the skill name is "spec", parse the result text for a file path matching `specs/*.md` and set `pctx.SpecFile`
- Thread these fields into the `PromptContext` construction for subsequent skills (lines 424-433)

### 5. Add tests for deterministic commit

- In `internal/engine/executor_test.go`, add test cases:
  - `TestDeterministicCommit_NoChanges` — verify no commit is created when worktree is clean
  - `TestDeterministicCommit_CreatesCommit` — verify a conventional commit is created
  - `TestDeterministicCommit_DeduplicatesIdenticalMessage` — verify amend behavior when last commit message matches
- Use a temporary git repo (via `t.TempDir()` + `git init`) for test isolation

### 6. Add test for structured handoff

- In `internal/engine/prompt_test.go`, add test cases:
  - `TestBuildPrompt_IncludesSpecFile` — verify spec file path appears in prompt when set
  - `TestBuildPrompt_IncludesModifiedFiles` — verify file list appears in prompt when set
  - `TestBuildPrompt_OmitsEmptyHandoffFields` — verify no extra context when fields are empty

## Testing Strategy

**Existing tests must pass unchanged** — the refactoring preserves all external behavior:

- `make check` — full lint + test suite
- Verify TUI behavior is unaffected (commit-after-step is internal to the executor)
- The commit skill's standalone behavior is unchanged (only the auto-commit path changes)

**New tests** target the deterministic commit logic and prompt extension in isolation using temp git repos.

## Risk Assessment

- **Medium risk on commit message quality**: Deterministic commits produce simpler messages than LLM-driven ones. Mitigation: the final `commit` skill (when run explicitly) can squash/reword. The auto-commits are intermediate checkpoints, not final history.
- **Low risk on amend behavior**: The dedup guard only amends when messages are identical, which only happens when the same phase runs twice on unchanged files. Edge case: if a user manually commits between phases, the dedup check compares against the manual commit — this is fine because messages would differ.
- **Low risk on handoff context**: Adding fields to `PromptContext` is additive. Skills that don't use the new fields see no change.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
make check          # Parallel lint + test
make test           # Unit tests only
make lint           # go vet only
```

## Open Questions (Unresolved)

1. **Commit message inference from phase name**: The mapping of skill name → commit type (`build`→`feat`, `review`→`fix`) is a heuristic. Should this be configurable in `agtop.toml` per-phase? **Recommendation**: Start with a hardcoded map in the executor. Add configurability only if the heuristic proves insufficient.

2. **Amend vs. squash for duplicates**: The spec proposes `--amend` for identical messages. An alternative is to skip the commit entirely and let changes accumulate for the next phase's commit. **Recommendation**: Use `--amend` — it preserves the checkpoint semantic while avoiding duplicates.

## Sub-Tasks

The spec has 3 independent optimization areas that can be parallelized:

- **Sub-task A** (Steps 1-2, 5): Deterministic commit — highest cost savings, eliminates 2 Claude invocations per workflow
- **Sub-task B** (Step 2 dedup portion): Commit deduplication guard — 3-line addition, prevents duplicate commits
- **Sub-task C** (Steps 3-4, 6): Structured handoff — medium effort, reduces redundant file reads in downstream skills

Sub-tasks A and B are tightly coupled (B builds on A). Sub-task C is fully independent.
