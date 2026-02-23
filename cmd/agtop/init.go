package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/safety"
)

func runInit(cfg *config.Config) error {
	engine, err := safety.NewHookEngine(cfg.Safety)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	// 1. Create .agtop/hooks/ directory
	hooksDir := filepath.Join(".agtop", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	fmt.Printf("  created %s/\n", hooksDir)

	// 2. Write safety-guard.sh
	guardPath := filepath.Join(hooksDir, "safety-guard.sh")
	script := engine.GenerateGuardScript()
	if err := os.WriteFile(guardPath, []byte(script), 0o755); err != nil {
		return fmt.Errorf("write guard script: %w", err)
	}
	fmt.Printf("  created %s (%d patterns)\n", guardPath, engine.Matcher().PatternCount())

	// 3. Merge .claude/settings.json
	claudeDir := ".claude"
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings, err := mergeSettings(settingsPath, engine.GenerateSettings())
	if err != nil {
		return fmt.Errorf("merge settings: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal settings: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(settingsPath, data, 0o644); err != nil {
		return fmt.Errorf("write settings: %w", err)
	}
	fmt.Printf("  updated %s (PreToolUse hook)\n", settingsPath)

	// 4. Copy example config if agtop.toml doesn't exist
	if _, err := os.Stat("agtop.toml"); os.IsNotExist(err) {
		exampleData, readErr := os.ReadFile("agtop.example.toml")
		if readErr == nil {
			if writeErr := os.WriteFile("agtop.toml", exampleData, 0o644); writeErr == nil {
				fmt.Println("  created agtop.toml (from example)")
			}
		}
	}

	fmt.Println("\nagtop init complete.")
	return nil
}

// mergeSettings reads existing settings and merges the agtop hook config
// without overwriting user-defined hooks.
func mergeSettings(path string, agtopSettings map[string]interface{}) (map[string]interface{}, error) {
	existing := make(map[string]interface{})

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	// Get or create the hooks map
	hooks, _ := existing["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	// Get the agtop hook entry we want to add
	agtopHooks := agtopSettings["hooks"].(map[string]interface{})
	agtopPreToolUse := agtopHooks["PreToolUse"].([]interface{})
	agtopEntry := agtopPreToolUse[0].(map[string]interface{})

	// Get or create the PreToolUse list
	var preToolUse []interface{}
	if raw, ok := hooks["PreToolUse"]; ok {
		if list, ok := raw.([]interface{}); ok {
			preToolUse = list
		}
	}

	// Check if the agtop hook already exists (by command path)
	found := false
	for _, entry := range preToolUse {
		m, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		innerHooks, ok := m["hooks"].([]interface{})
		if !ok {
			continue
		}
		for _, h := range innerHooks {
			hm, ok := h.(map[string]interface{})
			if !ok {
				continue
			}
			if cmd, ok := hm["command"].(string); ok && cmd == ".agtop/hooks/safety-guard.sh" {
				found = true
				break
			}
		}
	}

	if !found {
		preToolUse = append(preToolUse, agtopEntry)
	}

	hooks["PreToolUse"] = preToolUse
	existing["hooks"] = hooks

	return existing, nil
}
