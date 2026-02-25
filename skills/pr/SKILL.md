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

### Step 0: Detect Repo Topology

First, determine whether this is a single-repo or multi-repo project:

```bash
git rev-parse --git-dir 2>/dev/null
```

- If this succeeds, you are in a **single-repo** project. Follow Steps 1-6 below.
- If this fails (not a git repo), scan for sub-repos by checking for `.git` directories in subdirectories:

```bash
find . -maxdepth 3 -name ".git" -type d 2>/dev/null
```

If multiple `.git` directories are found in subdirectories, you are in a **multi-repo** project. Follow the "Multi-Repo" instructions at the end of this section.

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

---

### Multi-Repo Instructions

When the working directory is NOT a git repo but contains subdirectories that ARE git repos, follow these steps instead:

#### Step M1: Discover Sub-Repos

Find all sub-repos with `.git` directories:

```bash
find . -maxdepth 3 -name ".git" -type d 2>/dev/null
```

For each discovered sub-repo directory, check its branch and commit status.

#### Step M2: Validate Each Sub-Repo

For each sub-repo, run these checks:

```bash
git -C <repo-path> branch --show-current
git -C <repo-path> status --short
git -C <repo-path> fetch origin main
git -C <repo-path> log origin/main..HEAD --oneline
```

- If on `main` or `master`, skip this sub-repo (nothing to PR).
- If there are uncommitted changes, stop and tell the user to commit first.
- If no commits ahead of `origin/main`, skip this sub-repo.

If **no sub-repo** has a feature branch with changes, stop and tell the user there is nothing to PR.

#### Step M3: Generate Shared PR Content

Collect commit logs from all sub-repos that have changes and generate a **single shared title** for consistency across all PRs:

**Title:** Derive from the combined commit history across all sub-repos. Keep under 70 characters.

- Use the same title for all sub-repo PRs
- Do not mention AI, Claude, or automated tooling in the title
- Match conventional commit style when appropriate

**Body:** Generate per sub-repo, but use similar structure.

#### Step M4: Push and Create PRs

For each sub-repo with changes:

```bash
git -C <repo-path> push -u origin <branch-name>
cd <repo-path> && gh pr create --title "<shared-title>" --body "$(cat <<'EOF'
## Summary

- bullet points here
EOF
)"
```

Note: `gh pr create` must be run from within the repo directory since `gh` does not support a `-C` flag.

#### Step M5: Report

Print all PR URLs grouped by sub-repo:

```
Pull requests created:

[<sub-repo-name>] <PR URL>
[<sub-repo-name>] <PR URL>
```

## Workflow

1. **Detect** — Determine single-repo or multi-repo topology
2. **Base branch** — Detect the default branch (main/master/etc.)
3. **Validate** — Confirm feature branch, no uncommitted changes, commits ahead of base
4. **Gather** — Fetch latest base, collect commit log, read spec if provided
5. **Generate** — Create conventional title and structured body
6. **Push** — Push branch to origin with tracking, create PR(s)
7. **Report** — Return the PR URL(s)

## Cookbook

<If: `gh` command not found>
<Then: tell the user to install GitHub CLI (see https://cli.github.com/) and authenticate with `gh auth login`>

<If: not authenticated with GitHub>
<Then: tell the user to run `gh auth login` and follow the prompts>

<If: a PR already exists for this branch>
<Then: run `gh pr view --web` (or `cd <repo-path> && gh pr view --web` for multi-repo) to open the existing PR instead of creating a duplicate>

<If: branch has no commits ahead of the base branch>
<Then: tell the user there are no changes to PR and check if work was done on a different branch>

<If: invoked with a spec path>
<Then: use the spec to write better summary bullets but don't paste the entire spec into the body>

<If: multi-repo and only one sub-repo has changes>
<Then: create a PR for just that sub-repo — don't error about the others>

<If: multi-repo and sub-repos share the same branch name>
<Then: use the same PR title for consistency across all sub-repo PRs>

## Validation

Before creating each PR:

- Branch is not the default branch (main/master/etc.)
- No uncommitted changes exist
- At least one commit exists ahead of the base branch
- `gh auth status` succeeds (user is authenticated)
- Title is under 70 characters
- Title does not mention "claude", "ai", "automated", or "copilot"
- For multi-repo: all git commands use `-C <repo-path>` to target the correct repository

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

### Example 3: Multi-Repo PR

**User says:** "create a pr"

**Discovery:** `app/server` on `agtop/001`, `app/client` on `agtop/001`, root is not a git repo.

**Actions:**

1. Both sub-repos on feature branches with commits ahead
2. Generate shared title: "feat: add attendee management"
3. Push and create PR for `app/server`
4. Push and create PR for `app/client` (same title)
5. Report:
   ```
   Pull requests created:

   [app/server] https://github.com/org/server-repo/pull/88
   [app/client] https://github.com/org/client-repo/pull/43
   ```
