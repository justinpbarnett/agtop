package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/BurntSushi/toml"
)

// Load discovers a config file, merges it with defaults, applies environment
// variable overrides, validates the result, and returns the final config.
func Load() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("getting working directory: %w", err)
	}
	return LoadFrom(cwd)
}

// LoadFrom loads config using the given directory as the project root for file
// discovery. This is the testable entry point — Load() calls it with os.Getwd().
func LoadFrom(dir string) (*Config, error) {
	cfg := DefaultConfig()

	path, err := discoverConfigPath(dir)
	if err != nil {
		return nil, fmt.Errorf("config discovery: %w", err)
	}

	if path != "" {
		override, err := loadFromFile(path)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", path, err)
		}
		merge(&cfg, override)
	}

	applyEnvOverrides(&cfg)

	if err := validate(&cfg); err != nil {
		return nil, fmt.Errorf("config validation: %w", err)
	}

	return &cfg, nil
}

// LocalConfigExists reports whether an agtop.toml file exists in the current
// working directory. This checks only the local project config — a user-level
// config at ~/.config/agtop/config.toml does not count.
func LocalConfigExists() bool {
	cwd, err := os.Getwd()
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(cwd, "agtop.toml"))
	return err == nil
}

// discoverConfigPath searches the discovery chain and returns the first config
// file that exists. Returns empty string if none found (defaults-only mode).
func discoverConfigPath(dir string) (string, error) {
	// 1. ./agtop.toml (relative to project dir)
	local := filepath.Join(dir, "agtop.toml")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// 2. ~/.config/agtop/config.toml
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil // can't resolve home, skip
	}
	user := filepath.Join(home, ".config", "agtop", "config.toml")
	if _, err := os.Stat(user); err == nil {
		return user, nil
	}

	return "", nil
}

// loadFromFile reads and unmarshals a TOML config file.
// It pre-processes the raw TOML to normalize shorthand workflow syntax
// (e.g. quick = ["build"]) into the table form the Config struct expects
// (e.g. [workflows.quick] skills = ["build"]).
func loadFromFile(path string) (*Config, error) {
	var raw map[string]interface{}
	if _, err := toml.DecodeFile(path, &raw); err != nil {
		return nil, fmt.Errorf("parsing TOML: %w", err)
	}

	normalizeWorkflows(raw)

	var cfg Config
	if err := mapToConfig(raw, &cfg); err != nil {
		return nil, fmt.Errorf("decoding config: %w", err)
	}

	return &cfg, nil
}

// normalizeWorkflows converts shorthand workflow arrays into table form.
// e.g. workflows.quick = ["build"] -> workflows.quick = {skills: ["build"]}
func normalizeWorkflows(raw map[string]interface{}) {
	wf, ok := raw["workflows"]
	if !ok {
		return
	}
	wfMap, ok := wf.(map[string]interface{})
	if !ok {
		return
	}
	for k, v := range wfMap {
		if arr, ok := v.([]interface{}); ok {
			skills := make([]interface{}, len(arr))
			copy(skills, arr)
			wfMap[k] = map[string]interface{}{"skills": skills}
		}
	}
}

// mapToConfig re-encodes a raw map to TOML and decodes it into Config.
func mapToConfig(raw map[string]interface{}, cfg *Config) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(raw); err != nil {
		return err
	}
	if _, err := toml.NewDecoder(&buf).Decode(cfg); err != nil {
		return err
	}
	return nil
}

