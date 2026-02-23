---
name: skill-builder
description: >
  Creates, converts, or improves Claude skills. Converts existing
  .claude/commands/ slash commands into skills, builds new skills from a
  prompt, or refactors existing skills to follow best practices. Use when
  a user wants to "convert a command to a skill", "turn this slash command
  into a skill", "create a skill", "build a skill", "make a new skill for X",
  "scaffold a skill", "generate a skill", "improve this skill", "refactor
  this skill", or "update this skill". Also triggers for "upgrade this
  command", "make a skill that does X", or when given a path to an existing
  skill with an improvement request. Do NOT use for implementing features,
  fixing bugs, running existing skills, or general code generation that is
  not a Claude skill.
---

# Purpose

Creates, converts, or improves Claude skill folders — ensuring every skill follows the standard structure (frontmatter, Purpose, Variables, Instructions, Workflow, Cookbook) and delegates to existing skills where appropriate.

## Variables

- `argument` — Path to an existing skill folder (improve mode), slash command file (convert mode), or omitted (create mode). Determines the operating mode.

## Instructions

### Step 1: Determine Mode

Identify which mode to operate in:

- **Convert mode** — The user references an existing `.claude/commands/*.md` file or asks to "convert", "upgrade", or "turn a command into a skill".
- **Create mode** — The user describes a new skill or asks to "create", "build", "scaffold", or "make" a skill.
- **Improve mode** — The user references an existing `.claude/skills/` folder or SKILL.md file and asks to "improve", "refactor", "update", or "fix" it. Also triggers when a skill path is given as an argument (e.g., `/skill-builder .claude/skills/pr`).

If ambiguous, ask the user.

### Step 2: Gather Input

**Convert mode:**

1. Read the slash command file the user referenced.
2. Note the filename and full contents.
3. Collect any additional context about purpose, usage, or edge cases.

**Create mode:**

1. Collect the user's description of what the skill should do.
2. Ask clarifying questions if vague — you need: the core workflow, required tools, trigger conditions, and output format.
3. If the user references existing skills or commands, read those for inspiration.

**Improve mode:**

1. Read the entire skill folder — SKILL.md plus all files in `references/`, `scripts/`, and `assets/`.
2. Run the Step 8 validation checklist against the existing skill to identify every violation.
3. Collect any specific feedback from the user (e.g., "the frontmatter is too long", "add a Cookbook section", "it should delegate to /commit").
4. If the user gave no specific feedback, the validation checklist is the improvement plan.

### Step 3: Analyze and Design

1. **Extract core intent** — One sentence: what does this skill accomplish?
2. **Identify triggers** — What phrases activate this skill? Think beyond the literal name — include paraphrases, adjacent tasks, and natural language variations.
3. **Check for existing skills** — Read `.claude/skills/` to see what skills already exist. If any step in the new skill overlaps with an existing skill's responsibility, the new skill must delegate to it rather than reimplementing that behavior. For example: need to commit? Use `/commit`. Need a PR? Use `/pr`. Need to run tests? Use `/test`. Never write custom git commit logic, custom PR creation, custom test runners, etc. when a skill already handles it.
4. **Map the workflow** — Break into ordered steps. Note dependencies and branching paths. Mark which steps delegate to existing skills.
5. **Identify gaps** — What's missing? Error handling, edge cases, validation?

### Step 4: Design the Folder Structure

Decide what files the skill needs:

- `SKILL.md` — Always required. Core instructions.
- `references/` — Detailed docs, API patterns, style guides. Use when content would bloat SKILL.md.
- `scripts/` — Executable code for deterministic tasks (validation, transformation). Prefer scripts over language instructions for programmatic work.
- `assets/` — Templates, static resources.

Keep SKILL.md under 500 lines. Move detailed reference material to `references/`.

### Step 5: Write the Frontmatter

The frontmatter has exactly two fields: `name` and `description`. Nothing else.

