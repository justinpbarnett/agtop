# Feature: Ignore Skills Config Option

## Metadata

type: `feat`
task_id: `ignore-skills`
prompt: `Add a config option that ignores specific skills from external sources, so users can prefer agtop built-in defaults over .claude/skills or other external skill directories.`

## Feature Description

When `agtop init` is run in a repo that already has `.claude/skills/`, the existing Claude skills override agtop's built-in defaults due to the priority system (`.claude/skills/` priority 1 beats built-in priority 7). Users who know the agtop built-in skills are better need a way to ignore the external versions and fall back to the built-in or `.agtop/skills/` versions.

This feature adds two complementary config options:

1. **Per-skill ignore** (`[skills.X] ignore = true`) — skip a specific skill from non-agtop/non-builtin sources
2. **Source-level ignore** (`[project] ignore_skill_sources = [...]`) — skip all skills from entire source categories

## User Story

As an agtop user
I want to ignore specific external skills or entire skill source directories
So that agtop uses its built-in defaults instead of lower-quality external versions

## Relevant Files

- `internal/config/config.go` — Config structs; needs `Ignore` field on `SkillConfig` and `IgnoreSkillSources` on `ProjectConfig`
- `internal/config/defaults.go` — Default config values; needs defaults for new fields
- `internal/engine/registry.go` — Skill loading and resolution; needs ignore filtering in `loadFromDir()` and `loadFromFS()`
- `internal/engine/skill.go` — Priority constants; used to map priorities to labels for source-level ignoring
- `internal/engine/registry_test.go` — Tests; needs new test cases for ignore behavior
- `internal/engine/skill_test.go` — Skill parsing tests
- `agtop.example.toml` — Example config; needs documentation of new options

### New Files

None — all changes are to existing files.

## Implementation Plan

### Phase 1: Config Changes

Add the `Ignore` field to `SkillConfig` and `IgnoreSkillSources` to `ProjectConfig`. Define source label constants and a helper to map priority levels to labels.

### Phase 2: Registry Filtering

Add a `shouldIgnore` method to `Registry` that checks both per-skill and source-level ignore rules. Integrate it into `loadFromDir()` and `loadFromFS()` so ignored skills are never inserted into the registry map.

### Phase 3: Documentation & Example Config

Update `agtop.example.toml` with commented examples of both ignore options.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `Ignore` field to `SkillConfig`

- In `internal/config/config.go`, add `Ignore bool` field to the `SkillConfig` struct with TOML tag `"ignore"`

```go
type SkillConfig struct {
	Model        string   `toml:"model"`
	Timeout      int      `toml:"timeout"`
	Parallel     bool     `toml:"parallel"`
	AllowedTools []string `toml:"allowed_tools"`
	Ignore       bool     `toml:"ignore"`
}
```

### 2. Add `IgnoreSkillSources` field to `ProjectConfig`

- In `internal/config/config.go`, add `IgnoreSkillSources []string` field to `ProjectConfig` with TOML tag `"ignore_skill_sources"`

```go
type ProjectConfig struct {
	Name               string          `toml:"name"`
	Root               string          `toml:"root"`
	TestCommand        string          `toml:"test_command"`
	DevServer          DevServerConfig `toml:"dev_server"`
	IgnoreSkillSources []string        `toml:"ignore_skill_sources"`
}
```

### 3. Add priority-to-label mapping in `skill.go`

- In `internal/engine/skill.go`, add a `PriorityLabel` function that maps priority constants to human-readable labels:

```go
func PriorityLabel(priority int) string {
	switch priority {
	case PriorityProjectAgtop:
		return "project-agtop"
	case PriorityProjectClaude:
		return "project-claude"
	case PriorityProjectOpenCode:
		return "project-opencode"
	case PriorityProjectAgents:
		return "project-agents"
	case PriorityUserAgtop:
		return "user-agtop"
	case PriorityUserClaude:
		return "user-claude"
	case PriorityUserOpenCode:
		return "user-opencode"
	case PriorityBuiltIn:
		return "builtin"
	default:
		return ""
	}
}
```

### 4. Add `shouldIgnore` method to `Registry`

- In `internal/engine/registry.go`, add a `shouldIgnore(name string, priority int) bool` method that:
  1. Checks if the skill name has `Ignore: true` in `cfg.Skills[name]` AND the priority is NOT `PriorityProjectAgtop`, `PriorityUserAgtop`, or `PriorityBuiltIn` → return true
  2. Checks if `PriorityLabel(priority)` matches any entry in `cfg.Project.IgnoreSkillSources` → return true
  3. Otherwise returns false

```go
func (r *Registry) shouldIgnore(name string, priority int) bool {
	// Per-skill ignore: skip external sources for this skill name
	if sc, ok := r.cfg.Skills[name]; ok && sc.Ignore {
		switch priority {
		case PriorityProjectAgtop, PriorityUserAgtop, PriorityBuiltIn:
			// Always allow agtop and built-in sources
		default:
			return true
		}
	}

	// Source-level ignore: skip all skills from this source category
	label := PriorityLabel(priority)
	for _, src := range r.cfg.Project.IgnoreSkillSources {
		if src == label {
			return true
		}
	}

	return false
}
```

