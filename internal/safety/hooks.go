package safety

import (
	"fmt"

	"github.com/justinpbarnett/agtop/internal/config"
)

type HookEngine struct {
	matcher *PatternMatcher
	cfg     config.SafetyConfig
}

// NewHookEngine creates a HookEngine from the safety config.
// Returns a usable engine even when some patterns fail to compile.
func NewHookEngine(cfg config.SafetyConfig) (*HookEngine, error) {
	matcher, err := NewPatternMatcher(cfg.BlockedPatterns)
	return &HookEngine{matcher: matcher, cfg: cfg}, err
}

// CheckCommand tests a command against blocked patterns and returns a
// human-readable reason when blocked.
func (h *HookEngine) CheckCommand(command string) (blocked bool, reason string) {
	blocked, pattern := h.matcher.Check(command)
	if blocked {
		return true, fmt.Sprintf("blocked by safety pattern: %s", pattern)
	}
	return false, ""
}

// GenerateGuardScript returns the full safety-guard.sh script content
// with all blocked patterns embedded.
func (h *HookEngine) GenerateGuardScript() string {
	return renderGuardScript(h.matcher.Patterns())
}

// GenerateSettings returns the Claude Code settings structure for
// PreToolUse hooks that point to the safety guard script.
func (h *HookEngine) GenerateSettings() map[string]interface{} {
	return map[string]interface{}{
		"hooks": map[string]interface{}{
			"PreToolUse": []interface{}{
				map[string]interface{}{
					"matcher": "Bash",
					"hooks": []interface{}{
						map[string]interface{}{
							"type":    "command",
							"command": ".agtop/hooks/safety-guard.sh",
						},
					},
				},
			},
		},
	}
}

// GenerateOpenCodeSettings returns the opencode.json permission structure
// that allows tools needed by agtop workflows in non-interactive mode.
func (h *HookEngine) GenerateOpenCodeSettings() map[string]interface{} {
	bash := map[string]interface{}{
		"*": "allow",
	}
	for _, p := range h.matcher.Patterns() {
		bash[p] = "deny"
	}
	return map[string]interface{}{
		"permission": map[string]interface{}{
			"read": "allow",
			"edit": "allow",
			"bash": bash,
			"glob": "allow",
			"grep": "allow",
			"list": "allow",
		},
	}
}

// Matcher returns the underlying PatternMatcher.
func (h *HookEngine) Matcher() *PatternMatcher {
	return h.matcher
}
