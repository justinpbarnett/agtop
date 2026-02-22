package config

import (
	"fmt"
	"regexp"
	"strings"
)

// ValidationError collects multiple validation failures.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("config validation failed:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// validate checks the config for internal consistency and returns a
// ValidationError if any checks fail. All checks run — errors are collected,
// not short-circuited.
func validate(cfg *Config) error {
	var errs []string

	// Runtime must be a known value
	switch cfg.Runtime.Default {
	case "claude", "opencode":
	default:
		errs = append(errs, fmt.Sprintf("runtime.default %q must be \"claude\" or \"opencode\"", cfg.Runtime.Default))
	}

	// Permission mode must be a known value
	switch cfg.Runtime.Claude.PermissionMode {
	case "acceptEdits", "acceptAll", "manual":
	default:
		errs = append(errs, fmt.Sprintf("runtime.claude.permission_mode %q must be \"acceptEdits\", \"acceptAll\", or \"manual\"", cfg.Runtime.Claude.PermissionMode))
	}

	// Port strategy must be a known value
	switch cfg.Project.DevServer.PortStrategy {
	case "hash", "sequential", "fixed":
	default:
		errs = append(errs, fmt.Sprintf("project.dev_server.port_strategy %q must be \"hash\", \"sequential\", or \"fixed\"", cfg.Project.DevServer.PortStrategy))
	}

	// Workflow integrity: every skill referenced must exist in the skills map
	for wfName, wf := range cfg.Workflows {
		for _, skillName := range wf.Skills {
			if _, ok := cfg.Skills[skillName]; !ok {
				errs = append(errs, fmt.Sprintf("workflow %q references undefined skill %q", wfName, skillName))
			}
		}
	}

	// Positive value checks
	if cfg.Runtime.Claude.MaxTurns <= 0 {
		errs = append(errs, "runtime.claude.max_turns must be positive")
	}
	if cfg.Limits.MaxTokensPerRun <= 0 {
		errs = append(errs, "limits.max_tokens_per_run must be positive")
	}
	if cfg.Limits.MaxCostPerRun <= 0 {
		errs = append(errs, "limits.max_cost_per_run must be positive")
	}
	if cfg.Limits.MaxConcurrentRuns <= 0 {
		errs = append(errs, "limits.max_concurrent_runs must be positive")
	}
	if cfg.Project.DevServer.BasePort <= 0 {
		errs = append(errs, "project.dev_server.base_port must be positive")
	}

	// Safety patterns must be valid regex
	for i, pattern := range cfg.Safety.BlockedPatterns {
		if _, err := regexp.Compile(pattern); err != nil {
			errs = append(errs, fmt.Sprintf("safety.blocked_patterns[%d] %q is not valid regex: %v", i, pattern, err))
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}
