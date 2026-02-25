package detect

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectProjectName(t *testing.T) {
	dir := t.TempDir()
	named := filepath.Join(dir, "my-cool-app")
	if err := os.Mkdir(named, 0o755); err != nil {
		t.Fatal(err)
	}

	r, err := Detect(named)
	if err != nil {
		t.Fatal(err)
	}
	if r.ProjectName != "my-cool-app" {
		t.Errorf("got %q, want %q", r.ProjectName, "my-cool-app")
	}
}

func TestDetectLanguage_Go(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/test\n\ngo 1.21\n")

	got := detectLanguage(dir)
	if got != "go" {
		t.Errorf("got %q, want %q", got, "go")
	}
}

func TestDetectLanguage_Node(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"name":"test"}`)

	got := detectLanguage(dir)
	if got != "node" {
		t.Errorf("got %q, want %q", got, "node")
	}
}

func TestDetectLanguage_Rust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), `[package]\nname = "test"`)

	got := detectLanguage(dir)
	if got != "rust" {
		t.Errorf("got %q, want %q", got, "rust")
	}
}

func TestDetectLanguage_Python(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[project]\nname = "test"`)

	got := detectLanguage(dir)
	if got != "python" {
		t.Errorf("got %q, want %q", got, "python")
	}
}

func TestDetectLanguage_None(t *testing.T) {
	dir := t.TempDir()

	got := detectLanguage(dir)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestDetectTestCommand_Go(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "go.mod"), "module example.com/test\n\ngo 1.21\n")

	got := detectTestCommand(dir)
	if got != "go test ./..." {
		t.Errorf("got %q, want %q", got, "go test ./...")
	}
}

func TestDetectTestCommand_Rust(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Cargo.toml"), `[package]\nname = "test"`)

	got := detectTestCommand(dir)
	if got != "cargo test" {
		t.Errorf("got %q, want %q", got, "cargo test")
	}
}

func TestDetectTestCommand_Node(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"test":"jest"}}`)

	got := detectTestCommand(dir)
	if got != "npm test" {
		t.Errorf("got %q, want %q", got, "npm test")
	}
}

func TestDetectTestCommand_NodeYarn(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"test":"jest"}}`)
	writeFile(t, filepath.Join(dir, "yarn.lock"), "")

	got := detectTestCommand(dir)
	if got != "yarn test" {
		t.Errorf("got %q, want %q", got, "yarn test")
	}
}

func TestDetectTestCommand_NodeDefaultScript(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"test":"echo \"Error: no test specified\" && exit 1"}}`)

	got := detectTestCommand(dir)
	if got != "" {
		t.Errorf("default npm test script should be ignored, got %q", got)
	}
}

func TestDetectTestCommand_Makefile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Makefile"), "build:\n\tgo build\n\ntest:\n\tgo test ./...\n")

	got := detectTestCommand(dir)
	if got != "make test" {
		t.Errorf("got %q, want %q", got, "make test")
	}
}

func TestDetectTestCommand_PythonPytest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[tool.pytest]\ntestpaths = ["tests"]`)

	got := detectTestCommand(dir)
	if got != "pytest" {
		t.Errorf("got %q, want %q", got, "pytest")
	}
}

func TestDetectTestCommand_PythonUnittest(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "pyproject.toml"), `[project]\nname = "test"`)

	got := detectTestCommand(dir)
	if got != "python -m unittest" {
		t.Errorf("got %q, want %q", got, "python -m unittest")
	}
}

func TestDetectTestCommand_None(t *testing.T) {
	dir := t.TempDir()

	got := detectTestCommand(dir)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestDetectDevServer_NodeDev(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"dev":"next dev","start":"next start"}}`)

	got := detectDevServer(dir)
	if got != "npm run dev" {
		t.Errorf("got %q, want %q", got, "npm run dev")
	}
}

func TestDetectDevServer_NodeStart(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"start":"node server.js"}}`)

	got := detectDevServer(dir)
	if got != "npm start" {
		t.Errorf("got %q, want %q", got, "npm start")
	}
}

func TestDetectDevServer_Pnpm(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "package.json"), `{"scripts":{"dev":"vite"}}`)
	writeFile(t, filepath.Join(dir, "pnpm-lock.yaml"), "")

	got := detectDevServer(dir)
	if got != "pnpm run dev" {
		t.Errorf("got %q, want %q", got, "pnpm run dev")
	}
}

func TestDetectDevServer_Makefile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Makefile"), "dev:\n\tgo run ./cmd/...\n")

	got := detectDevServer(dir)
	if got != "make dev" {
		t.Errorf("got %q, want %q", got, "make dev")
	}
}

func TestDetectDevServer_MakefileRun(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Makefile"), "run:\n\tgo run ./cmd/...\n")

	got := detectDevServer(dir)
	if got != "make run" {
		t.Errorf("got %q, want %q", got, "make run")
	}
}

