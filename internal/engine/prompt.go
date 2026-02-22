package engine

import (
	"fmt"
	"strings"
)

type PromptContext struct {
	WorkDir        string   // Worktree path
	Branch         string   // Git branch name
	PreviousOutput string   // Summary from previous skill (empty for first skill)
	UserPrompt     string   // The user's original task description
	SafetyPatterns []string // Blocked command patterns for safety preamble
}

// BuildPrompt assembles the final prompt for a claude -p invocation by
// combining the skill's markdown body with run context.
func BuildPrompt(skill *Skill, pctx PromptContext) string {
	var b strings.Builder

	b.WriteString(skill.Content)

	if len(pctx.SafetyPatterns) > 0 {
		b.WriteString("\n\n---\n\n## Safety Constraints\n\n")
		b.WriteString("You MUST NOT execute any of the following command patterns under any circumstances:\n")
		for _, p := range pctx.SafetyPatterns {
			b.WriteString(fmt.Sprintf("- `%s`\n", p))
		}
		b.WriteString("\nIf a task requires any of these operations, STOP and report that the operation is blocked by safety policy. Do not attempt workarounds.")
	}

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
