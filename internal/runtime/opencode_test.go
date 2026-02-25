package runtime

import (
	"reflect"
	"testing"
)

func TestOpenCodeBuildArgsMinimal(t *testing.T) {
	rt := &OpenCodeRuntime{opencodePath: "/usr/bin/opencode"}
	args := rt.BuildArgs("do something", RunOptions{})

	expected := []string{"run", "do something", "--format", "json"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestOpenCodeBuildArgsAllFlags(t *testing.T) {
	rt := &OpenCodeRuntime{opencodePath: "/usr/bin/opencode"}
	args := rt.BuildArgs("build feature", RunOptions{
		Model: "anthropic/claude-sonnet-4-5",
		Agent: "code",
	})

	expected := []string{
		"run", "build feature",
		"--format", "json",
		"--model", "anthropic/claude-sonnet-4-5",
		"--agent", "code",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestOpenCodeBuildArgsModelOnly(t *testing.T) {
	rt := &OpenCodeRuntime{opencodePath: "/usr/bin/opencode"}
	args := rt.BuildArgs("test", RunOptions{Model: "openai/gpt-4o"})

	expected := []string{"run", "test", "--format", "json", "--model", "openai/gpt-4o"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestOpenCodeBuildArgsAgentOnly(t *testing.T) {
	rt := &OpenCodeRuntime{opencodePath: "/usr/bin/opencode"}
	args := rt.BuildArgs("test", RunOptions{Agent: "build"})

	expected := []string{"run", "test", "--format", "json", "--agent", "build"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestOpenCodeBuildArgsIgnoresClaudeFields(t *testing.T) {
	rt := &OpenCodeRuntime{opencodePath: "/usr/bin/opencode"}
	args := rt.BuildArgs("test", RunOptions{
		AllowedTools:   []string{"Read", "Write"},
		MaxTurns:       50,
		PermissionMode: "acceptEdits",
		WorkDir:        "/tmp/work",
	})

	// AllowedTools, MaxTurns, PermissionMode, WorkDir should not appear in args.
	// WorkDir is set via cmd.Dir, not as a CLI flag.
	expected := []string{"run", "test", "--format", "json"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}
