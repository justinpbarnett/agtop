package runtime

import (
	"testing"

	"github.com/justinpbarnett/agtop/internal/config"
)

func TestNewRuntimeDefaultClaude(t *testing.T) {
	cfg := &config.RuntimeConfig{Default: "claude"}

	rt, name, err := NewRuntime(cfg)
	// On CI or machines without claude, this may fall back to opencode or error.
	// We only test the logic — if no binaries are available, we accept the error.
	if err != nil {
		t.Skipf("no runtime binaries available: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	if name != RuntimeClaude && name != RuntimeOpenCode {
		t.Errorf("expected runtime name 'claude' or 'opencode', got %q", name)
	}
}

func TestNewRuntimeDefaultOpenCode(t *testing.T) {
	cfg := &config.RuntimeConfig{Default: "opencode"}

	rt, name, err := NewRuntime(cfg)
	if err != nil {
		t.Skipf("no runtime binaries available: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	if name != RuntimeOpenCode && name != RuntimeClaude {
		t.Errorf("expected runtime name 'opencode' or 'claude', got %q", name)
	}
}

func TestNewRuntimeEmptyDefault(t *testing.T) {
	cfg := &config.RuntimeConfig{Default: ""}

	rt, name, err := NewRuntime(cfg)
	if err != nil {
		t.Skipf("no runtime binaries available: %v", err)
	}
	if rt == nil {
		t.Fatal("expected non-nil runtime")
	}
	// Empty default should behave like "claude"
	if name != RuntimeClaude && name != RuntimeOpenCode {
		t.Errorf("expected runtime name 'claude' or 'opencode', got %q", name)
	}
}
