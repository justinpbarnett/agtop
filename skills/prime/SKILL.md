---
name: prime
description: >
  Builds deep codebase context by systematically scanning the project structure,
  reading key documentation, and summarizing understanding. Use when starting a
  new session, onboarding to the codebase, or when asked to "prime", "get context",
  "learn the codebase", "orient yourself", "understand the project", or
  "familiarize yourself with the code". Also triggers when asked "what does this
  project do" or "summarize the codebase". Do NOT use for implementing features,
  fixing bugs, or running commands. Do NOT use when already primed in the current
  session.
---

# Purpose

Builds codebase context by scanning the project structure, reading key documentation, and understanding the architecture so you can work effectively in subsequent tasks.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Survey Current State

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

If `git ls-files` fails (not a git repo), fall back to listing the directory tree.

### Step 2: Read Core Documentation

Read these files in order. If a file does not exist, skip it and note its absence:

1. **`README.md`** — Project overview, setup instructions, architecture
2. **`CLAUDE.md`** or **`.claude/CLAUDE.md`** — Project-specific agent instructions
3. **`CONTRIBUTING.md`** — Contribution guidelines and conventions (if it exists)

### Step 3: Understand the Tooling

Check for a task runner or build system to understand available commands:

1. **`justfile`** — Read for available recipes
2. **`package.json`** — Read the `scripts` section
3. **`Makefile`** — Read for available targets
4. **`pyproject.toml`** — Read for project metadata and scripts

Read whichever exists (check in that order, read the first one found plus package.json if it exists).

### Step 4: Check Active Specs

If a `specs/` directory exists, list files sorted by modification time. Read the 1-2 most recently modified specs (first 60 lines each) to understand current development direction.

### Step 5: Branch Context (if not on main)

If on a feature branch, run `git diff main...HEAD --stat` to understand branch scope.

### Step 6: Summarize

Provide a concise structured summary covering:

- **What the project is** — Core purpose and domain
- **How it's structured** — Key directories and their roles
- **How to run it** — Available commands, scripts, entry points
- **Key patterns** — Conventions, naming schemes, architectural decisions
- **Technology stack** — Languages, frameworks, tools, dependencies
- **Current state** — Branch, uncommitted changes, active specs

## Workflow

1. **Scan** — `git ls-files`, `git status`, `git log` (parallel)
2. **Read docs** — README, CLAUDE.md, CONTRIBUTING.md
3. **Tooling** — Read justfile / package.json / Makefile / pyproject.toml
4. **Specs** — Check for active specs in `specs/`
5. **Branch** — Understand feature branch scope if applicable
6. **Summarize** — Structured summary of the project

## Cookbook

<If: already primed in the current session>
<Then: quick refresh — run `git status` and `git log --oneline -5` only. Skip all reads.>

<If: not a git repository>
<Then: fall back to `ls -la` and directory exploration. Note this in the summary.>

<If: no README.md exists>
<Then: look for alternative documentation (docs/, wiki/, CONTRIBUTING.md, or package.json description). Note the absence.>

<If: monorepo with multiple packages>
<Then: identify the top-level structure and note the key packages/workspaces. Don't read every sub-package in detail during priming — read those on-demand when the task requires it.>
