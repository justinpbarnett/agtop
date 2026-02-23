---
name: test
description: >
  Discovers and executes the project's validation suite — linting, type checking,
  unit tests, and integration/e2e tests — returning results in a standardized
  JSON format for automated processing. Auto-detects available test commands
  from the project's task runner or package manager. Use when a user wants to
  run tests, validate the application, check code quality, or verify the app
  is healthy. Triggers on "run tests", "test the app", "validate the application",
  "run the test suite", "check for errors", "run all checks", "is the app healthy".
  Do NOT use for implementing features (use the implement skill). Do NOT use for
  reviewing against a spec (use the review skill). Do NOT use for starting the
  dev server (use the start skill).
---

# Purpose

Discovers and executes the project's full validation suite — linting, type checking, unit tests, and integration/e2e tests — returning results as a standardized JSON report for automated processing.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Discover Available Test Commands

Detect which test commands are available by checking these sources in priority order:

1. **justfile** — If a `justfile` exists, look for recipes like `check`, `lint`, `typecheck`, `test`, `test-e2e`
2. **package.json** — If it exists, look for scripts like `lint`, `typecheck`, `test`, `test:e2e`, `test:unit`, `check`
3. **Makefile** — If it exists, look for targets like `check`, `lint`, `test`
4. **pyproject.toml / setup.cfg** — For Python projects, look for test configuration (pytest, ruff, mypy)

Map discovered commands to these test categories:

| Category | What it validates | Common commands |
|----------|-------------------|-----------------|
| `linting` | Code quality, style | `just lint`, `npm run lint`, `make lint`, `ruff check` |
| `type_check` | Type annotations | `just typecheck`, `tsc --noEmit`, `mypy .`, `pyright` |
| `unit_tests` | Unit/component tests | `just test`, `npm test`, `pytest`, `go test ./...` |
| `e2e_tests` | End-to-end tests | `just test-e2e`, `npm run test:e2e`, `playwright test` |

If a category has no discoverable command, skip it. If no test commands are found at all, report this and stop.

### Step 2: Execute Tests in Sequence

Execute each discovered test in category order (linting → type check → unit → e2e). For each test:

1. Run the command with a **5 minute timeout**
2. Capture the result (passed/failed) and any error output
3. If a test **fails** (non-zero exit code), mark it as failed, capture stderr, and **stop processing immediately** — do not run subsequent tests
4. If a test **passes**, continue to the next test

### Step 3: Produce the Report

Return ONLY a JSON array — no surrounding text, markdown formatting, or explanation. The output must be valid for `JSON.parse()`.

- Sort the array with failed tests (`passed: false`) at the top
- Include all executed tests (both passed and failed)
- If a test passed, omit the `error` field
- If a test failed, include the error message in the `error` field
- The `execution_command` field should contain the exact command that can be run to reproduce the test

### Output Structure

```json
[
  {
    "test_name": "string",
    "passed": boolean,
    "execution_command": "string",
    "test_purpose": "string",
    "error": "optional string"
  }
]
```

### Example Output

```json
[
  {
    "test_name": "type_check",
    "passed": false,
    "execution_command": "npx tsc --noEmit",
    "test_purpose": "Validates TypeScript type annotations and catches type mismatches",
    "error": "src/app/page.tsx(42,5): error TS2345: Argument of type 'number' is not assignable to parameter of type 'string'."
  },
  {
    "test_name": "linting",
    "passed": true,
    "execution_command": "npm run lint",
    "test_purpose": "Validates code quality using ESLint"
  }
]
```

## Workflow

1. **Discover** — Detect available test commands from justfile, package.json, Makefile, or language-specific config
2. **Run** — Execute tests sequentially (lint → typecheck → unit → e2e), stopping on first failure
3. **Report** — Produce a JSON array with results, failed tests sorted to top

## Cookbook

<If: no test commands discovered>
<Then: report that no test infrastructure was found. Suggest the user check their project setup or specify commands manually.>

<If: a combined check command exists (e.g., `just check`, `npm run check`)>
<Then: still prefer running individual test categories separately for granular reporting. Only fall back to the combined command if individual commands are not available.>

<If: test runner discovers no tests>
<Then: verify test files exist. Check the project's config for test file patterns.>

<If: a test fails and subsequent tests are not run>
<Then: this is by design — stop on first failure to avoid cascading noise. Include only executed tests in the report.>

<If: error messages are very long>
<Then: keep them concise but include enough context to locate and fix the issue>

<If: Python project detected>
<Then: look for pytest, ruff/flake8, mypy/pyright. Run with appropriate commands (e.g., `pytest`, `ruff check .`, `mypy .`)>

## Validation

Before returning the report:
- Verify the JSON is valid and parseable
- Confirm failed tests are sorted to the top
- Confirm that if any test failed, no subsequent tests were executed
- Verify each `execution_command` can be copy-pasted and run from the project root

## Examples

### Example 1: Node.js Project with justfile

**Discovery:** justfile has `lint`, `typecheck`, `test`, `test-e2e` recipes
**Actions:**
1. Run `just lint`, `just typecheck`, `just test`, `just test-e2e` in sequence
2. Return JSON report

### Example 2: Python Project

**Discovery:** pyproject.toml with ruff + mypy + pytest config
**Actions:**
1. Run `ruff check .`, `mypy .`, `pytest` in sequence
2. Return JSON report

### Example 3: Package.json Only

**Discovery:** package.json has `lint` and `test` scripts
**Actions:**
1. Run `npm run lint`, `npm test` in sequence
2. Return JSON report (no typecheck or e2e — not configured)
