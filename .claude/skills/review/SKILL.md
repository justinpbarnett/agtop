---
name: review
description: >
  Reviews implemented features against a specification file to verify the
  implementation matches requirements. Compares git diffs with spec criteria,
  optionally captures screenshots of critical UI paths using Playwright, and
  produces a structured JSON report with issue severity classification. Use
  when a user wants to review work against a spec, validate an implementation,
  check if features match requirements, or verify work before merging. Triggers
  on "review the spec", "review my work", "validate against the spec", "check
  the implementation", "review this feature", "does this match the spec",
  "spec review", "review before merge". Do NOT use for implementing features
  (use the implement skill). Do NOT use for creating or writing specs (use the
  spec skill). Do NOT use for running tests or linting directly.
---

# Purpose

Reviews implemented features against a specification file to verify the work matches requirements. Produces a structured JSON report with screenshots and issue severity classification.

## Variables

- `argument` — Optional spec file path (e.g., `specs/feat-ADW-042-auth.md`). If omitted, the skill discovers the spec from the current branch name.

## Instructions

### Step 1: Determine the Spec File

Identify which spec to review against:

- **Explicit path** — If the user provides a spec file path (e.g., `specs/feat-ADW-042-feature.md`), use it directly.
- **Branch-based discovery** — If no spec is provided, run `git branch --show-current` to get the current branch name, then search `specs/` for a matching file. Match by looking for spec filenames that contain the branch name or a shared identifier (e.g., branch `feat/ADW-042-auth` matches `specs/feat-ADW-042-auth.md`).
- **Ambiguous** — If multiple specs match or none match, list available specs in `specs/` and ask the user to confirm which one to review against.

### Step 2: Gather Context

Run these commands to understand what was built:

1. `git branch --show-current` — Identify the working branch
2. `git diff origin/main` — See all changes made relative to main. Continue the review even if the diff is empty or unrelated to the spec.
3. Read the identified spec file thoroughly. Extract:
   - Required features and acceptance criteria
   - UI/UX requirements (if any)
   - API or backend requirements (if any)
   - Edge cases or constraints mentioned

### Step 3: Determine Review Strategy

Based on the spec requirements, decide which review paths apply:

- **Code review** — Always performed. Compare the git diff against spec requirements to verify all stated criteria are addressed.
- **UI review** — Performed only if the spec describes user-facing features (pages, components, visual elements). Requires the application to be running and Playwright MCP to be available.

If UI review is needed, proceed to Step 4. Otherwise, skip to Step 5.

### Step 4: UI Review with Screenshots

This step validates visible functionality. The goal is to visually confirm that implemented features match the spec.

#### 4a: Prepare the Application

Start a temporary dev server for screenshot capture. **You MUST stop it after screenshots are done** (see Step 4d).

1. Derive a deterministic port from the current directory name (the ADW ID), then start the dev server in the background:
   ```bash
   REVIEW_PORT=$(( (0x$(basename "$(pwd)" | md5sum | cut -c1-4) % 999) + 3001 ))
   pnpm dev --port $REVIEW_PORT > /dev/null 2>&1 &
   echo $! > .review-dev-pid
   echo "Review server on port $REVIEW_PORT"
   ```
2. Wait for the server to be available (poll with `curl -s -o /dev/null http://localhost:$REVIEW_PORT` until it responds, max 30 seconds).
3. Use `http://localhost:<REVIEW_PORT>` as the base URL for all screenshots in Step 4b.

**Do NOT use `just dev` — it runs in the foreground and will block indefinitely.**

#### 4b: Capture Screenshots

Use Playwright MCP server commands to navigate the application and capture screenshots:

- Navigate to the critical paths described in the spec using the port from Step 4a (e.g., `http://localhost:<REVIEW_PORT>`)
- Capture **1-5 targeted screenshots** that demonstrate the implemented functionality
- Focus on critical functionality — avoid screenshots of routine or unchanged areas
- If an issue is found, capture a screenshot of the issue specifically

Screenshot naming convention: `01_descriptive_name.png`, `02_descriptive_name.png`, etc.

Screenshot storage: Store screenshots in the `review_img/` directory. If an ADW ID or agent context is provided, use `agents/{adw_id}/{agent_name}/review_img/` relative to the project root. Create the directory if it does not exist.

#### 4c: Compare Against Spec

