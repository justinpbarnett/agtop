---
name: document
description: >
  Generates concise markdown documentation for implemented features by analyzing
  git diffs against the base branch and optionally referencing the original
  specification. Creates docs in docs/ with technical details, usage instructions,
  and optional screenshots. Use when a user wants to document a feature, generate
  feature docs, write up what was built, create implementation documentation, or
  summarize changes for a completed feature. Triggers on "document this feature",
  "generate docs for this feature", "write up what was built", "create feature
  documentation", "write docs for this branch". Do NOT use for implementing
  features (use the implement skill). Do NOT use for reviewing features against
  specs (use the review skill). Do NOT use for creating plans or specs (use the
  spec skill). Do NOT use for general README or project documentation.
---

# Purpose

Generates concise markdown documentation for implemented features by analyzing code changes against the base branch and optionally referencing the original specification. Creates documentation files in `docs/` with a consistent format.

## Variables

- `argument` — Optional. A feature name or identifier, optionally followed by a spec path (e.g., `user-auth specs/feat-user-auth.md`). If omitted, derives the feature name from the current branch.

## Instructions

### Step 1: Collect Inputs

Determine the required inputs from the user's request:

- **feature_name** — A short identifier for the feature. If not provided, derive from the current branch name.
- **spec_path** (optional) — Path to the feature specification file. If not provided, search `specs/` for a matching file based on the branch name.
- **screenshots_dir** (optional) — Directory containing screenshots to include in the documentation.

### Step 2: Determine the Base Branch

Detect the default branch:

```bash
git remote show origin | grep 'HEAD branch' | sed 's/.*: //'
```

Falls back to `main` if detection fails.

### Step 3: Analyze Code Changes

Run these git commands to understand what was built:

1. `git diff origin/<base-branch> --stat` — See files changed and lines modified
2. `git diff origin/<base-branch> --name-only` — Get the list of changed files

For files with significant changes (>50 lines in the stat output), run `git diff origin/<base-branch> <file>` on those specific files to understand implementation details.

Read key changed files directly if the diff alone is insufficient to understand the feature.

### Step 4: Read Specification

If `spec_path` was provided or discovered in `specs/`:

1. Read the specification file
2. Extract:
   - Original requirements and goals
   - Expected functionality
   - Success criteria
3. Frame the documentation around what was requested vs. what was built

If no spec is available, proceed without it — the git diff analysis is sufficient.

### Step 5: Handle Screenshots

If `screenshots_dir` was provided:

1. List files in the screenshots directory
2. Create `docs/assets/` directory if it does not exist: `mkdir -p docs/assets`
3. Copy all image files from the screenshots directory to `docs/assets/`
4. Examine the screenshots to understand visual changes and reference them in the documentation

If no screenshots directory was provided, skip this step and omit the Screenshots section from the output.

### Step 6: Generate Documentation

1. Create `docs/` directory if it does not exist: `mkdir -p docs`
2. Determine a descriptive name from the feature (e.g., "user-auth", "data-export", "search-ui")
3. Create the documentation file at `docs/feature-{descriptive-name}.md`
4. Follow the template in `references/doc-template.md`
5. Focus on:
   - What was built (from git diff analysis)
   - How it works (technical implementation)
   - How to use it (user perspective)
   - Any configuration or setup required

### Step 7: Return Result

Return exclusively the path to the documentation file created and nothing else.

## Workflow

1. **Collect** — Gather feature name, spec_path, and screenshots_dir from user input
2. **Base** — Detect the default branch
3. **Analyze** — Run git diff to understand what changed
4. **Spec** — Read the feature specification if available
5. **Screenshots** — Copy screenshots to `docs/assets/` if provided
6. **Generate** — Write the documentation file following the template
7. **Report** — Return the documentation file path

## Cookbook

<If: git diff against the base branch is empty>
<Then: check `git status` for uncommitted changes and `git log origin/<base>..HEAD` for commits. If no changes exist at all, inform the user there is nothing to document.>

<If: screenshots directory does not exist or is empty>
<Then: warn the user that no screenshots were found. Proceed without screenshots and omit the Screenshots section.>

<If: feature name not provided and not on a feature branch>
<Then: ask the user for a feature name before proceeding.>

<If: multiple spec files could match>
<Then: list the matching files and ask the user to confirm which spec to use.>

<If: large diff spanning many files>
<Then: group related changes by feature area in the documentation. Focus on architectural overview rather than line-by-line detail.>

## Validation

Before writing the documentation file, verify:

- The git diff was successfully analyzed (at least one changed file identified)
- All referenced screenshot files actually exist in `docs/assets/` after copying
- The documentation covers the key sections: Overview, What Was Built, Technical Implementation, How to Use
- File paths in the document are relative to `docs/`

## Examples

### Example 1: Document with Spec and Screenshots

**User says:** "Document this feature using specs/feat-user-auth.md with screenshots from review_img/"

**Actions:**

1. Set feature_name from branch, spec_path=specs/feat-user-auth.md, screenshots_dir=review_img/
2. Run git diff analysis against base branch
3. Read the spec to understand requirements
4. Copy screenshots to docs/assets/
5. Generate documentation referencing both spec and screenshots
6. Return: `docs/feature-user-auth.md`

### Example 2: Document from Branch Only

**User says:** "Document this feature"

**Actions:**

1. Derive feature_name from current branch name
2. Run git diff analysis against base branch
3. Search specs/ for a matching spec file (optional)
4. Generate documentation based on code changes
5. Omit Screenshots section from output
6. Return: `docs/feature-descriptive-name.md`
