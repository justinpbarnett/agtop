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