For each spec requirement with a UI component:
- Verify the visual implementation matches the described behavior
- Check layout, content, interactions, and error states as described
- Note any discrepancies as review issues

#### 4d: Stop the Temporary Dev Server

**CRITICAL — you MUST run this before finishing the review.** Failure to stop the dev server will prevent the parent workflow process from exiting.

```bash
if [ -f .review-dev-pid ]; then kill $(cat .review-dev-pid) 2>/dev/null; rm -f .review-dev-pid; fi
```

### Step 5: Classify Issues

For each issue found during review, classify its severity using the guidelines in `references/severity-guide.md`:

- **blocker** — Prevents release. The feature does not function as specified or will harm the user experience.
- **tech_debt** — Does not prevent release but creates debt that should be addressed in a future iteration.
- **skippable** — Non-blocking and minor. A real problem but not critical to the feature's core value.

Think carefully about impact before classifying. When in doubt, lean toward the less severe classification — only `blocker` should prevent a release.

### Step 6: Produce the Report

Output the review result as a JSON object. Return ONLY the JSON — no surrounding text, markdown formatting, or explanation. The output must be valid for `JSON.parse()`.

Use the schema defined in `references/output-schema.json`.

```json
{
  "success": true,
  "review_summary": "2-4 sentence summary of what was built and whether it matches the spec.",
  "review_issues": [],
  "screenshots": []
}
```

Key rules:
- `success` is `true` if there are NO `blocker` issues (can still have `skippable` or `tech_debt` issues)
- `success` is `false` ONLY if there are `blocker` issues
- `screenshots` should always contain paths to captured screenshots, regardless of success status
- All paths must be absolute
- `review_summary` should read like a standup update: what was built, does it match, any concerns

## Workflow

1. **Determine spec** — Find the spec file from argument or branch-based discovery
2. **Gather context** — Collect git diff and extract spec requirements
3. **Review strategy** — Decide code-only or code + UI review
4. **UI review** — If needed: start temp dev server, capture screenshots, compare against spec, **stop dev server**
5. **Classify** — Assign severity to each issue found
6. **Report** — Produce a JSON report with summary, issues, and screenshots

## Cookbook

<If: no spec file found matching the current branch>
<Then: list all files in `specs/` and ask the user to specify which spec to review against>

<If: Playwright MCP server not available>
<Then: skip UI review. Perform code-only review and note in the `review_summary` that visual validation was not performed. Do not fail the review for this reason.>

<If: application fails to start for UI review>
<Then: attempt `pnpm install` for dependencies. If the hashed port is occupied, try port+1, port+2, etc. If still failing, skip UI review and note it in the summary. Code review can still proceed. Always clean up `.review-dev-pid` even on failure.>

<If: git diff is empty>
<Then: continue the review. Check `git status` for uncommitted changes. If there truly are no changes, note this in the summary but still verify whether the current codebase satisfies the spec.>

<If: unsure about issue severity>
<Then: lean toward the less severe classification. Over-classifying as `blocker` creates unnecessary churn.>

## Validation

Before finalizing the report, verify:

- Every spec requirement has been checked against the implementation
- All `blocker` issues genuinely prevent release (not just cosmetic preferences)
- Screenshots clearly demonstrate the critical functionality paths
- The JSON output is valid and parseable
- All file paths in the output are absolute

## Examples

### Example 1: Review a Specific Spec

**User says:** "Review the implementation against specs/feat-ADW-042-auth.md"

**Actions:**

1. Read `specs/feat-ADW-042-auth.md`
2. Run `git diff origin/main` to see changes
3. Compare changes against spec requirements
4. If spec includes UI requirements, start the app and capture screenshots
5. Classify any issues found
6. Output JSON report

### Example 2: Review Current Branch Work

**User says:** "Review my work against the spec"

**Actions:**

1. Run `git branch --show-current` to identify the branch
2. Search `specs/` for a matching spec file
3. If found, proceed with review; if ambiguous, ask the user
4. Compare diff against spec, capture screenshots if needed
5. Output JSON report

### Example 3: Code-Only Review

**User says:** "Review this feature against the spec — it's all backend, no UI"

**Actions:**

1. Identify the spec file
2. Run `git diff origin/main`
3. Compare code changes against spec requirements (skip UI review)
4. Classify issues
5. Output JSON report with empty `screenshots` array
