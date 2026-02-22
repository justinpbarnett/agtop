package runtime

import (
	"fmt"
	"log"

	"github.com/justinpbarnett/agtop/internal/config"
)

// RuntimeName identifies which runtime implementation is active.
const (
	RuntimeClaude   = "claude"
	RuntimeOpenCode = "opencode"
)

// NewRuntime creates a Runtime based on the configured default. If the preferred
// runtime binary is missing, it falls back to the other. Returns an error only
// if neither runtime is available.
func NewRuntime(cfg *config.RuntimeConfig) (Runtime, string, error) {
	switch cfg.Default {
	case RuntimeOpenCode:
		rt, err := NewOpenCodeRuntime()
		if err == nil {
			return rt, RuntimeOpenCode, nil
		}
		log.Printf("warning: %v — falling back to claude", err)
		rt2, err2 := NewClaudeRuntime()
		if err2 != nil {
			return nil, "", fmt.Errorf("no runtime available: opencode (%v), claude (%v)", err, err2)
		}
		return rt2, RuntimeClaude, nil

	default: // "claude" or unset
		rt, err := NewClaudeRuntime()
		if err == nil {
			return rt, RuntimeClaude, nil
		}
		log.Printf("warning: %v — falling back to opencode", err)
		rt2, err2 := NewOpenCodeRuntime()
		if err2 != nil {
			return nil, "", fmt.Errorf("no runtime available: claude (%v), opencode (%v)", err, err2)
		}
		return rt2, RuntimeOpenCode, nil
	}
}
