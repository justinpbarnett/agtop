# Feature: Multi-Repo Project Directory Support

## Metadata

type: `feat`
task_id: `multi-repo-support`
prompt: `The agtop application should be able to work in a directory with a two or more repo structure. The most common use-case will be having 'agtop' run in the root of a project and that project is also a singular git repo. There are some cases though where 'agtop' will be run in a root project dir that has no git repo because there might be an app/client repo and an app/server repo and an app/db repo that all live in the dir. have the project be able to handle this use case.`

## Feature Description

Currently, agtop assumes the working directory is the root of a single git repository. The `WorktreeManager`, `DiffGenerator`, `Pipeline`, and app initialization all take a single `repoRoot` path and call git commands against it. If agtop is launched from a non-git directory that contains multiple sub-repos (e.g., `app/client/`, `app/server/`, `app/db/`), worktree creation fails immediately because there is no git repo at the project root.

This feature adds multi-repo awareness so agtop can:

1. **Auto-discover** git repositories in immediate subdirectories when the project root is not itself a git repo.
2. **Create worktrees per-repo** — each run gets a worktree in every discovered sub-repo, and the agent's working directory is set to the project root (with sub-repo worktrees mounted at the same relative paths).
3. **Merge per-repo** — accepting a run merges each sub-repo's worktree branch back into its respective main branch.
4. **Generate diffs per-repo** — diffs are concatenated across all sub-repos for the detail panel.
5. **Degrade gracefully** — the common single-repo case works exactly as before with zero config changes.

## User Story

As a developer running agtop in a project directory that contains multiple git repositories (e.g., `client/`, `server/`, `db/`)
I want agtop to automatically discover those repos and create isolated worktrees in each one
So that agent runs can modify code across all repos and I can accept/reject changes as a single unit

## Relevant Files

- `internal/git/worktree.go` — `WorktreeManager` currently takes a single `repoRoot`. Needs to support multiple repo roots and create/remove/merge/list worktrees across all of them. The core abstraction boundary.
- `internal/git/diff.go` — `DiffGenerator` takes a single `repoRoot` and diffs against `main`. Needs to diff across multiple worktrees and concatenate results.
- `internal/ui/app.go` — `NewApp()` (line 90) creates a single `WorktreeManager` and `DiffGenerator` from `projectRoot`. The `StartRunMsg` handler (line 397) calls `a.worktrees.Create(runID)` and stores a single `wtPath` and `branch`. The accept handler (line 921) calls `MergeWithOptions` on the single worktree manager.
- `internal/engine/pipeline.go` — `Pipeline` stores a single `repoRoot` (line 23) and uses it for rebase, push, and conflict resolution. Needs to iterate over repos.
- `internal/run/run.go` — `Run` struct has single `Worktree` and `Branch` fields (lines 29-30). In multi-repo mode, each run needs to track multiple worktree paths and branches.
- `internal/config/config.go` — `ProjectConfig` has a single `Root` field (line 29). May need optional `repos` field for explicit multi-repo configuration.
- `internal/engine/executor.go` — Sets `opts.WorkDir = r.Worktree` when running skills (line 421). In multi-repo mode, the working directory should be the project root (parent of all worktrees).
- `cmd/agtop/cleanup.go` — Creates a single `WorktreeManager` (line 36) for orphan cleanup. Needs to clean up across all repos.
- `internal/engine/prompt.go` — `PromptContext` includes `WorkDir` and `Branch`. May need to list all repo branches.
- `internal/run/persistence.go` — Serializes `Run` to JSON including `Worktree` and `Branch` fields. Needs to handle multi-worktree serialization.

### New Files

None — all changes are to existing files. The multi-repo logic is an extension of the existing `WorktreeManager` abstraction.

## Implementation Plan

### Phase 1: Repo Discovery

Add a function to detect whether the project root is a git repo or a multi-repo parent:

- If `projectRoot` contains `.git`, use the existing single-repo behavior (backward compatible).
- Otherwise, scan immediate subdirectories for `.git` dirs. If any are found, enter multi-repo mode.
- Store the discovered repo list in `WorktreeManager` so it can operate on each one.
- Add an optional `repos` field to `ProjectConfig` so users can explicitly list sub-repos (overriding auto-discovery).

