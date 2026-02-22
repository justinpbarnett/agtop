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

Creates a GitHub pull request from the current branch by analyzing commits against `origin/main`, generating a conventional title and structured body, and submitting via `gh pr create`.

## Variables

- `argument` — Optional spec file path to enrich the PR body with specification context, optionally followed by a screenshot directory path (e.g., `specs/feat-abc.md review_img`).

## Instructions

### Step 1: Validate Branch State

Run these checks before proceeding:

```bash
git branch --show-current
```

- If on `main` or `master`, stop and tell the user they need to be on a feature branch.
- If the branch has no commits ahead of `origin/main`, stop and tell the user there is nothing to PR.

Check for uncommitted changes:

```bash
git status --short
```

- If there are uncommitted changes, stop and tell the user to commit first (suggest `/commit`).

### Step 2: Gather Context

Fetch the latest main from origin to ensure comparisons are accurate:

```bash
git fetch origin main
```

Run these commands to understand the branch:

```bash
git log origin/main..HEAD --oneline
```

If a spec path is provided as an argument, read the spec file to enrich the PR body.

### Step 3: Generate PR Content

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

### Step 4: Push and Create PR

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

### Step 5: Post Screenshot Comment

After the PR is created, check if there are new screenshots on this branch to include in a comment.

1. Check for new `.png` files on this branch vs `origin/main` in `docs/assets/`:
   ```bash
   git diff origin/main --name-only -- docs/assets/*.png
   ```

2. If no new screenshots are found, skip this step entirely.

3. If screenshots exist, get the GitHub repo info and current branch:
   ```bash
   gh repo view --json owner,name --jq '.owner.login + "/" + .name'
   git branch --show-current
   ```

4. Build a markdown comment body with embedded images using GitHub blob URLs with `?raw=true` in the format `https://github.com/{owner}/{repo}/blob/{branch}/docs/assets/{filename}?raw=true`. This format works for both public and private repos because it goes through GitHub's authenticated web server. Use image alt text derived from the filename (strip the number prefix, replace underscores with spaces). Arrange images in a table layout for readability:
   ```markdown
   ## Screenshots

   | | |
   |---|---|
   | ![description](url) | ![description](url) |
   ```

5. Post the comment using `gh pr comment`:
   ```bash
   gh pr comment --body "$(cat <<'EOF'
   ## Screenshots

   ...image markdown table...
   EOF
   )"
   ```

6. This step is **best-effort** — if `gh pr comment` fails, log the error but do not fail the overall PR creation.

### Step 6: Report

Print the PR URL returned by `gh pr create`.

If the command included an ADW ID context, include it in the output for traceability.

## Workflow

1. **Validate** — Confirm feature branch, no uncommitted changes, commits ahead of main
2. **Gather** — Fetch latest main, collect commit log, read spec if provided
3. **Generate** — Create conventional title and structured body
4. **Push** — Push branch to origin with tracking, create PR
5. **Screenshot** — Post PR comment with embedded screenshots (if any new `.png` files exist on the branch)
6. **Report** — Return the PR URL

## Cookbook

<If: `gh` command not found>
<Then: tell the user to install GitHub CLI (`brew install gh` on macOS or see https://cli.github.com/) and authenticate with `gh auth login`>

<If: not authenticated with GitHub>
<Then: tell the user to run `gh auth login` and follow the prompts>

<If: a PR already exists for this branch>
<Then: run `gh pr view --web` to open the existing PR instead of creating a duplicate>

<If: branch has no commits ahead of origin/main>
<Then: tell the user there are no changes to PR and check if work was done on a different branch>

<If: invoked with a spec path>
<Then: use the spec to write better summary bullets but don't paste the entire spec into the body>

<If: new screenshots exist in `docs/assets/` on the branch>
<Then: post a PR comment with embedded images in a table layout using raw GitHub URLs — this makes UI changes visible to reviewers without checking out the branch>

<If: `gh pr comment` fails when posting screenshots>
<Then: log the error but do not fail the PR creation — screenshot comments are best-effort and should never block shipping>

## Validation

Before creating the PR:

- Branch is not `main` or `master`
- No uncommitted changes exist
- At least one commit exists ahead of `origin/main`
- `gh auth status` succeeds (user is authenticated)
- Title is under 70 characters
- Title does not mention "claude", "ai", "automated", or "copilot"

## Examples

### Example 1: Simple PR from Feature Branch

**User says:** "create a pr"

**Actions:**

1. Check branch: `feat/add-auth` — not main, good
2. Check status: clean working tree
3. Fetch `origin/main`, gather: 3 commits ahead of `origin/main`
4. Generate title from commits: "feat: add user authentication"
5. Generate body with summary
6. Push and create PR
7. Report: `https://github.com/user/repo/pull/42`

### Example 2: PR with Spec Reference

**User says:** "/pr specs/feat-ADW-042-auth.md"

**Actions:**

1. Validate branch state
2. Read `specs/feat-ADW-042-auth.md` for context
3. Generate richer PR body incorporating spec details
4. Push and create PR
5. Report URL

### Example 3: PR from ADW Pipeline with Screenshots

**Invoked by SDLC as:** "/pr specs/feat-abc123-feature.md review_img"

**Actions:**

1. Validate branch state, gather commits, read spec
2. Generate conventional title and structured body from spec context
3. Push and create PR
4. Detect new `.png` files on the branch in `docs/assets/`
5. Build raw GitHub URLs for each screenshot and post a PR comment with embedded images in a table layout
6. Report PR URL

### Example 4: PR from ADW Pipeline without Screenshots

**Invoked by SDLC as:** "/pr specs/feat-abc123-backend-fix.md"

**Actions:**

1. Validate branch state, gather commits, read spec
2. Generate conventional title and structured body
3. Push and create PR
4. No new `.png` files found — skip screenshot comment step
5. Report PR URL
