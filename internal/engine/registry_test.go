package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinpbarnett/agtop/internal/config"
)

func writeSkillFile(t *testing.T, dir, name, content string) {
	t.Helper()
	skillDir := filepath.Join(dir, name)
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	return &cfg
}

func TestRegistryLoadFromDirectory(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "build", `---
name: build
description: Build skill
---

Build things.
`)
	writeSkillFile(t, skillsDir, "test", `---
name: test
description: Test skill
---

Test things.
`)

	reg := NewRegistry(testConfig())
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if len(reg.skills) < 2 {
		t.Fatalf("expected at least 2 skills, got %d", len(reg.skills))
	}

	s, ok := reg.Get("build")
	if !ok {
		t.Fatal("expected to find 'build' skill")
	}
	if s.Description != "Build skill" {
		t.Errorf("build.Description = %q, want %q", s.Description, "Build skill")
	}

	s, ok = reg.Get("test")
	if !ok {
		t.Fatal("expected to find 'test' skill")
	}
	if s.Description != "Test skill" {
		t.Errorf("test.Description = %q, want %q", s.Description, "Test skill")
	}
}

func TestRegistryPrecedenceOverride(t *testing.T) {
	tmp := t.TempDir()

	// Lower precedence: .agents/skills (priority 2)
	agentsDir := filepath.Join(tmp, ".agents", "skills")
	writeSkillFile(t, agentsDir, "build", `---
name: build
description: agents build
---

Agents build content.
`)

	// Higher precedence: .agtop/skills (priority 0)
	agtopDir := filepath.Join(tmp, ".agtop", "skills")
	writeSkillFile(t, agtopDir, "build", `---
name: build
description: agtop build
---

Agtop build content.
`)

	reg := NewRegistry(testConfig())
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	s, ok := reg.Get("build")
	if !ok {
		t.Fatal("expected to find 'build' skill")
	}
	if s.Description != "agtop build" {
		t.Errorf("build.Description = %q, want %q (higher precedence)", s.Description, "agtop build")
	}
	if s.Priority != PriorityProjectAgtop {
		t.Errorf("build.Priority = %d, want %d", s.Priority, PriorityProjectAgtop)
	}
}

func TestRegistryConfigMerge(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "route", `---
name: route
description: Route skill
model: haiku
---

Route content.
`)

	cfg := testConfig()
	// Config overrides frontmatter model
	cfg.Skills["route"] = config.SkillConfig{Model: "opus", Timeout: 30}

	reg := NewRegistry(cfg)
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	s, ok := reg.Get("route")
	if !ok {
		t.Fatal("expected to find 'route' skill")
	}
	if s.Model != "opus" {
		t.Errorf("route.Model = %q, want %q (config override)", s.Model, "opus")
	}
	if s.Timeout != 30 {
		t.Errorf("route.Timeout = %d, want %d (config override)", s.Timeout, 30)
	}
}

func TestRegistryConfigAllowedToolsMerge(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "review", `---
name: review
description: Review skill
allowed-tools:
  - Read
---

Review content.
`)

	cfg := testConfig()
	cfg.Skills["review"] = config.SkillConfig{
		Model:        "opus",
		AllowedTools: []string{"Read", "Write"},
	}

	reg := NewRegistry(cfg)
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	s, ok := reg.Get("review")
	if !ok {
		t.Fatal("expected to find 'review' skill")
	}
	if len(s.AllowedTools) != 2 || s.AllowedTools[0] != "Read" || s.AllowedTools[1] != "Write" {
		t.Errorf("review.AllowedTools = %v, want [Read Write] (config override)", s.AllowedTools)
	}
}

func TestRegistrySkipsMissingDirectories(t *testing.T) {
	tmp := t.TempDir()
	// No skill directories exist at all

	reg := NewRegistry(testConfig())
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}
	// Should not error, registry may have skills from user home dirs
	// but the test project root has none
}

func TestRegistrySkipsMalformedSkills(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "good", `---
name: good
description: Good skill
---

Good content.
`)
	writeSkillFile(t, skillsDir, "bad", `---
name: [invalid
---

Bad content.
`)

	reg := NewRegistry(testConfig())
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	if _, ok := reg.Get("good"); !ok {
		t.Error("expected 'good' skill to be loaded despite malformed sibling")
	}
	if _, ok := reg.Get("bad"); ok {
		t.Error("expected 'bad' skill to be skipped")
	}
}

func TestRegistryGet(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "build", `---
name: build
description: Build
---

Build.
`)

	reg := NewRegistry(testConfig())
	_ = reg.Load(tmp, nil)

	if _, ok := reg.Get("build"); !ok {
		t.Error("Get('build') returned false, want true")
	}
	if _, ok := reg.Get("nonexistent"); ok {
		t.Error("Get('nonexistent') returned true, want false")
	}
}

