# Fix: Merge fails on binary golden file conflicts during accept

## Metadata

type: `fix`
task_id: `merge-golden-file-conflict`
prompt: `Accept run fails with merge conflict on binary .golden test snapshot files. Git cannot auto-merge binary files, so any golden file divergence between main and the worktree branch causes a fatal error.`

## Bug Description

When accepting a run, the merge step fails if both main and the worktree branch have modified the same `.golden` test snapshot file. Git reports a binary merge conflict and the accept flow aborts, leaving the run in a failed state. The user sees:

```
Error: merge failed: merge agtop/001: warning: Cannot merge binary files:
internal/ui/panels/testdata/TestNewRunModalDefaultSnapshot.golden (HEAD vs. agtop/001)
CONFLICT (content): Merge conflict in ...
Automatic merge failed; fix conflicts and then commit the result.: exit status 1
```

**What happens:** `WorktreeManager.Merge()` runs `git merge branch`, the merge hits a binary conflict on `.golden` files, the method aborts the merge and returns an error. The accept flow sets the run to `StateFailed`.

**What should happen:** Golden file conflicts should be auto-resolved by taking the incoming branch version (the branch has code changes + matching golden snapshots), then optionally regenerating golden files post-merge to ensure consistency with the merged codebase.

## Reproduction Steps

1. Start agtop, create a run that modifies a UI panel (e.g., the new run modal)
2. While the run is in progress, accept a different run (or manually make changes on main) that also modifies the same panel's golden file
3. When the first run completes, press `a` to accept
4. Observe: merge fails with binary conflict on the `.golden` file

**Expected behavior:** The merge succeeds. Golden file conflicts are auto-resolved by preferring the branch version. After merge, golden files are regenerated if a post-merge command is configured.

## Root Cause Analysis

1. `.gitattributes` (line 1) declares `**/testdata/**/*.golden binary` — git treats golden files as binary
2. Binary files cannot be three-way merged — git always reports a conflict when both sides modify the same binary file
3. `WorktreeManager.Merge()` (`internal/git/worktree.go:70-85`) runs a plain `git merge branch`. On any conflict it aborts and returns an error with no attempt to resolve
4. The legacy accept path (`internal/ui/app.go:833`) calls `worktrees.Merge(runID)` and treats any error as fatal
5. The pipeline rebase path (`internal/engine/pipeline.go:162-193`) delegates conflict resolution to an agent via `resolveConflicts()`, but the agent cannot resolve binary file conflicts since there are no conflict markers to edit

The core problem: there is no mechanism to auto-resolve binary file conflicts. Golden files are a special case where the branch version is always preferred during merge (the branch generated them from its code), and they can be regenerated post-merge to account for combined changes.

## Relevant Files

- `internal/git/worktree.go` — `Merge()` (line 70) is where the conflict occurs. This is the primary fix location. Needs conflict detection and golden-file auto-resolution logic.
- `internal/git/worktree_test.go` — `TestWorktreeMergeConflict()` (line 159) tests the current abort-on-conflict behavior. Needs new tests for golden file auto-resolution.
- `.gitattributes` — Declares golden files as binary (line 1). Read-only context.
- `internal/ui/app.go` — `handleAccept()` (line 792) calls `Merge()` in the legacy path. May need to handle the post-merge golden regeneration step.
- `internal/engine/pipeline.go` — `resolveConflicts()` (line 195) handles rebase conflicts in the auto-merge pipeline. Needs to auto-resolve binary golden files before invoking the agent.
- `internal/config/config.go` — `MergeConfig` (line 82) holds merge settings. Needs a new field for the post-merge golden update command.
- `Makefile` — `update-golden` target (line 22) runs `go test ./internal/ui/... -update`. This is the command to regenerate golden files.

## Fix Strategy

### Part 1: Auto-resolve golden file conflicts in `Merge()`

After a failed `git merge`, detect conflicted files. If any are `.golden` files (matching the `**/testdata/**/*.golden` pattern), auto-resolve them by checking out the incoming branch version (`git checkout --theirs <file>`) and staging them. If all conflicts are resolved, complete the merge commit. If non-golden conflicts remain, abort as before.

### Part 2: Auto-resolve golden files in pipeline `resolveConflicts()`

Before invoking the build skill agent to resolve rebase conflicts, auto-resolve any `.golden` files by checking out the incoming version. Only invoke the agent if non-golden conflicts remain. This avoids wasting agent tokens on files it can't meaningfully resolve.

### Part 3: Post-merge golden regeneration (optional config)

Add a `golden_update_command` field to `MergeConfig`. When set (e.g., `"go test ./internal/ui/... -update"`), run it after a successful merge that involved golden file auto-resolution. If the command produces changes, stage and amend the merge commit. This ensures golden files match the combined codebase rather than just the branch's version.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `GoldenUpdateCommand` to `MergeConfig`

In `internal/config/config.go`:

- Add `GoldenUpdateCommand string \`toml:"golden_update_command"\`` to the `MergeConfig` struct

### 2. Add golden file conflict resolution to `WorktreeManager.Merge()`

