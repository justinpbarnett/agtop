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
				Model:          "opus",
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
			"build":      {Skills: []string{"build", "test"}},
			"plan-build": {Skills: []string{"spec", "build", "test"}},
			"sdlc":       {Skills: []string{"spec", "decompose", "build", "test", "review", "document"}},
			"quick-fix":  {Skills: []string{"build", "test", "commit"}},
		},
		Skills: map[string]SkillConfig{
			"route":     {Model: "haiku", Timeout: 300, AllowedTools: []string{"Read", "Grep", "Glob"}},
			"spec":      {Model: "opus", AllowedTools: []string{"Read", "Write", "Grep", "Glob"}},
			"decompose": {Model: "opus", AllowedTools: []string{"Read", "Grep", "Glob"}},
			"build":     {Model: "sonnet", Timeout: 1800, Parallel: true},
			"test":      {Model: "sonnet", Timeout: 900},
			"review":    {Model: "opus", AllowedTools: []string{"Read", "Grep", "Glob"}},
			"document":  {Model: "haiku", AllowedTools: []string{"Read", "Write", "Grep", "Glob"}},
			"commit":    {Model: "haiku", Timeout: 300},
			"pr":        {Model: "haiku", Timeout: 300},
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
			MaxTokensPerRun:     500000,
			MaxCostPerRun:       5.00,
			MaxConcurrentRuns:   5,
			RateLimitBackoff:    60,
			RateLimitMaxRetries: 3,
		},
		UI: UIConfig{
			Theme:          "default",
			ShowTokenCount: boolPtr(true),
			ShowCost:       boolPtr(true),
			LogScrollSpeed: 5,
		},
	}
}
