---
name: prime
description: >
  Builds deep codebase context by systematically reading project structure,
  key files, and architecture for the Keep platform. Use when starting a
  new session, onboarding to the codebase, or when asked to "prime",
  "get context", "learn the codebase", "orient yourself", "understand
  the project", or "familiarize yourself with the code". Also triggers
  when asked "what does this project do" or "summarize the codebase".
  Do NOT use for implementing features, fixing bugs, or running commands.
  Do NOT use when already primed in the current session.
---

# Purpose

Builds codebase context for the Keep platform so you can work effectively in subsequent tasks. Uses a pre-built reference document instead of reading source files directly.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Survey current state

Run in parallel:

```bash
git ls-files
```

```bash
git status
```

```bash
git log --oneline -10
```

### Step 2: Read context files

Read in parallel:

1. **`README.md`** — Project overview
2. **`.claude/skills/prime/references/codebase-map.md`** — Complete architectural reference (schema, engine, API routes, app structure)

### Step 3: Check active specs

List files in `specs/` directory sorted by modification time. Read the 1-2 most recently modified specs (first 60 lines each) to understand current development direction.

### Step 4: Branch context (if not on main)

If on a feature branch, run `git diff main...HEAD --stat` to understand branch scope.

### Step 5: Summarize

Provide a concise structured summary covering: project purpose, data flow, current branch state, and active specs. Reference specifics from the codebase map (tables, engine layers, risk levels) — not just generic tech stack info.

## Cookbook

<If: already primed in the current session>
<Then: quick refresh — run `git status` and `git log --oneline -5` only. Skip all reads.>
