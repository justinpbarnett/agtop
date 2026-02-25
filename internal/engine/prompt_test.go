package engine

import (
	"strings"
	"testing"
)

func TestBuildPromptComplete(t *testing.T) {
	skill := &Skill{
		Name:    "build",
		Content: "# Build Skill\n\nBuild the code.",
	}
	pctx := PromptContext{
		WorkDir:        "/tmp/worktree/001",
		Branch:         "agtop/001",
		PreviousOutput: "Spec written to SPEC.md",
		UserPrompt:     "Add JWT authentication",
	}

	result := BuildPrompt(skill, pctx)

	if !strings.Contains(result, "# Build Skill") {
		t.Error("prompt missing skill content")
	}
	if !strings.Contains(result, "Build the code.") {
		t.Error("prompt missing skill body")
	}
	if !strings.Contains(result, "/tmp/worktree/001") {
		t.Error("prompt missing workdir")
	}
	if !strings.Contains(result, "agtop/001") {
		t.Error("prompt missing branch")
	}
	if !strings.Contains(result, "Spec written to SPEC.md") {
		t.Error("prompt missing previous output")
	}
	if !strings.Contains(result, "Add JWT authentication") {
		t.Error("prompt missing user prompt")
	}
	if !strings.Contains(result, "## Context") {
		t.Error("prompt missing Context section")
	}
	if !strings.Contains(result, "## Task") {
		t.Error("prompt missing Task section")
	}
}

func TestBuildPromptNoPreviousOutput(t *testing.T) {
	skill := &Skill{
		Name:    "build",
		Content: "Build things.",
	}
	pctx := PromptContext{
		WorkDir:    "/tmp/worktree",
		Branch:     "main",
		UserPrompt: "Fix the bug",
	}

	result := BuildPrompt(skill, pctx)

	if strings.Contains(result, "Previous skill output") {
		t.Error("prompt should not contain 'Previous skill output' when empty")
	}
	if !strings.Contains(result, "Fix the bug") {
		t.Error("prompt missing user prompt")
	}
}

func TestBuildPromptMinimal(t *testing.T) {
	skill := &Skill{
		Name:    "quick",
		Content: "Do it.",
	}
	pctx := PromptContext{
		UserPrompt: "Hello world",
	}

	result := BuildPrompt(skill, pctx)

	if !strings.HasPrefix(result, "Do it.") {
		t.Errorf("prompt should start with skill content, got: %q", result[:20])
	}
	if !strings.Contains(result, "Hello world") {
		t.Error("prompt missing user prompt")
	}
	if strings.Contains(result, "Previous skill output") {
		t.Error("prompt should not contain 'Previous skill output'")
	}
}

func TestBuildPromptSafetyPreamblePresent(t *testing.T) {
	skill := &Skill{
		Name:    "build",
		Content: "Build things.",
	}
	pctx := PromptContext{
		WorkDir:    "/tmp/worktree",
		Branch:     "main",
		UserPrompt: "Fix the bug",
		SafetyPatterns: []string{
			`rm\s+-[rf]+\s+/`,
			`git\s+push.*--force`,
			`DROP\s+TABLE`,
		},
	}

	result := BuildPrompt(skill, pctx)

	if !strings.Contains(result, "## Safety Constraints") {
		t.Error("prompt missing Safety Constraints section")
	}
	if !strings.Contains(result, "MUST NOT execute") {
		t.Error("prompt missing safety instruction")
	}
	if !strings.Contains(result, `rm\s+-[rf]+\s+/`) {
		t.Error("prompt missing rm pattern")
	}
	if !strings.Contains(result, `git\s+push.*--force`) {
		t.Error("prompt missing git push pattern")
	}
	if !strings.Contains(result, `DROP\s+TABLE`) {
		t.Error("prompt missing DROP TABLE pattern")
	}
	if !strings.Contains(result, "blocked by safety policy") {
		t.Error("prompt missing safety policy instruction")
	}
}

func TestBuildPromptSafetyPreambleAbsent(t *testing.T) {
	skill := &Skill{
		Name:    "build",
		Content: "Build things.",
	}
	pctx := PromptContext{
		WorkDir:    "/tmp/worktree",
		Branch:     "main",
		UserPrompt: "Fix the bug",
	}

	result := BuildPrompt(skill, pctx)

	if strings.Contains(result, "## Safety Constraints") {
		t.Error("prompt should not contain Safety Constraints when no patterns provided")
	}
}

