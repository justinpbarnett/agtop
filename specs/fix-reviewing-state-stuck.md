# Fix: Runs stuck in "reviewing" state after successful review

## Metadata

type: `fix`
task_id: `reviewing-state-stuck`
prompt: `investigate worktree/run 002. why did it not complete? seems to be caught in a reviewing status. find and fix the underlying issue.`

## Bug Description

Runs that use workflows ending with the "review" skill (e.g., `plan-build`: `[spec, build, test, review]`) always transition to `StateReviewing` after the review skill completes, regardless of the review outcome. The run appears "stuck" because it requires manual user intervention (accept/reject) even when the review found no blockers and reported `success: true`.

**What happens:** After the review skill finishes with a passing report (`{"success": true, ...}`), the run transitions to `StateReviewing` and stops. The user must manually press `a` (accept) to advance it.

**What should happen:** If the review skill reports `success: true` (no unfixed blockers), the run should auto-advance to `StateCompleted`. Only when `success: false` (unfixed blockers requiring human judgment) should it remain in `StateReviewing`.

## Reproduction Steps

1. Start agtop and create a new run using the `plan-build` workflow (skills: `[spec, build, test, review]`)
2. Let the workflow execute all four skills to completion
3. The review skill produces a JSON report with `"success": true`
4. Observe: the run transitions to `StateReviewing` instead of `StateCompleted`

**Expected behavior:** The run should transition to `StateCompleted` when the review passes.

## Root Cause Analysis

The bug is in `internal/engine/executor.go:931-940`, the `terminalState()` function:

```go
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
```

This function only inspects the **skill name list** — it does not consider the review skill's output. It unconditionally returns `StateReviewing` when the last skill is `"review"`, even when the review passed with no blockers.

The review skill produces a structured JSON report (per `skills/review/references/output-schema.json`) with a `success` boolean:
- `success: true` — no unfixed blocker issues; the run is ready to merge
- `success: false` — unfixed blockers exist; human review is needed

The executor stores the review output in `previousOutput` (`executor.go:465`) but never passes it to `terminalState()`.

Call site at `executor.go:524`:
```go
finalState := terminalState(skills)
```

The `previousOutput` variable containing the review JSON is available in scope but unused.

## Relevant Files

- `internal/engine/executor.go` — `terminalState()` function (line 931) and its call site (line 524). The core fix location.
- `internal/engine/executor_test.go` — `TestTerminalStateReview` (line 339) and `TestTerminalStateCompleted` (line 346) need updates. `TestExecutorResumeReconnectedSkillIndex` (line 528) asserts `StateReviewing` and will need updating.
- `skills/review/references/output-schema.json` — Defines the review report schema with the `success` field.
- `internal/run/run.go` — State constants. No changes needed.

## Fix Strategy

Modify `terminalState()` to accept the last skill's output text and parse the review result. If the review reports `success: true`, return `StateCompleted`. Otherwise return `StateReviewing`.

This is a minimal, targeted change:
1. Add a `lastOutput` parameter to `terminalState()`
2. Add a `reviewPassed()` helper that parses the review JSON
3. Update the call site to pass `previousOutput`
4. Update tests

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `reviewPassed` helper to `executor.go`

- Add a function `reviewPassed(output string) bool` in `internal/engine/executor.go` near `terminalState()`
- It should `json.Unmarshal` the output into a struct with a `Success bool` field
- The review output may contain non-JSON text before/after the JSON body (LLM prose), so try to extract the first `{...}` block if direct unmarshalling fails
- Return `true` only if parsing succeeds and `Success` is `true`
- Return `false` for any parse failure (conservative: assume review needs human attention if output is malformed)

### 2. Update `terminalState` signature and logic

- Change signature from `terminalState(skills []string) run.State` to `terminalState(skills []string, lastOutput string) run.State`
- When the last skill is `"review"`, call `reviewPassed(lastOutput)`:
  - If `true`, return `run.StateCompleted`
  - If `false`, return `run.StateReviewing`

