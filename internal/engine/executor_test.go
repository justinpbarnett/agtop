package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/justinpbarnett/agtop/internal/config"
)

func executorTestConfig() *config.Config {
	return &config.Config{
		Workflows: map[string]config.WorkflowConfig{
			"build":      {Skills: []string{"build", "test"}},
			"plan-build": {Skills: []string{"spec", "build", "test"}},
			"sdlc":       {Skills: []string{"spec", "decompose", "build", "test", "review", "document"}},
			"quick-fix":  {Skills: []string{"build", "test", "commit"}},
			"empty":      {Skills: []string{}},
		},
		Skills: map[string]config.SkillConfig{
			"route":     {Model: "haiku", Timeout: 300},
			"spec":      {Model: "opus"},
			"decompose": {Model: "opus"},
			"build":     {Model: "sonnet", Timeout: 1800},
			"test":      {Model: "sonnet", Timeout: 900},
			"review":    {Model: "opus"},
			"document":  {Model: "haiku"},
			"commit":    {Model: "haiku", Timeout: 300},
		},
	}
}

// --- ResolveWorkflow tests ---

func TestResolveWorkflow(t *testing.T) {
	cfg := executorTestConfig()

	skills, err := ResolveWorkflow(cfg, "build")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 2 {
		t.Fatalf("expected 2 skills, got %d", len(skills))
	}
	if skills[0] != "build" || skills[1] != "test" {
		t.Errorf("expected [build, test], got %v", skills)
	}
}

func TestResolveWorkflowUnknown(t *testing.T) {
	cfg := executorTestConfig()

	_, err := ResolveWorkflow(cfg, "nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown workflow")
	}
}

func TestResolveWorkflowEmpty(t *testing.T) {
	cfg := executorTestConfig()

	_, err := ResolveWorkflow(cfg, "empty")
	if err == nil {
		t.Fatal("expected error for empty workflow")
	}
}

func TestResolveWorkflowSDLC(t *testing.T) {
	cfg := executorTestConfig()

	skills, err := ResolveWorkflow(cfg, "sdlc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(skills) != 6 {
		t.Fatalf("expected 6 skills, got %d", len(skills))
	}
	expected := []string{"spec", "decompose", "build", "test", "review", "document"}
	for i, s := range expected {
		if skills[i] != s {
			t.Errorf("skill[%d]: expected %q, got %q", i, s, skills[i])
		}
	}
}

// --- ValidateWorkflow tests ---

func TestValidateWorkflowAllPresent(t *testing.T) {
	cfg := executorTestConfig()
	reg := NewRegistry(cfg)
	// Manually add skills to registry for testing
	reg.skills["build"] = &Skill{Name: "build"}
	reg.skills["test"] = &Skill{Name: "test"}

	missing := ValidateWorkflow([]string{"build", "test"}, reg)
	if len(missing) != 0 {
		t.Errorf("expected no missing skills, got %v", missing)
	}
}

func TestValidateWorkflowMissing(t *testing.T) {
	cfg := executorTestConfig()
	reg := NewRegistry(cfg)
	reg.skills["build"] = &Skill{Name: "build"}

	missing := ValidateWorkflow([]string{"build", "test", "review"}, reg)
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing skills, got %d: %v", len(missing), missing)
	}
	if missing[0] != "test" || missing[1] != "review" {
		t.Errorf("expected [test, review], got %v", missing)
	}
}

// --- parseRouteResult tests ---

