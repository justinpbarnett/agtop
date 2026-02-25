package detect

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type Result struct {
	ProjectName string
	Runtime     string       // "claude" or "opencode" or ""
	TestCommand string
	DevServer   string
	Repos       []RepoResult // empty = single-repo
	Language    string       // "go", "node", "rust", "python", ""
}

type RepoResult struct {
	Name string
	Path string
}

func Detect(root string) (*Result, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve root: %w", err)
	}

	r := &Result{
		ProjectName: detectProjectName(absRoot),
		Runtime:     detectRuntime(),
		Language:    detectLanguage(absRoot),
		Repos:       detectRepos(absRoot),
	}
	r.TestCommand = detectTestCommand(absRoot)
	r.DevServer = detectDevServer(absRoot)
	return r, nil
}

func detectProjectName(root string) string {
	return filepath.Base(root)
}

func detectRuntime() string {
	if _, err := exec.LookPath("claude"); err == nil {
		return "claude"
	}
	if _, err := exec.LookPath("opencode"); err == nil {
		return "opencode"
	}
	return ""
}

func detectLanguage(root string) string {
	if fileExists(filepath.Join(root, "go.mod")) {
		return "go"
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return "rust"
	}
	if fileExists(filepath.Join(root, "package.json")) {
		return "node"
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) || fileExists(filepath.Join(root, "requirements.txt")) {
		return "python"
	}
	return ""
}

func detectTestCommand(root string) string {
	if fileExists(filepath.Join(root, "go.mod")) {
		return "go test ./..."
	}
	if fileExists(filepath.Join(root, "Cargo.toml")) {
		return "cargo test"
	}
	if cmd := detectNodeTestCommand(root); cmd != "" {
		return cmd
	}
	if hasMakefileTarget(root, "test") {
		return "make test"
	}
	if fileExists(filepath.Join(root, "pyproject.toml")) {
		content, err := os.ReadFile(filepath.Join(root, "pyproject.toml"))
		if err == nil && strings.Contains(string(content), "pytest") {
			return "pytest"
		}
		return "python -m unittest"
	}
	return ""
}

func detectNodeTestCommand(root string) string {
	pkgPath := filepath.Join(root, "package.json")
	if !fileExists(pkgPath) {
		return ""
	}
	scripts := readPackageJSONScripts(pkgPath)
	testScript, ok := scripts["test"]
	if !ok || strings.Contains(testScript, "echo \"Error") {
		return ""
	}
	runner := detectNodeRunner(root)
	return runner + " test"
}

func detectDevServer(root string) string {
	pkgPath := filepath.Join(root, "package.json")
	if fileExists(pkgPath) {
		scripts := readPackageJSONScripts(pkgPath)
		runner := detectNodeRunner(root)
		if _, ok := scripts["dev"]; ok {
			return runner + " run dev"
		}
		if _, ok := scripts["start"]; ok {
			if runner == "npm" {
				return "npm start"
			}
			return runner + " run start"
		}
	}
	if hasMakefileTarget(root, "dev") {
		return "make dev"
	}
	if hasMakefileTarget(root, "run") {
		return "make run"
	}
	return ""
}

func detectRepos(root string) []RepoResult {
	var repos []RepoResult
	skipDirs := map[string]bool{
		".git": true, ".agtop": true, "node_modules": true,
		"vendor": true, ".cache": true,
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if skipDirs[name] || strings.HasPrefix(name, ".") {
			continue
		}

		subPath := filepath.Join(root, name)

		if isGitRepo(subPath) {
			repos = append(repos, RepoResult{Name: name, Path: name})
			continue
		}

		subEntries, err := os.ReadDir(subPath)
		if err != nil {
			continue
		}
		for _, subEntry := range subEntries {
			if !subEntry.IsDir() {
				continue
			}
			subName := subEntry.Name()
			if skipDirs[subName] || strings.HasPrefix(subName, ".") {
				continue
			}
			deepPath := filepath.Join(subPath, subName)
			if isGitRepo(deepPath) {
				relPath := filepath.Join(name, subName)
				repos = append(repos, RepoResult{Name: subName, Path: relPath})
			}
		}
	}

	return repos
}

// --- AI-powered detection ---

type aiResponse struct {
	ProjectName string     `json:"project_name,omitempty"`
	TestCommand string     `json:"test_command,omitempty"`
	DevServer   string     `json:"dev_server_command,omitempty"`
	Repos       []aiRepo   `json:"repos,omitempty"`
}