### 5. Integrate filtering into `loadFromDir`

- In `internal/engine/registry.go`, modify `loadFromDir()` to call `shouldIgnore` after parsing each skill and before inserting into the map:

```go
func (r *Registry) loadFromDir(dir string, priority int) {
	pattern := filepath.Join(dir, "*", "SKILL.md")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, path := range matches {
		skill, err := ParseSkillFile(path, priority)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping %s: %v\n", path, err)
			continue
		}
		if r.shouldIgnore(skill.Name, priority) {
			continue
		}
		r.skills[skill.Name] = skill
	}
}
```

### 6. Integrate filtering into `loadFromFS`

- In `internal/engine/registry.go`, modify `loadFromFS()` to call `shouldIgnore` after parsing each embedded skill:

```go
func (r *Registry) loadFromFS(fsys fs.FS, priority int) {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		path := entry.Name() + "/SKILL.md"
		data, err := fs.ReadFile(fsys, path)
		if err != nil {
			continue
		}
		skill, err := ParseSkill(data, "builtin://"+entry.Name()+"/SKILL.md", priority)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: skipping embedded skill %s: %v\n", entry.Name(), err)
			continue
		}
		if r.shouldIgnore(skill.Name, priority) {
			continue
		}
		r.skills[skill.Name] = skill
	}
}
```

### 7. Add tests for per-skill ignore

- In `internal/engine/registry_test.go`, add `TestRegistryIgnoreSkill`:
  - Create skills in both `.claude/skills/` (priority 1) and embedded/built-in source
  - Set `cfg.Skills["build"] = SkillConfig{Ignore: true}`
  - Verify the `.claude/skills/build` is skipped and the built-in version is loaded
  - Verify non-ignored skills from `.claude/skills/` still load normally

### 8. Add tests for source-level ignore

- In `internal/engine/registry_test.go`, add `TestRegistryIgnoreSource`:
  - Create skills in `.claude/skills/` and `.agtop/skills/`
  - Set `cfg.Project.IgnoreSkillSources = []string{"project-claude"}`
  - Verify ALL `.claude/skills/` skills are skipped
  - Verify `.agtop/skills/` skills still load normally

### 9. Add test for ignore with no fallback

- In `internal/engine/registry_test.go`, add `TestRegistryIgnoreSkillNoFallback`:
  - Create a skill ONLY in `.claude/skills/` (no built-in equivalent)
  - Set `cfg.Skills["custom"] = SkillConfig{Ignore: true}`
  - Verify the skill is NOT in the registry at all

### 10. Add test for `PriorityLabel`

- In `internal/engine/skill_test.go`, add `TestPriorityLabel`:
  - Verify each priority constant maps to the expected label string
  - Verify unknown priority values return `""`

### 11. Update `agtop.example.toml`

- Add commented examples showing both ignore patterns:

```toml
[project]
# ignore_skill_sources = ["project-claude"]  # Ignore all skills from .claude/skills/

[skills.commit]
# ignore = true   # Skip external versions; use agtop built-in instead
```

## Testing Strategy

### Unit Tests

- `TestRegistryIgnoreSkill` — per-skill ignore filters `.claude/skills/` but allows built-in
- `TestRegistryIgnoreSource` — source-level ignore filters entire source category
- `TestRegistryIgnoreSkillNoFallback` — ignored skill with no fallback results in missing skill
- `TestRegistryIgnoreSkillAgtopSourceNotFiltered` — `ignore = true` does NOT filter `.agtop/skills/` versions
- `TestPriorityLabel` — all priority constants map to correct labels

### Edge Cases

- Skill ignored from all sources (no agtop/built-in fallback) — skill simply absent from registry
- `ignore = true` on a skill that only exists in `.agtop/skills/` — ignored flag has no effect, skill loads normally
- `ignore_skill_sources` contains `"project-agtop"` — user intentionally ignoring their own agtop skills (valid, not blocked)
- `ignore_skill_sources` contains `"builtin"` — user can ignore built-in skills (valid, allows custom-only setups)
- Both per-skill and source-level ignore active simultaneously — either match causes skip
- Empty `ignore_skill_sources` — no effect (default)
- Skill with `ignore = true` still has other config (model, timeout) — config applies to the version that IS loaded

## Risk Assessment

- **Low risk**: Changes are additive — new bool field defaults to `false`, new slice defaults to `nil`. Existing behavior unchanged.
- **No migration needed**: Zero-value of new fields preserves current behavior.
- **Registry loading order unchanged**: The priority/override system works exactly as before; `shouldIgnore` only prevents insertion.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go vet ./...
go test ./internal/engine/ -v -run "TestRegistry|TestPriorityLabel"
go test ./internal/config/ -v
go build ./cmd/agtop/
```

## Open Questions (Unresolved)

- **Should `ignore = true` also prevent the skill from appearing in workflow validation warnings?** Recommendation: No — if a workflow references an ignored skill with no fallback, `ValidateWorkflow` should still report it as missing. This is correct behavior since the skill genuinely isn't available.

## Sub-Tasks

Single task — no decomposition needed.
