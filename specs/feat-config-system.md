# Feature: Configuration System

## Metadata

type: `feat`
task_id: `config-system`
prompt: `Implement the agtop configuration system: YAML config loader with file discovery chain, defaults merging, validation, and environment variable overrides`

## Feature Description

The configuration system is the backbone of agtop's customizability. It loads project-level and user-level YAML config files, merges them with built-in defaults, validates the result, and applies environment variable overrides. Every other subsystem (workflows, skills, safety, cost limits, runtime selection) reads from this config at startup.

The config structs and defaults already exist as stubs from Step 1. This task adds the loading, discovery, merging, validation, and env override logic — turning the stubs into a functional config system.

## User Story

As a developer using agtop
I want to configure workflows, models, safety rules, and limits via a YAML file
So that I can customize agtop's behavior per-project without modifying source code

## Problem Statement

The config package currently defines Go structs with YAML tags and a `DefaultConfig()` function, but has no mechanism to:
1. Discover and load config files from the filesystem
2. Merge user config on top of defaults (preserving unset fields)
3. Validate that the loaded config is internally consistent (e.g., workflows reference defined skills, models are valid)
4. Override specific values via environment variables for CI/scripting use cases

Without this, every subsystem would need to hardcode values or implement its own config handling.

## Solution Statement

Implement a `Load()` function that:
1. Starts with `DefaultConfig()` as the base
2. Searches the discovery chain (`./agtop.yaml` → `~/.config/agtop/config.yaml`) and loads the first file found
3. Deep-merges the loaded YAML onto the defaults — only overwriting fields explicitly set in the file
4. Applies environment variable overrides on top
5. Validates the final config and returns clear errors for invalid configurations
6. Returns the merged, validated `Config` for use by the rest of the application

## Relevant Files

Use these files to implement the feature:

- `internal/config/config.go` — Existing config struct definitions with YAML tags. Will be extended with validation tags and any missing fields.
- `internal/config/defaults.go` — Existing `DefaultConfig()` function. No changes needed.
- `agtop.example.yaml` — Reference for the expected YAML schema. No changes needed.
- `go.mod` — Needs `gopkg.in/yaml.v3` added as a dependency.
- `cmd/agtop/main.go` — Will be updated to call `config.Load()` at startup.

### New Files

- `internal/config/loader.go` — Config file discovery, YAML parsing, defaults merging, and env var override logic. The `Load()` function lives here.
- `internal/config/validate.go` — Validation logic for the merged config. Checks referential integrity (workflows → skills), value ranges, and enum validity.
- `internal/config/loader_test.go` — Tests for the loader: file discovery, merging, env overrides, error cases.
- `internal/config/validate_test.go` — Tests for validation logic.

## Implementation Plan

### Phase 1: Dependencies and Loader

Add `gopkg.in/yaml.v3` to the module. Implement the file discovery chain and YAML loading into the existing `Config` struct. Implement deep-merge of loaded config onto defaults.

### Phase 2: Environment Overrides and Validation

Add environment variable override layer. Implement validation that checks referential integrity, value ranges, and enum fields. Return structured errors.

### Phase 3: Integration

Wire `config.Load()` into `main.go`. Pass the loaded config to the TUI model so downstream subsystems can access it.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add yaml.v3 Dependency

- Run `go get gopkg.in/yaml.v3` in the project root
- Run `go mod tidy`

### 2. Implement Config Loader

Create `internal/config/loader.go` with:

- `Load() (*Config, error)` — public entry point. Calls discovery, parse, merge, env override, validate in sequence. Returns a pointer to the final config.
- `discoverConfigPath() (string, error)` — searches the discovery chain in order:
  1. `./agtop.yaml` (relative to current working directory)
  2. `~/.config/agtop/config.yaml` (XDG-style user config)
  - Returns the first path that exists, or empty string if none found (defaults-only mode is valid)
- `loadFromFile(path string) (*Config, error)` — reads the file, unmarshals YAML into a `Config` struct using `yaml.v3`
- `merge(base, override *Config) *Config` — deep-merges override onto base:
  - For scalar fields: override wins if non-zero-value
  - For maps (`Workflows`, `Skills`): merge keys, override values for matching keys
  - For slices (`AllowedTools`, `BlockedPatterns`): override replaces entire slice if present (non-nil)
  - Rationale: partial map merging lets users add a custom workflow without losing defaults; slice replacement is simpler and more predictable for safety patterns

### 3. Implement Environment Variable Overrides

In `internal/config/loader.go`, add `applyEnvOverrides(cfg *Config)`:

- `AGTOP_RUNTIME` → `cfg.Runtime.Default` (values: `claude`, `opencode`)
- `AGTOP_MODEL` → `cfg.Runtime.Claude.Model` (overrides default model for Claude runtime)
- `AGTOP_MAX_COST` → `cfg.Limits.MaxCostPerRun` (parse as float64)
- `AGTOP_MAX_TOKENS` → `cfg.Limits.MaxTokensPerRun` (parse as int)
- `AGTOP_MAX_CONCURRENT` → `cfg.Limits.MaxConcurrentRuns` (parse as int)
- `AGTOP_PERMISSION_MODE` → `cfg.Runtime.Claude.PermissionMode`
- Only override if the env var is set and non-empty
- Log a warning (to stderr) if an env var value fails to parse

### 4. Implement Config Validation

Create `internal/config/validate.go` with:

- `validate(cfg *Config) error` — runs all checks, collects errors, returns a combined error or nil
- Checks to implement:
  - **Runtime**: `cfg.Runtime.Default` must be `"claude"` or `"opencode"`
  - **Permission mode**: `cfg.Runtime.Claude.PermissionMode` must be one of `"acceptEdits"`, `"acceptAll"`, `"manual"`
  - **Port strategy**: `cfg.Project.DevServer.PortStrategy` must be one of `"hash"`, `"sequential"`, `"fixed"`
  - **Workflow integrity**: every skill name in every workflow's `Skills` list must exist as a key in `cfg.Skills`
  - **Positive values**: `MaxTurns > 0`, `MaxTokensPerRun > 0`, `MaxCostPerRun > 0`, `MaxConcurrentRuns > 0`, `BasePort > 0`
  - **Safety patterns**: each pattern in `BlockedPatterns` must be valid regex (compile with `regexp.Compile`)
- Return a `ValidationError` type that wraps multiple error messages for clear reporting:
  ```go
  type ValidationError struct {
      Errors []string
  }
  func (e *ValidationError) Error() string // joins errors with newlines
  ```

### 5. Wire Config into main.go

Update `cmd/agtop/main.go`:

- Call `cfg, err := config.Load()` before creating the TUI
- On error, print the error to stderr and exit with code 1
- Pass `cfg` to `tui.NewApp(cfg)` — update `NewApp` signature to accept `*config.Config`
- Update `App` struct in `internal/tui/app.go` to store `*config.Config`

### 6. Write Loader Tests

Create `internal/config/loader_test.go`:

- **TestLoadDefaults**: no config file, no env vars → returns `DefaultConfig()` values
- **TestLoadFromFile**: write a minimal YAML to a temp file, set working directory, verify loaded values override defaults
- **TestMergePreservesDefaults**: override only `project.name`, verify all other fields retain defaults
- **TestMergeWorkflows**: override adds a new workflow, verify default workflows still present
- **TestMergeSkills**: override changes one skill's model, verify other skills unchanged
- **TestMergeSliceReplacement**: override `blocked_patterns` with a shorter list, verify full replacement
- **TestEnvOverrideRuntime**: set `AGTOP_RUNTIME=opencode`, verify `cfg.Runtime.Default == "opencode"`
- **TestEnvOverrideMaxCost**: set `AGTOP_MAX_COST=10.50`, verify `cfg.Limits.MaxCostPerRun == 10.50`
- **TestEnvOverrideInvalidFloat**: set `AGTOP_MAX_COST=notanumber`, verify config still loads (env override skipped)
- **TestDiscoveryChain**: create temp dirs simulating both discovery locations, verify priority order

### 7. Write Validation Tests

Create `internal/config/validate_test.go`:

- **TestValidateDefaultConfig**: `DefaultConfig()` passes validation
- **TestValidateInvalidRuntime**: set `Runtime.Default = "invalid"` → validation error
- **TestValidateInvalidPermissionMode**: set to `"yolo"` → validation error
- **TestValidateWorkflowMissingSkill**: workflow references `"nonexistent"` → validation error listing the missing skill
- **TestValidateBadRegex**: add `"[invalid"` to `BlockedPatterns` → validation error
- **TestValidateZeroLimits**: set `MaxConcurrentRuns = 0` → validation error
- **TestValidateMultipleErrors**: config with several issues → all errors reported in one `ValidationError`

## Testing Strategy

### Unit Tests

- **Loader tests** (`loader_test.go`): test file discovery, YAML parsing, merge semantics, env overrides independently using temp files and `t.Setenv()`
- **Validation tests** (`validate_test.go`): test each validation rule with targeted invalid configs
- All tests use `t.TempDir()` for filesystem isolation — no global state

### Edge Cases

- No config file exists anywhere → pure defaults, no error
- Config file is empty (`---\n`) → pure defaults, no error
- Config file has unknown keys → silently ignored (YAML decoder default behavior)
- Config file has valid YAML but wrong types (e.g., string where int expected) → YAML unmarshal error, clear message
- Environment variable set to empty string → treated as unset
- `~` in discovery path resolves correctly regardless of `HOME` value
- Config file is not readable (permissions) → clear error message with path

## Acceptance Criteria

- `config.Load()` returns a valid `*Config` with no config file present (defaults only)
- `config.Load()` discovers and loads `./agtop.yaml` when present
- `config.Load()` falls back to `~/.config/agtop/config.yaml` when `./agtop.yaml` is absent
- Partial YAML configs merge cleanly — unset fields retain defaults
- Map fields (workflows, skills) merge at the key level
- Slice fields (tools, patterns) replace entirely when specified
- All `AGTOP_*` environment variables override their respective fields
- Invalid env var values are skipped with a warning, not fatal
- Validation catches: invalid runtime, invalid permission mode, workflows referencing undefined skills, invalid regex patterns, zero/negative limits
- `ValidationError` reports all issues in a single error, not just the first
- `main.go` loads config at startup and passes it to the TUI
- All tests pass: `go test ./internal/config/...`
- `go vet ./...` and `go build ./...` pass cleanly

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Dependencies resolve
go mod tidy

# All packages compile
go build ./...

# No vet issues
go vet ./...

# Config tests pass
go test ./internal/config/... -v

# All tests pass
go test ./...

# Binary builds and runs
make build
```

## Notes

- `gopkg.in/yaml.v3` is already listed in the design doc as a dependency but was not added during Step 1 scaffolding. This step adds it.
- The merge strategy (maps merge, slices replace) is a deliberate design choice. Map merging lets users add workflows without redeclaring defaults. Slice replacement avoids confusing append/dedup semantics for safety patterns — if you override patterns, you want exactly what you specified.
- Environment variable overrides are intentionally limited to the most common tuning knobs. Adding more is trivial later.
- The `Load()` function returns `*Config` (pointer) so callers share a single config instance. Future hot-reload (Phase 3 in the design doc) will swap the pointer atomically.
- No config hot-reload in this step — that's a Phase 3 concern per the design doc.
