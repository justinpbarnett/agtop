---
name: decompose
description: >
  Analyzes a feature spec and decomposes it into smaller, focused sub-tasks
  when the feature is too large for a single agent. Produces a task graph
  with mini-specs for each sub-task. Use when a user wants to break down
  a large feature, decompose a spec, split a plan into subtasks, or when
  a spec is too big to implement in one pass. Triggers on "decompose this
  spec", "break this down", "split into subtasks", "this is too big".
  Do NOT use for implementing features (use the implement skill).
  Do NOT use for creating specs (use the spec skill).
---

# Purpose

Analyzes a feature spec and determines whether it should be decomposed into smaller sub-tasks. For small features, outputs a single-task graph. For large features, breaks the work into focused sub-tasks with mini-specs, each targeting a narrow scope to keep agent context minimal.

## Variables

- `argument` — Two space-separated values: `{spec_file_path} {task_id}` (e.g., `specs/feat-user-auth.md AUTH-042`). If no task_id is provided, derive one from the spec filename.

## Instructions

### Step 1: Read the Spec

Read the full feature spec file provided in the argument. Identify:

1. **Implementation steps** — The numbered steps or tasks in the spec
2. **Files touched** — All files listed in the "Relevant Files" section
3. **New files** — Files that need to be created
4. **Dependencies between steps** — Which steps depend on which

### Step 2: Evaluate Complexity

Apply these heuristics to decide whether decomposition is needed:

- **Count implementation steps** in the "Step by Step Tasks" or "Implementation Plan" section
- **Count distinct files** that will be modified or created
- **Threshold**: If fewer than 5 implementation steps AND fewer than 8 files touched → **single task** (no decomposition)
- **Otherwise** → **decompose** into sub-tasks

### Step 3a: Single Task (No Decomposition)

If the feature is small enough for a single agent:

1. Output a task graph JSON with `is_decomposed: false`
2. The single task points to the original spec file
3. No mini-specs are created

Output this JSON to stdout:

```json
{
  "parent_spec": "{spec_file_path}",
  "task_id": "{task_id}",
  "is_decomposed": false,
  "tasks": [
    {
      "id": "single",
      "title": "{feature title from spec}",
      "stage": 1,
      "spec_file": "{spec_file_path}",
      "depends_on": [],
      "status": "pending",
      "context_files": []
    }
  ]
}
```

### Step 3b: Decompose into Sub-Tasks

If the feature needs decomposition:

1. **Group related steps** into focused sub-tasks using these heuristics:
   - **Data model + migration** → early task (stage 1), other tasks depend on it
   - **Service/business logic** → separate task per service, depends on data model
   - **Route/API endpoints** → separate task, depends on services
   - **Components/Pages** → can often parallelize (assign same stage if independent)
   - **Tests** → bundled with the thing they test (each sub-task includes its own tests)
   - **Seed/fixture data** → separate late task, depends on model + service
   - **Config/admin pages** → separate task, can parallelize with main UI

2. **Assign stages** — Stage 1 has no dependencies. Stage N depends on all previous stages completing.

3. **Create mini-specs** — For each sub-task, write a mini-spec file to `specs/subtasks/{task_id}/{sub_task_id}.md`

4. **Output the task graph** as JSON to stdout

### Step 4: Write Mini-Specs

Each mini-spec at `specs/subtasks/{task_id}/{sub_task_id}.md` must contain:

```markdown
# Sub-task: {title}

> Part of: [{parent feature title}]({parent_spec_path})
> Sub-task {N} of {total} for {task_id}

## Scope

{1-2 sentence description of what this sub-task accomplishes}

## Steps

{Only the specific numbered steps from the parent spec that belong to this sub-task}

## Relevant Files

### Existing Files (modify)

| File | Role | Change |
|------|------|--------|
{Only files this sub-task touches}

### New Files

| File | Role |
|------|------|
{Only new files this sub-task creates, if any}

## Validation

{Specific validation criteria for this sub-task — what commands to run, what to check}
```

### Step 5: Output Task Graph JSON

After writing any mini-specs, output the task graph as **clean, parseable JSON** to stdout. This must be the final output.

```json
{
  "parent_spec": "{spec_file_path}",
  "task_id": "{task_id}",
  "is_decomposed": true,
  "tasks": [
    {
      "id": "task-1-data-model",
      "title": "Add data model and migration",
      "stage": 1,
      "spec_file": "specs/subtasks/{task_id}/task-1-data-model.md",
      "depends_on": [],
      "status": "pending",
      "context_files": ["src/db/schema/", "src/db/index.ts"]
    },
    {
      "id": "task-2-service-layer",
      "title": "Implement service layer",
      "stage": 2,
      "spec_file": "specs/subtasks/{task_id}/task-2-service-layer.md",
      "depends_on": ["task-1-data-model"],
      "status": "pending",
      "context_files": ["src/lib/"]
    }
  ]
}
```

## Workflow

1. **Read** — Parse the spec file path and task ID from arguments
2. **Analyze** — Count steps, files, and evaluate complexity
3. **Decide** — Single task or decompose based on heuristics
4. **Write** — Create mini-spec files if decomposing
5. **Output** — Print task graph JSON to stdout

## Cookbook

<If: spec has fewer than 5 steps and fewer than 8 files>
<Then: output single-task graph with is_decomposed: false. Do not create mini-specs.>

<If: steps have clear data → service → route → UI layering>
<Then: decompose by layer, with each layer in a successive stage.>

<If: multiple independent UI components or pages>
<Then: assign them the same stage so they can run in parallel.>

<If: spec includes seed data or test fixtures>
<Then: make seed data a separate late-stage task.>

<If: spec is ambiguous about step boundaries>
<Then: prefer larger sub-tasks over smaller ones. 3-5 sub-tasks is ideal. Avoid creating more than 8 sub-tasks.>

<If: all steps are tightly coupled and cannot be separated>
<Then: output single-task graph even if the step/file count exceeds thresholds. Note this in the output.>

## Validation

- Task graph JSON is valid and parseable
- All `spec_file` paths in the task graph point to files that exist (or the original spec for single-task)
- Mini-specs contain only the steps relevant to their sub-task
- Stage numbering starts at 1 and increments
- `depends_on` references are valid task IDs
- No circular dependencies

## Examples

### Example 1: Small Feature (No Decomposition)

**Spec:** 3 implementation steps, 5 files touched

**Output:**
```json
{
  "parent_spec": "specs/feat-health-check.md",
  "task_id": "health-check",
  "is_decomposed": false,
  "tasks": [
    {
      "id": "single",
      "title": "Add health check endpoint",
      "stage": 1,
      "spec_file": "specs/feat-health-check.md",
      "depends_on": [],
      "status": "pending",
      "context_files": []
    }
  ]
}
```

### Example 2: Large Feature (Decomposed)

**Spec:** 10 implementation steps, 15 files touched (model + service + routes + pages + seed data)

**Output:** Task graph with 5 sub-tasks across 3 stages:
- Stage 1: Data model
- Stage 2: Service layer, API routes
- Stage 3: Pages/Components (parallel), Seed data