In `internal/git/worktree.go`:

- After a failed `git merge`, instead of immediately aborting, get the list of conflicted files with `git diff --name-only --diff-filter=U`
- Identify which conflicted files are golden files (path contains `/testdata/` and ends with `.golden`)
- For each golden file: run `git checkout --theirs <file>` then `git add <file>`
- After resolving golden files, check if unresolved conflicts remain (`git diff --name-only --diff-filter=U` again)
- If no conflicts remain: complete the merge with `git commit --no-edit`
- If non-golden conflicts remain: abort the merge as before and return the error
- Return a boolean or metadata indicating whether golden files were auto-resolved (for the post-merge regeneration step)

The updated method signature should remain `Merge(runID string) error` for backward compatibility, but add a new method `MergeWithOptions(runID string, opts MergeOptions) (MergeResult, error)` that returns resolution details. Have `Merge()` call `MergeWithOptions()` internally.

### 3. Add `MergeOptions` and `MergeResult` types

In `internal/git/worktree.go`:

- Add `MergeOptions` struct with a `GoldenUpdateCommand string` field
- Add `MergeResult` struct with `GoldenFilesResolved []string` field listing which golden files were auto-resolved

### 4. Auto-resolve golden files in pipeline `resolveConflicts()`

In `internal/engine/pipeline.go`, in `resolveConflicts()`:

- Before invoking the build skill agent, get the list of conflicted files
- Auto-resolve any golden files by checking out their incoming version and staging
- Re-check for remaining conflicts
- If no conflicts remain, skip the agent invocation and continue the rebase
- If non-golden conflicts remain, invoke the agent as before (with the golden files already resolved)

### 5. Wire post-merge golden regeneration into accept flow

In `internal/ui/app.go`:

- In the legacy merge path of `handleAccept()`, use `MergeWithOptions()` with the configured `GoldenUpdateCommand`
- After a successful merge where golden files were auto-resolved: if `GoldenUpdateCommand` is set, run it in the repo root. If it produces changes (`git status --porcelain`), stage all and amend the merge commit

In `internal/git/worktree.go`:

- Add a `RunGoldenUpdate(command string) error` method on `WorktreeManager` that runs the command in the repo root, stages any changes, and amends the last commit if there are changes

### 6. Update tests

In `internal/git/worktree_test.go`:

- Add `TestWorktreeMergeGoldenFileConflict`: creates a repo with a `.golden` file in a `testdata/` directory, diverges on main and worktree, verifies merge succeeds with the branch version
- Add `TestWorktreeMergeMixedConflict`: golden + non-golden conflicts, verifies merge still fails (only golden auto-resolved, non-golden blocks)
- Add `TestWorktreeMergeGoldenUpdateCommand`: tests post-merge golden regeneration
- Update `TestWorktreeMergeConflict` to verify it still correctly fails for non-golden conflicts

In `internal/engine/pipeline_test.go`:

- Add test for golden file auto-resolution in rebase conflict flow (if test infrastructure supports it; the pipeline tests may be integration-level)

## Regression Testing

### Tests to Add

- `TestWorktreeMergeGoldenFileConflict` — verifies binary golden file conflicts are auto-resolved
- `TestWorktreeMergeMixedConflict` — verifies non-golden conflicts still cause failure
- `TestWorktreeMergeGoldenUpdateCommand` — verifies post-merge command runs and amends

### Existing Tests to Verify

- `TestWorktreeMergeConflict` — must still fail for non-golden text file conflicts
- `TestWorktreeMerge` — clean merge must still work unchanged
- `TestWorktreeCreate`, `TestWorktreeRemove` — unaffected, verify no regression
- All UI panel teatest snapshot tests (`make update-golden` may need to run if test infra is affected)

## Risk Assessment

- **Low risk**: The auto-resolution only applies to `.golden` files in `testdata/` directories. Non-golden conflicts are still treated as fatal errors.
- **Edge case**: If both main and the branch changed the same panel's test code AND golden file, the branch's golden snapshot may not match the merged test code. The `golden_update_command` mitigates this, but if not configured, the user will need to run `make update-golden` manually.
- **Pipeline path**: Changes to `resolveConflicts()` reduce agent invocations for golden-only conflicts, saving cost. If golden resolution fails mid-rebase, the existing abort logic still applies.

## Validation Commands

```bash
go vet ./...
go test ./internal/git/... -v
go test ./internal/engine/... -v
go test ./internal/ui/... -v
make update-golden
go test ./internal/ui/... -v  # verify golden files match after update
```

## Open Questions (Unresolved)

1. **Should `golden_update_command` be auto-detected from the Makefile?** The `update-golden` target already exists. We could check for it automatically instead of requiring config. **Recommendation**: Keep it as an explicit config option for now. Auto-detection can be added later.

2. **Should the pipeline path also run `golden_update_command` after rebase?** The pipeline already pushes and creates a PR, so regenerated golden files would appear in CI. **Recommendation**: Yes — run the golden update after rebase conflict resolution in the pipeline too, before pushing. This avoids CI failures from stale golden files.
