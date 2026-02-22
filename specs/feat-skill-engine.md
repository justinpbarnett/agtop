# Feature: Skill Engine

## Metadata

type: `feat`
task_id: `skill-engine`
prompt: `Implement skill discovery, SKILL.md parsing, skill registry with precedence-based override resolution, and prompt assembly for the injection model — enabling agtop to find, load, and invoke skills as hyper-targeted claude -p calls`

## Feature Description

The Skill Engine is the bridge between agtop's workflow configuration and the process manager's subprocess execution. It discovers `SKILL.md` files from a precedence-ordered list of directories, parses their YAML frontmatter and markdown body, caches them in an in-memory registry, and assembles fully-formed prompts that inject skill instructions directly into each `claude -p` invocation.

This implements agtop's **injection model**: rather than relying on the agent runtime's native skill discovery, agtop reads SKILL.md content and injects it as the system prompt for each subprocess call. This guarantees deterministic skill ordering, fresh context per skill, and runtime-agnostic execution. A user can write a skill once and use it both interactively in Claude Code (`/skill-name`) and in an agtop workflow — the file is the same, only the invocation mechanism differs.

The Skill Engine also extends the existing `SkillConfig` from `agtop.yaml` to support per-skill `allowed-tools` overrides, enabling defense-in-depth via Layer 3 tool restriction (a `review` skill gets `Read,Grep`; a `build` skill gets the full set).

## User Story

As a developer configuring agtop workflows
I want skills to be automatically discovered from standard directories, with project-local overrides taking precedence over built-in defaults
So that I can customize agent behavior per-project without modifying global configurations, and new skills work immediately by adding a SKILL.md file

## Problem Statement

The current codebase has:
- An empty `engine.Skill` struct with fields but no parsing logic (`internal/engine/skill.go`)
- An empty `engine.Workflow` struct with no execution capability (`internal/engine/workflow.go`)
- An empty `engine.Executor` struct (`internal/engine/executor.go`)
- A fully functional `process.Manager` that can spawn and monitor subprocesses
- A fully functional `runtime.ClaudeRuntime` that builds `claude -p` commands
- A config system with `SkillConfig` (model, timeout, parallel) but no frontmatter parsing or directory discovery

There is no way to:
- Discover SKILL.md files from the filesystem
- Parse YAML frontmatter from SKILL.md to extract metadata
- Resolve skill name conflicts across multiple directories (precedence)
- Assemble a prompt that injects SKILL.md content with run context
- Map a workflow's skill list to actual skill invocations
- Pass per-skill allowed-tools restrictions to the runtime

Without this, the workflow executor (step 7) cannot function — it needs the Skill Engine to translate skill names into executable prompts.

## Solution Statement

Implement the Skill Engine as three components within the existing `internal/engine/` package:

1. **Skill Loader** (`skill.go`) — discovers SKILL.md files from six precedence-ordered directories, parses YAML frontmatter and markdown body, and returns `Skill` structs. Uses a simple frontmatter parser (split on `---` delimiters) to avoid external dependencies.

2. **Skill Registry** (`registry.go`) — in-memory cache of loaded skills, keyed by name. Handles precedence-based override resolution: when multiple directories contain a skill with the same name, the highest-precedence source wins. Provides lookup by name with merged config from `agtop.yaml`.

3. **Prompt Builder** (`prompt.go`) — assembles the final prompt for a `claude -p` invocation by combining the SKILL.md body with run context (worktree path, branch name, previous skill output summary, user task description). Produces a single string ready for injection.

The `SkillConfig` struct in config is extended with an `AllowedTools` field for per-skill tool restrictions.

## Relevant Files

Use these files to implement the feature:

- `internal/engine/skill.go` — Currently defines empty `Skill` struct. Will be rewritten with frontmatter parsing, SKILL.md discovery, and the `Skill` type with all metadata fields.
- `internal/engine/executor.go` — Currently empty `Executor` struct. Will remain empty in this step (implemented in step 7), but the prompt builder will produce output consumed by the executor.
- `internal/engine/workflow.go` — Currently defines empty `Workflow` struct. No changes in this step — workflow sequencing is step 7.
- `internal/engine/decompose.go` — `DecomposeResult` and `DecomposeTask` types. No changes in this step.
- `internal/config/config.go` — `SkillConfig` struct. Will be extended with `AllowedTools` field.
- `internal/config/defaults.go` — Default skill configs. Will be extended with default `AllowedTools` per skill.
- `internal/runtime/runtime.go` — `RunOptions` struct. Already has `AllowedTools []string` — no changes needed.
- `internal/process/manager.go` — `Manager.Start()`. No changes — it already accepts `RunOptions` with `AllowedTools`.

