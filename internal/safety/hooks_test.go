package safety

import (
	"strings"
	"testing"

	"github.com/justinpbarnett/agtop/internal/config"
)

func testConfig() config.SafetyConfig {
	return config.SafetyConfig{
		BlockedPatterns: []string{
			`rm\s+-[rf]+\s+/`,
			`git\s+push.*--force`,
			`DROP\s+TABLE`,
		},
	}
}

func TestNewHookEngine(t *testing.T) {
	engine, err := NewHookEngine(testConfig())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if engine.Matcher().PatternCount() != 3 {
		t.Errorf("expected 3 patterns, got %d", engine.Matcher().PatternCount())
	}
}

func TestCheckCommandBlocked(t *testing.T) {
	engine, _ := NewHookEngine(testConfig())

	blocked, reason := engine.CheckCommand("rm -rf /home")
	if !blocked {
		t.Error("expected command to be blocked")
	}
	if !strings.Contains(reason, "blocked by safety pattern") {
		t.Errorf("expected reason to contain 'blocked by safety pattern', got %q", reason)
	}
}

func TestCheckCommandAllowed(t *testing.T) {
	engine, _ := NewHookEngine(testConfig())

	blocked, reason := engine.CheckCommand("ls -la")
	if blocked {
		t.Errorf("expected command to be allowed, got reason %q", reason)
	}
}

func TestGenerateGuardScript(t *testing.T) {
	engine, _ := NewHookEngine(testConfig())
	script := engine.GenerateGuardScript()

	if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
		t.Error("script should start with shebang")
	}
	if !strings.Contains(script, "exit 2") {
		t.Error("script should contain exit 2 for blocking")
	}
	if !strings.Contains(script, "exit 0") {
		t.Error("script should contain exit 0 for allowing")
	}
	if !strings.Contains(script, `rm\s+-[rf]+\s+/`) {
		t.Error("script should contain the rm pattern")
	}
	if !strings.Contains(script, `git\s+push.*--force`) {
		t.Error("script should contain the git push pattern")
	}
	if !strings.Contains(script, "agtop") {
		t.Error("script should mention agtop")
	}
}

func TestGenerateGuardScriptEmpty(t *testing.T) {
	engine, _ := NewHookEngine(config.SafetyConfig{})
	script := engine.GenerateGuardScript()

	if !strings.HasPrefix(script, "#!/usr/bin/env bash") {
		t.Error("script should start with shebang even with no patterns")
	}
	if !strings.Contains(script, "exit 0") {
		t.Error("script should still have exit 0")
	}
}

func TestGenerateGuardScriptFiltersBashUnsafe(t *testing.T) {
	backtickPattern := "pattern with " + string(rune(96)) + "backtick" + string(rune(96))
	cfg := config.SafetyConfig{
		BlockedPatterns: []string{
			`rm\s+-[rf]+\s+/`,       // safe
			`pattern with "quotes"`, // unsafe: double quotes
			backtickPattern,         // unsafe: backtick
			`DROP\s+TABLE`,          // safe
			`pattern with ]]`,       // unsafe: closes conditional
			`pattern with $(cmd)`,   // unsafe: command substitution
			`:(){.*};`,              // unsafe: semicolon terminates [[ ]]
		},
	}
	engine, _ := NewHookEngine(cfg)
	script := engine.GenerateGuardScript()

	if !strings.Contains(script, `rm\s+-[rf]+\s+/`) {
		t.Error("script should contain the safe rm pattern")
	}
	if !strings.Contains(script, `DROP\s+TABLE`) {
		t.Error("script should contain the safe DROP TABLE pattern")
	}
	if strings.Contains(script, `"quotes"`) {
		t.Error("script should not contain pattern with double quotes")
	}
	if strings.Contains(script, "backtick") {
		t.Error("script should not contain pattern with backtick")
	}
	if strings.Contains(script, `pattern with ]]`) {
		t.Error("script should not contain pattern with ]]")
	}
	if strings.Contains(script, `$(cmd)`) {
		t.Error("script should not contain pattern with command substitution")
	}
	if strings.Contains(script, `:(){.*};`) {
		t.Error("script should not contain fork bomb pattern with semicolon")
	}
}

func TestGenerateSettings(t *testing.T) {
	engine, _ := NewHookEngine(testConfig())
	settings := engine.GenerateSettings()

	hooks, ok := settings["hooks"].(map[string]interface{})
	if !ok {
		t.Fatal("settings should have hooks key")
	}

	preToolUse, ok := hooks["PreToolUse"].([]interface{})
	if !ok {
		t.Fatal("hooks should have PreToolUse key")
	}

	if len(preToolUse) != 1 {
		t.Fatalf("expected 1 PreToolUse entry, got %d", len(preToolUse))
	}

	entry, ok := preToolUse[0].(map[string]interface{})
	if !ok {
		t.Fatal("entry should be a map")
	}

	if entry["matcher"] != "Bash" {
		t.Errorf("expected matcher 'Bash', got %v", entry["matcher"])
	}

	innerHooks, ok := entry["hooks"].([]interface{})
	if !ok || len(innerHooks) != 1 {
		t.Fatal("expected 1 inner hook")
	}

	hook, ok := innerHooks[0].(map[string]interface{})
	if !ok {
		t.Fatal("inner hook should be a map")
	}

	if hook["type"] != "command" {
		t.Errorf("expected type 'command', got %v", hook["type"])
	}
	if hook["command"] != ".agtop/hooks/safety-guard.sh" {
		t.Errorf("expected command '.agtop/hooks/safety-guard.sh', got %v", hook["command"])
	}
}
