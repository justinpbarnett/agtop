package engine

import "strings"

type PromptContext struct {
	WorkDir        string // Worktree path
	Branch         string // Git branch name
	PreviousOutput string // Summary from previous skill (empty for first skill)
	UserPrompt     string // The user's original task description
}

// BuildPrompt assembles the final prompt for a claude -p invocation by
// combining the skill's markdown body with run context.
func BuildPrompt(skill *Skill, pctx PromptContext) string {
	var b strings.Builder

	b.WriteString(skill.Content)

	b.WriteString("\n\n---\n\n## Context\n")
	if pctx.WorkDir != "" {
		b.WriteString("\n- Working directory: ")
		b.WriteString(pctx.WorkDir)
	}
	if pctx.Branch != "" {
		b.WriteString("\n- Branch: ")
		b.WriteString(pctx.Branch)
	}

	if pctx.PreviousOutput != "" {
		b.WriteString("\n- Previous skill output:\n")
		b.WriteString(pctx.PreviousOutput)
	}

	b.WriteString("\n\n## Task\n\n")
	b.WriteString(pctx.UserPrompt)

	return b.String()
}