### New Files

- `internal/engine/registry.go` — Skill registry: in-memory cache, precedence resolution, config merging, lookup by name.
- `internal/engine/prompt.go` — Prompt builder: assembles SKILL.md body + context into a complete prompt string.
- `internal/engine/skill_test.go` — Tests for SKILL.md parsing (frontmatter extraction, body extraction, edge cases).
- `internal/engine/registry_test.go` — Tests for registry loading, precedence resolution, config merging.
- `internal/engine/prompt_test.go` — Tests for prompt assembly with various context combinations.
- `internal/engine/testdata/` — Test fixture SKILL.md files for parser tests.

## Implementation Plan

### Phase 1: Foundation

Build the SKILL.md parser that extracts YAML frontmatter and markdown body from a single file. This is a pure function with no filesystem dependencies — it takes a byte slice and returns a `Skill` struct. Establish the complete `Skill` type with all metadata fields.

### Phase 2: Core Implementation

Implement the skill registry that discovers SKILL.md files from the six precedence-ordered directories, loads them via the parser, resolves name conflicts, and merges per-skill config from `agtop.yaml`. Extend `SkillConfig` with `AllowedTools`. Build the prompt assembly function.

### Phase 3: Integration

Wire the registry into the app initialization flow so it's available when the workflow executor needs it (step 7). Ensure the registry is constructed during startup with the loaded config, and exposed to the components that will consume it.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Extend the Skill Struct

Rewrite `internal/engine/skill.go`:

- Replace the existing `Skill` struct with a complete definition:
  ```go
  type Skill struct {
      Name         string   // From frontmatter "name" field
      Description  string   // From frontmatter "description" field
      Model        string   // From frontmatter agtop extension or config override
      Timeout      int      // Seconds; from frontmatter or config override
      Parallel     bool     // From config; allows parallel execution in decomposed workflows
      AllowedTools []string // From frontmatter or config override; per-skill tool restrictions
      Content      string   // Full markdown body (everything after frontmatter)
      Source       string   // Filesystem path where this skill was loaded from
      Priority     int      // Precedence level (0 = highest, 5 = lowest)
  }
  ```
- Define the precedence levels as constants:
  ```go
  const (
      PriorityProjectAgtop  = 0 // .agtop/skills/*/SKILL.md
      PriorityProjectClaude = 1 // .claude/skills/*/SKILL.md
      PriorityProjectAgents = 2 // .agents/skills/*/SKILL.md
      PriorityUserAgtop     = 3 // ~/.config/agtop/skills/*/SKILL.md
      PriorityUserClaude    = 4 // ~/.claude/skills/*/SKILL.md
      PriorityBuiltIn       = 5 // <binary-dir>/skills/*/SKILL.md
  )
  ```
- Define a `skillFrontmatter` struct for YAML unmarshalling:
  ```go
  type skillFrontmatter struct {
      Name         string   `yaml:"name"`
      Description  string   `yaml:"description"`
      Model        string   `yaml:"model"`
      Timeout      int      `yaml:"timeout"`
      ParallelGroup string  `yaml:"parallel-group"`
      AllowedTools []string `yaml:"allowed-tools"`
  }
  ```

### 2. Implement SKILL.md Parser

Add parsing functions to `internal/engine/skill.go`:

- `ParseSkill(data []byte, source string, priority int) (*Skill, error)` — the core parser:
  - Convert `data` to string, trim leading whitespace
  - Check if the content starts with `---` (frontmatter delimiter)
  - If yes: find the closing `---` delimiter, extract the frontmatter YAML between them, extract the body as everything after the closing delimiter
  - If no frontmatter: treat the entire content as the body, use the parent directory name as the skill name (e.g., for `/path/to/build/SKILL.md`, name is `build`)
  - Unmarshal frontmatter YAML into `skillFrontmatter` using `gopkg.in/yaml.v3`
  - Construct and return `*Skill` with all fields populated
  - If `Name` is empty after parsing, derive it from the parent directory name of `source`
  - Trim leading/trailing whitespace from `Content`
