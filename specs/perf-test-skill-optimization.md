# Perf: Test Skill & Makefile Optimization

## Metadata

type: `perf`
task_id: `test-skill-optimization`
prompt: `Optimize the test skill and testing workflow to reduce execution time`

## Performance Issue Description

The test skill takes too long to complete. Three compounding factors:

1. **Sequential execution** — The skill runs lint → type_check → unit_tests → e2e_tests one at a time. For this Go project, `go vet` and `go test` are independent and can run concurrently.
2. **Discovery overhead** — The skill probes justfile, package.json, Makefile, and pyproject.toml each invocation. This costs LLM inference tokens and time to read files and reason about which commands to use, even though this project always uses the same Makefile targets.
3. **Missing Makefile targets** — The Makefile has `lint` (`go vet ./...`) but no `test` target. The skill must infer `go test ./...` from context instead of reading it directly from the Makefile.

## Baseline Metrics

- **Test categories discovered**: 2 (linting via `make lint`, unit_tests inferred as `go test ./...`)
- **Execution model**: Strictly sequential — lint finishes before tests start
- **Discovery steps**: Skill reads multiple config files (justfile, package.json, Makefile, pyproject.toml) each run to figure out what commands are available

## Target Metrics

- **Execution model**: Parallel — lint and tests run concurrently via a single `make check` invocation
- **Discovery steps**: Skill reads Makefile, immediately finds `check` and `test` targets
- **Overall wall time**: Reduced by running `go vet` and `go test` in parallel (approximately cut in half since both take similar time)

## Relevant Files

- `Makefile` — Currently has `lint` but no `test` or `check` target. Needs new targets added.
- `.claude/skills/test/SKILL.md` — The test skill instructions. Currently mandates sequential execution and deprioritizes combined check commands.

## Optimization Strategy

Two changes, one in each file:

### 1. Add `test` and `check` targets to the Makefile

Add a `test` target that runs `go test ./...` and a `check` target that runs both `go vet` and `go test` in parallel. GNU Make supports parallel execution of prerequisites with `-j`, but the simplest portable approach is to background one process:

```makefile
test:
	go test ./...

check:
	go vet ./... & go test ./... & wait
```

This runs both commands concurrently and waits for both to finish. If either fails, `make check` returns non-zero.

### 2. Update the test skill to run independent tests in parallel

The current skill says:

> Execute each discovered test in **category order** (linting → type check → unit → e2e)

And in the cookbook:

> still prefer running individual test categories separately for granular reporting

Change the skill to:
- Run independent test categories (linting, type_check, unit_tests) **in parallel** using concurrent Bash tool calls
- Only run e2e_tests after unit_tests pass (since e2e tests often depend on the build being correct)
- Prefer a combined `check` command when available, mapping its result to the appropriate test categories in the report

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `test` and `check` targets to Makefile

- In `Makefile`, add a `test` target with recipe `go test ./...`
- Add a `check` target with recipe `go vet ./... & go test ./... & wait`
- Add both `test` and `check` to the `.PHONY` declaration

### 2. Update the test skill to support parallel execution

- In `.claude/skills/test/SKILL.md`, update Step 2 ("Execute Tests in Sequence") to:
  - Rename to "Execute Tests"
  - Instruct the agent to run **linting** and **unit_tests** in parallel (concurrent Bash tool calls)
  - Run **e2e_tests** only after unit_tests pass
  - Keep **stop on first failure** semantics: if any parallel test fails, do not proceed to e2e
- Update the cookbook entry about combined check commands to prefer using them when available, falling back to individual commands only when a combined command is not present
- Update the workflow summary to reflect parallel execution

## Benchmarking Plan

After changes:

1. Run `time make check` to verify both vet and test run in parallel and measure wall time
2. Run `time make lint` and `time make test` separately to confirm parallel savings
3. Invoke the test skill and verify the JSON report is still valid and includes both linting and unit_tests results

## Risk Assessment

- **Low risk**: The Makefile changes are additive — existing `lint` target is unchanged
- **Parallel failure reporting**: If both `go vet` and `go test` fail simultaneously in `make check`, only one failure shows. This is acceptable for a combined check; the skill can fall back to individual commands when `check` fails to get granular error messages
- **Skill behavior change**: Parallel execution changes the order of the JSON report, but the spec already requires sorting by pass/fail status, not execution order

## Validation Commands

```bash
make check
make test
make lint
```

## Open Questions (Unresolved)

None — the changes are straightforward and low-risk.
