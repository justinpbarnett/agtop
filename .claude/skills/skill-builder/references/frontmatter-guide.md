# YAML Frontmatter Reference

Rules and examples for writing skill frontmatter.

## Fields

Skill frontmatter has exactly two fields: `name` and `description`. Nothing else.

```yaml
---
name: skill-name-in-kebab-case
description: >
  [WHAT it does — 1-2 sentences]. Use when [TRIGGER CONDITIONS —
  specific phrases users would say, file types they'd mention,
  or situations they'd describe]. Also triggers for [PARAPHRASED
  VARIATIONS]. Do NOT use for [NEGATIVE TRIGGERS — things that
  sound similar but should use a different skill or no skill].
---
```

Do not add `metadata`, `license`, `compatibility`, or any other fields. Only `name` and `description` are loaded; everything else is wasted tokens in the system prompt.

## Rules for `name`

- **kebab-case only** — lowercase, hyphens between words
- Must match the folder name exactly
- No spaces, underscores, or capitals
- Cannot contain "claude" or "anthropic" (reserved)
- Derive from the slash command filename (convert mode) or the skill's purpose (create mode)

## Rules for `description`

- **Must include** what the skill does AND when to use it
- Under 1024 characters total
- **No XML angle brackets** (`<` or `>`) — frontmatter appears in system prompt
- Include specific trigger phrases users would actually say
- Include negative triggers to prevent over-triggering

### Good Example

```yaml
description: >
  Creates structured implementation specs for development tasks categorized
  by conventional commit types (feat, fix, refactor, perf, chore, docs,
  test, build, ci). Use when a user wants to spec, plan, design, or
  scope work before implementing it. Triggers on "spec a feature", "create
  a spec", "scope this work", "design the approach", "write a spec for".
  Do NOT use for implementing or executing existing specs.
  Do NOT use for quick single-line changes that need no spec phase.
```

### Bad Example

```yaml
description: "Helps with planning"
```

Too vague, no triggers, no negative triggers, would over-trigger on any mention of "planning".

## Trigger Phrase Design

Think about:

1. **Exact matches** — Literal words users would type: "create a migration", "review this PR"
2. **Paraphrases** — Alternative wordings: "add a schema change" instead of "create a migration"
3. **Negative triggers** — Similar-sounding but wrong: "run the migration" (execution) vs. "create a migration" (creation)

For each trigger phrase ask: "Would a user actually say this?" and "Could this trigger on unrelated requests?"
