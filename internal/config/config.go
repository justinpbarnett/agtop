package config

type Config struct {
	Project      ProjectConfig              `toml:"project"`
	Repos        []RepoConfig               `toml:"repos"`
	Runtime      RuntimeConfig              `toml:"runtime"`
	Workflows    map[string]WorkflowConfig  `toml:"workflows"`
	Skills       map[string]SkillConfig     `toml:"skills"`
	Safety       SafetyConfig               `toml:"safety"`
	Limits       LimitsConfig               `toml:"limits"`
	Merge        MergeConfig                `toml:"merge"`
	UI           UIConfig                   `toml:"ui"`
	Update       UpdateConfig               `toml:"update"`
	Integrations IntegrationsConfig         `toml:"integrations"`
}

type RepoConfig struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type IntegrationsConfig struct {
	Jira *JiraConfig `toml:"jira,omitempty"`
}

type JiraConfig struct {
	BaseURL    string `toml:"base_url"`
	ProjectKey string `toml:"project_key"`
	AuthEnv    string `toml:"auth_env"`
	UserEnv    string `toml:"user_env"`
}

type ProjectConfig struct {
	Name               string          `toml:"name"`
	Root               string          `toml:"root"`
	WorktreePath       string          `toml:"worktree_path"`
	TestCommand        string          `toml:"test_command"`
	DevServer          DevServerConfig `toml:"dev_server"`
	IgnoreSkillSources []string        `toml:"ignore_skill_sources"`
}

type DevServerConfig struct {
	Command      string `toml:"command"`
	PortStrategy string `toml:"port_strategy"`
	BasePort     int    `toml:"base_port"`
}

type RuntimeConfig struct {
	Default  string            `toml:"default"`
	Claude   ClaudeConfig      `toml:"claude"`
	OpenCode OpenCodeConfig    `toml:"opencode"`
}

type ClaudeConfig struct {
	Model          string   `toml:"model"`
	PermissionMode string   `toml:"permission_mode"`
	MaxTurns       int      `toml:"max_turns"`
	AllowedTools   []string `toml:"allowed_tools"`
	Subscription   bool     `toml:"subscription"`
}

type OpenCodeConfig struct {
	Model string `toml:"model"`
	Agent string `toml:"agent"`
}

type WorkflowConfig struct {
	Skills []string `toml:"skills"`
}

type SkillConfig struct {
	Model        string   `toml:"model"`
	Timeout      int      `toml:"timeout"`
	Parallel     bool     `toml:"parallel"`
	AllowedTools []string `toml:"allowed_tools"`
	Ignore       bool     `toml:"ignore"`
}

type SafetyConfig struct {
	BlockedPatterns []string `toml:"blocked_patterns"`
	AllowOverrides  *bool    `toml:"allow_overrides"`
}

type LimitsConfig struct {
	MaxTokensPerRun     int     `toml:"max_tokens_per_run"`
	MaxCostPerRun       float64 `toml:"max_cost_per_run"`
	MaxConcurrentRuns   int     `toml:"max_concurrent_runs"`
	RateLimitBackoff    int     `toml:"rate_limit_backoff"`
	RateLimitMaxRetries int     `toml:"rate_limit_max_retries"`
}

type MergeConfig struct {
	TargetBranch        string `toml:"target_branch"`
	AutoMerge           bool   `toml:"auto_merge"`
	MergeStrategy       string `toml:"merge_strategy"`
	FixAttempts         int    `toml:"fix_attempts"`
	PollInterval        int    `toml:"poll_interval"`
	PollTimeout         int    `toml:"poll_timeout"`
	GoldenUpdateCommand string `toml:"golden_update_command"`
}

type UIConfig struct {
	Theme          string `toml:"theme"`
	ShowTokenCount *bool  `toml:"show_token_count"`
	ShowCost       *bool  `toml:"show_cost"`
	LogScrollSpeed int    `toml:"log_scroll_speed"`
}

type UpdateConfig struct {
	AutoCheck bool   `toml:"auto_check"`
	Repo      string `toml:"repo"`
}
