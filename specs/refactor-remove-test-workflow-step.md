# Refactor: Remove test skill as a separate workflow step

## Metadata

type: `refactor`
task_id: `remove-test-workflow-step`
prompt: `Remove the test skill from all default workflows and have the build skill run the full validation suite as its final step instead. The test skill remains available for standalone use but is no longer a workflow step.`

## Refactor Description

The `test` skill currently runs as a dedicated step in every workflow that includes `build` — specifically `build`, `plan-build`, and `sdlc`. In practice, the `build` skill's agent already runs tests while implementing (especially when creating new tests as part of a spec), making the separate `test` step redundant. The `test` step spins up a fresh agent session that re-discovers the project, re-reads files, and re-runs the same test suite — burning tokens and time for little additional value.

This refactor consolidates test responsibility into the `build` skill and removes `test` from all default workflow definitions. The `test` skill itself is preserved for standalone invocation (e.g., "run tests").

## Current State

### Workflow definitions (`internal/config/defaults.go:28-33`)

```go
"build":      {Skills: []string{"build", "test"}},
"plan-build": {Skills: []string{"spec", "build", "test", "review"}},
"sdlc":       {Skills: []string{"spec", "decompose", "build", "test", "review", "document"}},
```

### Build skill instructions (`skills/build/SKILL.md`, `.claude/skills/implement/SKILL.md`)

Explicitly tells the agent NOT to run the full test suite:
- Line 13: `"Do NOT run the full test suite — the test skill handles validation."`
- Line 18: `"Does NOT run the full test/lint suite — that responsibility belongs to the test skill which runs as a separate workflow step."`
- Line 59: `"Do NOT run the full test or lint suite — that is the test skill's job."`
- Line 68: `"Do NOT run the full test/lint/check suite — the test skill handles that as the next workflow step"`

### Spec templates (`skills/spec/references/spec-templates.md`, `.claude/skills/spec/references/spec-templates.md`)

Five occurrences of:
> `NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.`

### Decompose skill (`skills/decompose/SKILL.md`, `.claude/skills/decompose/SKILL.md`)

Line 124:
> `NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.`

### Review skill (`skills/review/SKILL.md`, `.claude/skills/review/SKILL.md`, `.agtop/skills/document/SKILL.md`)

References "test skill handles that" as justification for not running tests itself.

### Example config (`agtop.example.toml:32-38`)

All three workflows include `"test"` in their skill lists.

### Project config (`agtop.toml:42,75-76`)

```toml
order = ["spec", "build", "test", "review"]
standard = ["spec", "build", "test"]
full = ["spec", "build", "test", "review"]
```

## Target State

### Workflows no longer include `test`

```go
"build":      {Skills: []string{"build"}},
"plan-build": {Skills: []string{"spec", "build", "review"}},
"sdlc":       {Skills: []string{"spec", "decompose", "build", "review", "document"}},
```

### Build skill runs the full validation suite as its final step

After all tasks are implemented, the build skill runs the project's full validation suite (e.g., `make check`, `go test ./...`), diagnoses and fixes any failures (up to 3 attempts), then reports.

### Spec templates and decompose skill updated

The "Validation Commands" notes are updated to reflect that the build skill now runs validation, not a separate test step.

### Review skill unchanged

The review skill's note about not running tests is still correct — it delegates to whatever ran before it in the workflow (now the build skill).

### Test skill preserved

The `test` skill itself is not deleted. It remains in the skills map and is available for standalone use. Its `SkillConfig` entry in defaults is preserved.

## Relevant Files

- `internal/config/defaults.go` — Default workflow definitions (lines 28-33) and test skill config (line 39)
- `agtop.example.toml` — Example config showing workflows (lines 32-38) and test skill config (lines 57-59)
- `agtop.toml` — Project config with phases and workflows (lines 42, 75-76)
- `skills/build/SKILL.md` — Build skill instructions (lines 13, 18, 59, 68) — embedded skills dir
- `.claude/skills/implement/SKILL.md` — Build skill instructions (Claude-facing copy, lines 13, 18, 59, 68)
- `skills/spec/references/spec-templates.md` — Validation notes in all 5 templates
- `.claude/skills/spec/references/spec-templates.md` — Validation notes (Claude-facing copy)
- `skills/decompose/SKILL.md` — Validation note (line 124)
- `.claude/skills/decompose/SKILL.md` — Validation note (Claude-facing copy)
- `skills/review/SKILL.md` — Reference to test skill (line 20)
- `.claude/skills/review/SKILL.md` — Reference to test skill (Claude-facing copy)
- `.agtop/skills/review/SKILL.md` — Reference to test skill (project copy)
- `internal/config/loader_test.go` — Tests that assert default workflow count (line 27)

