---
name: commit
description: >
  Reviews all uncommitted changes, groups them by logical concern, and creates
  atomic git commits — one per distinct change — using conventional commit
  message format. Use when a user wants to "commit", "commit my changes",
  "create commits", "commit this work", "stage and commit", "save my progress",
  "generate commits", "make atomic commits", or "commit everything". Also
  triggers on "git commit" or "check in my changes". Do NOT use for pushing
  to remote (use git push directly). Do NOT use for creating pull requests
  (use the pr skill). Do NOT use for reverting or amending commits.
---

# Purpose

Reviews all uncommitted changes, groups them by logical concern, and creates atomic git commits with conventional commit messages — one per distinct change.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Check for Changes

Run `git status --short` to see all changed and untracked files.

If there are no changes, report that there is nothing to commit and stop.

### Step 2: Review the Full Diff

For tracked files with modifications:

```bash
git diff HEAD
```

For untracked files, read their contents to understand what they contain.

### Step 3: Plan and Execute the Commits

Analyze the diff and group changed files by logical concern, then immediately execute all commits without waiting for user confirmation.

**Commit message format:** `<type>: <description>`

- **type**: `feat`, `fix`, `chore`, `refactor`, `test`, `docs`, `style`, `perf`, `build`, `ci`
- **description**: present tense, lowercase, 50 chars or less, no period at the end
- Never mention AI, Claude, or automated tooling in commit messages
- Never add Co-Authored-By trailers or any other attribution to AI, agents, or bots

**Grouping rules:**

- Files that work together for a single feature or fix belong in one commit
- A Drizzle schema change and its corresponding migration are one commit
- A new route (page.tsx) and its related components or server actions are one commit
- Dependency updates (`package.json`, `pnpm-lock.yaml`) are their own commit
- Formatting or lint fixes are their own commit, separate from behavioral changes
- Test additions/changes accompany the code they test, unless they are standalone test improvements
- Spec files (`specs/`) are their own commit unless created alongside the implementation
- Skill or ADW changes (`.claude/skills/`, `adws/`) are their own commit
- Config changes group with the feature they support, or stand alone if independent

**Revert test:** "Could this commit be reverted independently without breaking the other changes?" If yes, it should be its own commit.

For each planned commit, stage only the relevant files and commit:

```bash
git add <file1> <file2> ... && git commit -m "<message>"
```

Important:
- Stage files by explicit path — never use `git add -A` or `git add .`
- If a file is in a new directory, make sure to include the full path
- For deleted files, use `git add <deleted-file>` (git handles deletions in staging)

### Step 4: Report

After all commits are created, show a summary:

```
Commits created:
  <hash> <type>: <description> (<files>)
  <hash> <type>: <description> (<files>)
```

Then run `git status --short` to confirm the working tree is clean (or show any remaining uncommitted files).

## Workflow

1. **Check** — Run `git status` to detect changes
2. **Diff** — Review full diff and untracked file contents
3. **Group** — Organize changes into logical atomic commits
4. **Commit** — Stage and commit each group in sequence
5. **Report** — Show commit summary and final working tree status

## Cookbook

<If: no changes detected but user expects changes>
<Then: run `git status` (without `--short`) for a detailed view and check `git diff --cached` for already-staged changes>

<If: a pre-commit hook fails>
<Then: fix the reported issues, re-stage the affected files, and create a NEW commit (do not amend). Include trivial fixes (formatting) in the same logical commit. If the fix reveals a real issue, create a separate fix commit.>

<If: user wants to include only some changes>
<Then: ask which files or changes to include. Only commit the specified subset. Report remaining uncommitted files at the end.>

<If: changes span many files but serve one purpose>
<Then: that is one commit, not many — prefer fewer, well-scoped commits over many tiny ones>

<If: the diff alone does not make the purpose clear>
<Then: read file contents to understand the change before grouping>

## Validation

Before executing each commit:
- Verify each commit message is under 50 characters (type + colon + space + description)
- Verify no commit message or trailer contains "claude", "ai", "automated", "copilot", or "Co-Authored-By" referencing an AI/agent
- Verify each file appears in exactly one commit group (no duplicates, no omissions)
- Verify all changed files from `git status` are accounted for in the plan

## Examples

### Example 1: Single Feature Commit

**User says:** "commit my changes"

**Git status shows:** Modified `src/app/oauth/page.tsx`, `src/lib/planning-center.ts`, `src/app/oauth/actions.ts`

**Result:**
```
Planned commits:
1. feat: add planning center oauth flow — src/app/oauth/page.tsx, src/lib/planning-center.ts, src/app/oauth/actions.ts
```

### Example 2: Mixed Changes Requiring Multiple Commits

**User says:** "commit everything"

**Git status shows:** Modified `package.json`, `pnpm-lock.yaml`, new `src/db/schema/users.ts`, modified `src/app/oauth/page.tsx`, new `src/__tests__/oauth.test.ts`, modified `justfile`

**Result:**
```
Planned commits:
1. chore: update dependencies — package.json, pnpm-lock.yaml
2. feat: add user schema and oauth page — src/db/schema/users.ts, src/app/oauth/page.tsx
3. test: add oauth integration tests — src/__tests__/oauth.test.ts
4. chore: add new just recipes — justfile
```
