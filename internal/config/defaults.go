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
				Model: "anthropic/claude-sonnet-4-6",
				Agent: "build",
			},
		},
		Workflows: map[string]WorkflowConfig{
			"auto":       {Skills: []string{"route"}},
			"build":      {Skills: []string{"build"}},
			"plan-build": {Skills: []string{"spec", "build", "review"}},
			"sdlc":       {Skills: []string{"spec", "decompose", "build", "review", "document"}},
			"quick-fix":  {Skills: []string{}},
		},
		Skills: map[string]SkillConfig{
			"route":     {Model: "haiku", Timeout: 600, AllowedTools: []string{"Read", "Grep", "Glob"}},
			"spec":      {Model: "opus", AllowedTools: []string{"Read", "Write", "Grep", "Glob"}},
			"decompose": {Model: "opus", AllowedTools: []string{"Read", "Grep", "Glob"}},
			"build":     {Model: "sonnet", Timeout: 3600, Parallel: true},
			"test":      {Model: "sonnet", Timeout: 1800},
			"review":    {Model: "opus", AllowedTools: []string{"Read", "Grep", "Glob"}},
			"document":  {Model: "haiku", AllowedTools: []string{"Read", "Write", "Grep", "Glob"}},
			"commit":    {Model: "haiku", Timeout: 1200},
			"pr":        {Model: "haiku", Timeout: 600},
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
			MaxCostPerRun:       50.00,
			MaxConcurrentRuns:   5,
			RateLimitBackoff:    60,
			RateLimitMaxRetries: 3,
		},
		Merge: MergeConfig{
			ConflictResolutionAttempts: 3,
		},
		UI: UIConfig{
			Theme:          "default",
			ShowTokenCount: boolPtr(true),
			ShowCost:       boolPtr(true),
			LogScrollSpeed: 5,
		},
		Update: UpdateConfig{
			AutoCheck: true,
			Repo:      "justinpbarnett/agtop",
		},
	}
}
