package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	if cfg.Runtime.Default != "claude" {
		t.Errorf("expected runtime default %q, got %q", "claude", cfg.Runtime.Default)
	}
	if cfg.Runtime.Claude.Model != "sonnet" {
		t.Errorf("expected claude model %q, got %q", "sonnet", cfg.Runtime.Claude.Model)
	}
	if cfg.Limits.MaxCostPerRun != 5.00 {
		t.Errorf("expected max cost 5.00, got %f", cfg.Limits.MaxCostPerRun)
	}
	if len(cfg.Workflows) != 4 {
		t.Errorf("expected 4 default workflows, got %d", len(cfg.Workflows))
	}
	if cfg.UI.ShowTokenCount == nil || !*cfg.UI.ShowTokenCount {
		t.Error("expected ShowTokenCount default to be true")
	}
	if cfg.UI.ShowCost == nil || !*cfg.UI.ShowCost {
		t.Error("expected ShowCost default to be true")
	}
	if cfg.Safety.AllowOverrides == nil || *cfg.Safety.AllowOverrides {
		t.Error("expected AllowOverrides default to be false")
	}
}

func TestLoadFromFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()

	yaml := `
project:
  name: test-project
  test_command: "go test ./..."
runtime:
  default: opencode
limits:
  max_cost_per_run: 10.00
`
	os.WriteFile(filepath.Join(tmp, "agtop.yaml"), []byte(yaml), 0644)

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	if cfg.Project.Name != "test-project" {
		t.Errorf("expected project name %q, got %q", "test-project", cfg.Project.Name)
	}
	if cfg.Runtime.Default != "opencode" {
		t.Errorf("expected runtime %q, got %q", "opencode", cfg.Runtime.Default)
	}
	if cfg.Limits.MaxCostPerRun != 10.00 {
		t.Errorf("expected max cost 10.00, got %f", cfg.Limits.MaxCostPerRun)
	}
}

func TestMergePreservesDefaults(t *testing.T) {
	t.Parallel()
	base := DefaultConfig()
	override := &Config{
		Project: ProjectConfig{Name: "override-name"},
	}

	merge(&base, override)

	if base.Project.Name != "override-name" {
		t.Errorf("expected name %q, got %q", "override-name", base.Project.Name)
	}
	if base.Runtime.Default != "claude" {
		t.Errorf("expected runtime default preserved as %q, got %q", "claude", base.Runtime.Default)
	}
	if base.Runtime.Claude.MaxTurns != 50 {
		t.Errorf("expected max turns preserved as 50, got %d", base.Runtime.Claude.MaxTurns)
	}
	if len(base.Safety.BlockedPatterns) != 6 {
		t.Errorf("expected 6 blocked patterns preserved, got %d", len(base.Safety.BlockedPatterns))
	}
}

func TestMergeWorkflows(t *testing.T) {
	t.Parallel()
	base := DefaultConfig()
	override := &Config{
		Workflows: map[string]WorkflowConfig{
			"custom": {Skills: []string{"build", "test"}},
		},
	}

	merge(&base, override)

	if _, ok := base.Workflows["custom"]; !ok {
		t.Error("expected custom workflow to be added")
	}
	if _, ok := base.Workflows["build"]; !ok {
		t.Error("expected default 'build' workflow to be preserved")
	}
	if _, ok := base.Workflows["sdlc"]; !ok {
		t.Error("expected default 'sdlc' workflow to be preserved")
	}
	if len(base.Workflows) != 5 {
		t.Errorf("expected 5 workflows (4 default + 1 custom), got %d", len(base.Workflows))
	}
}

func TestMergeSkills(t *testing.T) {
	t.Parallel()
	base := DefaultConfig()
	override := &Config{
		Skills: map[string]SkillConfig{
			"build": {Model: "opus", Timeout: 600},
		},
	}

	merge(&base, override)

	if base.Skills["build"].Model != "opus" {
		t.Errorf("expected build model %q, got %q", "opus", base.Skills["build"].Model)
	}
	if base.Skills["build"].Timeout != 600 {
		t.Errorf("expected build timeout 600, got %d", base.Skills["build"].Timeout)
	}
	if base.Skills["route"].Model != "haiku" {
		t.Errorf("expected route model preserved as %q, got %q", "haiku", base.Skills["route"].Model)
	}
}

func TestMergeSliceReplacement(t *testing.T) {
	t.Parallel()
	base := DefaultConfig()
	override := &Config{
		Safety: SafetyConfig{
			BlockedPatterns: []string{`rm\s+-rf`},
		},
	}

	merge(&base, override)

	if len(base.Safety.BlockedPatterns) != 1 {
		t.Errorf("expected 1 blocked pattern (full replacement), got %d", len(base.Safety.BlockedPatterns))
	}
	if base.Safety.BlockedPatterns[0] != `rm\s+-rf` {
		t.Errorf("expected pattern %q, got %q", `rm\s+-rf`, base.Safety.BlockedPatterns[0])
	}
}

