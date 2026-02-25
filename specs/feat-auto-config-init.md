# Feature: Auto-Configure Project on Init

## Metadata

type: `feat`
task_id: `auto-config-init`
prompt: `Add an automated pass to the agtop init command that auto-configures the project configuration based on the project. Sub-repos would be auto-detected, test commands, run commands, runtime, etc.`

## Feature Description

`agtop init` currently creates a static `agtop.toml` from a template (`agtop.example.toml` embedded in the binary). Users must then manually edit every field — project name, test command, dev server command, runtime, sub-repos, etc. This is tedious and error-prone.

This feature adds a two-phase auto-detection system to `agtop init`:

1. **Static detection** — Fast, deterministic file-based inspection that detects project name, available runtimes, test commands, dev server commands, sub-repo layout, and project language/framework.
2. **AI-powered detection** (default, `--ai` flag) — After static detection, shells out to the detected runtime (e.g., `claude -p`) to analyze the project more deeply. The AI can inspect source code, README files, and project structure to refine or fill in values that static detection missed — for example, detecting a custom test runner, identifying the correct dev server port, or understanding a non-standard project layout.

The `--ai` flag is **on by default**. Users can disable it with `--no-ai` for fast, offline, deterministic init. The generated config still uses the default template as a base but overwrites detected fields with real values. Init is fully non-interactive — detected values are printed to stdout but never prompt for confirmation.

## User Story

As a developer setting up agtop in a new project
I want `agtop init` to auto-detect my project's configuration
So that I get a working `agtop.toml` without manual editing

## Relevant Files

- `cmd/agtop/init.go` — Current init command. Copies the embedded template verbatim. Needs to call the detector and write detected values.
- `cmd/agtop/assets.go` — Embeds `agtop.example.toml` as `defaultConfig`. No changes needed.
- `cmd/agtop/main.go` — Routes `agtop init`. Needs to parse `--ai`/`--no-ai` flags and pass to `runInit`.
- `internal/config/config.go` — Config struct definitions. The detector builds a partial `Config` to populate detected fields.
- `internal/config/defaults.go` — `DefaultConfig()` function. Used as the base, with detected values merged on top.
- `internal/config/loader.go` — Config loading. No changes needed (init writes the file, loader reads it later).
- `internal/runtime/factory.go` — `NewRuntime` does runtime detection via `exec.LookPath`. The detector reuses the same approach.

### New Files

- `internal/detect/detect.go` — Core detection engine. Scans the project directory and returns a `DetectedConfig` struct with all discovered values.
- `internal/detect/detect_test.go` — Tests for each detector using temp directories with known project layouts.

## Implementation Plan

### Phase 1: Detection Engine

Create `internal/detect/` package with a `Detect(projectRoot string) (*Result, error)` function. This function runs a series of detectors and aggregates results into a single struct.

Detectors:
1. **Project name** — Use the directory basename.
2. **Runtime** — Check `exec.LookPath("claude")` and `exec.LookPath("opencode")`. Pick the first available, prefer claude.
3. **Test command** — Inspect `package.json` scripts (test, check), `Makefile` targets (test, check), `pyproject.toml` scripts, `Cargo.toml`, `go.mod`. Return the first match.
4. **Dev server command** — Inspect `package.json` scripts (dev, start, serve), `Makefile` targets (dev, run, serve). Return the first match.
5. **Sub-repos** — Walk immediate subdirectories (one level deep, then their immediate children — max depth 2) looking for `.git` directories. If the root itself is a git repo and has no sub-repos, return nothing (single-repo mode). If sub-repos are found, return `[]RepoConfig` with name (dir basename) and path (relative).
6. **Language/framework hint** — Used internally to choose better defaults. Detect from marker files: `go.mod` → Go, `package.json` → Node, `Cargo.toml` → Rust, `pyproject.toml`/`requirements.txt` → Python.

### Phase 2: TOML Generation

Rather than building a `Config` struct and marshaling it (which would produce a flat, commentless TOML file), use a template-based approach:

1. Start with the embedded `agtop.example.toml` content as a string.
2. Use string replacement to substitute placeholder values with detected values.
3. For sub-repos: if detected, uncomment and populate the `[[repos]]` section; if not detected, leave it commented out.

This preserves the example file's comments, section headers, and documentation for the user to reference.

### Phase 3: AI-Powered Detection

Add an AI refinement pass that runs after static detection. This shells out to the detected runtime to analyze the project more deeply.

