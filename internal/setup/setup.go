package setup

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

type Options struct {
	Runtime string // "claude" or "opencode"
	UseAI   bool
	Root    string // project root directory (empty = cwd)
}

// DefaultConfig is the embedded example config template.
// Must be set by the caller (e.g. from the embedded agtop.example.toml).
var DefaultConfig []byte

func Run(cfg *config.Config, opts Options) error {
	if opts.Root != "" {
		if err := os.Chdir(opts.Root); err != nil {
			return fmt.Errorf("chdir to root: %w", err)
		}
	}

	engine, err := safety.NewHookEngine(cfg.Safety)
	if err != nil {
		fmt.Fprintf(os.Stderr, "warning: %v\n", err)
	}

	hooksDir := filepath.Join(".agtop", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return fmt.Errorf("create hooks dir: %w", err)
	}
	fmt.Printf("  created %s/\n", hooksDir)

	guardPath := filepath.Join(hooksDir, "safety-guard.sh")
	script := engine.GenerateGuardScript()
	if err := os.WriteFile(guardPath, []byte(script), 0o755); err != nil {
		return fmt.Errorf("write guard script: %w", err)
	}
	fmt.Printf("  created %s (%d patterns)\n", guardPath, engine.Matcher().PatternCount())

	switch opts.Runtime {
	case "claude":
		if err := setupClaude(engine); err != nil {
			return err
		}
	case "opencode":
		if err := setupOpenCode(engine); err != nil {
			return err
		}
	}

	if _, err := os.Stat("agtop.toml"); os.IsNotExist(err) {
		result, detectErr := detect.Detect(".")
		if detectErr != nil {
			fmt.Fprintf(os.Stderr, "  warning: detection failed: %v\n", detectErr)
		}

		if result != nil {
			result.Runtime = opts.Runtime
		}

		if result != nil && opts.UseAI && result.NeedsAI() {
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

		template := DefaultConfig
		if local, readErr := os.ReadFile("agtop.example.toml"); readErr == nil {
			template = local
		}

		content := template
		if result != nil {
			content = RenderConfig(template, result)
		}
		if writeErr := os.WriteFile("agtop.toml", content, 0o644); writeErr == nil {
			fmt.Println("  created agtop.toml")
		}
	}

	fmt.Println("\nagtop init complete.")
	return nil
}

func setupClaude(engine *safety.HookEngine) error {
	claudeDir := ".claude"
	if err := os.MkdirAll(claudeDir, 0o755); err != nil {
		return fmt.Errorf("create .claude dir: %w", err)
	}

	settingsPath := filepath.Join(claudeDir, "settings.json")
	settings, err := MergeClaudeSettings(settingsPath, engine.GenerateSettings())
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
	return nil
}

func setupOpenCode(engine *safety.HookEngine) error {
	opencodeConfigPath := "opencode.json"
	ocSettings, err := MergeOpenCodeConfig(opencodeConfigPath, engine.GenerateOpenCodeSettings())
	if err != nil {
		return fmt.Errorf("merge opencode config: %w", err)
	}

	ocData, err := json.MarshalIndent(ocSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode config: %w", err)
	}
	ocData = append(ocData, '\n')
	if err := os.WriteFile(opencodeConfigPath, ocData, 0o644); err != nil {
		return fmt.Errorf("write opencode config: %w", err)
	}
	fmt.Printf("  updated %s (permissions)\n", opencodeConfigPath)
	return nil
}

// MergeClaudeSettings reads existing .claude/settings.json and merges the agtop
// hook and permission config without overwriting user-defined entries.
func MergeClaudeSettings(path string, agtopSettings map[string]interface{}) (map[string]interface{}, error) {
	existing := make(map[string]interface{})

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

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

// MergeOpenCodeConfig reads an existing opencode.json and merges the agtop
// permission config without overwriting user-defined entries.
func MergeOpenCodeConfig(path string, agtopSettings map[string]interface{}) (map[string]interface{}, error) {
	existing := make(map[string]interface{})

	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}

	agtopPerms, _ := agtopSettings["permission"].(map[string]interface{})
	if agtopPerms == nil {
		return existing, nil
	}

	perms, _ := existing["permission"].(map[string]interface{})
	if perms == nil {
		perms = make(map[string]interface{})
	}

	for tool, value := range agtopPerms {
		existingVal, exists := perms[tool]
		if !exists {
			perms[tool] = value
			continue
		}

		newMap, newIsMap := value.(map[string]interface{})
		existMap, existIsMap := existingVal.(map[string]interface{})
		if newIsMap && existIsMap {
			for k, v := range newMap {
				if _, has := existMap[k]; !has {
					existMap[k] = v
				}
			}
			perms[tool] = existMap
		}
	}

	existing["permission"] = perms
	return existing, nil
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

func RenderConfig(template []byte, r *detect.Result) []byte {
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
			if idx := strings.Index(s, "# [integrations"); idx >= 0 {
				s = s[:idx] + replacement + "\n" + s[idx:]
			} else {
				s += "\n" + replacement
			}
		}
	}

	return []byte(s)
}
