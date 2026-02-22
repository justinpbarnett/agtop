package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
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

// discoverConfigPath searches the discovery chain and returns the first config
// file that exists. Returns empty string if none found (defaults-only mode).
func discoverConfigPath(dir string) (string, error) {
	// 1. ./agtop.yaml (relative to project dir)
	local := filepath.Join(dir, "agtop.yaml")
	if _, err := os.Stat(local); err == nil {
		return local, nil
	}

	// 2. ~/.config/agtop/config.yaml
	home, err := os.UserHomeDir()
	if err != nil {
		return "", nil // can't resolve home, skip
	}
	user := filepath.Join(home, ".config", "agtop", "config.yaml")
	if _, err := os.Stat(user); err == nil {
		return user, nil
	}

	return "", nil
}

// loadFromFile reads and unmarshals a YAML config file.
func loadFromFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	return &cfg, nil
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
