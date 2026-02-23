package config

import (
	"os"
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

func TestValidateJiraConfigValid(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Integrations.Jira = &JiraConfig{
		BaseURL:    "https://company.atlassian.net",
		ProjectKey: "PROJ",
		AuthEnv:    "JIRA_TOKEN",
		UserEnv:    "JIRA_EMAIL",
	}

	if err := validate(&cfg); err != nil {
		t.Fatalf("expected valid JIRA config to pass, got: %v", err)
	}
}

func TestValidateJiraNilPassesValidation(t *testing.T) {
	cfg := DefaultConfig()
	// Jira is nil by default — should pass
	if err := validate(&cfg); err != nil {
		t.Fatalf("expected nil JIRA config to pass, got: %v", err)
	}
}

func TestValidateJiraEnvVarWarning(t *testing.T) {
	// Capture stderr to verify warnings are emitted
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	cfg := DefaultConfig()
	cfg.Integrations.Jira = &JiraConfig{
		BaseURL:    "https://company.atlassian.net",
		ProjectKey: "PROJ",
		AuthEnv:    "AGTOP_TEST_JIRA_AUTH_NONEXISTENT",
		UserEnv:    "AGTOP_TEST_JIRA_USER_NONEXISTENT",
	}

	err := validate(&cfg)
	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("expected valid config to pass, got: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])

	if !strings.Contains(stderr, "AGTOP_TEST_JIRA_AUTH_NONEXISTENT") {
		t.Errorf("expected stderr warning about auth env var, got: %q", stderr)
	}
	if !strings.Contains(stderr, "AGTOP_TEST_JIRA_USER_NONEXISTENT") {
		t.Errorf("expected stderr warning about user env var, got: %q", stderr)
	}
}

func TestValidateJiraRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *JiraConfig
		wantErr string
	}{
		{
			name:    "missing base_url",
			cfg:     &JiraConfig{ProjectKey: "P", AuthEnv: "A", UserEnv: "U"},
			wantErr: "base_url",
		},
		{
			name:    "missing project_key",
			cfg:     &JiraConfig{BaseURL: "https://x", AuthEnv: "A", UserEnv: "U"},
			wantErr: "project_key",
		},
		{
			name:    "missing auth_env",
			cfg:     &JiraConfig{BaseURL: "https://x", ProjectKey: "P", UserEnv: "U"},
			wantErr: "auth_env",
		},
		{
			name:    "missing user_env",
			cfg:     &JiraConfig{BaseURL: "https://x", ProjectKey: "P", AuthEnv: "A"},
			wantErr: "user_env",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Integrations.Jira = tt.cfg

			err := validate(&cfg)
			if err == nil {
				t.Fatal("expected validation error")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("expected error about %q, got: %v", tt.wantErr, err)
			}
		})
	}
}