### Phase 2: Multi-Repo WorktreeManager

Extend `WorktreeManager` to handle multiple repositories:

- `Create(runID)` creates a worktree in each discovered repo, staging them under a unified directory structure (e.g., `.agtop/worktrees/<runID>/client/`, `.agtop/worktrees/<runID>/server/`).
- `Remove(runID)` removes worktrees from all repos.
- `Merge(runID)` and `MergeWithOptions(runID)` merge in each repo.
- `List()` aggregates worktrees from all repos.
- Return a composite worktree path (the parent directory containing all sub-worktrees) so the agent sees the same directory structure as the original project.

### Phase 3: Run State Extension

Extend the `Run` struct to track multiple worktrees/branches:

- Add `Worktrees map[string]string` (repo-name → worktree path) and `Branches map[string]string` (repo-name → branch) fields.
- Keep the existing `Worktree` and `Branch` fields for backward compatibility (single-repo mode).
- In single-repo mode, these new fields are `nil` and behavior is unchanged.

### Phase 4: Integration Wiring

Wire multi-repo support into the UI, executor, pipeline, and diff generator:

- `DiffGenerator.Diff()` / `DiffStat()`: iterate over sub-repos and concatenate diffs with repo-name headers.
- `Pipeline`: iterate over repos for rebase/push/merge operations.
- `Executor`: set `opts.WorkDir` to the composite worktree root (not an individual repo).
- `PromptContext`: list all branches when in multi-repo mode.
- Cleanup: iterate over all repos when removing orphaned worktrees.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add Repo Discovery to git Package

- In `internal/git/worktree.go`, add a `DiscoverRepos(projectRoot string) ([]string, error)` function that:
  - Checks if `projectRoot` itself is a git repo (has `.git`). If so, returns `[]string{projectRoot}`.
  - Otherwise, scans immediate subdirectories for `.git` dirs and returns their paths.
  - Returns an error only if the scan itself fails — an empty result is valid (no repos found).
- Add a `repos` field to `WorktreeManager` struct (type `[]string`) populated by discovery or explicit config.
- Update `NewWorktreeManager` to accept a `[]string` of repo roots instead of a single root. When the slice has one element, behavior is identical to today.

### 2. Extend WorktreeManager.Create for Multi-Repo