func TestRegistryList(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "charlie", `---
name: charlie
description: C
---

C.
`)
	writeSkillFile(t, skillsDir, "alpha", `---
name: alpha
description: A
---

A.
`)
	writeSkillFile(t, skillsDir, "bravo", `---
name: bravo
description: B
---

B.
`)

	reg := NewRegistry(testConfig())
	_ = reg.Load(tmp, nil)

	list := reg.List()
	// Filter to just our test skills (user home dirs may add others)
	testSkills := []*Skill{}
	for _, s := range list {
		if s.Name == "alpha" || s.Name == "bravo" || s.Name == "charlie" {
			testSkills = append(testSkills, s)
		}
	}

	if len(testSkills) != 3 {
		t.Fatalf("expected 3 test skills, got %d", len(testSkills))
	}
	if testSkills[0].Name != "alpha" {
		t.Errorf("List()[0].Name = %q, want %q", testSkills[0].Name, "alpha")
	}
	if testSkills[1].Name != "bravo" {
		t.Errorf("List()[1].Name = %q, want %q", testSkills[1].Name, "bravo")
	}
	if testSkills[2].Name != "charlie" {
		t.Errorf("List()[2].Name = %q, want %q", testSkills[2].Name, "charlie")
	}
}

func TestRegistryNames(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "zeta", `---
name: zeta
description: Z
---

Z.
`)
	writeSkillFile(t, skillsDir, "alpha", `---
name: alpha
description: A
---

A.
`)

	reg := NewRegistry(testConfig())
	_ = reg.Load(tmp, nil)

	names := reg.Names()
	// Find our test names
	foundAlpha, foundZeta := false, false
	for i, n := range names {
		if n == "alpha" {
			foundAlpha = true
			// Check that zeta comes after alpha (sorted)
			for j := i + 1; j < len(names); j++ {
				if names[j] == "zeta" {
					foundZeta = true
				}
			}
		}
	}
	if !foundAlpha {
		t.Error("Names() missing 'alpha'")
	}
	if !foundZeta {
		t.Error("Names() missing 'zeta' or not sorted after 'alpha'")
	}
}

func TestRegistryOpenCodeProjectSkills(t *testing.T) {
	tmp := t.TempDir()

	// Place a skill in .opencode/skills/
	opencodeDir := filepath.Join(tmp, ".opencode", "skills")
	writeSkillFile(t, opencodeDir, "deploy", `---
name: deploy
description: opencode deploy
---

Deploy content.
`)

	reg := NewRegistry(testConfig())
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	s, ok := reg.Get("deploy")
	if !ok {
		t.Fatal("expected to find 'deploy' skill from .opencode/skills/")
	}
	if s.Description != "opencode deploy" {
		t.Errorf("deploy.Description = %q, want %q", s.Description, "opencode deploy")
	}
	if s.Priority != PriorityProjectOpenCode {
		t.Errorf("deploy.Priority = %d, want %d", s.Priority, PriorityProjectOpenCode)
	}
}

func TestRegistryOpenCodeOverriddenByClaudeAndAgtop(t *testing.T) {
	tmp := t.TempDir()

	// .opencode/skills/ (lower precedence)
	opencodeDir := filepath.Join(tmp, ".opencode", "skills")
	writeSkillFile(t, opencodeDir, "build", `---
name: build
description: opencode build
---

OpenCode build.
`)

	// .claude/skills/ (higher precedence)
	claudeDir := filepath.Join(tmp, ".claude", "skills")
	writeSkillFile(t, claudeDir, "build", `---
name: build
description: claude build
---

Claude build.
`)

	reg := NewRegistry(testConfig())
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	s, ok := reg.Get("build")
	if !ok {
		t.Fatal("expected to find 'build' skill")
	}
	if s.Description != "claude build" {
		t.Errorf("build.Description = %q, want %q (claude should override opencode)", s.Description, "claude build")
	}
	if s.Priority != PriorityProjectClaude {
		t.Errorf("build.Priority = %d, want %d", s.Priority, PriorityProjectClaude)
	}
}

