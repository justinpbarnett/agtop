package jira

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestServer(key, summary, description string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if strings.HasSuffix(r.URL.Path, "/comment") {
			w.Write([]byte(`{"comments":[]}`))
			return
		}

		descADF, _ := json.Marshal(map[string]interface{}{
			"type": "doc",
			"content": []map[string]interface{}{
				{
					"type": "paragraph",
					"content": []map[string]interface{}{
						{"type": "text", "text": description},
					},
				},
			},
		})

		resp := map[string]interface{}{
			"key": key,
			"fields": map[string]interface{}{
				"summary":     summary,
				"description": json.RawMessage(descADF),
				"issuetype":   map[string]string{"name": "Story"},
				"priority":    map[string]string{"name": "Medium"},
				"status":      map[string]string{"name": "To Do"},
				"labels":      []string{},
			},
		}
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
}

func TestExpandMatchesIssueKey(t *testing.T) {
	t.Parallel()
	srv := newTestServer("PROJ-123", "Fix the widget", "Widget is broken")
	defer srv.Close()

	c := NewClient(srv.URL, "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	expanded, taskID, err := e.Expand("PROJ-123")
	if err != nil {
		t.Fatalf("Expand() error: %v", err)
	}
	if taskID != "PROJ-123" {
		t.Errorf("expected taskID 'PROJ-123', got %q", taskID)
	}
	if !strings.Contains(expanded, "## PROJ-123: Fix the widget") {
		t.Errorf("expected expanded prompt to contain issue header, got:\n%s", expanded)
	}
	if !strings.Contains(expanded, "Widget is broken") {
		t.Errorf("expected expanded prompt to contain description")
	}
}

func TestExpandWithTrailingText(t *testing.T) {
	t.Parallel()
	srv := newTestServer("PROJ-456", "Add avatar upload", "Upload endpoint needed")
	defer srv.Close()

	c := NewClient(srv.URL, "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	expanded, taskID, err := e.Expand("PROJ-456 focus on the API layer only")
	if err != nil {
		t.Fatalf("Expand() error: %v", err)
	}
	if taskID != "PROJ-456" {
		t.Errorf("expected taskID 'PROJ-456', got %q", taskID)
	}
	if strings.Contains(expanded, "### Additional Context") {
		t.Error("expected no Additional Context section — trailing text should appear naturally after the block")
	}
	if !strings.Contains(expanded, "focus on the API layer only") {
		t.Error("expected trailing text preserved in expanded prompt")
	}
	if !strings.Contains(expanded, "## PROJ-456: Add avatar upload") {
		t.Error("expected issue header in expanded prompt")
	}
}

func TestExpandNoMatch(t *testing.T) {
	t.Parallel()
	c := NewClient("http://unused", "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	expanded, taskID, err := e.Expand("refactor the auth module")
	if err != nil {
		t.Fatalf("Expand() error: %v", err)
	}
	if taskID != "" {
		t.Errorf("expected empty taskID, got %q", taskID)
	}
	if expanded != "refactor the auth module" {
		t.Errorf("expected prompt unchanged, got %q", expanded)
	}
}

func TestExpandCaseInsensitive(t *testing.T) {
	t.Parallel()
	srv := newTestServer("PROJ-10", "Lowercase test", "Testing lowercase")
	defer srv.Close()

	c := NewClient(srv.URL, "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	_, taskID, err := e.Expand("proj-10")
	if err != nil {
		t.Fatalf("Expand() error: %v", err)
	}
	if taskID != "PROJ-10" {
		t.Errorf("expected taskID 'PROJ-10', got %q", taskID)
	}
}

func TestExpandFetchError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, "Internal Server Error")
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	expanded, taskID, err := e.Expand("PROJ-999")
	if err == nil {
		t.Fatal("expected error when fetch fails")
	}
	if taskID != "" {
		t.Errorf("expected empty taskID on error, got %q", taskID)
	}
	if expanded != "PROJ-999" {
		t.Errorf("expected original prompt returned on error, got %q", expanded)
	}
}

func TestExpandInlineMatch(t *testing.T) {
	t.Parallel()
	srv := newTestServer("PROJ-123", "Inline task", "Inline description")
	defer srv.Close()

	c := NewClient(srv.URL, "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	expanded, taskID, err := e.Expand("implement PROJ-123 focusing on the API")
	if err != nil {
		t.Fatalf("Expand() error: %v", err)
	}
	if taskID != "PROJ-123" {
		t.Errorf("expected taskID 'PROJ-123', got %q", taskID)
	}
	if !strings.Contains(expanded, "implement ") {
		t.Error("expected prefix text preserved")
	}
	if !strings.Contains(expanded, "## PROJ-123: Inline task") {
		t.Error("expected issue header in expanded prompt")
	}
	if !strings.Contains(expanded, "focusing on the API") {
		t.Error("expected suffix text preserved")
	}
	if strings.Contains(expanded, "PROJ-123 focusing") {
		t.Error("expected JIRA key to be replaced, not left in place alongside suffix")
	}
}

func TestExpandMidSentence(t *testing.T) {
	t.Parallel()
	srv := newTestServer("PROJ-42", "Mid-sentence task", "Mid description")
	defer srv.Close()

	c := NewClient(srv.URL, "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	expanded, taskID, err := e.Expand("please look at PROJ-42 and implement it")
	if err != nil {
		t.Fatalf("Expand() error: %v", err)
	}
	if taskID != "PROJ-42" {
		t.Errorf("expected taskID 'PROJ-42', got %q", taskID)
	}
	if !strings.Contains(expanded, "please look at ") {
		t.Error("expected prefix text preserved")
	}
	if !strings.Contains(expanded, "## PROJ-42: Mid-sentence task") {
		t.Error("expected issue header in expanded prompt")
	}
	if !strings.Contains(expanded, " and implement it") {
		t.Error("expected suffix text preserved")
	}
}

func TestExtractKey(t *testing.T) {
	t.Parallel()
	c := NewClient("http://unused", "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	tests := []struct {
		prompt string
		want   string
	}{
		{"PROJ-123", "PROJ-123"},
		{"proj-42", "PROJ-42"},
		{"implement PROJ-456 now", "PROJ-456"},
		{"please look at PROJ-99 and fix it", "PROJ-99"},
		{"no key here", ""},
		{"OTHER-123", ""},
		{"", ""},
	}

	for _, tt := range tests {
		if got := e.ExtractKey(tt.prompt); got != tt.want {
			t.Errorf("ExtractKey(%q) = %q, want %q", tt.prompt, got, tt.want)
		}
	}
}

func TestIsIssueKey(t *testing.T) {
	t.Parallel()
	c := NewClient("http://unused", "u@t.com", "tok")
	e := NewExpander(c, "PROJ")

	tests := []struct {
		prompt string
		want   bool
	}{
		{"PROJ-123", true},
		{"proj-1", true},
		{"PROJ-999 extra text", true},
		{"implement PROJ-123", true},
		{"OTHER-123", false},
		{"just a normal prompt", false},
		{"", false},
	}

	for _, tt := range tests {
		if got := e.IsIssueKey(tt.prompt); got != tt.want {
			t.Errorf("IsIssueKey(%q) = %v, want %v", tt.prompt, got, tt.want)
		}
	}
}
