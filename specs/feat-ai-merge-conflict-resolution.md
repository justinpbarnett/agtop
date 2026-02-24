# Feature: AI-Assisted Merge Conflict Resolution in Legacy Accept Flow

## Metadata

type: `feat`
task_id: `ai-merge-conflict-resolution`
prompt: `Harden the accept and PR merge process. When auto-merge doesn't work and there's conflicts, have AI handle the merge — resolving conflicts intelligently instead of failing.`

## Feature Description

When a user presses `a` to accept a completed run, there are two merge paths:

1. **Pipeline flow** (`auto_merge = true`): Already has AI conflict resolution via `Pipeline.resolveConflicts()` — rebases onto the target, invokes the build skill to resolve non-golden conflicts, and retries up to 3 times.

2. **Legacy flow** (`auto_merge = false`): Calls `WorktreeManager.MergeWithOptions()` which only auto-resolves `.golden` test snapshot files. Any non-golden conflict causes the entire merge to abort, transitioning the run to `StateFailed` with no recovery path.

The gap is in the **legacy flow**. When a user isn't using the full pipeline (no GitHub remote, no CI, or just wants a local merge), non-golden conflicts cause a hard failure. The user has to manually resolve conflicts outside agtop.

This feature adds AI-assisted conflict resolution to the legacy merge path by:

1. Detecting non-golden conflicts during the legacy `WorktreeManager.Merge` flow
2. Invoking an AI agent (via the build skill) to resolve the conflict markers
3. Completing the merge if the agent succeeds, or aborting cleanly if it fails
4. Showing merge progress status in the UI so the user knows what's happening

## User Story

As a developer using agtop without auto_merge (no CI/GitHub pipeline)
I want merge conflicts to be resolved by AI when I press accept
So that I don't have to manually resolve conflicts outside agtop

## Relevant Files

- `internal/git/worktree.go` — `MergeWithOptions()` at line 83 is the legacy merge entry point. Currently aborts on non-golden conflicts (lines 132-139). Needs to return conflict info so the caller can invoke AI resolution.
- `internal/ui/app.go` — `handleAccept()` at line 921 dispatches to either pipeline or legacy flow. The legacy path (lines 958-977) needs to handle the new "conflicts need AI resolution" case.
- `internal/engine/pipeline.go` — `resolveConflicts()` at line 196 already implements AI conflict resolution for the pipeline path. The conflict resolution logic should be extracted or reused.
- `internal/engine/executor.go` — Houses `runSkill()` which is needed to invoke the build skill for conflict resolution.
- `internal/run/run.go` — Run struct with `MergeStatus` field (line 49), already supports merge status tracking.
- `internal/config/config.go` — `MergeConfig` at line 85, may need a new field to control AI resolution behavior.

### New Files

None — all changes are to existing files.

## Implementation Plan

### Phase 1: Extend WorktreeManager to Return Conflict Details

Instead of aborting immediately on non-golden conflicts, `MergeWithOptions` should return a structured error that includes the list of conflicted files. This allows the caller (app.go) to decide whether to invoke AI resolution.

### Phase 2: Extract Reusable Conflict Resolution

Extract the AI conflict resolution logic from `Pipeline.resolveConflicts()` into a standalone function that can be called from both the pipeline and legacy paths. The core logic is: get conflicted files → auto-resolve golden files → invoke build skill on remaining files → stage and complete.

### Phase 3: Wire into Legacy Accept Flow

Update `handleAccept()` to detect merge conflicts in the legacy path, show merge status ("resolving-conflicts"), invoke the extracted AI resolution, and either complete the merge or fail gracefully.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `MergeConflictError` type to `internal/git/worktree.go`

- Create a new exported error type `MergeConflictError` with fields: `Branch string`, `ConflictedFiles []string`, `Output string`
- Implement the `Error() string` method on it
- This allows callers to type-assert and extract conflict details

```go
type MergeConflictError struct {
    Branch          string
    ConflictedFiles []string
    Output          string
}

func (e *MergeConflictError) Error() string {
    return fmt.Sprintf("merge %s: %d conflicted files: %s", e.Branch, len(e.ConflictedFiles), e.Output)
}
```

### 2. Update `MergeWithOptions` to return `MergeConflictError` instead of aborting

- In `internal/git/worktree.go`, modify the merge failure path (lines 123-139)
- When there are remaining non-golden conflicts, instead of aborting the merge, abort and return a `*MergeConflictError` with the list of conflicted files
- The caller can then type-assert `errors.As(err, &conflictErr)` to detect this case
- Keep the existing behavior for the golden-only path (all golden conflicts resolved → commit → succeed)

### 3. Add `ResolveConflicts` method to `Pipeline` that works standalone

