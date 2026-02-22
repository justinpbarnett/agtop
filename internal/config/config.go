package config

type Config struct {
	Project   ProjectConfig              `yaml:"project"`
	Runtime   RuntimeConfig              `yaml:"runtime"`
	Workflows map[string]WorkflowConfig  `yaml:"workflows"`
	Skills    map[string]SkillConfig     `yaml:"skills"`
	Safety    SafetyConfig               `yaml:"safety"`
	Limits    LimitsConfig               `yaml:"limits"`
	UI        UIConfig                   `yaml:"ui"`
}

type ProjectConfig struct {
	Name        string          `yaml:"name"`
	Root        string          `yaml:"root"`
	TestCommand string          `yaml:"test_command"`
	DevServer   DevServerConfig `yaml:"dev_server"`
}

type DevServerConfig struct {
	Command      string `yaml:"command"`
	PortStrategy string `yaml:"port_strategy"`
	BasePort     int    `yaml:"base_port"`
}

type RuntimeConfig struct {
	Default  string            `yaml:"default"`
	Claude   ClaudeConfig      `yaml:"claude"`
	OpenCode OpenCodeConfig    `yaml:"opencode"`
}

type ClaudeConfig struct {
	Model          string   `yaml:"model"`
	PermissionMode string   `yaml:"permission_mode"`
	MaxTurns       int      `yaml:"max_turns"`
	AllowedTools   []string `yaml:"allowed_tools"`
}

type OpenCodeConfig struct {
	Model string `yaml:"model"`
	Agent string `yaml:"agent"`
}

type WorkflowConfig struct {
	Skills []string `yaml:"skills"`
}

type SkillConfig struct {
	Model    string `yaml:"model"`
	Timeout  int    `yaml:"timeout"`
	Parallel bool   `yaml:"parallel"`
}

type SafetyConfig struct {
	BlockedPatterns []string `yaml:"blocked_patterns"`
	AllowOverrides  bool     `yaml:"allow_overrides"`
}

type LimitsConfig struct {
	MaxTokensPerRun   int     `yaml:"max_tokens_per_run"`
	MaxCostPerRun     float64 `yaml:"max_cost_per_run"`
	MaxConcurrentRuns int     `yaml:"max_concurrent_runs"`
	RateLimitBackoff  int     `yaml:"rate_limit_backoff"`
}

type UIConfig struct {
	Theme          string `yaml:"theme"`
	ShowTokenCount bool   `yaml:"show_token_count"`
	ShowCost       bool   `yaml:"show_cost"`
	LogScrollSpeed int    `yaml:"log_scroll_speed"`
}
