package config

import (
	"strings"
	"testing"
)

func TestValidateDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if err := validate(&cfg); err != nil {
		t.Fatalf("DefaultConfig() should pass validation, got: %v", err)
	}
}

func TestValidateInvalidRuntime(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Runtime.Default = "invalid"

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid runtime")
	}
	if !strings.Contains(err.Error(), "runtime.default") {
		t.Errorf("expected error about runtime.default, got: %v", err)
	}
}

func TestValidateInvalidPermissionMode(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Runtime.Claude.PermissionMode = "yolo"

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid permission mode")
	}
	if !strings.Contains(err.Error(), "permission_mode") {
		t.Errorf("expected error about permission_mode, got: %v", err)
	}
}

func TestValidateInvalidPortStrategy(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.DevServer.PortStrategy = "random"

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for invalid port strategy")
	}
	if !strings.Contains(err.Error(), "port_strategy") {
		t.Errorf("expected error about port_strategy, got: %v", err)
	}
}

func TestValidateWorkflowMissingSkill(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workflows["broken"] = WorkflowConfig{
		Skills: []string{"build", "nonexistent"},
	}

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for missing skill reference")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected error mentioning 'nonexistent', got: %v", err)
	}
}

func TestValidateBadRegex(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Safety.BlockedPatterns = append(cfg.Safety.BlockedPatterns, "[invalid")

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for bad regex")
	}
	if !strings.Contains(err.Error(), "[invalid") {
		t.Errorf("expected error mentioning the bad pattern, got: %v", err)
	}
}

func TestValidateZeroLimits(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Limits.MaxConcurrentRuns = 0

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for zero max_concurrent_runs")
	}
	if !strings.Contains(err.Error(), "max_concurrent_runs") {
		t.Errorf("expected error about max_concurrent_runs, got: %v", err)
	}
}

func TestValidateZeroMaxTurns(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Runtime.Claude.MaxTurns = 0

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for zero max_turns")
	}
	if !strings.Contains(err.Error(), "max_turns") {
		t.Errorf("expected error about max_turns, got: %v", err)
	}
}

func TestValidateZeroBasePort(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Project.DevServer.BasePort = 0

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for zero base_port")
	}
	if !strings.Contains(err.Error(), "base_port") {
		t.Errorf("expected error about base_port, got: %v", err)
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Runtime.Default = "invalid"
	cfg.Runtime.Claude.PermissionMode = "yolo"
	cfg.Limits.MaxConcurrentRuns = 0
	cfg.Safety.BlockedPatterns = []string{"[bad"}

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation errors")
	}

	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected *ValidationError, got %T", err)
	}

	if len(ve.Errors) != 4 {
		t.Errorf("expected 4 validation errors, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

func TestValidateNegativeCost(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Limits.MaxCostPerRun = -1.0

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for negative max_cost_per_run")
	}
	if !strings.Contains(err.Error(), "max_cost_per_run") {
		t.Errorf("expected error about max_cost_per_run, got: %v", err)
	}
}

func TestValidateNegativeTokens(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Limits.MaxTokensPerRun = -100

	err := validate(&cfg)
	if err == nil {
		t.Fatal("expected validation error for negative max_tokens_per_run")
	}
	if !strings.Contains(err.Error(), "max_tokens_per_run") {
		t.Errorf("expected error about max_tokens_per_run, got: %v", err)
	}
}