func TestRegistrySkillForRun(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "review", `---
name: review
description: Review skill
---

Review content.
`)

	cfg := testConfig()
	cfg.Skills["review"] = config.SkillConfig{
		Model:        "opus",
		AllowedTools: []string{"Read", "Grep", "Glob"},
	}
	cfg.Runtime.Claude.Model = "sonnet"
	cfg.Runtime.Claude.MaxTurns = 50
	cfg.Runtime.Claude.PermissionMode = "acceptEdits"

	reg := NewRegistry(cfg)
	_ = reg.Load(tmp, nil)

	skill, opts, ok := reg.SkillForRun("review")
	if !ok {
		t.Fatal("SkillForRun('review') returned false")
	}
	if skill.Name != "review" {
		t.Errorf("skill.Name = %q, want %q", skill.Name, "review")
	}
	// Model should be skill-specific config (opus), not runtime default (sonnet)
	if opts.Model != "opus" {
		t.Errorf("opts.Model = %q, want %q", opts.Model, "opus")
	}
	if len(opts.AllowedTools) != 3 {
		t.Errorf("opts.AllowedTools = %v, want [Read Grep Glob]", opts.AllowedTools)
	}
	if opts.MaxTurns != 50 {
		t.Errorf("opts.MaxTurns = %d, want %d", opts.MaxTurns, 50)
	}
	if opts.PermissionMode != "acceptEdits" {
		t.Errorf("opts.PermissionMode = %q, want %q", opts.PermissionMode, "acceptEdits")
	}

	// Nonexistent skill
	_, _, ok = reg.SkillForRun("nonexistent")
	if ok {
		t.Error("SkillForRun('nonexistent') returned true, want false")
	}
}

func TestRegistrySkillForRunFallsBackToRuntimeDefault(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	// Skill with no model in frontmatter or config
	writeSkillFile(t, skillsDir, "custom", `---
name: custom
description: Custom skill
---

Custom content.
`)

	cfg := testConfig()
	cfg.Runtime.Claude.Model = "sonnet"
	// No config override for "custom"

	reg := NewRegistry(cfg)
	_ = reg.Load(tmp, nil)

	_, opts, ok := reg.SkillForRun("custom")
	if !ok {
		t.Fatal("SkillForRun('custom') returned false")
	}
	// Should fall back to runtime default
	if opts.Model != "sonnet" {
		t.Errorf("opts.Model = %q, want %q (runtime default)", opts.Model, "sonnet")
	}
}

func TestRegistrySkillForRunOpenCode(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	writeSkillFile(t, skillsDir, "build", `---
name: build
description: Build skill
---

Build content.
`)

	cfg := testConfig()
	cfg.Runtime.Default = "opencode"
	cfg.Runtime.OpenCode.Model = "anthropic/claude-sonnet-4-5"
	cfg.Runtime.OpenCode.Agent = "code"

	reg := NewRegistry(cfg)
	_ = reg.Load(tmp, nil)

	skill, opts, ok := reg.SkillForRun("build")
	if !ok {
		t.Fatal("SkillForRun('build') returned false")
	}
	if skill.Name != "build" {
		t.Errorf("skill.Name = %q, want %q", skill.Name, "build")
	}
	// Skill config overrides to "sonnet" (via mergeConfig), which gets translated
	// to the OpenCode provider/model format.
	if opts.Model != "anthropic/claude-sonnet-4-6" {
		t.Errorf("opts.Model = %q, want %q (skill config override, translated)", opts.Model, "anthropic/claude-sonnet-4-6")
	}
	if opts.Agent != "code" {
		t.Errorf("opts.Agent = %q, want %q", opts.Agent, "code")
	}
	// OpenCode doesn't set these
	if opts.MaxTurns != 0 {
		t.Errorf("opts.MaxTurns = %d, want 0 for OpenCode", opts.MaxTurns)
	}
	if opts.PermissionMode != "" {
		t.Errorf("opts.PermissionMode = %q, want empty for OpenCode", opts.PermissionMode)
	}
	if len(opts.AllowedTools) != 0 {
		t.Errorf("opts.AllowedTools = %v, want empty for OpenCode", opts.AllowedTools)
	}
}

func TestRegistryIgnoreSkill(t *testing.T) {
	tmp := t.TempDir()

	// claude/skills has a "build" skill (priority 1)
	claudeDir := filepath.Join(tmp, ".claude", "skills")
	writeSkillFile(t, claudeDir, "build", `---
name: build
description: claude build
---

Claude build.
`)
	// claude/skills also has an unrelated skill that should still load
	writeSkillFile(t, claudeDir, "deploy", `---
name: deploy
description: claude deploy
---

Claude deploy.
`)

	cfg := testConfig()
	cfg.Skills["build"] = config.SkillConfig{Ignore: true}

	reg := NewRegistry(cfg)
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// "build" from .claude/skills should be ignored
	if _, ok := reg.Get("build"); ok {
		t.Error("expected 'build' from .claude/skills to be ignored, but it was loaded")
	}

	// "deploy" from .claude/skills is not ignored — should still load
	s, ok := reg.Get("deploy")
	if !ok {
		t.Fatal("expected 'deploy' to still load (not ignored)")
	}
	if s.Description != "claude deploy" {
		t.Errorf("deploy.Description = %q, want %q", s.Description, "claude deploy")
	}
}