type aiRepo struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func DetectWithAI(root string, static *Result) (*Result, error) {
	binary, runtimeName := findRuntimeBinary()
	if binary == "" {
		return static, nil
	}

	prompt := buildAIPrompt(static)
	args := buildAIArgs(runtimeName, prompt)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = root

	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: AI analysis failed: %v\n", err)
		return static, nil
	}

	parsed, err := parseAIResponse(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "  warning: could not parse AI response: %v\n", err)
		return static, nil
	}

	return mergeAIResult(static, parsed), nil
}

func findRuntimeBinary() (string, string) {
	if path, err := exec.LookPath("claude"); err == nil {
		return path, "claude"
	}
	if path, err := exec.LookPath("opencode"); err == nil {
		return path, "opencode"
	}
	return "", ""
}

func buildAIPrompt(static *Result) string {
	var sb strings.Builder
	sb.WriteString("Analyze this project and output ONLY a JSON object (no markdown, no explanation) with the following fields. Only include fields you are confident about — omit uncertain fields.\n\n")
	sb.WriteString("Fields:\n")
	sb.WriteString("- project_name: string — a good short name for this project\n")
	sb.WriteString("- test_command: string — the command to run the full test suite\n")
	sb.WriteString("- dev_server_command: string — the command to start the development server\n")
	sb.WriteString("- repos: array of {name, path} — sub-repositories if this is a poly-repo/multi-repo project. Only include if there are multiple independent git repos in subdirectories.\n\n")
	sb.WriteString("Current static detection found:\n")
	if static.ProjectName != "" {
		sb.WriteString(fmt.Sprintf("- project_name: %s\n", static.ProjectName))
	}
	if static.TestCommand != "" {
		sb.WriteString(fmt.Sprintf("- test_command: %s\n", static.TestCommand))
	}
	if static.DevServer != "" {
		sb.WriteString(fmt.Sprintf("- dev_server_command: %s\n", static.DevServer))
	}
	if static.Language != "" {
		sb.WriteString(fmt.Sprintf("- language: %s\n", static.Language))
	}
	if len(static.Repos) > 0 {
		sb.WriteString("- repos:")
		for _, r := range static.Repos {
			sb.WriteString(fmt.Sprintf(" %s (%s)", r.Name, r.Path))
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nRead the project's README, configuration files, and source structure to verify or improve these values. Output ONLY the JSON object.")
	return sb.String()
}

func buildAIArgs(runtimeName, prompt string) []string {
	switch runtimeName {
	case "claude":
		return []string{
			"-p", prompt,
			"--output-format", "json",
			"--max-turns", "5",
			"--allowedTools", "Read,Glob,Grep",
			"--permission-mode", "manual",
		}
	case "opencode":
		return []string{
			"run", prompt,
			"--format", "json",
			"--agent", "build",
		}
	}
	return nil
}

func parseAIResponse(data []byte) (*aiResponse, error) {
	jsonStr := extractJSON(string(data))
	if jsonStr == "" {
		return nil, fmt.Errorf("no JSON object found in response")
	}

	var resp aiResponse
	if err := json.Unmarshal([]byte(jsonStr), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}
	return &resp, nil
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func mergeAIResult(static *Result, ai *aiResponse) *Result {
	merged := *static
	if ai.ProjectName != "" {
		merged.ProjectName = ai.ProjectName
	}
	if ai.TestCommand != "" {
		merged.TestCommand = ai.TestCommand
	}
	if ai.DevServer != "" {
		merged.DevServer = ai.DevServer
	}
	if len(ai.Repos) > 0 {
		merged.Repos = make([]RepoResult, len(ai.Repos))
		for i, r := range ai.Repos {
			merged.Repos[i] = RepoResult{Name: r.Name, Path: r.Path}
		}
	}
	return &merged
}

// --- helpers ---

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func readPackageJSONScripts(path string) map[string]string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	return pkg.Scripts
}

func detectNodeRunner(root string) string {
	if fileExists(filepath.Join(root, "bun.lockb")) || fileExists(filepath.Join(root, "bun.lock")) {
		return "bun"
	}
	if fileExists(filepath.Join(root, "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if fileExists(filepath.Join(root, "yarn.lock")) {
		return "yarn"
	}
	return "npm"
}

var makeTargetRe = regexp.MustCompile(`(?m)^([a-zA-Z_][a-zA-Z0-9_-]*)\s*:`)

func hasMakefileTarget(root, target string) bool {
	data, err := os.ReadFile(filepath.Join(root, "Makefile"))
	if err != nil {
		return false
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		matches := makeTargetRe.FindStringSubmatch(line)
		if len(matches) > 1 && matches[1] == target {
			return true
		}
	}
	return false
}