```yaml
---
name: skill-name
description: >
  [WHAT it does — 1-2 sentences]. Use when [TRIGGER CONDITIONS —
  phrases users would say]. Also triggers for [PARAPHRASES].
  Do NOT use for [NEGATIVE TRIGGERS].
---
```

Rules:

- `name`: kebab-case, must match folder name, no "claude" or "anthropic"
- `description`: under 1024 chars, includes WHAT + WHEN + WHEN NOT, no XML angle brackets
- **Do not add** `metadata`, `license`, `compatibility`, or any other fields

See `references/frontmatter-guide.md` for detailed rules and examples.

### Step 6: Write the SKILL.md Body

Every generated skill must follow this section structure:

```markdown
# Purpose

[1-2 sentences: what this skill accomplishes and why it exists.]

## Variables

- `variable_name` — What this input controls and its expected format
- [List all arguments, flags, or contextual inputs]
- If none: "This skill requires no additional input."

## Instructions

### Step 1: [First Major Step]
[Clear, specific, actionable explanation]

### Step 2: [Next Step]
[Continue for all steps...]

## Workflow

[High-level summary of the overall process — the 10,000-foot view.
A reader should understand the full flow from this section alone.]

1. [Phase 1] → [Phase 2] → [Phase 3]

## Cookbook

<If: condition or scenario>
<Then: what to do>
<Example: concrete example if helpful>

<If: another scenario>
<Then: corresponding action>
```

**Section guidance:**

- **Purpose** — 1-2 sentences. What and why, not how.
- **Variables** — Every input documented with defaults and constraints.
- **Instructions** — The core procedure. Be specific: `Run python scripts/validate.py --input {file}` beats `Validate the data`. Include error handling for likely failures. When a step can be handled by an existing skill, write "Use the `/skill-name` skill" instead of inline logic.
- **Workflow** — The big picture. Numbered phases or a flow showing the overall sequence. Steps that delegate to existing skills should name them explicitly.
- **Cookbook** — Conditional recipes for branching logic, edge cases, and alternate paths. Each recipe is an `<If:>` / `<Then:>` pair with an optional `<Example:>`. Include as many recipes as the skill needs to cover its scenarios.

### Step 7: Create Supporting Files

**Scripts (`scripts/`):**

- Use for deterministic tasks: validation, transformation, format checking
- Include error handling, exit codes, and usage messages
- Make executable with shebangs (`#!/usr/bin/env python3` or `#!/bin/bash`)

**References (`references/`):**

- Move detailed documentation here to keep SKILL.md lean
- Good candidates: API docs, style guides, schema definitions, lengthy examples

### Step 8: Validate

**Structure:**

- Folder named in kebab-case
- SKILL.md exists (exact case)
- Frontmatter has only `name` and `description` — no other fields
- `name` is kebab-case, matches folder name
- `description` includes WHAT, WHEN, and WHEN NOT
- No XML tags in frontmatter
- No README.md in the skill folder
- SKILL.md under 500 lines

**Sections:**

- `# Purpose` exists with 1-2 sentence summary
- `## Variables` exists (even if "no additional input")
- `## Instructions` exists with numbered steps
- `## Workflow` exists with high-level overview
- `## Cookbook` exists with at least one `<If:>` / `<Then:>` recipe

**Skill reuse:**

- No step reimplements what an existing skill already does
- Steps that overlap with existing skills delegate via `/skill-name`
- The skill's description includes negative triggers pointing to skills it delegates to (e.g., "Do NOT use for committing changes (use the commit skill)")

**Quality:**

- Instructions are specific and actionable
- Error handling covers likely failure modes
- References linked from SKILL.md body where relevant

**Triggering:**

- Description includes natural trigger phrases
- Description includes negative triggers
- Would trigger on paraphrased requests
- Would NOT trigger on unrelated queries

### Step 9: Write the Files

Create all files in `.claude/skills/{skill-name}/`.

### Step 10: Report

Present:

1. **File tree** — Complete skill folder structure
2. **Summary** — What was created or changed. In convert mode, highlight improvements over the original. In improve mode, list every change made and why.
3. **Trigger test cases** — 5-10 examples split into "should trigger" and "should NOT trigger" (skip in improve mode if triggers didn't change)

## Workflow

1. **Determine mode** (convert / create / improve)
2. **Gather input** — read existing files or collect user description
3. **Analyze & design** — extract intent, check for skill reuse, map workflow
4. **Structure** — decide folder layout (SKILL.md + references/ + scripts/ + assets/)
5. **Write frontmatter** — name + description only
6. **Write body** — Purpose, Variables, Instructions, Workflow, Cookbook
7. **Create supporting files** — scripts, references, assets as needed
8. **Validate** — run full checklist
9. **Write files** — create/update all files in `.claude/skills/{skill-name}/`
10. **Report** — file tree, summary, trigger test cases

## Cookbook

<If: mode is ambiguous (user doesn't clearly indicate convert, create, or improve)>
<Then: ask the user which mode they intend before proceeding>

<If: a step in the new skill overlaps with an existing skill's responsibility>
<Then: delegate to the existing skill via `/skill-name` instead of reimplementing the logic. Add a negative trigger to the description pointing to that skill.>
<Example: If a skill needs to commit changes, write "Use the `/commit` skill" instead of inline git commit logic.>

<If: SKILL.md exceeds 500 lines during writing>
<Then: move detailed reference material (API docs, style guides, lengthy examples) to `references/` and link from SKILL.md>

<If: improve mode and the user gave no specific feedback>
<Then: use the Step 8 validation checklist as the improvement plan — fix every violation found>

<If: convert mode>
<Then: don't just translate the slash command — improve it. Add missing error handling, fill in gaps, apply the full section template. The skill should be better than the original command.>

<If: create mode and the user's description is vague>
<Then: ask clarifying questions before proceeding. You need: the core workflow, required tools, trigger conditions, and output format. A skill built on wrong assumptions wastes effort.>

<If: a deterministic task (validation, transformation, format checking) needs to be performed>
<Then: write a script in `scripts/` rather than describing the task in natural language instructions. Scripts are more reliable and reproducible.>

<If: the user references an invalid path or the skill folder doesn't exist>
<Then: report the error clearly and ask the user to provide a valid path>

## Examples

### Example 1: Converting a Slash Command

**User says:** "Convert the /spec command to a skill"

**Actions:**

1. Read `.claude/commands/spec.md`
2. Analyze: extract intent, triggers, workflow, gaps
3. Design: SKILL.md + references/ for spec templates
4. Write frontmatter with name + description only
5. Write SKILL.md: Purpose, Variables, Instructions, Workflow, Cookbook
6. Create reference files
7. Validate and report

### Example 2: Creating from Scratch

**User says:** "Create a skill that helps me write database migrations"

**Actions:**

1. Gather details — what ORM? what database? what conventions?
2. Design: SKILL.md + scripts/validate-migration.py + references/migration-patterns.md
3. Write frontmatter with targeted triggers
4. Write SKILL.md following the section template
5. Create supporting files
6. Validate and report

### Example 3: Improving an Existing Skill

**User says:** `/skill-builder .claude/skills/pr`

**Actions:**

1. Read all files in `.claude/skills/pr/`
2. Run validation checklist — find issues: frontmatter has extra `metadata` field, missing `## Variables` and `## Cookbook` sections, custom git logic that should delegate to `/commit`
3. Rewrite SKILL.md: strip metadata from frontmatter, restructure body into Purpose/Variables/Instructions/Workflow/Cookbook, replace inline commit logic with `/commit` delegation
4. Validate the updated skill against all checklists
5. Report: list every change with rationale, show before/after line counts

## Performance Notes

- Quality over speed — a poorly triggered skill is worse than no skill
- The description field is the most important thing you write — invest disproportionate effort there
- Scripts beat language instructions for deterministic tasks
- In convert mode: don't just translate, improve. Add what was missing.
- In create mode: ask questions rather than guess. A skill built on wrong assumptions wastes effort.
