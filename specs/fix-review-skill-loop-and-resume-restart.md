# Fix: Review Skill Loop and Resume Restart

## Metadata

type: `fix`
task_id: `review-skill-loop-and-resume-restart`
prompt: `Investigate the review skill (both .agtop and .claude versions should be the same and both should be updated). When doing a run lately it seemed to get put into a loop and re-running and re-trying to commit things. Also pausing and resuming the run actually even ended up putting it back into a spec state and almost restarting the whole run.`

## Bug Description

Two distinct bugs in the workflow execution system:

**Bug A — Review skill internal loop and spurious commits:** The review skill is configured with read-only tools (`Read, Grep, Glob`) in `internal/config/defaults.go:41`, but both its SKILL.md content (Step 7: "Fix Blocker and Tech Debt Issues") and the task override in `internal/engine/prompt.go:25` instruct it to "Fix any blocker and tech_debt issues found." This causes the underlying Claude agent to repeatedly attempt to call Write/Edit tools that are denied, burning through turns in an internal retry loop. After the review skill eventually exits (timeout or max turns), `commitAfterStep` fires unnecessarily because `isNonModifyingSkill` does not include "review".

**Bug B — Resume resets SkillIndex, restarting from spec:** When a paused or failed run is resumed via `Executor.Resume()`, `executeWorkflow` is called with a *subset* of the skill list (sliced from the resume point). Inside the loop, `SkillIndex` is set to `i + 1` where `i` is relative to that subset — not the full workflow. This overwrites the absolute SkillIndex with a low relative value. If the run pauses/fails again and Resume is called a second time, the low SkillIndex maps to an early position in the full workflow, causing skills like "spec" to re-run.

## Reproduction Steps

### Bug A — Review loop

1. Start an agtop run with a workflow that includes review (e.g., `plan-build` = ["spec", "build", "test", "review"])
2. Let it reach the review skill
3. Observe: the review agent loops internally trying to fix issues with denied tools, consuming turns/tokens
4. After review completes, `commitAfterStep` runs even though review made no file changes

**Expected behavior:** Review should be read-only — it should classify issues and produce a report, but NOT attempt to fix them. No commit should run after a read-only skill.

### Bug B — Resume restart

