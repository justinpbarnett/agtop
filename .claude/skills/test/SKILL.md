---
name: test
description: >
  Execute comprehensive validation tests for the Keep application, returning
  results in a standardized JSON format for automated processing. Runs ESLint
  linting, TypeScript type checking, unit tests, and e2e tests in sequence.
  Use when a user wants to run tests, validate the application, check code
  quality, or verify the app is healthy. Triggers on "run tests", "test the
  app", "validate the application", "run the test suite", "check for errors",
  "run all checks", "is the app healthy". Do NOT use for implementing features
  (use the implement skill). Do NOT use for reviewing against a spec (use the
  review skill). Do NOT use for starting the dev server (use the start skill).
---

# Purpose

Executes the full validation suite for the Keep application — linting, type checking, unit tests, and e2e browser tests — returning results as a standardized JSON report for automated processing.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Confirm Working Directory

Run `pwd` to confirm you are in the project root. If not, navigate to the project root before proceeding.

### Step 2: Execute Tests in Sequence

Execute each test in the order listed below. For each test:

1. Run the command with a **5 minute timeout**
2. Capture the result (passed/failed) and any error output
3. If a test **fails** (non-zero exit code), mark it as failed, capture stderr, and **stop processing immediately** — do not run subsequent tests
4. If a test **passes**, continue to the next test

#### Test 1: Linting

- **Command:** `pnpm lint`
- **test_name:** `linting`
- **test_purpose:** "Validates code quality using ESLint, identifies unused imports, style violations, and potential bugs"

#### Test 2: Type Checking

- **Command:** `pnpm exec tsc --noEmit`
- **test_name:** `type_check`
- **test_purpose:** "Validates TypeScript type annotations and catches type mismatches, missing return types, and incorrect function signatures"

#### Test 3: Unit Tests

- **Command:** `pnpm test`
- **test_name:** `unit_tests`
- **test_purpose:** "Runs the Vitest unit and component test suite"

#### Test 4: E2E Tests

- **Command:** `pnpm test:e2e`
- **test_name:** `e2e_tests`
- **test_purpose:** "Runs Playwright end-to-end browser tests against a local dev server"

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
    "execution_command": "pnpm exec tsc --noEmit",
    "test_purpose": "Validates TypeScript type annotations and catches type mismatches, missing return types, and incorrect function signatures",
    "error": "src/app/page.tsx(42,5): error TS2345: Argument of type 'number' is not assignable to parameter of type 'string'."
  },
  {
    "test_name": "linting",
    "passed": true,
    "execution_command": "pnpm lint",
    "test_purpose": "Validates code quality using ESLint, identifies unused imports, style violations, and potential bugs"
  }
]
```

## Workflow

1. **Confirm** — Verify working directory is the project root
2. **Run** — Execute tests sequentially (lint → typecheck → unit → e2e), stopping on first failure
3. **Report** — Produce a JSON array with results, failed tests sorted to top

## Cookbook

<If: `pnpm` command not found>
<Then: check if pnpm is available with `which pnpm`. If not found, it can be installed with `npm install -g pnpm`. Ensure the shell profile has been reloaded.>

<If: test runner discovers no tests>
<Then: verify test files exist. Check `package.json` for configured test scripts.>

<If: a test fails and subsequent tests are not run>
<Then: this is by design — stop on first failure to avoid cascading noise. Include only executed tests in the report.>

<If: error messages are very long>
<Then: keep them concise but include enough context to locate and fix the issue>

## Validation

Before returning the report:
- Verify the JSON is valid and parseable
- Confirm failed tests are sorted to the top
- Confirm that if any test failed, no subsequent tests were executed
- Verify each `execution_command` can be copy-pasted and run from the project root

## Examples

### Example 1: Run Full Test Suite

**User says:** "Run the tests"

**Actions:**
1. Confirm working directory
2. Execute all 4 tests in sequence
3. Return JSON report

### Example 2: Quick Health Check

**User says:** "Is the app healthy?" or "Check for errors"

**Actions:**
1. Confirm working directory
2. Execute all 4 tests in sequence
3. Return JSON report

### Example 3: Validate Before Merge

**User says:** "Run all checks before I merge"

**Actions:**
1. Confirm working directory
2. Execute all 4 tests in sequence
3. Return JSON report