func TestRegistryIgnoreSkillAgtopSourceNotFiltered(t *testing.T) {
	tmp := t.TempDir()

	// .agtop/skills has "build" (priority 0 — always allowed when ignore=true)
	agtopDir := filepath.Join(tmp, ".agtop", "skills")
	writeSkillFile(t, agtopDir, "build", `---
name: build
description: agtop build
---

Agtop build.
`)

	// .claude/skills also has "build" but with lower priority
	claudeDir := filepath.Join(tmp, ".claude", "skills")
	writeSkillFile(t, claudeDir, "build", `---
name: build
description: claude build
---

Claude build.
`)

	cfg := testConfig()
	cfg.Skills["build"] = config.SkillConfig{Ignore: true}

	reg := NewRegistry(cfg)
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// .agtop/skills version should win (ignore=true never blocks agtop/builtin)
	s, ok := reg.Get("build")
	if !ok {
		t.Fatal("expected 'build' from .agtop/skills to be loaded")
	}
	if s.Description != "agtop build" {
		t.Errorf("build.Description = %q, want %q (agtop should survive ignore)", s.Description, "agtop build")
	}
	if s.Priority != PriorityProjectAgtop {
		t.Errorf("build.Priority = %d, want %d", s.Priority, PriorityProjectAgtop)
	}
}

func TestRegistryIgnoreSkillNoFallback(t *testing.T) {
	tmp := t.TempDir()

	// "custom" only exists in .claude/skills — no agtop/builtin version
	claudeDir := filepath.Join(tmp, ".claude", "skills")
	writeSkillFile(t, claudeDir, "custom", `---
name: custom
description: custom skill
---

Custom.
`)

	cfg := testConfig()
	cfg.Skills["custom"] = config.SkillConfig{Ignore: true}

	reg := NewRegistry(cfg)
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// No fallback — skill should be absent entirely
	if _, ok := reg.Get("custom"); ok {
		t.Error("expected 'custom' to be absent (ignored with no fallback)")
	}
}

func TestRegistryIgnoreSource(t *testing.T) {
	tmp := t.TempDir()

	// .claude/skills (project-claude) — should be ignored
	claudeDir := filepath.Join(tmp, ".claude", "skills")
	writeSkillFile(t, claudeDir, "build", `---
name: build
description: claude build
---

Claude build.
`)
	writeSkillFile(t, claudeDir, "test", `---
name: test
description: claude test
---

Claude test.
`)

	// .agtop/skills (project-agtop) — should still load
	agtopDir := filepath.Join(tmp, ".agtop", "skills")
	writeSkillFile(t, agtopDir, "deploy", `---
name: deploy
description: agtop deploy
---

Agtop deploy.
`)

	cfg := testConfig()
	cfg.Project.IgnoreSkillSources = []string{"project-claude"}

	reg := NewRegistry(cfg)
	if err := reg.Load(tmp, nil); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	// All .claude/skills should be filtered
	if _, ok := reg.Get("build"); ok {
		t.Error("expected 'build' from project-claude to be ignored")
	}
	if _, ok := reg.Get("test"); ok {
		t.Error("expected 'test' from project-claude to be ignored")
	}

	// .agtop/skills should still load
	s, ok := reg.Get("deploy")
	if !ok {
		t.Fatal("expected 'deploy' from .agtop/skills to be loaded")
	}
	if s.Description != "agtop deploy" {
		t.Errorf("deploy.Description = %q, want %q", s.Description, "agtop deploy")
	}
}

func TestRegistrySkillForRunOpenCodeFallsBackToRuntimeDefault(t *testing.T) {
	tmp := t.TempDir()
	skillsDir := filepath.Join(tmp, ".agtop", "skills")

	// Skill with no model override
	writeSkillFile(t, skillsDir, "custom", `---
name: custom
description: Custom skill
---

Custom content.
`)

	cfg := testConfig()
	cfg.Runtime.Default = "opencode"
	cfg.Runtime.OpenCode.Model = "openai/gpt-4o"
	cfg.Runtime.OpenCode.Agent = "build"
	// Remove any config override for "custom"
	delete(cfg.Skills, "custom")

	reg := NewRegistry(cfg)
	_ = reg.Load(tmp, nil)

	_, opts, ok := reg.SkillForRun("custom")
	if !ok {
		t.Fatal("SkillForRun('custom') returned false")
	}
	if opts.Model != "openai/gpt-4o" {
		t.Errorf("opts.Model = %q, want %q (opencode runtime default)", opts.Model, "openai/gpt-4o")
	}
	if opts.Agent != "build" {
		t.Errorf("opts.Agent = %q, want %q", opts.Agent, "build")
	}
}