// merge deep-merges override onto base. Scalar fields override when non-zero.
// Maps merge at the key level. Slices replace entirely when non-nil.
// Pointer-to-bool fields override when non-nil.
func merge(base *Config, override *Config) {
	// Project
	if override.Project.Name != "" {
		base.Project.Name = override.Project.Name
	}
	if override.Project.Root != "" {
		base.Project.Root = override.Project.Root
	}
	if override.Project.Repos != nil {
		base.Project.Repos = override.Project.Repos
	}
	if override.Project.TestCommand != "" {
		base.Project.TestCommand = override.Project.TestCommand
	}
	if override.Project.DevServer.Command != "" {
		base.Project.DevServer.Command = override.Project.DevServer.Command
	}
	if override.Project.DevServer.PortStrategy != "" {
		base.Project.DevServer.PortStrategy = override.Project.DevServer.PortStrategy
	}
	if override.Project.DevServer.BasePort != 0 {
		base.Project.DevServer.BasePort = override.Project.DevServer.BasePort
	}

	// Runtime
	if override.Runtime.Default != "" {
		base.Runtime.Default = override.Runtime.Default
	}
	if override.Runtime.Claude.Model != "" {
		base.Runtime.Claude.Model = override.Runtime.Claude.Model
	}
	if override.Runtime.Claude.PermissionMode != "" {
		base.Runtime.Claude.PermissionMode = override.Runtime.Claude.PermissionMode
	}
	if override.Runtime.Claude.MaxTurns != 0 {
		base.Runtime.Claude.MaxTurns = override.Runtime.Claude.MaxTurns
	}
	if override.Runtime.Claude.AllowedTools != nil {
		base.Runtime.Claude.AllowedTools = override.Runtime.Claude.AllowedTools
	}
	if override.Runtime.Claude.Subscription {
		base.Runtime.Claude.Subscription = true
	}
	if override.Runtime.OpenCode.Model != "" {
		base.Runtime.OpenCode.Model = override.Runtime.OpenCode.Model
	}
	if override.Runtime.OpenCode.Agent != "" {
		base.Runtime.OpenCode.Agent = override.Runtime.OpenCode.Agent
	}

	// Workflows — merge at key level
	if override.Workflows != nil {
		if base.Workflows == nil {
			base.Workflows = make(map[string]WorkflowConfig)
		}
		for k, v := range override.Workflows {
			base.Workflows[k] = v
		}
	}

	// Skills — merge at key level
	if override.Skills != nil {
		if base.Skills == nil {
			base.Skills = make(map[string]SkillConfig)
		}
		for k, v := range override.Skills {
			base.Skills[k] = v
		}
	}

	// Safety — slices replace entirely, *bool overrides when non-nil
	if override.Safety.BlockedPatterns != nil {
		base.Safety.BlockedPatterns = override.Safety.BlockedPatterns
	}
	if override.Safety.AllowOverrides != nil {
		base.Safety.AllowOverrides = override.Safety.AllowOverrides
	}

	// Limits
	if override.Limits.MaxTokensPerRun != 0 {
		base.Limits.MaxTokensPerRun = override.Limits.MaxTokensPerRun
	}
	if override.Limits.MaxCostPerRun != 0 {
		base.Limits.MaxCostPerRun = override.Limits.MaxCostPerRun
	}
	if override.Limits.MaxConcurrentRuns != 0 {
		base.Limits.MaxConcurrentRuns = override.Limits.MaxConcurrentRuns
	}
	if override.Limits.RateLimitBackoff != 0 {
		base.Limits.RateLimitBackoff = override.Limits.RateLimitBackoff
	}

	// Merge
	if override.Merge.TargetBranch != "" {
		base.Merge.TargetBranch = override.Merge.TargetBranch
	}
	if override.Merge.AutoMerge {
		base.Merge.AutoMerge = override.Merge.AutoMerge
	}
	if override.Merge.MergeStrategy != "" {
		base.Merge.MergeStrategy = override.Merge.MergeStrategy
	}
	if override.Merge.FixAttempts != 0 {
		base.Merge.FixAttempts = override.Merge.FixAttempts
	}
	if override.Merge.PollInterval != 0 {
		base.Merge.PollInterval = override.Merge.PollInterval
	}
	if override.Merge.PollTimeout != 0 {
		base.Merge.PollTimeout = override.Merge.PollTimeout
	}

	// Integrations — pointer overrides when non-nil
	if override.Integrations.Jira != nil {
		base.Integrations.Jira = override.Integrations.Jira
	}

	// UI — *bool overrides when non-nil
	if override.UI.Theme != "" {
		base.UI.Theme = override.UI.Theme
	}
	if override.UI.LogScrollSpeed != 0 {
		base.UI.LogScrollSpeed = override.UI.LogScrollSpeed
	}
	if override.UI.ShowTokenCount != nil {
		base.UI.ShowTokenCount = override.UI.ShowTokenCount
	}
	if override.UI.ShowCost != nil {
		base.UI.ShowCost = override.UI.ShowCost
	}
}

// applyEnvOverrides applies AGTOP_* environment variables on top of the config.
func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("AGTOP_RUNTIME"); v != "" {
		cfg.Runtime.Default = v
	}
	if v := os.Getenv("AGTOP_MODEL"); v != "" {
		cfg.Runtime.Claude.Model = v
	}
	if v := os.Getenv("AGTOP_PERMISSION_MODE"); v != "" {
		cfg.Runtime.Claude.PermissionMode = v
	}
	if v := os.Getenv("AGTOP_MAX_COST"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Limits.MaxCostPerRun = f
		} else {
			fmt.Fprintf(os.Stderr, "warning: AGTOP_MAX_COST=%q is not a valid number, ignoring\n", v)
		}
	}
	if v := os.Getenv("AGTOP_MAX_TOKENS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Limits.MaxTokensPerRun = n
		} else {
			fmt.Fprintf(os.Stderr, "warning: AGTOP_MAX_TOKENS=%q is not a valid integer, ignoring\n", v)
		}
	}
	if v := os.Getenv("AGTOP_MAX_CONCURRENT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Limits.MaxConcurrentRuns = n
		} else {
			fmt.Fprintf(os.Stderr, "warning: AGTOP_MAX_CONCURRENT=%q is not a valid integer, ignoring\n", v)
		}
	}
}