## Migration Strategy

No backwards compatibility concerns. Users who have customized their `agtop.toml` workflows to include `test` will continue to work — the `test` skill config still exists. This only changes the built-in defaults and the skill instructions.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Update default workflow definitions

- In `internal/config/defaults.go`, remove `"test"` from the `"build"`, `"plan-build"`, and `"sdlc"` workflow skill lists
- Keep the `"test"` entry in the `Skills` map (line 39) — it is still a valid skill for standalone use

### 2. Update the build skill to run full validation

- In `skills/build/SKILL.md`:
  - Remove all "Do NOT run the full test suite" instructions (lines 13, 18, 59, 68)
  - Add a new Step 4 (renumber existing Step 4 to Step 5): **"Run Validation Suite"** — after all tasks are implemented, run the project's full validation suite (`make check` or equivalent). If any tests fail, diagnose and fix (up to 3 attempts). Only report completion after validation passes or max attempts are exhausted.
  - Update the description frontmatter to remove the "Do NOT run the full test suite" clause
- In `.claude/skills/implement/SKILL.md`: apply the same changes (this is the Claude-facing copy)

### 3. Update spec templates

- In `skills/spec/references/spec-templates.md`: update all 5 "Validation Commands" notes from `"These commands are run by the **test skill**, not the build skill"` to `"The build skill runs these commands as its final validation step."`
- In `.claude/skills/spec/references/spec-templates.md`: apply the same changes

### 4. Update decompose skill

- In `skills/decompose/SKILL.md` line 124: update the validation note to match the new wording
- In `.claude/skills/decompose/SKILL.md` line 124: apply the same change

### 5. Update review skill references

- In `skills/review/SKILL.md` line 20: change `"Does NOT run the test/lint suite — the test skill handles that as a separate workflow step."` to `"Does NOT run the test/lint suite — the build skill handles validation before review runs."`
- In `.claude/skills/review/SKILL.md`: apply the same change
- In `.agtop/skills/review/SKILL.md`: apply the same change (if it exists as a separate copy)

### 6. Update example and project configs

- In `agtop.example.toml`: remove `"test"` from the `build`, `plan-build`, and `sdlc` workflow skill lists (lines 32-38)
- In `agtop.toml`: remove `"test"` from `phases.order`, remove the `[phases.test]` section, and remove `"test"` from the `standard` and `full` workflow lists

### 7. Update tests

- In `internal/config/loader_test.go` line 27: the assertion `len(cfg.Workflows) != 5` should still pass (workflow count unchanged — we're modifying contents, not removing workflows)
- Run `make check` to verify all tests pass

## Testing Strategy

- `make check` must pass — this validates both `go vet` and `go test ./...`
- `TestValidateDefaultConfig` must pass — confirms default config is internally consistent (all workflow skill references resolve)
- `TestLoadDefaults` must pass — confirms expected workflow count is still 5
- `TestValidateWorkflowMissingSkill` must still pass — the `test` skill entry remains in the skills map

## Risk Assessment

**Low risk.** This is a content-only change to skill instructions and default config values. No Go logic changes beyond the defaults map. Users with custom `agtop.toml` workflows that include `test` are unaffected — the skill config entry is preserved.

The main risk is that the build skill's agent might not consistently run the full test suite even with explicit instructions. However, in practice agents already do this — the current "do NOT run tests" instruction is what prevents it.

## Validation Commands

```bash
make check
```

## Open Questions (Unresolved)

None — the approach is straightforward and was discussed before speccing.

## Sub-Tasks

Single task — no decomposition needed.
