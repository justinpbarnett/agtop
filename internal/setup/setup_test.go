package setup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/justinpbarnett/agtop/internal/config"
)

func TestRunClaudeOnlyCreatesClaudeSettings(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	DefaultConfig = []byte(`[project]
name = "my-project"
test_command = "npm test"

[project.dev_server]
command = "npm run dev"

[runtime]
default = "claude"
`)

	cfg := config.DefaultConfig()

	if err := Run(&cfg, Options{Runtime: "claude"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(".claude", "settings.json")); err != nil {
		t.Error("expected .claude/settings.json to exist")
	}

	if _, err := os.Stat("opencode.json"); err == nil {
		t.Error("expected opencode.json to NOT exist for claude runtime")
	}

	if _, err := os.Stat("agtop.toml"); err != nil {
		t.Error("expected agtop.toml to exist")
	}

	if _, err := os.Stat(filepath.Join(".agtop", "hooks", "safety-guard.sh")); err != nil {
		t.Error("expected safety-guard.sh to exist")
	}
}

func TestRunOpenCodeOnlyCreatesOpenCodeConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	DefaultConfig = []byte(`[project]
name = "my-project"
test_command = "npm test"

[project.dev_server]
command = "npm run dev"

[runtime]
default = "claude"
`)

	cfg := config.DefaultConfig()

	if err := Run(&cfg, Options{Runtime: "opencode"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	if _, err := os.Stat("opencode.json"); err != nil {
		t.Error("expected opencode.json to exist")
	}

	if _, err := os.Stat(filepath.Join(".claude", "settings.json")); err == nil {
		t.Error("expected .claude/settings.json to NOT exist for opencode runtime")
	}

	if _, err := os.Stat("agtop.toml"); err != nil {
		t.Error("expected agtop.toml to exist")
	}
}

func TestRunSetsRuntimeInConfig(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	DefaultConfig = []byte(`[runtime]
default = "claude"       # claude | opencode
`)

	cfg := config.DefaultConfig()

	if err := Run(&cfg, Options{Runtime: "opencode"}); err != nil {
		t.Fatalf("Run() error: %v", err)
	}

	data, err := os.ReadFile("agtop.toml")
	if err != nil {
		t.Fatal("expected agtop.toml")
	}
	content := string(data)
	if !contains(content, `default = "opencode"`) {
		t.Errorf("expected runtime default to be opencode in agtop.toml, got:\n%s", content)
	}
}

func TestMergeClaudeSettingsPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")
	existing := map[string]interface{}{
		"permissions": map[string]interface{}{
			"allow": []interface{}{"Write"},
		},
		"custom": "value",
	}
	data, _ := json.Marshal(existing)
	os.WriteFile(path, data, 0o644)

	agtop := map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": ".agtop/hooks/safety-guard.sh",
						},
					},
				},
			},
		},
	}

	result, err := MergeClaudeSettings(path, agtop)
	if err != nil {
		t.Fatalf("MergeClaudeSettings() error: %v", err)
	}

	if result["custom"] != "value" {
		t.Error("expected custom field to be preserved")
	}

	perms := result["permissions"].(map[string]interface{})
	allow := perms["allow"].([]interface{})
	if len(allow) != 1 || allow[0] != "Write" {
		t.Errorf("expected existing allow to be preserved, got %v", allow)
	}
}

func TestMergeOpenCodeConfigPreservesExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	existing := map[string]interface{}{
		"permission": map[string]interface{}{
			"bash": "deny",
		},
		"model": "gpt-4",
	}
	data, _ := json.Marshal(existing)
	os.WriteFile(path, data, 0o644)

	agtop := map[string]interface{}{
		"permission": map[string]interface{}{
			"bash": map[string]interface{}{"*": "allow"},
			"read": "allow",
		},
	}

	result, err := MergeOpenCodeConfig(path, agtop)
	if err != nil {
		t.Fatalf("MergeOpenCodeConfig() error: %v", err)
	}

	if result["model"] != "gpt-4" {
		t.Error("expected model field to be preserved")
	}

	perms := result["permission"].(map[string]interface{})
	if perms["bash"] != "deny" {
		t.Error("expected existing bash permission (string) to be preserved")
	}
	if perms["read"] != "allow" {
		t.Error("expected new read permission to be added")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