func TestDetectDevServer_None(t *testing.T) {
	dir := t.TempDir()

	got := detectDevServer(dir)
	if got != "" {
		t.Errorf("got %q, want empty", got)
	}
}

func TestDetectRepos_SingleRepo(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)

	got := detectRepos(dir)
	if len(got) != 0 {
		t.Errorf("single repo should return empty repos, got %d", len(got))
	}
}

func TestDetectRepos_PolyRepo(t *testing.T) {
	dir := t.TempDir()
	os.Mkdir(filepath.Join(dir, ".git"), 0o755)

	for _, sub := range []string{"frontend", "backend"} {
		subDir := filepath.Join(dir, sub)
		os.Mkdir(subDir, 0o755)
		os.Mkdir(filepath.Join(subDir, ".git"), 0o755)
	}

	got := detectRepos(dir)
	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(got))
	}

	names := map[string]bool{}
	for _, r := range got {
		names[r.Name] = true
	}
	if !names["frontend"] || !names["backend"] {
		t.Errorf("expected frontend and backend, got %v", got)
	}
}

func TestDetectRepos_NestedSubRepo(t *testing.T) {
	dir := t.TempDir()

	appDir := filepath.Join(dir, "app")
	os.Mkdir(appDir, 0o755)

	for _, sub := range []string{"client", "server"} {
		subDir := filepath.Join(appDir, sub)
		os.Mkdir(subDir, 0o755)
		os.Mkdir(filepath.Join(subDir, ".git"), 0o755)
	}

	got := detectRepos(dir)
	if len(got) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(got))
	}

	paths := map[string]bool{}
	for _, r := range got {
		paths[r.Path] = true
	}
	if !paths[filepath.Join("app", "client")] || !paths[filepath.Join("app", "server")] {
		t.Errorf("expected app/client and app/server paths, got %v", got)
	}
}

func TestDetectRepos_SkipsHiddenDirs(t *testing.T) {
	dir := t.TempDir()

	hidden := filepath.Join(dir, ".hidden")
	os.Mkdir(hidden, 0o755)
	os.Mkdir(filepath.Join(hidden, ".git"), 0o755)

	got := detectRepos(dir)
	if len(got) != 0 {
		t.Errorf("hidden dirs should be skipped, got %d repos", len(got))
	}
}

func TestDetectRepos_SkipsNodeModules(t *testing.T) {
	dir := t.TempDir()

	nm := filepath.Join(dir, "node_modules")
	os.Mkdir(nm, 0o755)
	sub := filepath.Join(nm, "some-pkg")
	os.MkdirAll(sub, 0o755)
	os.Mkdir(filepath.Join(sub, ".git"), 0o755)

	got := detectRepos(dir)
	if len(got) != 0 {
		t.Errorf("node_modules should be skipped, got %d repos", len(got))
	}
}

func TestDetectWithAI_Fallback(t *testing.T) {
	static := &Result{
		ProjectName: "test-project",
		TestCommand: "go test ./...",
		Language:    "go",
	}

	// DetectWithAI should return static result unchanged when no runtime is available
	// (this test depends on the CI environment not having claude/opencode in PATH,
	// which is the expected case)
	result, err := DetectWithAI(t.TempDir(), static)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ProjectName != "test-project" {
		t.Errorf("got %q, want %q", result.ProjectName, "test-project")
	}
	if result.TestCommand != "go test ./..." {
		t.Errorf("got %q, want %q", result.TestCommand, "go test ./...")
	}
}

func TestParseAIResponse_Valid(t *testing.T) {
	resp := aiResponse{
		ProjectName: "detected-name",
		TestCommand: "make test",
		DevServer:   "make dev",
		Repos: []aiRepo{
			{Name: "client", Path: "app/client"},
		},
	}
	data, _ := json.Marshal(resp)

	parsed, err := parseAIResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ProjectName != "detected-name" {
		t.Errorf("got %q, want %q", parsed.ProjectName, "detected-name")
	}
	if parsed.TestCommand != "make test" {
		t.Errorf("got %q, want %q", parsed.TestCommand, "make test")
	}
	if parsed.DevServer != "make dev" {
		t.Errorf("got %q, want %q", parsed.DevServer, "make dev")
	}
	if len(parsed.Repos) != 1 || parsed.Repos[0].Name != "client" {
		t.Errorf("unexpected repos: %v", parsed.Repos)
	}
}

