---
name: test
description: >
  Discovers and executes the project's validation suite — linting, type checking,
  unit tests, and integration/e2e tests — fixing any failures found, then returning
  results in a standardized JSON format for automated processing. Auto-detects
  available test commands from the project's task runner or package manager. Use
  when a user wants to run tests, validate the application, check code quality,
  or verify the app is healthy. Triggers on "run tests", "test the app", "validate
  the application", "run the test suite", "check for errors", "run all checks",
  "is the app healthy". Do NOT use for implementing features (use the implement
  skill). Do NOT use for reviewing against a spec (use the review skill). Do NOT
  use for starting the dev server (use the start skill).
---

# Purpose

Discovers and executes the project's full validation suite — linting, type checking, unit tests, and integration/e2e tests. If any tests fail, diagnoses and fixes the issues, then re-runs to confirm the fix. Returns results as a standardized JSON report for automated processing.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Discover Available Test Commands

Read all config files in **parallel** (single batch of tool calls): check for `justfile`, `package.json`, `Makefile`, and `pyproject.toml` / `setup.cfg` simultaneously. From the results, identify available commands:

1. **justfile** — look for recipes like `check`, `lint`, `typecheck`, `test`, `test-e2e`
2. **package.json** — look for scripts like `lint`, `typecheck`, `test`, `test:e2e`, `test:unit`, `check`
3. **Makefile** — look for targets like `check`, `lint`, `test`
4. **pyproject.toml / setup.cfg** — For Python projects, look for test configuration (pytest, ruff, mypy)

Map discovered commands to these test categories:

| Category | What it validates | Common commands |
|----------|-------------------|-----------------|
| `linting` | Code quality, style | `just lint`, `npm run lint`, `make lint`, `ruff check` |
| `type_check` | Type annotations | `just typecheck`, `tsc --noEmit`, `mypy .`, `pyright` |
| `unit_tests` | Unit/component tests | `just test`, `npm test`, `pytest`, `go test ./...` |
| `e2e_tests` | End-to-end tests | `just test-e2e`, `npm run test:e2e`, `playwright test` |

If a category has no discoverable command, skip it. If no test commands are found at all, report this and stop.

### Step 2: Execute Tests

If a combined `check` command is available (e.g., `make check`, `just check`, `npm run check`), prefer it — it runs all independent checks in one shot. Map its result to the appropriate test categories in the report.

Otherwise, run independent test categories in **parallel** using concurrent Bash tool calls:

- Launch **linting** and **unit_tests** simultaneously (concurrent Bash tool calls)
- Wait for both to complete
- If either fails, mark it as failed and **do not proceed to e2e_tests**
- Run **e2e_tests** only after unit_tests pass (e2e tests often depend on a correct build)
- Run **type_check** in parallel with linting and unit_tests when available

For each test:

1. Run the command with a **5 minute timeout**
2. Capture the result (passed/failed) and any error output
3. If any parallel test **fails** (non-zero exit code), mark it as failed, capture stderr, and do not proceed to e2e_tests

### Step 3: Fix Failures (if any)

If all tests passed, skip to Step 4.

If any tests failed, fix them:

1. Read the error output carefully — identify the root cause (broken test, lint violation, type error, or a real bug in the implementation)
2. Fix the issue:
   - **Lint/type errors** — fix the code to satisfy the linter or type checker
   - **Test failures from implementation bugs** — fix the implementation code, not the test assertions
   - **Test failures from outdated test expectations** — update the test to match the new correct behavior (e.g., golden files, snapshot assertions, changed return values)
3. Re-run **only the failed test category** to confirm the fix
4. If it passes, re-run the full suite to check for regressions
5. If it still fails, repeat from sub-step 1

**Maximum 3 fix attempts.** If tests still fail after 3 rounds of fixes, stop and include the remaining failures in the report.

### Step 4: Produce the Report

Return ONLY a JSON array — no surrounding text, markdown formatting, or explanation. The output must be valid for `JSON.parse()`.

