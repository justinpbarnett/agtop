package runtime

import (
	"reflect"
	"testing"
)

func TestBuildArgsMinimal(t *testing.T) {
	rt := &ClaudeRuntime{claudePath: "/usr/bin/claude"}
	args := rt.BuildArgs("do something", RunOptions{})

	expected := []string{"-p", "do something", "--output-format", "stream-json", "--verbose"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestBuildArgsAllFlags(t *testing.T) {
	rt := &ClaudeRuntime{claudePath: "/usr/bin/claude"}
	args := rt.BuildArgs("build feature", RunOptions{
		Model:          "sonnet",
		MaxTurns:       10,
		AllowedTools:   []string{"Read", "Write", "Bash"},
		PermissionMode: "plan",
		WorkDir:        "/home/user/project",
	})

	expected := []string{
		"-p", "build feature",
		"--output-format", "stream-json",
		"--verbose",
		"--model", "sonnet",
		"--max-turns", "10",
		"--allowedTools", "Read,Write,Bash",
		"--permission-mode", "plan",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestBuildArgsModelOnly(t *testing.T) {
	rt := &ClaudeRuntime{claudePath: "/usr/bin/claude"}
	args := rt.BuildArgs("test", RunOptions{Model: "opus"})

	expected := []string{"-p", "test", "--output-format", "stream-json", "--verbose", "--model", "opus"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestBuildArgsMaxTurnsZero(t *testing.T) {
	rt := &ClaudeRuntime{claudePath: "/usr/bin/claude"}
	args := rt.BuildArgs("test", RunOptions{MaxTurns: 0})

	expected := []string{"-p", "test", "--output-format", "stream-json", "--verbose"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("max turns 0 should be omitted: expected %v, got %v", expected, args)
	}
}

func TestBuildArgsSingleTool(t *testing.T) {
	rt := &ClaudeRuntime{claudePath: "/usr/bin/claude"}
	args := rt.BuildArgs("test", RunOptions{AllowedTools: []string{"Read"}})

	expected := []string{"-p", "test", "--output-format", "stream-json", "--verbose", "--allowedTools", "Read"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}

func TestBuildArgsWorkDirOnly(t *testing.T) {
	rt := &ClaudeRuntime{claudePath: "/usr/bin/claude"}
	args := rt.BuildArgs("test", RunOptions{WorkDir: "/tmp/work"})

	// WorkDir is set via cmd.Dir, not as a CLI flag
	expected := []string{"-p", "test", "--output-format", "stream-json", "--verbose"}
	if !reflect.DeepEqual(args, expected) {
		t.Errorf("expected %v, got %v", expected, args)
	}
}