func TestParseAIResponse_WithSurroundingText(t *testing.T) {
	data := []byte(`Here is the analysis:\n{"project_name":"my-app","test_command":"npm test"}\nDone.`)

	parsed, err := parseAIResponse(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ProjectName != "my-app" {
		t.Errorf("got %q, want %q", parsed.ProjectName, "my-app")
	}
}

func TestParseAIResponse_Empty(t *testing.T) {
	_, err := parseAIResponse([]byte("no json here"))
	if err == nil {
		t.Error("expected error for response with no JSON")
	}
}

func TestParseAIResponse_Malformed(t *testing.T) {
	_, err := parseAIResponse([]byte(`{"project_name": broken}`))
	if err == nil {
		t.Error("expected error for malformed JSON")
	}
}

func TestParseAIResponse_Envelope(t *testing.T) {
	// Claude --output-format json wraps the response in an envelope
	inner := `{"test_command": "go test ./...", "dev_server_command": "go run ."}`
	envelope := map[string]interface{}{
		"type":    "result",
		"subtype": "success",
		"result":  "Here is the analysis:\n" + inner,
	}
	data, _ := json.Marshal(envelope)

	resp, err := parseAIResponse(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TestCommand != "go test ./..." {
		t.Errorf("test_command = %q, want %q", resp.TestCommand, "go test ./...")
	}
	if resp.DevServer != "go run ." {
		t.Errorf("dev_server_command = %q, want %q", resp.DevServer, "go run .")
	}
}

func TestMergeAIResult(t *testing.T) {
	static := &Result{
		ProjectName: "static-name",
		TestCommand: "static-test",
		Language:    "go",
	}
	ai := &aiResponse{
		ProjectName: "ai-name",
		DevServer:   "ai-dev",
	}

	merged := mergeAIResult(static, ai)

	if merged.ProjectName != "ai-name" {
		t.Errorf("AI should override: got %q", merged.ProjectName)
	}
	if merged.TestCommand != "static-test" {
		t.Errorf("static should be preserved: got %q", merged.TestCommand)
	}
	if merged.DevServer != "ai-dev" {
		t.Errorf("AI should fill in: got %q", merged.DevServer)
	}
	if merged.Language != "go" {
		t.Errorf("language should be preserved: got %q", merged.Language)
	}
}

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"clean", `{"a":"b"}`, `{"a":"b"}`},
		{"surrounded", `text {"a":"b"} more`, `{"a":"b"}`},
		{"nested", `{"a":{"b":"c"}}`, `{"a":{"b":"c"}}`},
		{"no json", `no json here`, ""},
		{"unclosed", `{"a":"b"`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNodeRunner(t *testing.T) {
	tests := []struct {
		name     string
		lockfile string
		want     string
	}{
		{"npm", "", "npm"},
		{"yarn", "yarn.lock", "yarn"},
		{"pnpm", "pnpm-lock.yaml", "pnpm"},
		{"bun lockb", "bun.lockb", "bun"},
		{"bun lock", "bun.lock", "bun"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if tt.lockfile != "" {
				writeFile(t, filepath.Join(dir, tt.lockfile), "")
			}
			got := detectNodeRunner(dir)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasMakefileTarget(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Makefile"), "build:\n\tgo build\n\ntest:\n\tgo test\n\nclean:\n\trm -rf bin/\n")

	if !hasMakefileTarget(dir, "test") {
		t.Error("expected to find test target")
	}
	if !hasMakefileTarget(dir, "build") {
		t.Error("expected to find build target")
	}
	if hasMakefileTarget(dir, "deploy") {
		t.Error("should not find deploy target")
	}
}

func TestDetectTestCommand_Justfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "justfile"), "build:\n    go build\n\ntest:\n    go test ./...\n")

	got := detectTestCommand(dir)
	if got != "just test" {
		t.Errorf("got %q, want %q", got, "just test")
	}
}

func TestDetectDevServer_Justfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "justfile"), "start:\n    docker compose up\n")

	got := detectDevServer(dir)
	if got != "just start" {
		t.Errorf("got %q, want %q", got, "just start")
	}
}

func TestDetectDevServer_JustfileDev(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "justfile"), "dev:\n    npm run dev\n")

	got := detectDevServer(dir)
	if got != "just dev" {
		t.Errorf("got %q, want %q", got, "just dev")
	}
}

func TestHasJustfileRecipe(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "justfile"), "set dotenv-load := false\n\n# Start dev\nstart id=\"\":\n    echo start\n\ntest:\n    echo test\n\nbuild-image tag:\n    docker build\n")

	if !hasJustfileRecipe(dir, "start") {
		t.Error("expected to find start recipe")
	}
	if !hasJustfileRecipe(dir, "test") {
		t.Error("expected to find test recipe")
	}
	if !hasJustfileRecipe(dir, "build-image") {
		t.Error("expected to find build-image recipe")
	}
	if hasJustfileRecipe(dir, "deploy") {
		t.Error("should not find deploy recipe")
	}
}

func TestHasJustfileRecipe_Justfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "Justfile"), "test:\n    echo test\n")

	if !hasJustfileRecipe(dir, "test") {
		t.Error("expected to find test recipe in Justfile (capitalized)")
	}
}

func TestNeedsAI(t *testing.T) {
	tests := []struct {
		name string
		r    Result
		want bool
	}{
		{"all found", Result{TestCommand: "go test", DevServer: "make dev"}, false},
		{"missing test", Result{DevServer: "make dev"}, true},
		{"missing dev server", Result{TestCommand: "go test"}, true},
		{"both missing", Result{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.r.NeedsAI(); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
