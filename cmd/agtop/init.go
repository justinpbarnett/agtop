package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/justinpbarnett/agtop/internal/config"
	"github.com/justinpbarnett/agtop/internal/detect"
	"github.com/justinpbarnett/agtop/internal/safety"
)

func runInit(cfg *config.Config, useAI bool) error {
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

	// 4. Create agtop.toml if it doesn't exist (with auto-detection)
	if _, err := os.Stat("agtop.toml"); os.IsNotExist(err) {
		result, detectErr := detect.Detect(".")
		if detectErr != nil {
			fmt.Fprintf(os.Stderr, "  warning: detection failed: %v\n", detectErr)
		}

		if result != nil && useAI && result.NeedsAI() {
			fmt.Println("  static detection incomplete, running AI analysis...")
			aiResult, _ := detect.DetectWithAI(".", result)
			if aiResult != result {
				fmt.Println("  AI analysis complete")
			}
			result = aiResult
		}

		if result != nil {
			printDetected(result)
		}

		template := defaultConfig
		if local, readErr := os.ReadFile("agtop.example.toml"); readErr == nil {
			template = local
		}

		content := template
		if result != nil {
			content = renderConfig(template, result)
		}
		if writeErr := os.WriteFile("agtop.toml", content, 0o644); writeErr == nil {
			fmt.Println("  created agtop.toml")
		}
	}

	fmt.Println("\nagtop init complete.")
	return nil
}

// mergeSettings reads existing settings and merges the agtop hook and
// permission config without overwriting user-defined entries.
func mergeSettings(path string, agtopSettings map[string]interface{}) (map[string]interface{}, error) {
	existing := make(map[string]interface{})

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	// Merge permissions — append agtop's entries to any existing allow/deny lists.
	if agtopPerms, ok := agtopSettings["permissions"].(map[string]interface{}); ok {
		perms, _ := existing["permissions"].(map[string]interface{})
		if perms == nil {
			perms = make(map[string]interface{})
		}
		if newAllow, ok := agtopPerms["allow"].([]interface{}); ok {
			cur, _ := perms["allow"].([]interface{})
			perms["allow"] = appendUnique(cur, newAllow)
		}
		if newDeny, ok := agtopPerms["deny"].([]interface{}); ok {
			cur, _ := perms["deny"].([]interface{})
			perms["deny"] = appendUnique(cur, newDeny)
		}
		existing["permissions"] = perms
	}

	// Merge hooks — for each hook event in agtopSettings, append entries that
	// aren't already present; all other event types in the existing file are
	// left untouched.
	agtopHooksRaw, ok := agtopSettings["hooks"].(map[string]interface{})
	if !ok {
		return existing, nil
	}

	hooks, _ := existing["hooks"].(map[string]interface{})
	if hooks == nil {
		hooks = make(map[string]interface{})
	}

	for event, rawEntries := range agtopHooksRaw {
		newEntries, ok := rawEntries.([]interface{})
		if !ok {
			continue
		}

		current, _ := hooks[event].([]interface{})

		for _, newEntry := range newEntries {
			if !hookEntryExists(current, newEntry) {
				current = append(current, newEntry)
			}
		}

		hooks[event] = current
	}

	existing["hooks"] = hooks

	return existing, nil
}

// hookEntryExists reports whether candidate already appears in the list,
// matched by comparing every inner hook command path.
func hookEntryExists(list []interface{}, candidate interface{}) bool {
	cMap, ok := candidate.(map[string]interface{})
	if !ok {
		return false
	}
	cInner, _ := cMap["hooks"].([]interface{})
	cCmds := hookCommands(cInner)
	if len(cCmds) == 0 {
		return false
	}

	for _, item := range list {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		inner, _ := m["hooks"].([]interface{})
		cmds := hookCommands(inner)
		for _, c := range cmds {
			for _, cc := range cCmds {
				if c == cc {
					return true
				}
			}
		}
	}
	return false
}