- When `len(w.repos) > 1`, create a composite worktree directory at `.agtop/worktrees/<runID>/` (under the project root, not a repo root).
- For each repo, create a git worktree at `.agtop/worktrees/<runID>/<repo-relative-path>/` with branch `agtop/<runID>`.
- Use symlinks or direct worktree paths so the composite directory mirrors the original project structure.
- Return the composite root path as the worktree path and a comma-separated branch list (or the branch name, since it's the same across repos).
- When `len(w.repos) == 1`, use the existing single-repo logic unchanged.

### 3. Extend WorktreeManager.Remove and List for Multi-Repo

- `Remove(runID)`: iterate over all repos and remove the worktree + branch in each.
- `List()`: aggregate worktree listings from all repos, grouping by run ID.
- Clean up the composite directory after removing all repo worktrees.

### 4. Extend WorktreeManager.Merge for Multi-Repo

- `MergeWithOptions(runID)`: iterate over all repos and merge each one. If any repo fails, abort all and return the error.
- Handle golden file resolution per-repo.
- Stash/unstash logic must operate per-repo.

### 5. Extend DiffGenerator for Multi-Repo

- Add a `repos` field to `DiffGenerator` (or accept the worktree manager).
- `Diff()` and `DiffStat()`: when in multi-repo mode, generate diffs for each repo's worktree and concatenate with headers like `=== client ===`.

### 6. Add Optional repos Config

- In `internal/config/config.go`, add `Repos []string` to `ProjectConfig`.
- In `internal/config/loader.go`, merge the `Repos` field.
- When `Repos` is set, use it instead of auto-discovery. When empty, auto-discover.

### 7. Update App Initialization

- In `internal/ui/app.go` `NewApp()`, call `DiscoverRepos(projectRoot)` (or use `cfg.Project.Repos` if set).
- Pass the repo list to `NewWorktreeManager` and `NewDiffGenerator`.
- The rest of the app code interacts with the same `WorktreeManager` interface — no changes needed to run creation flow.

### 8. Update Run Struct for Multi-Repo State

- Add `Worktrees map[string]string` and `Branches map[string]string` to `Run` struct with `json:"worktrees,omitempty"` and `json:"branches,omitempty"` tags.
- In `StartRunMsg` handler, populate these fields in multi-repo mode.
- Keep existing `Worktree` and `Branch` fields populated with the composite path / primary branch for backward compatibility with the UI and executor.

### 9. Update Pipeline for Multi-Repo

- In `Pipeline`, store the repo list instead of a single `repoRoot`.
- In `Pipeline.Run()`, iterate repos for rebase, push, and merge stages.
- In `ResolveConflictsInMerge`, operate per-repo.

### 10. Update Cleanup for Multi-Repo

- In `cmd/agtop/cleanup.go`, discover repos and create a `WorktreeManager` with the full list.
- Cleanup iterates all repos for orphaned worktrees.

### 11. Update Prompt Context

- In `PromptContext`, when multi-repo mode is active, include a note listing the sub-repos and their branches so the agent knows the directory structure.

## Testing Strategy

### Unit Tests

- `internal/git/worktree_test.go`:
  - Test `DiscoverRepos` with: single repo dir, multi-repo dir, empty dir, non-git dir.
  - Test `Create`/`Remove`/`List` in multi-repo mode using temp dirs with multiple git repos.
  - Test `MergeWithOptions` across multiple repos.
- `internal/git/diff_test.go`:
  - Test multi-repo diff concatenation.
- `internal/config/loader_test.go`:
  - Test `Repos` field merging.

### Edge Cases

- **No git repos found** — agtop should show a clear error message rather than silently failing.
- **Mixed mode** — project root is a git repo AND has sub-repos (some projects use git submodules). This should use single-repo mode (the root repo).
- **Partial merge failure** — if one repo's merge fails, all merges should be aborted/rolled back.
- **Sub-repo added/removed between runs** — handle gracefully when rehydrating sessions.
- **Deeply nested repos** — only scan immediate subdirectories, not recursive.
- **Single repo (regression)** — verify all existing single-repo behavior is unchanged.

## Risk Assessment

- **High risk: Merge atomicity** — merging across multiple repos is not atomic. If repo A merges successfully but repo B fails, we need rollback logic for repo A. This is the most complex part of the implementation.
- **Medium risk: Backward compatibility** — the `Run.Worktree` and `Run.Branch` fields are serialized to session files. Changing their semantics could break rehydration of existing sessions. Mitigation: keep these fields populated and add new fields alongside them.
- **Low risk: Performance** — discovering repos on startup adds negligible latency (just `os.ReadDir` + `os.Stat` calls).

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go vet ./...
go test ./...
```

## Open Questions (Unresolved)

1. **Should worktrees be created under the project root or under each repo?**
   - Recommendation: Under the project root (`.agtop/worktrees/<runID>/`) with the composite structure. This keeps the worktree directory structure in one place and mirrors the original project layout, which is what the agent needs to see.

2. **Should `agtop init` work in multi-repo mode?**
   - Recommendation: Yes — `agtop init` should create `.agtop/hooks/` at the project root and configure `.claude/settings.json` at the project root (not in each sub-repo). The safety guard applies project-wide.

3. **Should runs be able to target a specific sub-repo instead of all repos?**
   - Recommendation: Not in v1. Start with all-repos-per-run. If needed later, add a `--repo` flag or config option. This keeps the initial implementation simpler.

4. **How should the dev server work in multi-repo mode?**
   - Recommendation: The dev server command runs from the composite worktree root, same as single-repo mode. If a project needs per-repo dev servers, that's a separate feature.

## Sub-Tasks

Single task — no decomposition needed. The changes are interconnected (WorktreeManager changes flow into every consumer) and should be implemented as a unit to avoid intermediate broken states.