1. Build a prompt that describes what was already detected and asks the AI to verify/refine the config values. Include the static detection results and ask for structured JSON output.
2. The prompt instructs the AI to read the project's README, source files, and configuration to determine: test commands, dev server commands, sub-repo layout, and any other config values.
3. Parse the AI's JSON response and merge it with the static detection results. AI values override static values when present.
4. The AI pass uses the detected runtime with minimal options: read-only tools only (`Read`, `Glob`, `Grep`), low max turns (5), and a short timeout (60s).
5. If the AI pass fails (binary not found, timeout, parse error), fall back silently to static-only results with a warning printed to stderr.

### Phase 4: Init Integration

Update `cmd/agtop/init.go` to run detection before writing `agtop.toml`:

1. Parse `--ai`/`--no-ai` flags (default: `--ai`).
2. Call `detect.Detect(".")` for static detection.
3. If AI enabled, call `detect.DetectWithAI(root, staticResult)` to refine.
4. Print each detected value to stdout (e.g., `  detected runtime: claude`, `  detected test command: go test ./...`).
5. Generate `agtop.toml` with detected values merged into the template.
6. The rest of init (hooks, settings.json) proceeds as before.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Create the Detection Engine

- Create `internal/detect/detect.go` with:
  - `Result` struct containing all detected fields:
    ```go
    type Result struct {
        ProjectName  string
        Runtime      string        // "claude" or "opencode" or ""
        TestCommand  string
        DevServer    string
        Repos        []RepoResult  // empty = single-repo
        Language     string        // "go", "node", "rust", "python", ""
    }
    type RepoResult struct {
        Name string
        Path string
    }
    ```
  - `Detect(root string) (*Result, error)` that calls each sub-detector.
  - `detectProjectName(root string) string` — returns `filepath.Base(absRoot)`.
  - `detectRuntime() string` — checks `exec.LookPath` for "claude" then "opencode".
  - `detectTestCommand(root string) string` — reads `package.json`, `Makefile`, `pyproject.toml`, checks for `go.mod`/`Cargo.toml`.
  - `detectDevServer(root string) string` — reads `package.json` scripts, `Makefile` targets.
  - `detectRepos(root string) []RepoResult` — walks subdirs up to depth 2, checks for `.git`.
  - `detectLanguage(root string) string` — checks marker files.

### 2. Implement Test Command Detection

Detection priority for `test_command`:
1. `go.mod` exists → `"go test ./..."`
2. `Cargo.toml` exists → `"cargo test"`
3. `package.json` with `scripts.test` (and not the default `echo "Error"`) → `"npm test"` (or `"yarn test"` if `yarn.lock` exists, `"pnpm test"` if `pnpm-lock.yaml` exists, `"bun test"` if `bun.lockb` exists)
4. `Makefile` with a `test:` target → `"make test"`
5. `pyproject.toml` exists → `"pytest"` if pytest in dependencies, else `"python -m unittest"`

### 3. Implement Dev Server Detection

Detection priority for `dev_server.command`:
1. `package.json` with `scripts.dev` → `"npm run dev"` (adjusted for yarn/pnpm/bun per lockfile)
2. `package.json` with `scripts.start` → `"npm start"` (adjusted per lockfile)
3. `Makefile` with a `dev:` or `run:` target → `"make dev"` or `"make run"`

### 4. Implement Sub-Repo Detection

- Walk directories at depth 1 and 2 from root.
- For each, check if `.git` exists (directory or file — git worktrees use a `.git` file).
- Skip: `.git`, `.agtop`, `node_modules`, `vendor`, `.cache`, hidden dirs (starting with `.`).
- If root is a git repo AND sub-repos are found → poly-repo mode, return the sub-repos.
- If root is a git repo with no sub-repos → single-repo mode, return empty.
- If root is NOT a git repo but sub-repos are found → poly-repo mode, return the sub-repos.
- Use the directory name relative to root as `Name`, relative path as `Path`.

### 5. Implement AI-Powered Detection

- Add `DetectWithAI(root string, static *Result) (*Result, error)` to `internal/detect/detect.go`.
- Detect which runtime binary is available (prefer claude, fall back to opencode).
- Build the AI prompt:
  - Describe the project root path and what static detection found.
  - Ask the AI to read README.md, project config files, and source structure.
  - Request a JSON response with fields: `project_name`, `test_command`, `dev_server_command`, `repos` (array of `{name, path}`).
  - Instruct the AI to only include fields it is confident about — omit uncertain fields.
- Shell out to the runtime:
  - For Claude: `claude -p "<prompt>" --output-format json --max-turns 5 --allowedTools Read,Glob,Grep --permission-mode manual`
  - For OpenCode: `opencode run "<prompt>" --format json --agent build`
  - Set a 60-second timeout on the subprocess.
