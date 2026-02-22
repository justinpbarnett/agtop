# Spec Templates

Use the template matching the task type. Replace every placeholder (wrapped in angle brackets) with specific, researched content.

---

## Shared: Metadata Block

All plans start with this metadata block:

```md
## Metadata

type: `{type}`
adw_id: `{adw_id}`
prompt: `{prompt}`
```

---

## Comprehensive Spec — `feat`

Use for new features. Most detailed template with user stories, phased implementation, and testing strategy.

```md
# Feature: <feature name>

## Metadata

type: `feat`
adw_id: `{adw_id}`
prompt: `{prompt}`

## Feature Description

<describe the feature in detail, including its purpose and value to users>

## User Story

As a <type of user>
I want to <action/goal>
So that <benefit/value>

## Problem Statement

<clearly define the specific problem or opportunity this feature addresses>

## Solution Statement

<describe the proposed solution approach and how it solves the problem>

## Relevant Files

Use these files to implement the feature:

<list files relevant to the feature with bullet points explaining why>

### New Files

<list new files that need to be created with bullet points explaining their purpose>

## Implementation Plan

### Phase 1: Foundation

<describe the foundational work needed before implementing the main feature>

### Phase 2: Core Implementation

<describe the main implementation work for the feature>

### Phase 3: Integration

<describe how the feature will integrate with existing functionality>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

### 2. <Second Task Name>

- <specific action>
- <specific action>

<continue with additional tasks as needed>

## Testing Strategy

### Unit Tests

<describe unit tests needed for the feature>

### Edge Cases

<list edge cases that need to be tested>

## Acceptance Criteria

<list specific, measurable criteria that must be met for the feature to be considered complete>

## Validation Commands

Execute these commands to validate the feature is complete:

<list specific commands to validate the work>
- Example: `just check` — Run full lint, typecheck, and test suite

## Notes

<optional additional context, future considerations, or dependencies>
```

---

## Diagnostic Spec — `fix`

Use for bug fixes. Focuses on reproduction, root cause analysis, and regression prevention.

```md
# Fix: <bug name>

## Metadata

type: `fix`
adw_id: `{adw_id}`
prompt: `{prompt}`

## Bug Description

<describe the bug — what happens vs. what should happen>

## Reproduction Steps

1. <step to reproduce>
2. <step to reproduce>
3. <observe: describe the incorrect behavior>

**Expected behavior:** <what should happen instead>

## Root Cause Analysis

<explain why the bug occurs — trace through the code path and identify the exact failure point>

## Relevant Files

Use these files to fix the bug:

<list files relevant to the fix with bullet points explaining why>

### New Files

<list new files if needed, otherwise remove this section>

## Fix Strategy

<describe the targeted approach to fix the bug without introducing side effects>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

### 2. <Second Task Name>

- <specific action>
- <specific action>

<continue with additional tasks as needed>

## Regression Testing

### Tests to Add

<describe new tests that verify the fix and prevent regression>

### Existing Tests to Verify

<list existing tests that should still pass after the fix>

## Validation Commands

Execute these commands to validate the fix is complete:

<list specific commands to validate the work>
- Example: `just check` — Run full lint, typecheck, and test suite

## Notes

<optional additional context — related bugs, areas of concern, follow-up work>
```

---

## Architectural Spec — `refactor`, `perf`

Use for refactoring and performance improvements. Emphasizes current vs. target state and measurable outcomes.

### For `refactor`:

```md
# Refactor: <refactor name>

## Metadata

type: `refactor`
adw_id: `{adw_id}`
prompt: `{prompt}`

## Refactor Description

<describe what is being refactored and why the current approach is problematic>

## Current State

<describe the current code architecture, patterns, or structure being refactored>

## Target State

<describe what the code should look like after refactoring — the desired architecture, patterns, or structure>

## Relevant Files

Use these files to implement the refactor:

<list files relevant to the refactor with bullet points explaining why>

### New Files

<list new files if needed, otherwise remove this section>

## Migration Strategy

<describe how to move from current state to target state — especially if the change is incremental or requires backwards compatibility>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

### 2. <Second Task Name>

- <specific action>
- <specific action>

<continue with additional tasks as needed>

## Testing Strategy

<describe how to verify that behavior is unchanged after refactoring — existing tests that must still pass, new tests if coverage gaps exist>

## Validation Commands

Execute these commands to validate the refactor is complete:

<list specific commands to validate the work>
- Example: `just check` — Run full lint, typecheck, and test suite

## Notes

<optional additional context — risks, follow-up refactors, deprecation timeline>
```

### For `perf`:

```md
# Perf: <optimization name>

## Metadata

type: `perf`
adw_id: `{adw_id}`
prompt: `{prompt}`

## Performance Issue Description

<describe the performance problem — what is slow, what impact does it have>

## Baseline Metrics

<describe current performance measurements or how to measure them>

- <metric>: <current value or how to obtain it>
- <metric>: <current value or how to obtain it>

## Target Metrics

<describe the performance goals>

- <metric>: <target value>
- <metric>: <target value>

## Relevant Files

Use these files to implement the optimization:

<list files relevant to the optimization with bullet points explaining why>

### New Files

<list new files if needed, otherwise remove this section>

## Optimization Strategy

<describe the approach — what changes will improve performance and why>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

### 2. <Second Task Name>

- <specific action>
- <specific action>

<continue with additional tasks as needed>

## Benchmarking Plan

<describe how to measure the improvement — specific commands, tools, or test scenarios>

## Validation Commands

Execute these commands to validate the optimization is complete:

<list specific commands to validate the work>
- Example: `just check` — Run full lint, typecheck, and test suite

## Notes

<optional additional context — tradeoffs, memory vs speed, follow-up optimizations>
```

---

## Lightweight Spec — `chore`, `docs`, `test`, `build`, `ci`

Use for maintenance tasks, documentation, tests, build changes, and CI updates. Simple and direct.

```md
# <Type>: <task name>

## Metadata

type: `{type}`
adw_id: `{adw_id}`
prompt: `{prompt}`

## Description

<describe the task in detail based on the prompt>

## Relevant Files

Use these files to complete the task:

<list files relevant to the task with bullet points explaining why>

### New Files

<list new files if needed, otherwise remove this section>

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. <First Task Name>

- <specific action>
- <specific action>

### 2. <Second Task Name>

- <specific action>
- <specific action>

<continue with additional tasks as needed>

## Validation Commands

Execute these commands to validate the task is complete:

<list specific commands to validate the work>
- Example: `just check` — Run full lint, typecheck, and test suite

## Notes

<optional additional context or considerations>
```
