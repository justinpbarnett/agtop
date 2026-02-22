package config

func boolPtr(b bool) *bool { return &b }

func DefaultConfig() Config {
	return Config{
		Project: ProjectConfig{
			Root: ".",
			DevServer: DevServerConfig{
				PortStrategy: "hash",
				BasePort:     3100,
			},
		},
		Runtime: RuntimeConfig{
			Default: "claude",
			Claude: ClaudeConfig{
				Model:          "sonnet",
				PermissionMode: "acceptEdits",
				MaxTurns:       50,
				AllowedTools:   []string{"Read", "Write", "Edit", "MultiEdit", "Bash", "Grep", "Glob"},
			},
			OpenCode: OpenCodeConfig{
				Model: "anthropic/claude-sonnet-4-5",
				Agent: "build",
			},
		},
		Workflows: map[string]WorkflowConfig{
			"build":      {Skills: []string{"route", "build", "test"}},
			"plan-build": {Skills: []string{"route", "spec", "build", "test"}},
			"sdlc":       {Skills: []string{"route", "spec", "decompose", "build", "test", "review", "document"}},
			"quick-fix":  {Skills: []string{"build", "test", "commit"}},
		},
		Skills: map[string]SkillConfig{
			"route":     {Model: "haiku", Timeout: 60},
			"spec":      {Model: "opus"},
			"decompose": {Model: "opus"},
			"build":     {Model: "sonnet", Timeout: 300, Parallel: true},
			"test":      {Model: "sonnet", Timeout: 120},
			"review":    {Model: "opus"},
			"document":  {Model: "haiku"},
			"commit":    {Model: "haiku", Timeout: 30},
			"pr":        {Model: "haiku", Timeout: 30},
		},
		Safety: SafetyConfig{
			BlockedPatterns: []string{
				`rm\s+-[rf]+\s+/`,
				`git\s+push.*--force`,
				`DROP\s+TABLE`,
				`(curl|wget).*\|\s*(sh|bash)`,
				`chmod\s+777`,
				`:(){.*};`,
			},
			AllowOverrides: boolPtr(false),
		},
		Limits: LimitsConfig{
			MaxTokensPerRun:   500000,
			MaxCostPerRun:     5.00,
			MaxConcurrentRuns: 5,
			RateLimitBackoff:  60,
		},
		UI: UIConfig{
			Theme:          "default",
			ShowTokenCount: boolPtr(true),
			ShowCost:       boolPtr(true),
			LogScrollSpeed: 5,
		},
	}
}