- Sort the array with failed tests (`passed: false`) at the top
- Include all executed tests (both passed and failed)
- If a test passed, omit the `error` field
- If a test failed, include the error message in the `error` field
- If a test was fixed during Step 3, add `"fixed": true` to the entry
- The `execution_command` field should contain the exact command that can be run to reproduce the test

### Output Structure

```json
[
  {
    "test_name": "string",
    "passed": boolean,
    "execution_command": "string",
    "test_purpose": "string",
    "error": "optional string",
    "fixed": "optional boolean"
  }
]
```

### Example Output

```json
[
  {
    "test_name": "unit_tests",
    "passed": true,
    "execution_command": "go test ./...",
    "test_purpose": "Validates all unit tests pass",
    "fixed": true
  },
  {
    "test_name": "linting",
    "passed": true,
    "execution_command": "make lint",
    "test_purpose": "Validates code quality using go vet"
  }
]
```

## Workflow

1. **Discover** — Detect available test commands from justfile, package.json, Makefile, or language-specific config
2. **Run** — Prefer a combined `check` command when available; otherwise run linting, type_check, and unit_tests in parallel, then e2e_tests if all pass
3. **Fix** — If any tests failed, diagnose and fix (up to 3 attempts), re-running after each fix
4. **Report** — Produce a JSON array with results, failed tests sorted to top

## Cookbook

<If: no test commands discovered>
<Then: report that no test infrastructure was found. Suggest the user check their project setup or specify commands manually.>

<If: a combined check command exists (e.g., `make check`, `just check`, `npm run check`)>
<Then: prefer running the combined command — it handles parallelism internally and reduces overhead. Map its pass/fail result to the relevant test categories in the report.>

<If: test runner discovers no tests>
<Then: verify test files exist. Check the project's config for test file patterns.>

<If: error messages are very long>
<Then: keep them concise but include enough context to locate and fix the issue>

<If: Python project detected>
<Then: look for pytest, ruff/flake8, mypy/pyright. Run with appropriate commands (e.g., `pytest`, `ruff check .`, `mypy .`)>

<If: a fix introduces new failures>
<Then: this counts as one of the 3 fix attempts. Read the new error carefully — it may indicate the previous fix was wrong. Revert if needed and try a different approach.>

<If: golden file / snapshot mismatches>
<Then: check if the project has an update command (e.g., `make update-golden`, `npm test -- -u`). If the new output is correct, regenerate the snapshots. If the output is wrong, fix the code.>

## Validation

Before returning the report:
- Verify the JSON is valid and parseable
- Confirm failed tests are sorted to the top
- Verify each `execution_command` can be copy-pasted and run from the project root

## Examples

### Example 1: Go project — all tests pass

**Discovery:** Makefile has `check`, `lint`, `test` targets. `check` runs lint + tests together.
**Actions:**
1. `make check` is available — run it as a single command
2. All tests pass — no fixes needed
3. Return JSON report

```json
[
  {
    "test_name": "linting + unit_tests",
    "passed": true,
    "execution_command": "make check",
    "test_purpose": "Runs go vet and go test in parallel"
  }
]
```

### Example 2: Go project — test failure, fixed

**Discovery:** Makefile has `check` target.
**Actions:**
1. Run `make check` — unit test fails: `TestFoo expected "bar" got "baz"`
2. Read the failing test and the implementation. The implementation changed the return value correctly but the test expectation is outdated
3. Update the test assertion to match the new behavior
4. Re-run `make check` — all pass
5. Return JSON report with `"fixed": true`

### Example 3: Node.js project — lint failure, fixed

**Discovery:** package.json has `lint` and `test` scripts.
**Actions:**
1. Run `npm run lint` and `npm test` in parallel — lint fails with unused import
2. Remove the unused import
3. Re-run `npm run lint` — passes
4. Re-run full suite to confirm no regressions
5. Return JSON report

### Example 4: Unfixable failure after 3 attempts

**Discovery:** Makefile has `check` target.
**Actions:**
1. Run `make check` — test fails
2. Attempt fix 1 — still fails (different error)
3. Attempt fix 2 — still fails
4. Attempt fix 3 — still fails
5. Return JSON report with `"passed": false` and the error details