// hookCommands collects the "command" values from a hooks array.
func hookCommands(hooks []interface{}) []string {
	var out []string
	for _, h := range hooks {
		hm, ok := h.(map[string]interface{})
		if !ok {
			continue
		}
		if cmd, ok := hm["command"].(string); ok && cmd != "" {
			out = append(out, cmd)
		}
	}
	return out
}

// appendUnique appends items from add to base, skipping duplicates.
func appendUnique(base, add []interface{}) []interface{} {
	seen := make(map[string]bool, len(base))
	for _, v := range base {
		if s, ok := v.(string); ok {
			seen[s] = true
		}
	}
	result := append([]interface{}{}, base...)
	for _, v := range add {
		if s, ok := v.(string); ok && !seen[s] {
			result = append(result, v)
			seen[s] = true
		}
	}
	return result
}

func printDetected(r *detect.Result) {
	if r.ProjectName != "" {
		fmt.Printf("  detected project name: %s\n", r.ProjectName)
	}
	if r.Runtime != "" {
		fmt.Printf("  detected runtime: %s\n", r.Runtime)
	}
	if r.Language != "" {
		fmt.Printf("  detected language: %s\n", r.Language)
	}
	if r.TestCommand != "" {
		fmt.Printf("  detected test command: %s\n", r.TestCommand)
	}
	if r.DevServer != "" {
		fmt.Printf("  detected dev server: %s\n", r.DevServer)
	}
	if len(r.Repos) > 0 {
		fmt.Printf("  detected %d sub-repos:\n", len(r.Repos))
		for _, repo := range r.Repos {
			fmt.Printf("    %s (%s)\n", repo.Name, repo.Path)
		}
	}
}

var (
	reProjectName = regexp.MustCompile(`(?m)^name = "my-project"`)
	reTestCommand = regexp.MustCompile(`(?m)^test_command = "npm test"`)
	reDevServer   = regexp.MustCompile(`(?m)^command = "npm run dev"`)
	reRuntime     = regexp.MustCompile(`(?m)^default = "claude"(\s+#.*)?`)
)

func renderConfig(template []byte, r *detect.Result) []byte {
	s := string(template)

	if r.ProjectName != "" {
		s = reProjectName.ReplaceAllString(s, fmt.Sprintf(`name = "%s"`, r.ProjectName))
	}
	if r.TestCommand != "" {
		s = reTestCommand.ReplaceAllString(s, fmt.Sprintf(`test_command = "%s"`, r.TestCommand))
	}
	if r.DevServer != "" {
		s = reDevServer.ReplaceAllString(s, fmt.Sprintf(`command = "%s"`, r.DevServer))
	}
	if r.Runtime != "" {
		s = reRuntime.ReplaceAllString(s, fmt.Sprintf(`default = "%s"$1`, r.Runtime))
	}

	if len(r.Repos) > 0 {
		var repoLines strings.Builder
		repoLines.WriteString("\n")
		for _, repo := range r.Repos {
			repoLines.WriteString(fmt.Sprintf("[[repos]]\nname = \"%s\"\npath = \"%s\"\n\n", repo.Name, repo.Path))
		}

		commentedBlock := "# ── Multi-Repo (Poly-Repo) ────────────────────────────────────\n" +
			"# For projects with multiple independent git repos in subdirectories.\n" +
			"# When configured, agtop creates a worktree per sub-repo instead of\n" +
			"# one worktree for the root. PRs are created per sub-repo.\n" +
			"#\n" +
			"# [[repos]]\n" +
			"# name = \"client\"\n" +
			"# path = \"app/client\"\n" +
			"#\n" +
			"# [[repos]]\n" +
			"# name = \"server\"\n" +
			"# path = \"app/server\""

		replacement := "# ── Multi-Repo (Poly-Repo) ────────────────────────────────────" +
			repoLines.String()

		if strings.Contains(s, commentedBlock) {
			s = strings.Replace(s, commentedBlock, replacement, 1)
		} else {
			// Template doesn't have the commented block — append before integrations or at end
			if idx := strings.Index(s, "# [integrations"); idx >= 0 {
				s = s[:idx] + replacement + "\n" + s[idx:]
			} else {
				s += "\n" + replacement
			}
		}
	}

	return []byte(s)
}
