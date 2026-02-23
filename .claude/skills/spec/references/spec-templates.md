# Spec Templates

Use the template matching the task type. Replace every placeholder (wrapped in angle brackets) with specific, researched content.

---

## Shared: Metadata Block

All plans start with this metadata block:

```md
## Metadata

type: `{type}`
task_id: `{task_id}`
prompt: `{prompt}`
```

## Output Templates by Type

Use the template that matches the `type` field in the task spec metadata. Every implementation spec starts with the metadata block carried forward from the task spec, then follows the type-specific structure below.

---

### `feat` — Feature Implementation

```md
# Feature: <feature name>

## Metadata

type: `feat`
task_id: `<from task spec>`
prompt: `<from task spec>`

## Feature Description

<synthesize the problem and desired outcome from the task spec into a clear feature description>

## User Story

As a <type of user>
I want to <action/goal>
So that <benefit/value>

## Relevant Files

<list existing files relevant to the feature with bullet points explaining why>

### New Files

<list new files that need to be created with bullet points explaining their purpose>

## Implementation Plan

### Phase 1: Foundation

<foundational work — schemas, migrations, base classes>

### Phase 2: Core Implementation

<main feature logic>

### Phase 3: Integration

<wiring into existing functionality — routes, UI, events>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action with file path and function name>
- <specific action>

### 2. <Second Task Name>

- <specific action>
- <specific action>

## Testing Strategy

### Unit Tests

<specific test files to create, cases to cover>

### Edge Cases

<edge cases derived from acceptance criteria and constraints>

## Risk Assessment

<what existing functionality could break, migration concerns, rollback strategy>

## Validation Commands

<project's actual check/lint/test commands to verify completion>

## Open Questions (Unresolved)

<questions from the task spec that could not be answered from the codebase — these require a human decision before implementation - include a recommended suggestion for each>

## Sub-Tasks

<if decomposition is needed, list sub-tasks in execution order with scope boundaries — otherwise write "Single task — no decomposition needed.">
```

---

### `fix` — Bug Fix

```md
# Fix: <bug name>

## Metadata

type: `fix`
task_id: `<from task spec>`
prompt: `<from task spec>`

## Bug Description

<what happens vs. what should happen>

## Reproduction Steps

1. <step>
2. <step>
3. <observe: incorrect behavior>

**Expected behavior:** <what should happen>

## Root Cause Analysis

<trace through the code path — identify the exact failure point with file paths and line numbers>

## Relevant Files

<list files relevant to the fix with bullet points explaining why>

## Fix Strategy

<targeted approach to fix without introducing side effects>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action with file path and function name>
- <specific action>

## Regression Testing

### Tests to Add

<new tests that verify the fix and prevent regression>

### Existing Tests to Verify

<existing tests that must still pass>

## Risk Assessment

<what could break, related areas of concern>

## Validation Commands

<project's actual check/lint/test commands>

## Open Questions (Unresolved)

<unresolved questions requiring human decision with a suggested recommendation>
```

---

### `refactor` — Refactoring

```md
# Refactor: <refactor name>

## Metadata

type: `refactor`
task_id: `<from task spec>`
prompt: `<from task spec>`

## Refactor Description

<what is being refactored and why the current approach is problematic>

## Current State

<current code architecture, patterns, or structure — with file paths>

## Target State

<desired architecture, patterns, or structure after refactoring>

## Relevant Files

<list files with bullet points explaining why>

### New Files

<new files if needed>

## Migration Strategy

<how to move from current to target state — backwards compatibility, incremental steps>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

## Testing Strategy

<how to verify behavior is unchanged — existing tests that must pass, new tests for coverage gaps>

## Risk Assessment

<what could break, deployment concerns>

## Validation Commands

<project's actual check/lint/test commands>

## Open Questions (Unresolved)

<unresolved questions requiring human decision with a suggested recommendation>

## Sub-Tasks

<decomposition if needed>
```

---

### `perf` — Performance Optimization

```md
# Perf: <optimization name>

## Metadata

type: `perf`
task_id: `<from task spec>`
prompt: `<from task spec>`

## Performance Issue Description

<what is slow and what impact it has>

## Baseline Metrics

- <metric>: <current value or how to measure>
- <metric>: <current value or how to measure>

## Target Metrics

- <metric>: <target value>
- <metric>: <target value>

## Relevant Files

<list files with bullet points explaining why>

## Optimization Strategy

<what changes will improve performance and why>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

## Benchmarking Plan

<how to measure improvement — specific commands, tools, test scenarios>

## Risk Assessment

<tradeoffs — memory vs speed, complexity vs performance, what could regress>

## Validation Commands

<project's actual check/lint/test commands>

## Open Questions (Unresolved)

<unresolved questions requiring human decision with a suggested recommendation>
```

---

### `chore`, `docs`, `test`, `build`, `ci` — Lightweight Tasks

```md
# <Type>: <task name>

## Metadata

type: `<type>`
task_id: `<from task spec>`
prompt: `<from task spec>`

## Description

<synthesize the task from the spec — what needs to happen and why>

## Relevant Files

<list files with bullet points explaining why>

### New Files

<new files if needed>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

## Validation Commands

<project's actual check/lint/test commands>

## Notes

<additional context or considerations>
```
