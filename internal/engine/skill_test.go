package engine

import (
	"path/filepath"
	"testing"
)

func TestParseSkillWithFullFrontmatter(t *testing.T) {
	skill, err := ParseSkillFile(filepath.Join("testdata", "valid", "SKILL.md"), PriorityBuiltIn)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "test-skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "test-skill")
	}
	if skill.Description != "A test skill for unit testing" {
		t.Errorf("Description = %q, want %q", skill.Description, "A test skill for unit testing")
	}
	if skill.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", skill.Model, "sonnet")
	}
	if skill.Timeout != 120 {
		t.Errorf("Timeout = %d, want %d", skill.Timeout, 120)
	}
	if len(skill.AllowedTools) != 2 || skill.AllowedTools[0] != "Read" || skill.AllowedTools[1] != "Grep" {
		t.Errorf("AllowedTools = %v, want [Read Grep]", skill.AllowedTools)
	}
	if skill.Priority != PriorityBuiltIn {
		t.Errorf("Priority = %d, want %d", skill.Priority, PriorityBuiltIn)
	}
	// Content should contain the markdown body
	if !contains(skill.Content, "# Test Skill") {
		t.Errorf("Content missing heading, got: %q", skill.Content)
	}
	if !contains(skill.Content, "1. Do the first thing") {
		t.Errorf("Content missing list item, got: %q", skill.Content)
	}
}

func TestParseSkillWithMinimalFrontmatter(t *testing.T) {
	skill, err := ParseSkillFile(filepath.Join("testdata", "minimal", "SKILL.md"), PriorityUserAgtop)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "minimal-skill" {
		t.Errorf("Name = %q, want %q", skill.Name, "minimal-skill")
	}
	if skill.Description != "A minimal skill" {
		t.Errorf("Description = %q, want %q", skill.Description, "A minimal skill")
	}
	if skill.Model != "" {
		t.Errorf("Model = %q, want empty", skill.Model)
	}
	if skill.Timeout != 0 {
		t.Errorf("Timeout = %d, want 0", skill.Timeout)
	}
	if skill.AllowedTools != nil {
		t.Errorf("AllowedTools = %v, want nil", skill.AllowedTools)
	}
	if skill.Content != "Do the thing." {
		t.Errorf("Content = %q, want %q", skill.Content, "Do the thing.")
	}
}

func TestParseSkillWithNoFrontmatter(t *testing.T) {
	skill, err := ParseSkillFile(filepath.Join("testdata", "no-frontmatter", "SKILL.md"), PriorityProjectClaude)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Name != "no-frontmatter" {
		t.Errorf("Name = %q, want %q", skill.Name, "no-frontmatter")
	}
	if !contains(skill.Content, "# No Frontmatter Skill") {
		t.Errorf("Content missing heading, got: %q", skill.Content)
	}
	if !contains(skill.Content, "Just raw markdown content") {
		t.Errorf("Content missing body text, got: %q", skill.Content)
	}
}

func TestParseSkillWithEmptyFrontmatter(t *testing.T) {
	skill, err := ParseSkillFile(filepath.Join("testdata", "empty-frontmatter", "SKILL.md"), PriorityProjectAgtop)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Name derived from directory
	if skill.Name != "empty-frontmatter" {
		t.Errorf("Name = %q, want %q", skill.Name, "empty-frontmatter")
	}
	if skill.Content != "Content after empty frontmatter." {
		t.Errorf("Content = %q, want %q", skill.Content, "Content after empty frontmatter.")
	}
}

func TestParseSkillWithMalformedFrontmatter(t *testing.T) {
	_, err := ParseSkillFile(filepath.Join("testdata", "malformed", "SKILL.md"), PriorityBuiltIn)
	if err == nil {
		t.Fatal("expected error for malformed frontmatter, got nil")
	}
}

func TestParseSkillPreservesContent(t *testing.T) {
	data := []byte(`---
name: preserve-test
description: test content preservation
---

# Heading

Some text with **bold** and *italic*.

` + "```go" + `
func main() {
    fmt.Println("hello")
}
` + "```" + `

- item 1
- item 2
`)
	skill, err := ParseSkill(data, "/test/preserve-test/SKILL.md", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !contains(skill.Content, "# Heading") {
		t.Error("Content missing heading")
	}
	if !contains(skill.Content, "**bold**") {
		t.Error("Content missing bold formatting")
	}
	if !contains(skill.Content, "func main()") {
		t.Error("Content missing code block")
	}
	if !contains(skill.Content, "- item 1") {
		t.Error("Content missing list items")
	}
}

func TestSkillNameFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/.agtop/skills/build/SKILL.md", "build"},
		{"/home/user/.claude/skills/my-skill/SKILL.md", "my-skill"},
		{"skills/test/SKILL.md", "test"},
		{"/a/b/c/SKILL.md", "c"},
	}
	for _, tt := range tests {
		got := SkillNameFromPath(tt.path)
		if got != tt.want {
			t.Errorf("SkillNameFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestPriorityLabel(t *testing.T) {
	tests := []struct {
		priority int
		want     string
	}{
		{PriorityProjectAgtop, "project-agtop"},
		{PriorityProjectClaude, "project-claude"},
		{PriorityProjectOpenCode, "project-opencode"},
		{PriorityProjectAgents, "project-agents"},
		{PriorityUserAgtop, "user-agtop"},
		{PriorityUserClaude, "user-claude"},
		{PriorityUserOpenCode, "user-opencode"},
		{PriorityBuiltIn, "builtin"},
		{99, ""},
		{-1, ""},
	}
	for _, tt := range tests {
		got := PriorityLabel(tt.priority)
		if got != tt.want {
			t.Errorf("PriorityLabel(%d) = %q, want %q", tt.priority, got, tt.want)
		}
	}
}

func TestParseSkillFileMissing(t *testing.T) {
	_, err := ParseSkillFile("/nonexistent/path/SKILL.md", 0)
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestParseSkillSourceAndPriority(t *testing.T) {
	data := []byte(`---
name: src-test
description: test
---

Content.
`)
	source := "/my/skills/src-test/SKILL.md"
	priority := PriorityUserClaude

	skill, err := ParseSkill(data, source, priority)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if skill.Source != source {
		t.Errorf("Source = %q, want %q", skill.Source, source)
	}
	if skill.Priority != priority {
		t.Errorf("Priority = %d, want %d", skill.Priority, priority)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
