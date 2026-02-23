---
name: pr
description: >
  Creates a GitHub pull request from the current branch by analyzing commits,
  generating a conventional title and structured body, pushing to origin, and
  submitting via gh pr create. Use when a user wants to "create a pr", "open a
  pull request", "submit a pr", "push and create pr", "make a pr", "pr this",
  or "ship it". Also triggers on "create pull request" or "open pr". Do NOT use
  for committing changes (use the commit skill). Do NOT use for pushing without
  a PR (use git push directly). Do NOT use for reviewing existing PRs.
---

# Purpose

Creates a GitHub pull request from the current branch by analyzing commits against the base branch, generating a conventional title and structured body, and submitting via `gh pr create`.

## Variables

- `argument` — Optional spec file path to enrich the PR body with specification context (e.g., `specs/feat-auth.md`).

## Instructions

### Step 1: Determine the Base Branch

Identify the default branch:

```bash
git remote show origin | grep 'HEAD branch' | sed 's/.*: //'
```

Use this as the base branch for comparisons. Falls back to `main` if detection fails.

### Step 2: Validate Branch State

Run these checks before proceeding:

```bash
git branch --show-current
```

- If on the default branch (e.g., `main` or `master`), stop and tell the user they need to be on a feature branch.
- If the branch has no commits ahead of the base branch, stop and tell the user there is nothing to PR.

Check for uncommitted changes:

```bash
git status --short
```

- If there are uncommitted changes, stop and tell the user to commit first (suggest `/commit`).

### Step 3: Gather Context

Fetch the latest base branch from origin to ensure comparisons are accurate:

```bash
git fetch origin <base-branch>
```

Run these commands to understand the branch:

```bash
git log origin/<base-branch>..HEAD --oneline
```

If a spec path is provided as an argument, read the spec file to enrich the PR body.

### Step 4: Generate PR Content

**Title:** Derive from the commit history. If there is a single commit, use its message. If there are multiple commits, summarize the overall change. Keep under 70 characters.

- Do not mention AI, Claude, or automated tooling in the title
- Use lowercase, no period at the end
- Match conventional commit style when appropriate (e.g., "feat: add user auth")

**Body:** Use this structure:

```markdown
## Summary

- [1-3 bullet points describing what changed and why]
```

If a spec path was provided, add a `## Spec` section referencing it.

### Step 5: Push and Create PR

Push the branch to origin:

```bash
git push -u origin <branch-name>
```

Create the PR:

```bash
gh pr create --title "<title>" --body "$(cat <<'EOF'
## Summary

- bullet points here
EOF
)"
```

### Step 6: Report

Print the PR URL returned by `gh pr create`.

## Workflow

1. **Base branch** — Detect the default branch (main/master/etc.)
2. **Validate** — Confirm feature branch, no uncommitted changes, commits ahead of base
3. **Gather** — Fetch latest base, collect commit log, read spec if provided
4. **Generate** — Create conventional title and structured body
5. **Push** — Push branch to origin with tracking, create PR
6. **Report** — Return the PR URL

## Cookbook

<If: `gh` command not found>
<Then: tell the user to install GitHub CLI (see https://cli.github.com/) and authenticate with `gh auth login`>

<If: not authenticated with GitHub>
<Then: tell the user to run `gh auth login` and follow the prompts>

<If: a PR already exists for this branch>
<Then: run `gh pr view --web` to open the existing PR instead of creating a duplicate>

<If: branch has no commits ahead of the base branch>
<Then: tell the user there are no changes to PR and check if work was done on a different branch>

<If: invoked with a spec path>
<Then: use the spec to write better summary bullets but don't paste the entire spec into the body>

## Validation

Before creating the PR:

- Branch is not the default branch (main/master/etc.)
- No uncommitted changes exist
- At least one commit exists ahead of the base branch
- `gh auth status` succeeds (user is authenticated)
- Title is under 70 characters
- Title does not mention "claude", "ai", "automated", or "copilot"

## Examples

### Example 1: Simple PR from Feature Branch

**User says:** "create a pr"

**Actions:**

1. Check branch: `feat/add-auth` — not main, good
2. Check status: clean working tree
3. Fetch base, gather: 3 commits ahead
4. Generate title from commits: "feat: add user authentication"
5. Generate body with summary
6. Push and create PR
7. Report: `https://github.com/user/repo/pull/42`

### Example 2: PR with Spec Reference

**User says:** "/pr specs/feat-auth.md"

**Actions:**

1. Validate branch state
2. Read `specs/feat-auth.md` for context
3. Generate richer PR body incorporating spec details
4. Push and create PR
5. Report URL