- Parse the JSON response from stdout. Extract the structured fields.
- Merge AI results onto static results: AI values override static values for any non-empty field.
- If any step fails (binary missing, timeout, invalid JSON), log a warning to stderr and return the unmodified static result.

### 6. Implement TOML Template Rendering

- Add a `renderConfig(template []byte, result *detect.Result) []byte` function in `cmd/agtop/init.go`.
- Replace `name = "my-project"` with detected project name.
- Replace `test_command = "npm test"` with detected test command (or leave template default if not detected).
- Replace `command = "npm run dev"` with detected dev server command (or leave template default).
- Replace `default = "claude"` with detected runtime (or leave template default).
- If sub-repos detected: uncomment and populate the `[[repos]]` sections. Replace the example `[[repos]]` block.
- If no sub-repos: leave the commented example as-is.

### 7. Update `runInit` and Flag Parsing

- In `cmd/agtop/main.go`, update the `init` case to parse flags:
  - `--no-ai` flag disables the AI detection pass. Default is AI enabled.
  - Pass the flag value to `runInit`.
- Change `runInit` signature to `runInit(cfg *config.Config, useAI bool) error`.
- In `runInit`, before writing `agtop.toml`:
  - Call `detect.Detect(".")` for static detection.
  - If `useAI` is true, call `detect.DetectWithAI(".", staticResult)` and print `  running AI analysis...` before, `  AI analysis complete` after.
  - Print each detected value with a `  detected ` prefix.
  - Call `renderConfig(defaultConfig, result)` to get the populated TOML.
  - Write the rendered TOML instead of the raw template.

### 8. Write Tests

- Create `internal/detect/detect_test.go` with tests for each detector:
  - `TestDetectProjectName` — uses a temp dir with a known name.
  - `TestDetectTestCommand_Go` — creates temp dir with `go.mod`, expects `"go test ./..."`.
  - `TestDetectTestCommand_Node` — creates temp dir with `package.json` containing test script.
  - `TestDetectTestCommand_None` — empty dir, expects `""`.
  - `TestDetectDevServer_Node` — creates temp dir with `package.json` containing dev script.
  - `TestDetectRepos_SingleRepo` — creates temp dir with `.git`, expects empty repos.
  - `TestDetectRepos_PolyRepo` — creates temp dir with `.git` and subdirs that each have `.git`.
  - `TestDetectRepos_NestedSubRepo` — creates temp dir with `app/client/.git` and `app/server/.git` at depth 2.
  - `TestDetectLanguage` — creates temp dirs with various marker files.
  - `TestDetectWithAI_Fallback` — verifies that when no runtime binary is available, `DetectWithAI` returns the static result unchanged with no error.
  - `TestParseAIResponse` — tests JSON parsing of AI output with various valid and malformed inputs.

## Testing Strategy

### Unit Tests

- Each detector function is independently testable using temp directories.
- The `renderConfig` function is testable by passing a known template string and result, then asserting the output contains expected values.

### Edge Cases

- Project with no recognizable files → all fields empty, falls back to template defaults.
- `package.json` with default npm `test` script (`echo "Error: no test specified" && exit 1`) → should be ignored.
- Sub-repos at depth 2 (e.g., `app/client/.git`) → correctly detected with `app/client` as path and `client` as name.
- Symlinked directories → should follow symlinks for `.git` detection.
- Permission errors on subdirectories → skip gracefully, don't fail init.
- AI runtime not available (neither claude nor opencode installed) → falls back to static-only with a warning.
- AI returns invalid JSON or times out → falls back to static-only with a warning.
- `--no-ai` flag → skips AI pass entirely, uses static detection only.
- AI returns values that conflict with static detection → AI values win (AI has more context).

## Risk Assessment

- **Low risk**: This only affects `agtop init`, which is a one-time setup command. Existing `agtop.toml` files are never overwritten (the existing `os.IsNotExist` check remains).
- **Template replacement is fragile**: If the template format changes, the string replacements could break. Mitigate by using specific line-matching patterns rather than simple `strings.Replace`.
- **False positives in detection**: A `Makefile` with a `test:` target might not be the right test command. This is acceptable — the generated config is a starting point, not a final answer. Users can edit.
- **AI cost**: The AI pass consumes tokens. With max 5 turns and read-only tools, cost should be minimal (a few cents at most). The `--no-ai` flag provides an escape hatch.
- **AI latency**: The AI pass adds 10-30 seconds to init. This is acceptable for a one-time setup command. Print a "running AI analysis..." message so the user knows it's working.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go vet ./...
go test ./internal/detect/...
go test ./...
```

## Open Questions (Unresolved)

None — all design decisions resolved.

## Sub-Tasks

Single task — no decomposition needed.
