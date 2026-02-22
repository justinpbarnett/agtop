---
name: spec
description: >
  Creates structured implementation specs for development tasks categorized
  by conventional commit types (feat, fix, refactor, perf, chore, docs,
  test, build, ci). Use when a user wants to spec, plan, design, or
  scope work before implementing it. Triggers on "spec a feature", "create
  a spec", "scope this work", "design the approach", "write a spec for",
  "spec this fix", "spec a refactor", or when given a task description.
  Do NOT use for implementing or executing existing specs.
  Do NOT use for quick single-line changes that need no spec phase.
---

# Purpose

Creates structured, type-aware implementation specs for development tasks. Categorizes work using conventional commit types and generates spec documents in `specs/` with appropriate detail for the task type.

## Variables

- `argument` — Task description and optional tracking ID (e.g., "spec a feature for user authentication — ID is AUTH-042").

## Instructions

### Step 1: Gather Requirements

Identify these values from the user's request:

- **type** — One of the types below. If ambiguous, ask the user. If they say "feature" map to `feat`, "bug" or "bugfix" map to `fix`, etc.
- **prompt** — The task description. If not provided, stop and ask.
- **task_id** — An optional tracking identifier. If not provided, use a descriptive slug.

### Task Types

| Type         | Description                                             | Spec Depth                                                          |
| ------------ | ------------------------------------------------------- | ------------------------------------------------------------------- |
| **feat**     | A new feature                                           | Comprehensive — user story, phases, testing strategy                |
| **fix**      | A bug fix                                               | Diagnostic — reproduction steps, root cause, regression testing     |
| **refactor** | Code change that neither fixes a bug nor adds a feature | Architectural — current/target state, migration strategy            |
| **perf**     | Code change that improves performance                   | Architectural — baseline metrics, optimization strategy, benchmarks |
| **chore**    | Maintenance tasks (deps, configs, cleanup)              | Lightweight — description, steps, validation                        |
| **docs**     | Documentation only changes                              | Lightweight                                                         |
| **test**     | Adding or correcting tests                              | Lightweight                                                         |
| **build**    | Build system or external dependency changes             | Lightweight                                                         |
| **ci**       | CI configuration and scripts                            | Lightweight                                                         |

### Step 2: Research the Codebase

Before writing the spec, research the codebase to understand context:

1. Read `README.md` for project overview, tech stack, and conventions
2. Explore files relevant to the task using Glob and Grep
3. Read existing code that will be modified or extended
4. Check `specs/` for related specs that provide context
5. Check for a task runner (justfile, Makefile, package.json scripts) to understand available commands

### Step 3: Select Template and Create Spec

Based on the task type, consult `references/spec-templates.md` for the appropriate template:

- **feat** — Use the Comprehensive Spec template
- **fix** — Use the Diagnostic Spec template
- **refactor**, **perf** — Use the Architectural Spec template
- **chore**, **docs**, **test**, **build**, **ci** — Use the Lightweight Spec template

Create the spec file at: `specs/{type}-{task_id}-{descriptive-name}.md`

- Replace `{type}` with the conventional commit type
- Replace `{task_id}` with the provided ID or descriptive slug
- Replace `{descriptive-name}` with a short kebab-case name derived from the task

Fill in every section of the template. Replace all placeholders with researched, specific content. Do not leave generic placeholder text.

**Quality guidelines by type:**

- **feat**: Think hard about requirements, design, and implementation approach. Design for extensibility and maintainability. Follow existing patterns.
- **fix**: Focus on precise reproduction and root cause. The fix strategy should be minimal and targeted.
- **refactor/perf**: Clearly articulate current problems and target state. Ensure no behavior changes for refactor; define measurable targets for perf.
- **Lightweight types**: Keep specs simple, thorough, and precise. No unnecessary ceremony.

### Step 4: Validate

Before finalizing, verify:

- Every placeholder in the template has been replaced with specific content
- All referenced files actually exist in the codebase (or are clearly marked as new)
- Step-by-step tasks are ordered correctly with dependencies respected
- Validation commands are runnable and specific
- The spec follows existing codebase patterns and conventions

### Step 5: Report

Return the path to the created spec file.

## Workflow

1. **Gather** — Extract type, prompt, and task ID from the user's request
2. **Research** — Read relevant codebase files, existing specs, and project docs
3. **Template** — Select the appropriate spec template for the task type
4. **Create** — Write the spec file with researched, specific content
5. **Validate** — Verify all references, ordering, and completeness
6. **Report** — Return the spec file path

## Cookbook

<If: user doesn't specify a task type>
<Then: ask the user to clarify. Present the type table and let them choose. If it's clearly one type from context, infer it and state your reasoning.>

<If: no task_id provided>
<Then: generate a descriptive kebab-case slug from the task description. Do not block on missing IDs.>

<If: spec references files that don't exist>
<Then: re-run Glob/Grep to verify all referenced paths. Mark files that need to be created under a "New Files" subsection.>

<If: scope is unclear or requirements are ambiguous>
<Then: prefer a more detailed spec over a sparse one. When in doubt, ask the user.>

<If: research phase reveals the task is larger than expected>
<Then: note this in the spec and suggest breaking it into multiple tasks if appropriate>

## Validation

Run these checks to verify the spec is sound:

- Review that the spec file exists in `specs/` with the correct naming convention
- Verify all files referenced in "Relevant Files" exist or are clearly marked as new
- Verify validation commands are appropriate for the project's tooling

## Examples

### Example 1: Speccing a New Feature

**User says:** "Spec a feature for user authentication"

**Actions:**

1. Type: `feat`, task_id: `user-auth`, prompt extracted from request
2. Research codebase: read README.md, explore relevant directories, check existing specs
3. Use Comprehensive Spec template from `references/spec-templates.md`
4. Create `specs/feat-user-auth.md`
5. Report the file path

### Example 2: Speccing a Bug Fix

**User says:** "Spec a fix for the health endpoint returning 500 when the database is down"

**Actions:**

1. Type: `fix`, task_id: `health-endpoint-db-error`, prompt extracted
2. Research: read the health endpoint code, database connection code
3. Use Diagnostic Spec template
4. Create `specs/fix-health-endpoint-db-error.md`
5. Report the file path

### Example 3: Speccing a Chore

**User says:** "Spec — update all dependencies to latest versions"

**Actions:**

1. Type: `chore`, task_id: `update-dependencies`, prompt extracted
2. Research: read dependency manifests, check for breaking changes
3. Use Lightweight Spec template
4. Create `specs/chore-update-dependencies.md`
5. Report the file path