func TestMergeBoolPtrOverride(t *testing.T) {
	t.Parallel()
	base := DefaultConfig()

	f := false
	tr := true
	override := &Config{
		UI: UIConfig{
			ShowTokenCount: &f,
			ShowCost:       &f,
		},
		Safety: SafetyConfig{
			AllowOverrides: &tr,
		},
	}

	merge(&base, override)

	if base.UI.ShowTokenCount == nil || *base.UI.ShowTokenCount != false {
		t.Error("expected ShowTokenCount to be overridden to false")
	}
	if base.UI.ShowCost == nil || *base.UI.ShowCost != false {
		t.Error("expected ShowCost to be overridden to false")
	}
	if base.Safety.AllowOverrides == nil || *base.Safety.AllowOverrides != true {
		t.Error("expected AllowOverrides to be overridden to true")
	}
}

func TestMergeBoolPtrNilPreservesDefault(t *testing.T) {
	t.Parallel()
	base := DefaultConfig()
	override := &Config{}

	merge(&base, override)

	if base.UI.ShowTokenCount == nil || *base.UI.ShowTokenCount != true {
		t.Error("expected ShowTokenCount to remain true when override is nil")
	}
	if base.UI.ShowCost == nil || *base.UI.ShowCost != true {
		t.Error("expected ShowCost to remain true when override is nil")
	}
	if base.Safety.AllowOverrides == nil || *base.Safety.AllowOverrides != false {
		t.Error("expected AllowOverrides to remain false when override is nil")
	}
}

func TestLoadEmptyFile(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "agtop.yaml"), []byte("---\n"), 0644)

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error on empty file: %v", err)
	}

	if cfg.Runtime.Default != "claude" {
		t.Errorf("expected default runtime %q, got %q", "claude", cfg.Runtime.Default)
	}
}

func TestLoadBoolFromYAML(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	os.WriteFile(filepath.Join(tmp, "agtop.yaml"), []byte(`
ui:
  show_token_count: false
  show_cost: false
safety:
  allow_overrides: true
`), 0644)

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}

	if cfg.UI.ShowTokenCount == nil || *cfg.UI.ShowTokenCount != false {
		t.Error("expected show_token_count: false from YAML to override default true")
	}
	if cfg.UI.ShowCost == nil || *cfg.UI.ShowCost != false {
		t.Error("expected show_cost: false from YAML to override default true")
	}
	if cfg.Safety.AllowOverrides == nil || *cfg.Safety.AllowOverrides != true {
		t.Error("expected allow_overrides: true from YAML to override default false")
	}
}

func TestDiscoveryChain(t *testing.T) {
	// Uses t.Setenv so cannot be parallel
	tmp := t.TempDir()

	projectDir := filepath.Join(tmp, "project")
	os.MkdirAll(projectDir, 0755)
	os.WriteFile(filepath.Join(projectDir, "agtop.yaml"), []byte(`
project:
  name: project-level
`), 0644)

	homeDir := filepath.Join(tmp, "home")
	configDir := filepath.Join(homeDir, ".config", "agtop")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.yaml"), []byte(`
project:
  name: user-level
`), 0644)

	t.Setenv("HOME", homeDir)

	cfg, err := LoadFrom(projectDir)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Project.Name != "project-level" {
		t.Errorf("expected project-level config, got %q", cfg.Project.Name)
	}

	emptyDir := filepath.Join(tmp, "empty")
	os.MkdirAll(emptyDir, 0755)

	cfg, err = LoadFrom(emptyDir)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Project.Name != "user-level" {
		t.Errorf("expected user-level config fallback, got %q", cfg.Project.Name)
	}
}

// Env override tests use t.Setenv, so they cannot be parallel.

func TestEnvOverrideRuntime(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGTOP_RUNTIME", "opencode")

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Runtime.Default != "opencode" {
		t.Errorf("expected runtime %q, got %q", "opencode", cfg.Runtime.Default)
	}
}

func TestEnvOverrideMaxCost(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGTOP_MAX_COST", "10.50")

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Limits.MaxCostPerRun != 10.50 {
		t.Errorf("expected max cost 10.50, got %f", cfg.Limits.MaxCostPerRun)
	}
}

func TestEnvOverrideInvalidFloat(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGTOP_MAX_COST", "notanumber")

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() should succeed with invalid env override, got: %v", err)
	}
	if cfg.Limits.MaxCostPerRun != 5.00 {
		t.Errorf("expected default max cost 5.00 (invalid env ignored), got %f", cfg.Limits.MaxCostPerRun)
	}
}

func TestEnvOverrideModel(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGTOP_MODEL", "opus")

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Runtime.Claude.Model != "opus" {
		t.Errorf("expected model %q, got %q", "opus", cfg.Runtime.Claude.Model)
	}
}

func TestEnvOverrideMaxTokens(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGTOP_MAX_TOKENS", "1000000")

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Limits.MaxTokensPerRun != 1000000 {
		t.Errorf("expected max tokens 1000000, got %d", cfg.Limits.MaxTokensPerRun)
	}
}

func TestEnvOverrideMaxConcurrent(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGTOP_MAX_CONCURRENT", "10")

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Limits.MaxConcurrentRuns != 10 {
		t.Errorf("expected max concurrent 10, got %d", cfg.Limits.MaxConcurrentRuns)
	}
}

func TestEnvOverridePermissionMode(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("AGTOP_PERMISSION_MODE", "acceptAll")

	cfg, err := LoadFrom(tmp)
	if err != nil {
		t.Fatalf("LoadFrom() error: %v", err)
	}
	if cfg.Runtime.Claude.PermissionMode != "acceptAll" {
		t.Errorf("expected permission mode %q, got %q", "acceptAll", cfg.Runtime.Claude.PermissionMode)
	}
}
