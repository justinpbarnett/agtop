---
name: document-feature
description: >
  Generates concise markdown documentation for implemented features by analyzing
  git diffs against main and the original feature specification. Creates docs in
  docs/ with technical details, usage instructions, and optional screenshots.
  Use when a user wants to document a feature, generate feature docs, write up
  what was built, create implementation documentation, or summarize changes for
  a completed feature. Triggers on "document this feature", "generate docs for
  this feature", "write up what was built", "create feature documentation",
  "document ADW-xxx", "write docs for this branch". Do NOT use for implementing
  features (use the implement skill). Do NOT use for reviewing features against
  specs (use the review skill). Do NOT use for creating plans or specs (use the
  spec skill). Do NOT use for general README or project documentation.
---

# Purpose

Generates concise markdown documentation for implemented features by analyzing code changes against main and the original feature specification. Creates documentation files in `docs/` with a consistent format.

## Variables

- `adw_id` — Required. The ADW identifier for the feature (e.g., "ADW-042"). If not provided, ask the user.
- `spec_path` — Optional. Path to the feature specification file. If omitted, searches `specs/` by ADW ID or branch name.
- `documentation_screenshots_dir` — Optional. Directory containing screenshots to include in the documentation.

## Instructions

### Step 1: Collect Inputs

Determine the required inputs from the user's request:

- **adw_id** (required) — The ADW identifier for the feature (e.g., "ADW-042"). If not provided, ask the user.
- **spec_path** (optional) — Path to the feature specification file. If not provided, attempt to find a matching spec in `specs/` based on the ADW ID or branch name.
- **documentation_screenshots_dir** (optional) — Directory containing screenshots to include in the documentation.

### Step 2: Analyze Code Changes

Run these git commands to understand what was built:

1. `git diff origin/main --stat` — See files changed and lines modified
2. `git diff origin/main --name-only` — Get the list of changed files

For files with significant changes (>50 lines in the stat output), run `git diff origin/main <file>` on those specific files to understand implementation details.

Read key changed files directly if the diff alone is insufficient to understand the feature.

### Step 3: Read Specification

If `spec_path` was provided or discovered in `specs/`:

1. Read the specification file
2. Extract:
   - Original requirements and goals
   - Expected functionality
   - Success criteria
3. Frame the documentation around what was requested vs. what was built

If no spec is available, proceed without it — the git diff analysis is sufficient.

### Step 4: Handle Screenshots

If `documentation_screenshots_dir` was provided:

1. List files in the screenshots directory
2. Create `docs/assets/` directory if it does not exist: `mkdir -p docs/assets`
3. Copy all `.png` files from the screenshots directory to `docs/assets/`, preserving original filenames
4. Examine the screenshots to understand visual changes and reference them in the documentation

If no screenshots directory was provided, skip this step and omit the Screenshots section from the output.

### Step 5: Generate Documentation

1. Create `docs/` directory if it does not exist: `mkdir -p docs`
2. Determine a descriptive name from the feature (e.g., "user-auth", "data-export", "search-ui")
3. Create the documentation file at `docs/feature-{adw_id}-{descriptive-name}.md`
4. Follow the template in `references/doc-template.md`
5. Focus on:
   - What was built (from git diff analysis)
   - How it works (technical implementation)
   - How to use it (user perspective)
   - Any configuration or setup required

### Step 6: Update Conditional Documentation

After creating the documentation file:

1. Check if `.claude/commands/conditional_docs.md` exists
2. If it exists, read it and add an entry for the new documentation file:

   ```
   - docs/<documentation_file>.md
     - Conditions:
       - When working with <feature area>
       - When implementing <related functionality>
       - When troubleshooting <specific issues>
   ```

3. If the file does not exist, skip this step

### Step 7: Return Result

Return exclusively the path to the documentation file created and nothing else.

## Workflow

1. **Collect** — Gather adw_id, spec_path, and screenshots_dir from user input
2. **Analyze** — Run git diff to understand what changed
3. **Spec** — Read the feature specification if available
4. **Screenshots** — Copy screenshots to `docs/assets/` if provided
5. **Generate** — Write the documentation file following the template
6. **Update** — Add entry to conditional_docs.md if it exists
7. **Report** — Return the documentation file path

## Cookbook

<If: git diff against origin/main is empty>
<Then: check `git status` for uncommitted changes and `git log origin/main..HEAD` for commits. If no changes exist at all, inform the user there is nothing to document.>

<If: screenshots directory does not exist or is empty>
<Then: warn the user that no screenshots were found. Proceed without screenshots and omit the Screenshots section.>

<If: ADW ID not provided>
<Then: ask the user for the ADW ID before proceeding. This is required for the filename and metadata.>

<If: multiple spec files match the ADW ID>
<Then: list the matching files and ask the user to confirm which spec to use.>

<If: large diff spanning many files>
<Then: group related changes by feature area in the documentation. Focus on architectural overview rather than line-by-line detail.>

## Validation

Before writing the documentation file, verify:

- The ADW ID is present in the metadata
- The git diff was successfully analyzed (at least one changed file identified)
- All referenced screenshot files actually exist in `docs/assets/` after copying
- The documentation covers the key sections: Overview, What Was Built, Technical Implementation, How to Use
- File paths in the document are relative to `docs/`

## Examples

### Example 1: Document with Spec and Screenshots

**User says:** "Document feature ADW-042 using specs/feat-ADW-042-auth.md with screenshots from review_img/"

**Actions:**

1. Set adw_id=ADW-042, spec_path=specs/feat-ADW-042-auth.md, screenshots_dir=review_img/
2. Run git diff analysis against origin/main
3. Read the spec to understand requirements
4. Copy screenshots to docs/assets/
5. Generate documentation referencing both spec and screenshots
6. Update conditional_docs.md if it exists
7. Return: `docs/feature-ADW-042-user-auth.md`

### Example 2: Document from Branch Only

**User says:** "Document this feature, ADW-099"

**Actions:**

1. Set adw_id=ADW-099, no spec or screenshots
2. Run git diff analysis against origin/main
3. Search specs/ for a matching spec file (optional)
4. Generate documentation based purely on code changes
5. Omit Screenshots section from output
6. Return: `docs/feature-ADW-099-descriptive-name.md`

### Example 3: Document with Auto-Discovered Spec

**User says:** "Write up what was built for ADW-055"

**Actions:**

1. Set adw_id=ADW-055
2. Search specs/ for files containing "ADW-055" in the filename
3. If found, use it as the spec; if not, proceed without
4. Run git diff analysis
5. Generate documentation
6. Return: `docs/feature-ADW-055-descriptive-name.md`
