package engine

import (
	"fmt"
	"strings"
)

type PromptContext struct {
	WorkDir        string            // Worktree path
	Branch         string            // Git branch name
	PreviousOutput string            // Summary from previous skill (empty for first skill)
	UserPrompt     string            // The user's original task description
	SafetyPatterns []string          // Blocked command patterns for safety preamble
	WorkflowNames  []string          // Available workflow names (injected for route skill)
	SpecFile       string            // Path to the generated spec file (set after spec skill)
	ModifiedFiles  []string          // Files changed by the previous skill (from git diff --name-only)
	Repos          map[string]string // Multi-repo: relative path → worktree path (nil for single-repo)
}

// skillTaskOverrides maps skill names to fixed task descriptions.
// These skills have a well-defined job regardless of the user's original prompt.
// Without this, the raw user prompt (e.g. "review the project and implement
// refactoring opportunities") leaks into utility skills and causes them to go
// off-script — exploring, implementing, or duplicating work done by earlier skills.
var skillTaskOverrides = map[string]string{
	"test":     "Run the project's full validation suite (lint, typecheck, tests). If any checks fail, diagnose and fix the issues, then re-run to confirm. Produce the JSON report.",
	"commit":   "Review all uncommitted changes in this worktree and create atomic commits using conventional commit format. If there are no changes to commit, do nothing.",
	"review":   "Review the implemented changes against the spec to verify correctness and completeness. Classify any issues found by severity. Produce the structured review report.",
	"document": "Generate documentation for the changes made on this branch.",
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

	if len(pctx.Repos) > 0 {
		b.WriteString("\n- Multi-repo project with sub-repositories:")
		for relPath, wtPath := range pctx.Repos {
			b.WriteString(fmt.Sprintf("\n  - %s → %s", relPath, wtPath))
		}
	}
	if len(pctx.WorkflowNames) > 0 {
		b.WriteString("\n- Available workflows: ")
		b.WriteString(strings.Join(pctx.WorkflowNames, ", "))
	}

	if pctx.PreviousOutput != "" {
		b.WriteString("\n- Previous skill output:\n")
		b.WriteString(pctx.PreviousOutput)
	}

	if pctx.SpecFile != "" {
		b.WriteString("\n- Spec file: ")
		b.WriteString(pctx.SpecFile)
	}

	if len(pctx.ModifiedFiles) > 0 {
		b.WriteString("\n- Files modified by previous step: ")
		b.WriteString(strings.Join(pctx.ModifiedFiles, ", "))
	}

	// Use a fixed task for utility skills so the user's raw prompt doesn't
	// cause them to go off-script (e.g. test skill doing implementation).
	task := pctx.UserPrompt
	if override, ok := skillTaskOverrides[skill.Name]; ok {
		task = override
	}

	b.WriteString("\n\n## Task\n\n")
	b.WriteString(task)

	return b.String()
}