- `ParseSkillFile(path string, priority int) (*Skill, error)` — reads the file at `path` via `os.ReadFile` and calls `ParseSkill`
- `SkillNameFromPath(path string) string` — extracts the skill name from a SKILL.md file path by returning the name of the parent directory (e.g., `/foo/skills/build/SKILL.md` → `build`)

### 3. Extend SkillConfig with AllowedTools

Update `internal/config/config.go`:

- Add `AllowedTools []string` field to `SkillConfig`:
  ```go
  type SkillConfig struct {
      Model        string   `yaml:"model"`
      Timeout      int      `yaml:"timeout"`
      Parallel     bool     `yaml:"parallel"`
      AllowedTools []string `yaml:"allowed_tools"`
  }
  ```

Update `internal/config/defaults.go`:

- Add default `AllowedTools` for skills that should have restricted tool access:
  ```go
  "route":    {Model: "haiku", Timeout: 60, AllowedTools: []string{"Read", "Grep", "Glob"}},
  "spec":     {Model: "opus", AllowedTools: []string{"Read", "Write", "Grep", "Glob"}},
  "decompose": {Model: "opus", AllowedTools: []string{"Read", "Grep", "Glob"}},
  "build":    {Model: "sonnet", Timeout: 300, Parallel: true},
  "test":     {Model: "sonnet", Timeout: 120},
  "review":   {Model: "opus", AllowedTools: []string{"Read", "Grep", "Glob"}},
  "document": {Model: "haiku", AllowedTools: []string{"Read", "Write", "Grep", "Glob"}},
  "commit":   {Model: "haiku", Timeout: 30},
  "pr":       {Model: "haiku", Timeout: 30},
  ```
  Note: `build` and `test` intentionally have no `AllowedTools` restriction — they inherit the full set from `runtime.claude.allowed_tools`.

### 4. Implement the Skill Registry

Create `internal/engine/registry.go`:

- Define `Registry` struct:
  ```go
  type Registry struct {
      skills map[string]*Skill // name → skill (highest precedence wins)
      cfg    *config.Config
  }
  ```
- Define the directory search order as a type:
  ```go
  type SkillSource struct {
      Dir      string // Absolute path to the skills directory
      Priority int    // Precedence level
  }
  ```
- `NewRegistry(cfg *config.Config) *Registry` — constructor, initializes empty map
- `Load(projectRoot string) error` — discovers and loads all skills:
  - Build the search path list by computing absolute paths:
    1. `{projectRoot}/.agtop/skills` — priority 0
    2. `{projectRoot}/.claude/skills` — priority 1
    3. `{projectRoot}/.agents/skills` — priority 2
    4. `{userHome}/.config/agtop/skills` — priority 3
    5. `{userHome}/.claude/skills` — priority 4
    6. `{binaryDir}/skills` — priority 5 (where `binaryDir` is the directory containing the agtop binary, resolved via `os.Executable()`)
  - For each directory in **reverse precedence order** (lowest priority first, so higher priority overwrites):
    - If the directory doesn't exist, skip silently
    - Glob for `*/SKILL.md` within the directory
    - For each match, call `ParseSkillFile(path, priority)`
    - On parse error: log warning (`fmt.Fprintf(os.Stderr, ...)`) and continue — don't crash on bad SKILL.md files
    - Store the parsed skill in the map keyed by name — later (higher priority) entries overwrite earlier ones
  - After all directories are scanned, merge config overrides from `cfg.Skills`:
    - For each skill in the map, check if `cfg.Skills[skill.Name]` exists
    - If yes, apply non-zero config values: `Model`, `Timeout`, `Parallel`, `AllowedTools` override the frontmatter values
    - If `AllowedTools` is empty in both frontmatter and config, leave it nil (the caller will fall back to the runtime default)