func TestBuildPromptPreservesSkillContent(t *testing.T) {
	content := "# Big Skill\n\n## Section 1\n\nParagraph with **bold** text.\n\n```go\nfunc main() {}\n```\n\n- item a\n- item b"
	skill := &Skill{
		Name:    "big",
		Content: content,
	}
	pctx := PromptContext{
		UserPrompt: "do it",
	}

	result := BuildPrompt(skill, pctx)

	if !strings.Contains(result, content) {
		t.Error("prompt did not preserve skill content verbatim")
	}
}

func TestBuildPromptSkillTaskOverride(t *testing.T) {
	// Utility skills (test, commit, review, document) should get a fixed task,
	// not the user's raw prompt which can cause them to go off-script.
	for _, skillName := range []string{"test", "commit", "review", "document"} {
		t.Run(skillName, func(t *testing.T) {
			skill := &Skill{
				Name:    skillName,
				Content: "# " + skillName + " skill",
			}
			pctx := PromptContext{
				WorkDir:    "/tmp/worktree",
				Branch:     "main",
				UserPrompt: "do a comprehensive review and implement refactoring",
			}

			result := BuildPrompt(skill, pctx)

			if strings.Contains(result, "comprehensive review and implement refactoring") {
				t.Errorf("skill %q should not receive the raw user prompt as its task", skillName)
			}

			override := skillTaskOverrides[skillName]
			if !strings.Contains(result, override) {
				t.Errorf("skill %q missing its fixed task override in prompt", skillName)
			}
		})
	}
}

func TestBuildPromptNoOverrideForBuild(t *testing.T) {
	// Non-utility skills (build, spec, route, decompose) should receive the
	// user's original prompt — they need it to understand what to implement.
	skill := &Skill{
		Name:    "build",
		Content: "# Build skill",
	}
	pctx := PromptContext{
		UserPrompt: "Add JWT authentication",
	}

	result := BuildPrompt(skill, pctx)

	if !strings.Contains(result, "Add JWT authentication") {
		t.Error("build skill should receive the user's original prompt")
	}
}

func TestBuildPromptWorkflowNames(t *testing.T) {
	skill := &Skill{
		Name:    "route",
		Content: "# Route skill",
	}
	pctx := PromptContext{
		UserPrompt:    "Fix the bug",
		WorkflowNames: []string{"build", "plan-build", "sdlc", "quick-fix", "my-custom"},
	}

	result := BuildPrompt(skill, pctx)

	if !strings.Contains(result, "Available workflows: build, plan-build, sdlc, quick-fix, my-custom") {
		t.Error("prompt missing workflow names")
	}
}

func TestBuildPromptWorkflowNamesAbsent(t *testing.T) {
	skill := &Skill{
		Name:    "build",
		Content: "# Build skill",
	}
	pctx := PromptContext{
		UserPrompt: "Fix the bug",
	}

	result := BuildPrompt(skill, pctx)

	if strings.Contains(result, "Available workflows") {
		t.Error("prompt should not contain Available workflows when none provided")
	}
}

func TestBuildPrompt_IncludesSpecFile(t *testing.T) {
	skill := &Skill{
		Name:    "build",
		Content: "# Build skill",
	}
	pctx := PromptContext{
		UserPrompt: "Implement the feature",
		SpecFile:   "specs/feat-user-auth.md",
	}

	result := BuildPrompt(skill, pctx)

	if !strings.Contains(result, "Spec file: specs/feat-user-auth.md") {
		t.Error("prompt missing spec file path")
	}
}

func TestBuildPrompt_IncludesModifiedFiles(t *testing.T) {
	skill := &Skill{
		Name:    "review",
		Content: "# Review skill",
	}
	pctx := PromptContext{
		UserPrompt:    "Review the changes",
		ModifiedFiles: []string{"internal/engine/executor.go", "internal/engine/prompt.go"},
	}

	result := BuildPrompt(skill, pctx)

	if !strings.Contains(result, "Files modified by previous step: internal/engine/executor.go, internal/engine/prompt.go") {
		t.Error("prompt missing modified files list")
	}
}

func TestBuildPrompt_OmitsEmptyHandoffFields(t *testing.T) {
	skill := &Skill{
		Name:    "build",
		Content: "# Build skill",
	}
	pctx := PromptContext{
		UserPrompt: "Fix the bug",
	}

	result := BuildPrompt(skill, pctx)

	if strings.Contains(result, "Spec file:") {
		t.Error("prompt should not contain 'Spec file:' when SpecFile is empty")
	}
	if strings.Contains(result, "Files modified by previous step:") {
		t.Error("prompt should not contain 'Files modified by previous step:' when ModifiedFiles is empty")
	}
}