### 3. Update the call site in `executeWorkflow`

- At line 524 of `executor.go`, change:
  ```go
  finalState := terminalState(skills)
  ```
  to:
  ```go
  finalState := terminalState(skills, previousOutput)
  ```

### 4. Update existing tests

- In `internal/engine/executor_test.go`:
  - Update `TestTerminalStateReview` to pass an empty/failing review output as the second arg, confirming it still returns `StateReviewing`
  - Update `TestTerminalStateCompleted` to pass empty string as second arg
  - Update `TestExecutorResumeReconnectedSkillIndex` — the `completingRuntime` mock outputs `"ok"` as result text (not valid review JSON), so it should still reach `StateReviewing`

### 5. Add new tests for `reviewPassed` and updated `terminalState`

- `TestReviewPassedSuccess`: valid JSON with `"success": true` → returns `true`
- `TestReviewPassedFailure`: valid JSON with `"success": false` → returns `false`
- `TestReviewPassedMalformed`: non-JSON input → returns `false`
- `TestReviewPassedEmpty`: empty string → returns `false`
- `TestReviewPassedWithProse`: JSON embedded in LLM prose → returns `true` if success is true
- `TestTerminalStateReviewPassed`: skills ending with "review" + passing output → `StateCompleted`
- `TestTerminalStateReviewFailed`: skills ending with "review" + failing output → `StateReviewing`

## Regression Testing

### Tests to Add

- `TestReviewPassedSuccess` — verifies `true` for `{"success": true, "review_summary": "...", "review_issues": [], "screenshots": []}`
- `TestReviewPassedFailure` — verifies `false` for `{"success": false, ...}`
- `TestReviewPassedMalformed` — verifies `false` for `"not json"`
- `TestReviewPassedEmpty` — verifies `false` for `""`
- `TestReviewPassedWithProse` — verifies extraction from `"Here is my review:\n{\"success\": true, ...}\n"`
- `TestTerminalStateReviewPassed` — verifies `StateCompleted` for passing review
- `TestTerminalStateReviewFailed` — verifies `StateReviewing` for failing review

### Existing Tests to Verify

- `TestTerminalStateCompleted` — must still pass (non-review workflows unaffected)
- `TestExecutorShutdownPreservesRunState` — must still pass (shutdown behavior unchanged)
- `TestExecutorResumeReconnectedSkillIndex` — mock produces `"ok"` result text which is not valid review JSON, so `reviewPassed` returns `false` and terminal state remains `StateReviewing`. This test should still pass after updating the `terminalState` call signature.
- All tests in `internal/engine/` — `go test ./internal/engine/...`

## Risk Assessment

**Low risk.** The change is minimal and well-scoped:
- Only affects the terminal state decision when the last skill is "review"
- Conservative fallback: malformed/missing review output defaults to `StateReviewing` (current behavior), so no existing runs break
- Non-review workflows are completely unaffected (the `last == "review"` check is unchanged)
- The `StateReviewing` path still works for genuinely failed reviews

**Edge case:** If the review skill crashes or times out, `previousOutput` will be whatever the last successful event text was (likely empty or from a prior skill). The `reviewPassed` function returns `false` for non-JSON input, so the run correctly stays in `StateReviewing`.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go vet ./internal/engine/...
go test ./internal/engine/...
go test ./...
```

## Open Questions (Unresolved)

- **Should auto-completed runs also auto-commit?** Currently `commitAfterStep` runs after every modifying skill including "review". If the review is the last skill and the run auto-completes, the commit has already happened. No additional work needed. Recommendation: no change needed.
- **Should auto-completed runs trigger the merge pipeline?** Currently the merge pipeline only triggers on `handleAccept()` (user presses `a`). Auto-completing to `StateCompleted` means the user still needs to accept to merge. This seems correct — completing means "review passed, ready for your decision" vs reviewing means "review failed, needs attention." Recommendation: keep as-is for now; auto-merge on completion could be a separate feature.