func TestParseRouteResultPlainText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"build", "build"},
		{"plan-build", "plan-build"},
		{"sdlc\n", "sdlc"},
		{"  quick-fix  ", "quick-fix"},
		{"quick_fix", "quick_fix"},
		// Workflow name on first line with trailing text
		{"sdlc\nsome extra text", "sdlc"},
		// Workflow name on last line (common case: model adds explanation)
		{"Based on my analysis, this is a complex task.\nplan-build", "plan-build"},
		{"This task requires multi-file changes.\n\nbuild", "build"},
		// Multi-line triage assessment with workflow name at the end
		{"The task involves fixing auto-resume.\nRecommended approach: plan with spec first.\n\nplan-build\n", "plan-build"},
		// Workflow name buried in the middle
		{"Some preamble text.\nsdlc\nSome trailing explanation.", "sdlc"},
		// Wrapped in backticks (common LLM output)
		{"`build`", "build"},
		{"`plan-build`\n", "plan-build"},
		// Wrapped in quotes
		{`"sdlc"`, "sdlc"},
		{"'quick-fix'", "quick-fix"},
		// Trailing punctuation
		{"build.", "build"},
		{"plan-build,", "plan-build"},
		// Backticks on the last line after explanation
		{"Based on analysis:\n`build`", "build"},
	}

	for _, tt := range tests {
		got := parseRouteResult(tt.input)
		if got != tt.expected {
			t.Errorf("parseRouteResult(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseRouteResultJSON(t *testing.T) {
	got := parseRouteResult(`{"workflow": "plan-build"}`)
	if got != "plan-build" {
		t.Errorf("expected plan-build, got %q", got)
	}
}

func TestParseRouteResultEmpty(t *testing.T) {
	got := parseRouteResult("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestParseRouteResultInvalidChars(t *testing.T) {
	got := parseRouteResult("I think you should use the build workflow")
	if got != "" {
		t.Errorf("expected empty for sentence input, got %q", got)
	}
}

func TestParseRouteResultAllSentences(t *testing.T) {
	// No valid workflow name on any line
	got := parseRouteResult("I recommend the build workflow.\nIt seems like a good fit.")
	if got != "" {
		t.Errorf("expected empty when all lines are sentences, got %q", got)
	}
}

func TestIsWorkflowName(t *testing.T) {
	valid := []string{"build", "plan-build", "sdlc", "quick_fix", "my-workflow-v2"}
	for _, name := range valid {
		if !isWorkflowName(name) {
			t.Errorf("isWorkflowName(%q) = false, want true", name)
		}
	}

	invalid := []string{"", "has spaces", "has.dots", "has:colons", "has,commas"}
	for _, name := range invalid {
		if isWorkflowName(name) {
			t.Errorf("isWorkflowName(%q) = true, want false", name)
		}
	}
}

// --- ParseDecomposeResult tests ---

func TestParseDecomposeResultFromFile(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "decompose-result.json"))
	if err != nil {
		t.Fatalf("read test fixture: %v", err)
	}

	result, err := ParseDecomposeResult(string(data))
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(result.Tasks) != 4 {
		t.Fatalf("expected 4 tasks, got %d", len(result.Tasks))
	}
	if result.Tasks[0].Name != "implement-auth-middleware" {
		t.Errorf("first task: expected implement-auth-middleware, got %q", result.Tasks[0].Name)
	}
	if result.Tasks[0].ParallelGroup != "core" {
		t.Errorf("first task group: expected core, got %q", result.Tasks[0].ParallelGroup)
	}
}

func TestParseDecomposeResultMalformed(t *testing.T) {
	_, err := ParseDecomposeResult("not json")
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseDecomposeResultEmpty(t *testing.T) {
	result, err := ParseDecomposeResult(`{"tasks": []}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(result.Tasks))
	}
}

// --- GroupByParallel tests ---

func TestGroupByParallelBasic(t *testing.T) {
	result := &DecomposeResult{
		Tasks: []DecomposeTask{
			{Name: "a", ParallelGroup: "group1", Dependencies: nil},
			{Name: "b", ParallelGroup: "group1", Dependencies: nil},
			{Name: "c", ParallelGroup: "group2", Dependencies: []string{"a"}},
		},
	}

	groups := result.GroupByParallel()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// group1 should come first (no deps)
	if len(groups[0]) != 2 {
		t.Errorf("first group should have 2 tasks, got %d", len(groups[0]))
	}
	if len(groups[1]) != 1 {
		t.Errorf("second group should have 1 task, got %d", len(groups[1]))
	}
}

func TestGroupByParallelNoDeps(t *testing.T) {
	result := &DecomposeResult{
		Tasks: []DecomposeTask{
			{Name: "a", ParallelGroup: "alpha"},
			{Name: "b", ParallelGroup: "beta"},
		},
	}

	groups := result.GroupByParallel()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
}

func TestGroupByParallelEmpty(t *testing.T) {
	result := &DecomposeResult{Tasks: nil}
	groups := result.GroupByParallel()
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestGroupByParallelSingleTask(t *testing.T) {
	result := &DecomposeResult{
		Tasks: []DecomposeTask{
			{Name: "only-one", ParallelGroup: "solo"},
		},
	}

	groups := result.GroupByParallel()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0]) != 1 {
		t.Errorf("group should have 1 task, got %d", len(groups[0]))
	}
}

// --- terminalState tests ---

func TestTerminalStateReview(t *testing.T) {
	state := terminalState([]string{"build", "test", "review"})
	if state != "reviewing" {
		t.Errorf("expected reviewing, got %s", state)
	}
}

func TestTerminalStateCompleted(t *testing.T) {
	tests := [][]string{
		{"build", "test"},
		{"build", "test", "commit"},
		{"build"},
		{},
	}

	for _, skills := range tests {
		state := terminalState(skills)
		if state != "completed" {
			t.Errorf("terminalState(%v) = %s, want completed", skills, state)
		}
	}
}

// --- isNonModifyingSkill tests ---

func TestIsNonModifyingSkillTrue(t *testing.T) {
	for _, name := range []string{"route", "decompose", "review", "document"} {
		if !isNonModifyingSkill(name) {
			t.Errorf("isNonModifyingSkill(%q) = false, want true", name)
		}
	}
}

func TestIsNonModifyingSkillFalse(t *testing.T) {
	for _, name := range []string{"build", "test", "commit", "spec", ""} {
		if isNonModifyingSkill(name) {
			t.Errorf("isNonModifyingSkill(%q) = true, want false", name)
		}
	}
}