- `Get(name string) (*Skill, bool)` — lookup by name
- `List() []*Skill` — return all loaded skills sorted by name
- `Names() []string` — return all skill names sorted alphabetically
- `SkillForRun(name string, cfg *config.Config) (*Skill, runtime.RunOptions)` — convenience method that returns the skill and fully-resolved `RunOptions`:
  - Look up the skill by name
  - Build `RunOptions` with model resolution order: skill-specific config → skill frontmatter → runtime default (`cfg.Runtime.Claude.Model`)
  - Set `AllowedTools`: skill-specific config → skill frontmatter → runtime default (`cfg.Runtime.Claude.AllowedTools`)
  - Set `MaxTurns` from `cfg.Runtime.Claude.MaxTurns`
  - Set `PermissionMode` from `cfg.Runtime.Claude.PermissionMode`
  - `WorkDir` is NOT set here (the caller provides it based on the run's worktree)
  - Return the skill and options

### 5. Implement the Prompt Builder

Create `internal/engine/prompt.go`:

- Define `PromptContext` struct:
  ```go
  type PromptContext struct {
      WorkDir          string // Worktree path
      Branch           string // Git branch name
      PreviousOutput   string // Summary from previous skill (empty for first skill)
      UserPrompt       string // The user's original task description
  }
  ```
- `BuildPrompt(skill *Skill, pctx PromptContext) string` — assembles the final prompt:
  - Start with the skill's markdown body (`skill.Content`)
  - Append a context section:
    ```

    ---

    ## Context

    - Working directory: {WorkDir}
    - Branch: {Branch}
    ```
  - If `PreviousOutput` is non-empty, append:
    ```
    - Previous skill output:
    {PreviousOutput}
    ```
  - Append the task:
    ```

    ## Task

    {UserPrompt}
    ```
  - Return the assembled string
- Keep the function simple and deterministic — no side effects, no file I/O

### 6. Create Test Fixtures

Create `internal/engine/testdata/` directory with fixture files:

- `testdata/valid/SKILL.md` — a complete SKILL.md with frontmatter containing all fields:
  ```markdown
  ---
  name: test-skill
  description: A test skill for unit testing
  model: sonnet
  timeout: 120
  allowed-tools:
    - Read
    - Grep
  ---

  # Test Skill

  You are a test skill. Follow these instructions carefully.

  ## Steps

  1. Do the first thing
  2. Do the second thing
  ```
- `testdata/minimal/SKILL.md` — frontmatter with only `name` and `description`:
  ```markdown
  ---
  name: minimal-skill
  description: A minimal skill
  ---

  Do the thing.
  ```
- `testdata/no-frontmatter/SKILL.md` — no frontmatter at all:
  ```markdown
  # No Frontmatter Skill

  Just raw markdown content with no YAML frontmatter.
  ```
- `testdata/empty-frontmatter/SKILL.md` — empty frontmatter (just delimiters):
  ```markdown
  ---
  ---

  Content after empty frontmatter.
  ```
- `testdata/malformed/SKILL.md` — invalid YAML in frontmatter:
  ```markdown
  ---
  name: [invalid yaml
  description: missing bracket
  ---

  Content after malformed frontmatter.
  ```

### 7. Write Skill Parser Tests

Create `internal/engine/skill_test.go`:

- `TestParseSkillWithFullFrontmatter` — parse `testdata/valid/SKILL.md`, verify all fields (name, description, model, timeout, allowed-tools, content body)
- `TestParseSkillWithMinimalFrontmatter` — parse `testdata/minimal/SKILL.md`, verify name and description are set, other fields are zero-valued, content body is correct
- `TestParseSkillWithNoFrontmatter` — parse `testdata/no-frontmatter/SKILL.md`, verify name is derived from parent directory (`no-frontmatter`), content is the entire file
- `TestParseSkillWithEmptyFrontmatter` — parse `testdata/empty-frontmatter/SKILL.md`, verify name is derived from directory, content starts after the closing delimiter
- `TestParseSkillWithMalformedFrontmatter` — parse `testdata/malformed/SKILL.md`, verify it returns an error
- `TestParseSkillPreservesContent` — verify the content field preserves markdown formatting, headings, code blocks, and list items exactly
- `TestSkillNameFromPath` — test various path patterns:
  - `/home/user/.agtop/skills/build/SKILL.md` → `build`
  - `/home/user/.claude/skills/my-skill/SKILL.md` → `my-skill`
  - `skills/test/SKILL.md` → `test`
- `TestParseSkillFileMissing` — call `ParseSkillFile` with non-existent path, verify error
- `TestParseSkillSourceAndPriority` — verify that `Source` and `Priority` fields are set correctly from the arguments

### 8. Write Registry Tests

Create `internal/engine/registry_test.go`:

- `TestRegistryLoadFromDirectory` — create a temp directory with two skill subdirectories, each containing a SKILL.md. Call `Load`, verify both skills are in the registry.
- `TestRegistryPrecedenceOverride` — create two temp directories simulating different precedence levels (e.g., project-local and built-in). Both contain a skill named `build` with different content. Verify the higher-precedence (lower priority number) version wins.
- `TestRegistryConfigMerge` — load a skill with frontmatter `model: haiku`, then configure `cfg.Skills["test-skill"].Model = "opus"`. Verify the config value overrides the frontmatter value.
- `TestRegistryConfigAllowedToolsMerge` — load a skill with frontmatter `allowed-tools: [Read]`, config has `AllowedTools: [Read, Write]`. Verify config wins.
- `TestRegistrySkipsMissingDirectories` — call `Load` with a project root where none of the skill directories exist. Verify no error, empty registry.
- `TestRegistrySkipsMalformedSkills` — include a malformed SKILL.md in the scan directory. Verify it's skipped with no error, other skills still load.
- `TestRegistryGet` — load skills, verify `Get("build")` returns the skill, `Get("nonexistent")` returns false.
- `TestRegistryList` — load skills, verify `List()` returns all skills sorted by name.
- `TestRegistryNames` — verify `Names()` returns sorted skill names.
- `TestRegistrySkillForRun` — verify `SkillForRun` returns correct `RunOptions` with model resolution (skill config → frontmatter → runtime default).

### 9. Write Prompt Builder Tests

Create `internal/engine/prompt_test.go`:

- `TestBuildPromptComplete` — build a prompt with all context fields populated. Verify the output contains the skill content, context section with workdir/branch, previous output, and task.
- `TestBuildPromptNoPreviousOutput` — build with empty `PreviousOutput`. Verify the "Previous skill output" section is omitted.
- `TestBuildPromptMinimal` — build with only `UserPrompt` set (empty workdir, branch, previous output). Verify the output contains the skill content and task, with minimal context.
- `TestBuildPromptPreservesSkillContent` — verify that the skill's markdown content appears verbatim at the start of the prompt, not modified or truncated.

### 10. Wire Registry into App Initialization

Update `internal/tui/app.go` (or the appropriate initialization path):

- Add a `registry *engine.Registry` field to the `App` struct
- In `NewApp(cfg *config.Config)`:
  - Create a new registry: `reg := engine.NewRegistry(cfg)`
  - Determine project root: use `cfg.Project.Root` if set, otherwise use the current working directory
  - Call `reg.Load(projectRoot)` — log a warning to stderr on error but don't fail startup (the TUI can still work without skills for development/debugging)
  - Store the registry on the app
- Add a `Registry() *engine.Registry` accessor method for use by future components (workflow executor in step 7)

## Testing Strategy

### Unit Tests

- **Parser tests** — pure function tests using fixture files in `testdata/`. Test frontmatter extraction, body extraction, name derivation, error handling for malformed YAML. No filesystem mocking needed — tests read from committed fixture files.
- **Registry tests** — use `os.MkdirTemp` to create temporary directory trees with SKILL.md files. Tests verify discovery, precedence, config merging, and error tolerance. Clean up temp dirs with `t.Cleanup`.
- **Prompt builder tests** — pure function tests. Verify output string structure with specific assertions (contains skill content, contains context, etc.).

### Edge Cases

- **SKILL.md with no frontmatter** — treated as body-only, name derived from directory
- **SKILL.md with empty frontmatter** (`---\n---`) — treated as no frontmatter values, name derived from directory
- **SKILL.md with malformed YAML frontmatter** — returns parse error, registry skips it
- **SKILL.md with only frontmatter, no body** — valid but `Content` is empty string
- **Multiple skills with same name in different directories** — lowest priority number (highest precedence) wins
- **Skill referenced in config but no SKILL.md found** — `Get()` returns false; the workflow executor (step 7) will handle this as an error
- **Skill directory doesn't exist** — skipped silently during `Load()`
- **Skill directory exists but is empty** — no skills loaded from it, no error
- **SKILL.md is a symlink** — `os.ReadFile` follows symlinks, works transparently
- **Very large SKILL.md** (>100KB) — no size limit enforced, loads normally
- **Frontmatter with unknown fields** — `yaml.Unmarshal` ignores unknown fields (default behavior), no error
- **Config overrides with zero values** — only non-zero config values override frontmatter (empty string, 0 int, nil slice don't override)
- **Binary directory not writable** — irrelevant, we only read from it
- **`os.Executable()` fails** — skip the built-in skills directory, log a warning

## Acceptance Criteria

- [ ] `ParseSkill` extracts YAML frontmatter (name, description, model, timeout, allowed-tools) from SKILL.md content
- [ ] `ParseSkill` extracts the markdown body as `Content`, preserving all formatting
- [ ] `ParseSkill` handles SKILL.md files with no frontmatter (name derived from directory)
- [ ] `ParseSkill` returns an error for malformed YAML frontmatter
- [ ] `SkillNameFromPath` extracts the parent directory name from a SKILL.md path
- [ ] `Registry.Load` discovers SKILL.md files from six precedence-ordered directories
- [ ] `Registry.Load` silently skips directories that don't exist
- [ ] `Registry.Load` skips malformed SKILL.md files with a warning, doesn't crash
- [ ] Higher-precedence skills override lower-precedence skills with the same name
- [ ] `Registry.Load` merges non-zero `SkillConfig` values from `agtop.yaml` over frontmatter values
- [ ] `Registry.Get` returns the correct skill by name
- [ ] `Registry.List` returns all skills sorted by name
- [ ] `Registry.SkillForRun` returns fully-resolved `RunOptions` with correct model precedence (config → frontmatter → runtime default)
- [ ] `Registry.SkillForRun` includes per-skill `AllowedTools` when configured
- [ ] `BuildPrompt` produces a prompt containing the skill content, context section, and task
- [ ] `BuildPrompt` omits the previous output section when `PreviousOutput` is empty
- [ ] `SkillConfig` in config includes `AllowedTools` field with correct YAML tag
- [ ] Default configs include `AllowedTools` restrictions for read-only skills (route, review, decompose)
- [ ] Skill registry is created and loaded during app initialization
- [ ] All tests pass with `go test -race ./internal/engine/...`
- [ ] `go vet ./...` and `go build ./...` pass cleanly

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# All packages compile
go build ./...

# No vet issues
go vet ./...

# Engine package tests with race detector
go test -race ./internal/engine/... -v

# Config package tests still pass
go test -race ./internal/config/... -v

# TUI tests still pass
go test ./internal/tui/... -v

# All tests pass
go test -race ./...

# Binary builds
make build
```

## Notes

- The `SkillConfig.AllowedTools` field uses `yaml:"allowed_tools"` (snake_case) to match the existing config convention. The SKILL.md frontmatter uses `allowed-tools` (kebab-case) to match the SKILL.md standard. Both are read correctly — the config loader handles YAML, and the frontmatter parser handles the SKILL.md format independently.

- The registry loads in **reverse precedence order** (built-in first, project-local last) so that simple map assignment handles overrides — the last write wins, and the last source scanned is the highest precedence. This avoids needing to compare priority values during insertion.

- The `Priority` field on `Skill` is informational — it records where the skill was loaded from for debugging and display in the TUI's detail view. It's not used for override logic (that's handled by load order).

- The prompt builder intentionally does NOT add safety instructions (Layer 1 prompt injection defense from step 9). That will be handled by the workflow executor (step 7) which wraps the prompt builder's output with safety preamble. Keeping prompt assembly and safety injection separate maintains clear separation of concerns.

- `SkillForRun` does NOT set `WorkDir` on `RunOptions` because the worktree path comes from the run, not the skill. The workflow executor (step 7) will set `WorkDir` before calling `process.Manager.Start()`.

- The six-directory discovery path includes `.claude/skills/` which is also where the user's Claude Code skills live (the files in this repo's `.claude/skills/` directory). This is intentional — project skills written for interactive Claude Code use are automatically available to agtop workflows. The injection model means they execute differently (injected as system prompt vs. native skill discovery), but the content is shared.

- The registry does NOT watch for filesystem changes. Skill discovery happens once at startup. If a user adds or modifies a SKILL.md, they must restart agtop. Hot-reload of skills could be added in a future step but is not necessary for the initial implementation.