- In `internal/engine/pipeline.go`, create a new exported method `ResolveConflictsInMerge(ctx, runID, worktree, conflictedFiles)` that:
  - Auto-resolves golden files from the list using `gitpkg.ResolveGoldenConflictsFromList`
  - If only golden files remain, stages and returns
  - For non-golden files, invokes the build skill with a conflict resolution prompt
  - Stages resolved files with `git add -A`
  - Does NOT call `rebase --continue` (this is for merge conflicts, not rebase)
  - Returns error if resolution fails
- This is separate from the existing `resolveConflicts()` which is rebase-specific (calls `rebase --continue`)

### 4. Update legacy accept flow in `internal/ui/app.go`

- In `handleAccept()`, modify the legacy merge goroutine (lines 962-974)
- After `MergeWithOptions` fails, check if the error is a `*MergeConflictError`
- If it is:
  1. Set `MergeStatus` to `"resolving-conflicts"` and `State` to `StateMerging` so the UI shows progress
  2. Call `Pipeline.ResolveConflictsInMerge()` with the conflicted files
  3. If resolution succeeds, complete the merge with `git commit --no-edit` in the repo root
  4. If resolution fails, abort the merge (`git merge --abort`) and set `StateFailed` with error details
- If the error is NOT a `MergeConflictError`, fail as before
- Ensure `a.pipeline` and `a.executor` are available even when `auto_merge = false` (they may need to be initialized unconditionally in app setup)

### 5. Ensure executor/pipeline are initialized even without `auto_merge`

- Check `internal/ui/app.go` initialization to verify that the executor and pipeline are created even when `config.Merge.AutoMerge` is `false`
- If they're currently gated behind `AutoMerge`, move their initialization to be unconditional so the legacy flow can use the build skill for conflict resolution
- If the pipeline is nil, the legacy flow should fall back to failing as before (no regression)

### 6. Add `ConflictResolutionAttempts` config option

- In `internal/config/config.go`, add `ConflictResolutionAttempts int` to `MergeConfig` with TOML tag `"conflict_resolution_attempts"`
- Default to `3` in `internal/config/defaults.go` (add a `Merge` section to `DefaultConfig()`)
- Use this value in `ResolveConflictsInMerge` instead of hardcoding `3`
- Update `agtop.example.toml` with the new option (commented out with default)

### 7. Add tests

- In `internal/git/worktree_test.go`:
  - Test that `MergeConflictError` is returned when non-golden conflicts exist
  - Test that golden-only conflicts still auto-resolve and succeed
  - Test that the error includes the correct list of conflicted files

- In `internal/engine/pipeline_test.go`:
  - Test `ResolveConflictsInMerge` with golden-only conflict list (no agent needed)
  - Test that non-golden conflicts invoke the build skill (mock or integration)

## Testing Strategy

### Unit Tests

- `internal/git/worktree_test.go` — Test `MergeConflictError` type assertion, test that `MergeWithOptions` returns it for non-golden conflicts
- `internal/engine/pipeline_test.go` — Test `ResolveConflictsInMerge` golden-only path, test the method exists and accepts correct parameters

### Edge Cases

- Merge with zero conflicts (fast-forward) — should work as before
- Merge with only golden conflicts — should auto-resolve without AI
- Merge with mixed golden + non-golden conflicts — golden auto-resolved, AI handles the rest
- AI resolution fails on first attempt — retry up to configured limit
- AI resolution fails on all attempts — abort merge cleanly, preserve run state
- `pipeline` is nil (no executor configured) — fall back to old behavior (fail on conflict)
- Rebase conflicts in pipeline path — existing behavior unchanged

## Risk Assessment

- **Low risk**: The legacy flow currently hard-fails on non-golden conflicts. This feature adds a recovery path — worst case, it still fails (no regression).
- **Medium risk**: The executor/pipeline initialization change could affect startup if done incorrectly. Mitigate by checking for nil before using.
- **Low risk**: The `MergeConflictError` type change is additive — existing callers that don't type-assert will see the same error behavior (the error message is similar).

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go vet ./...
go test ./internal/git/...
go test ./internal/engine/...
go test ./internal/ui/...
go test ./...
```

## Open Questions (Unresolved)

1. **Should the legacy flow use the merge state UI?** Recommendation: Yes — set `StateMerging` and `MergeStatus` during AI resolution so the user sees progress. The detail panel already renders these states.

2. **Should there be a config toggle to disable AI conflict resolution?** Recommendation: No — the feature only activates when conflicts exist and an executor is available. If the user doesn't want it, they can reject the run instead of accepting. The `conflict_resolution_attempts` config set to `0` could serve as a disable toggle.

## Sub-Tasks

Single task — no decomposition needed.