1. Start an agtop run with workflow `plan-build` (skills: ["spec", "build", "test", "review"])
2. Wait until skill 3 (test) is running — SkillIndex = 3
3. Pause the run (Space)
4. Resume the run (Space)
5. If the underlying process died while paused (timeout, SIGSTOP'd process timeout), the run transitions to failed
6. Resume again (Space) — executor.Resume slices skills from startIdx = 3-1 = 2, passing ["test", "review"]
7. `executeWorkflow` sets SkillIndex = 1 for "test" (overwriting the absolute index 3)
8. If paused/failed again, Resume calculates startIdx = 1-1 = 0, restarting from "spec"

**Expected behavior:** Resume should always restart from the correct absolute position in the workflow, regardless of how many times it's been resumed.

## Root Cause Analysis

### Bug A: Review skill tool/instruction mismatch

**File:** `internal/config/defaults.go:41`
```go
"review": {Model: "opus", AllowedTools: []string{"Read", "Grep", "Glob"}},
```

**File:** `internal/engine/prompt.go:25`
```go
"review": "Review the implemented changes against the spec to verify correctness and completeness. Fix any blocker and tech_debt issues found. Produce the structured review report.",
```

**File:** `skills/review/SKILL.md` (and `.claude/skills/review/SKILL.md`) — Step 7 instructs the agent to fix blocker and tech_debt issues.

The review skill's SKILL.md and task override both tell the agent to fix issues, but the tool configuration only allows read operations. The Claude agent receives contradictory instructions: "fix this" but "you can only read." It enters an internal loop attempting writes that get denied.

Additionally, in `internal/engine/executor.go:870-876`:
```go
func isNonModifyingSkill(name string) bool {
    switch name {
    case "route", "decompose":
        return true
    }
    return false
}
```

Review is not listed, so `commitAfterStep` runs after review even when it shouldn't modify files.

### Bug B: SkillIndex relative vs absolute mismatch

**File:** `internal/engine/executor.go:152-196` — `Resume()` slices the full skill list to get `remainingSkills` and passes the subset to `executeWorkflow`.

**File:** `internal/engine/executor.go:374-516` — `executeWorkflow` sets `SkillIndex = i + 1` at line 403 where `i` is the loop index within the *received* skills slice:
```go
e.store.Update(runID, func(r *run.Run) {
    r.SkillIndex = i + 1  // i is relative to the subset, not absolute
    r.CurrentSkill = skillName
    r.State = run.StateRunning
})
```

When `Resume()` passes `skills[2:]` (e.g., ["test", "review"]), the loop starts at i=0 and sets SkillIndex=1. This overwrites the previously-correct absolute SkillIndex of 3. On a subsequent Resume, `startIdx = 1 - 1 = 0` maps back to "spec" in the full workflow.

The same bug exists in `ResumeReconnected()` at lines 201-232 — identical logic, same SkillIndex reset problem.

## Relevant Files

- `internal/engine/executor.go` — Core workflow execution, Resume/ResumeReconnected, `executeWorkflow`, `isNonModifyingSkill`, `commitAfterStep`
- `internal/engine/prompt.go` — `skillTaskOverrides` map with review's fix-encouraging task override
- `internal/config/defaults.go` — Default skill configs including review's AllowedTools
- `skills/review/SKILL.md` — Built-in review skill (embedded at compile time)
- `.claude/skills/review/SKILL.md` — Project-level review skill (Claude Code picks this up)
- `.agtop/skills/document/SKILL.md` — Exists; `.agtop/skills/review/SKILL.md` does NOT exist yet (needs to be created as a copy of the review skill)

## Fix Strategy

### Bug A: Make review read-only

1. **Remove the fix step from the review SKILL.md** — Change Step 7 from "Fix Blocker and Tech Debt Issues" to "Report Issues" only. The review skill should classify and document issues but never attempt to fix them. Fixing should be a separate follow-up or a different skill.
2. **Update the task override in prompt.go** — Remove "Fix any blocker and tech_debt issues found" from the review override.
3. **Add "review" to `isNonModifyingSkill`** — Prevents `commitAfterStep` from running after review (since it can't modify files).
4. **Also add "document" to `isNonModifyingSkill`** — Document is similarly configured as read-only (`AllowedTools: ["Read", "Write", "Grep", "Glob"]`), but even if it writes docs, the commit step already handles it. Actually, document has Write access, so leave it as-is. Only add "review".
5. **Update all three copies of the review skill** — `skills/review/SKILL.md`, `.claude/skills/review/SKILL.md`, and create `.agtop/skills/review/SKILL.md` (all identical).

### Bug B: Fix SkillIndex calculation in Resume

1. **Pass an offset to `executeWorkflow`** so it can compute absolute SkillIndex — Add a `startOffset int` parameter to `executeWorkflow`. Inside the loop, set `SkillIndex = startOffset + i + 1` instead of `i + 1`.
2. **Apply the same fix to `ResumeReconnected`** — Same offset calculation.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Fix SkillIndex calculation in executeWorkflow (Bug B)

- In `internal/engine/executor.go`, change the `executeWorkflow` signature to accept a `startOffset int` parameter:
  ```go
  func (e *Executor) executeWorkflow(ctx context.Context, runID string, skills []string, userPrompt string, startOffset int)
  ```
- Update the SkillIndex assignment at line ~403 to:
  ```go
  r.SkillIndex = startOffset + i + 1
  ```
- Update the SkillTotal assignment when the route skill overrides the workflow (line ~485) — reset startOffset to 0 since the workflow is restarted:
  ```go
  skills = newSkills
  i = -1
  startOffset = 0
  ```

### 2. Update all callers of executeWorkflow

- `Execute()` (line ~107): pass `startOffset: 0`
- `Resume()` (line ~191): pass `startOffset: startIdx`
- `ResumeReconnected()` (line ~229): pass `startOffset: startIdx`

### 3. Remove fix instructions from review SKILL.md (Bug A)

- In `skills/review/SKILL.md`:
  - **Step 7**: Replace the entire "Fix Blocker and Tech Debt Issues" section. Change it to a reporting-only step that documents the issues without attempting fixes. Remove all instructions about fixing code, staging files, or marking issues as `"fixed": true`.
  - **Step 8 (Report)**: Remove the `"fixed"` field guidance. Issues are reported but not fixed by review.
  - **Workflow summary**: Remove "Fix" from step 7 description
  - **Validation section**: Remove "All fixable blocker and tech_debt issues have been resolved"
  - **Examples**: Update examples to remove fix actions; review only classifies and reports
- Copy the updated file identically to `.claude/skills/review/SKILL.md`
- Create `.agtop/skills/review/SKILL.md` as an identical copy

### 4. Update the review task override in prompt.go

- In `internal/engine/prompt.go`, change the review entry in `skillTaskOverrides`:
  ```go
  "review": "Review the implemented changes against the spec to verify correctness and completeness. Classify any issues found by severity. Produce the structured review report.",
  ```

### 5. Add review to isNonModifyingSkill

- In `internal/engine/executor.go`, update `isNonModifyingSkill`:
  ```go
  func isNonModifyingSkill(name string) bool {
      switch name {
      case "route", "decompose", "review":
          return true
      }
      return false
  }
  ```

### 6. Update the review output schema

- In `skills/review/references/output-schema.json`: remove the `"fixed"` field from the issue schema since review no longer fixes issues. Issues are report-only.
- Copy the updated file to `.claude/skills/review/references/output-schema.json`
- Create `.agtop/skills/review/references/` directory and copy there too

### 7. Sync .agtop review skill references

- Create `.agtop/skills/review/` directory
- Copy `skills/review/SKILL.md` → `.agtop/skills/review/SKILL.md`
- Create `.agtop/skills/review/references/` directory
- Copy `skills/review/references/output-schema.json` → `.agtop/skills/review/references/output-schema.json`
- Copy `skills/review/references/severity-guide.md` → `.agtop/skills/review/references/severity-guide.md`

## Regression Testing

### Tests to Add

- **`internal/engine/executor_test.go`**: Add a test for Resume that verifies SkillIndex is preserved correctly across multiple resume cycles. Simulate: run 4-skill workflow, fail at skill 3, resume, fail at skill 3 again, resume — verify it starts at skill 3 both times (not skill 1).
- **`internal/engine/executor_test.go`**: Add a test for `isNonModifyingSkill("review")` returning true.
- **`internal/engine/prompt_test.go`**: Verify the review task override does not contain "fix" or "Fix".

### Existing Tests to Verify

- `go test ./internal/engine/...` — All existing executor, prompt, workflow, skill, and registry tests must pass.
- `go test ./internal/config/...` — Config defaults tests must pass.
- `go test ./...` — Full test suite.

## Risk Assessment

- **Review no longer fixes issues**: This changes the review skill's behavior. Workflows relying on review to fix blocker issues will now get `success: false` reports with unfixed issues. This is intentional — the review skill should observe, not modify. If auto-fixing is desired, it should be a separate "fix" skill in the workflow.
- **SkillIndex change**: The offset calculation changes how SkillIndex is set during execution. All callers must be updated consistently. The route skill's workflow-override path needs to reset the offset to 0.
- **Skill file sync**: Three copies of the review skill (built-in, .claude, .agtop) must remain identical. If any diverge, behavior will differ depending on precedence.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go vet ./...
go test ./internal/engine/...
go test ./...
```

## Open Questions (Unresolved)

- **Should a separate "fix" skill exist?** If the review skill no longer fixes issues, workflows that want auto-fixing need an alternative. Recommendation: defer this to a future feature spec. For now, review reports issues and the user handles fixes via follow-up prompts or manual intervention.
- **Should the `"fixed"` field be removed from the JSON schema entirely or kept as always-false?** Recommendation: remove it entirely. Consumers of the review report should not expect issues to be auto-fixed.
