---
name: route
description: >
  Analyzes the user's prompt and project context to select the most appropriate
  workflow. Returns a workflow name that the executor uses to override the
  current skill sequence. Use as the first skill in workflows that need
  intelligent routing based on task complexity.
model: haiku
allowed-tools: []
---

# Purpose

Analyzes a user's task description and selects the most appropriate workflow for execution. Acts as a lightweight triage step at the start of a workflow pipeline.

## Variables

- `argument` — The user's task prompt describing what they want to accomplish.

## Instructions

### Step 1: Select Workflow

Choose the best workflow based on these criteria:

| Workflow       | When to Use                                                                                           |
| -------------- | ----------------------------------------------------------------------------------------------------- |
| **quick-fix**  | Small, well-defined changes: typo fixes, one-line bugs, config tweaks, simple additions. No spec needed. |
| **build**      | Standard development tasks: adding features, fixing non-trivial bugs, moderate refactors. Clear enough to implement without a detailed spec. |
| **plan-build** | Tasks that benefit from upfront planning: new features with design decisions, multi-file changes, tasks where the approach isn't obvious. |
| **sdlc**       | Large, complex features: multi-component work, features needing decomposition, work that should be reviewed and documented. |

**Decision heuristic:**

- If it can be done in under 5 minutes with obvious changes → `quick-fix`
- If the task is clear but involves real implementation work → `build`
- If you'd want to think through the approach first → `plan-build`
- If it's a significant feature spanning many files/components → `sdlc`

When in doubt, prefer a simpler workflow. It's better to under-plan a small task than over-plan it.

### Step 4: Output

Return **only** the workflow name as plain text on a single line. No explanation, no JSON, no markdown — just the workflow name.

**CRITICAL:** You must always output a workflow name. Never ask for clarification, never report errors, never explain missing files. If anything is unclear, pick the best workflow based on what you do know and output it.

## Workflow

1. **Read** — Parse the user's task prompt
2. **Decide** — Select the appropriate workflow
3. **Output** — Print the workflow name

## Cookbook

<If: user says "fix typo", "quick change", "one-liner", or similar>
<Then: return `quick-fix`>

<If: user says "add feature", "implement", "build", or describes a clear task>
<Then: return `build`>

<If: user says "plan", "design", "spec", or the task has unclear requirements>
<Then: return `plan-build`>

<If: user describes a large feature, multi-phase work, or says "full lifecycle">
<Then: return `sdlc`>

<If: task is ambiguous and could go either way>
<Then: prefer the simpler workflow. `build` is a safe default.>

<If: referenced files (specs, configs, etc.) are missing or don't exist>
<Then: ignore the missing files and route based on the task description alone. Output a workflow name — never report the missing file.>

## Examples

### Example 1: Simple Bug Fix

**Prompt:** "Fix the off-by-one error in the pagination logic"

**Output:**
```
quick-fix
```

### Example 2: New Feature

**Prompt:** "Add a search bar to the dashboard that filters results in real-time"

**Output:**
```
build
```

### Example 3: Complex Feature

**Prompt:** "Implement user authentication with OAuth, session management, and role-based access control"

**Output:**
```
sdlc
```
