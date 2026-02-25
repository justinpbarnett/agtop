package jira

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/justinpbarnett/agtop/internal/config"
)

func issueJSON(key, summary, description, issueType, priority, status string, labels []string) string {
	var descField json.RawMessage
	if description != "" {
		descField, _ = json.Marshal(map[string]interface{}{
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
	} else {
		descField = json.RawMessage("null")
	}

	resp := map[string]interface{}{
		"key": key,
		"fields": map[string]interface{}{
			"summary":     summary,
			"description": json.RawMessage(descField),
			"issuetype":   map[string]string{"name": issueType},
			"priority":    map[string]string{"name": priority},
			"status":      map[string]string{"name": status},
			"labels":      labels,
		},
	}
	b, _ := json.Marshal(resp)
	return string(b)
}

func TestFetchIssue(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/3/issue/PROJ-123":
			if r.Header.Get("Authorization") == "" {
				t.Error("expected Authorization header")
			}
			w.Write([]byte(issueJSON("PROJ-123", "Fix login bug", "Users cannot log in", "Bug", "High", "To Do", []string{"backend", "auth"})))
		case "/rest/api/3/issue/PROJ-123/comment":
			w.Write([]byte(`{"comments":[]}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token123")
	issue, err := c.FetchIssue("PROJ-123")
	if err != nil {
		t.Fatalf("FetchIssue() error: %v", err)
	}

	if issue.Key != "PROJ-123" {
		t.Errorf("expected key PROJ-123, got %s", issue.Key)
	}
	if issue.Summary != "Fix login bug" {
		t.Errorf("expected summary 'Fix login bug', got %q", issue.Summary)
	}
	if issue.Description != "Users cannot log in" {
		t.Errorf("expected description 'Users cannot log in', got %q", issue.Description)
	}
	if issue.IssueType != "Bug" {
		t.Errorf("expected issue type Bug, got %q", issue.IssueType)
	}
	if issue.Priority != "High" {
		t.Errorf("expected priority High, got %q", issue.Priority)
	}
	if issue.Status != "To Do" {
		t.Errorf("expected status 'To Do', got %q", issue.Status)
	}
	if len(issue.Labels) != 2 || issue.Labels[0] != "backend" {
		t.Errorf("expected labels [backend auth], got %v", issue.Labels)
	}
}

func TestFetchIssueNotFound(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errorMessages":["Issue does not exist"]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token123")
	_, err := c.FetchIssue("PROJ-999")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if got := err.Error(); !contains(got, "404") {
		t.Errorf("expected error to contain '404', got %q", got)
	}
}

func TestFetchIssueUnauthorized(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"Unauthorized"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "badtoken")
	_, err := c.FetchIssue("PROJ-1")
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if got := err.Error(); !contains(got, "401") {
		t.Errorf("expected error to contain '401', got %q", got)
	}
}

func TestFetchIssueADFDescription(t *testing.T) {
	t.Parallel()

	adfDesc, _ := json.Marshal(map[string]interface{}{
		"type": "doc",
		"content": []map[string]interface{}{
			{
				"type": "heading",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Overview"},
				},
			},
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": "First paragraph."},
				},
			},
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Second paragraph with "},
					{"type": "text", "text": "bold text."},
				},
			},
		},
	})

	resp := map[string]interface{}{
		"key": "PROJ-456",
		"fields": map[string]interface{}{
			"summary":     "Complex description",
			"description": json.RawMessage(adfDesc),
			"issuetype":   map[string]string{"name": "Story"},
			"priority":    map[string]string{"name": "Medium"},
			"status":      map[string]string{"name": "In Progress"},
			"labels":      []string{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token")
	issue, err := c.FetchIssue("PROJ-456")
	if err != nil {
		t.Fatalf("FetchIssue() error: %v", err)
	}

	if !contains(issue.Description, "Overview") {
		t.Errorf("expected description to contain 'Overview', got %q", issue.Description)
	}
	if !contains(issue.Description, "First paragraph.") {
		t.Errorf("expected description to contain 'First paragraph.', got %q", issue.Description)
	}
	if !contains(issue.Description, "bold text.") {
		t.Errorf("expected description to contain 'bold text.', got %q", issue.Description)
	}
}

func TestFetchIssueNilDescription(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(issueJSON("PROJ-789", "No description issue", "", "Task", "Low", "Open", nil)))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token")
	issue, err := c.FetchIssue("PROJ-789")
	if err != nil {
		t.Fatalf("FetchIssue() error: %v", err)
	}
	if issue.Description != "" {
		t.Errorf("expected empty description, got %q", issue.Description)
	}
}

func TestFetchIssueAcceptanceCriteriaFromDescription(t *testing.T) {
	t.Parallel()

	adfDesc, _ := json.Marshal(map[string]interface{}{
		"type": "doc",
		"content": []map[string]interface{}{
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Build the upload endpoint."},
				},
			},
			{
				"type": "heading",
				"content": []map[string]interface{}{
					{"type": "text", "text": "Acceptance Criteria"},
				},
			},
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": "- Must accept PNG/JPG"},
				},
			},
			{
				"type": "paragraph",
				"content": []map[string]interface{}{
					{"type": "text", "text": "- Max 5MB file size"},
				},
			},
		},
	})

	resp := map[string]interface{}{
		"key": "PROJ-500",
		"fields": map[string]interface{}{
			"summary":     "Upload feature",
			"description": json.RawMessage(adfDesc),
			"issuetype":   map[string]string{"name": "Story"},
			"priority":    map[string]string{"name": "High"},
			"status":      map[string]string{"name": "To Do"},
			"labels":      []string{},
		},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(resp)
		w.Write(b)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token")
	issue, err := c.FetchIssue("PROJ-500")
	if err != nil {
		t.Fatalf("FetchIssue() error: %v", err)
	}

	if contains(issue.Description, "Acceptance Criteria") {
		t.Errorf("description should not contain the AC heading, got %q", issue.Description)
	}
	if !contains(issue.Description, "Build the upload endpoint.") {
		t.Errorf("expected description to contain main text, got %q", issue.Description)
	}
	if !contains(issue.AcceptanceCriteria, "Must accept PNG/JPG") {
		t.Errorf("expected acceptance criteria to contain first criterion, got %q", issue.AcceptanceCriteria)
	}
	if !contains(issue.AcceptanceCriteria, "Max 5MB file size") {
		t.Errorf("expected acceptance criteria to contain second criterion, got %q", issue.AcceptanceCriteria)
	}
}

func TestNewClientFromConfig(t *testing.T) {
	t.Setenv("TEST_JIRA_TOKEN", "mytoken")
	t.Setenv("TEST_JIRA_EMAIL", "me@company.com")

	cfg := &config.JiraConfig{
		BaseURL:    "https://company.atlassian.net",
		ProjectKey: "PROJ",
		AuthEnv:    "TEST_JIRA_TOKEN",
		UserEnv:    "TEST_JIRA_EMAIL",
	}

	c, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig() error: %v", err)
	}
	if c.email != "me@company.com" {
		t.Errorf("expected email 'me@company.com', got %q", c.email)
	}
	if c.token != "mytoken" {
		t.Errorf("expected token 'mytoken', got %q", c.token)
	}
}

func TestFetchComments(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"comments": [
				{
					"author": {"displayName": "Alice"},
					"created": "2024-01-15T10:00:00.000+0000",
					"body": {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"First comment"}]}]}
				},
				{
					"author": {"displayName": "Bob"},
					"created": "2024-01-16T12:30:00.000+0000",
					"body": {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"Second comment"}]}]}
				}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token")
	comments, err := c.FetchComments("PROJ-1")
	if err != nil {
		t.Fatalf("FetchComments() error: %v", err)
	}
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].Author != "Alice" {
		t.Errorf("expected author 'Alice', got %q", comments[0].Author)
	}
	if comments[0].Body != "First comment" {
		t.Errorf("expected body 'First comment', got %q", comments[0].Body)
	}
	if comments[0].Created != "2024-01-15T10:00:00.000+0000" {
		t.Errorf("unexpected created: %q", comments[0].Created)
	}
	if comments[1].Author != "Bob" {
		t.Errorf("expected author 'Bob', got %q", comments[1].Author)
	}
}

func TestFetchCommentsEmpty(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"comments":[]}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token")
	comments, err := c.FetchComments("PROJ-1")
	if err != nil {
		t.Fatalf("FetchComments() error: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("expected 0 comments, got %d", len(comments))
	}
}

func TestFetchCommentsError(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token")
	_, err := c.FetchComments("PROJ-1")
	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !contains(err.Error(), "500") {
		t.Errorf("expected error to contain '500', got %q", err.Error())
	}
}

func TestFetchIssueIncludesComments(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/rest/api/3/issue/PROJ-200":
			w.Write([]byte(issueJSON("PROJ-200", "Issue with comments", "", "Task", "Medium", "Open", nil)))
		case "/rest/api/3/issue/PROJ-200/comment":
			w.Write([]byte(`{
				"comments": [
					{
						"author": {"displayName": "Carol"},
						"created": "2024-02-01T09:00:00.000+0000",
						"body": {"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"A comment"}]}]}
					}
				]
			}`))
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "user@test.com", "token")
	issue, err := c.FetchIssue("PROJ-200")
	if err != nil {
		t.Fatalf("FetchIssue() error: %v", err)
	}
	if len(issue.Comments) != 1 {
		t.Fatalf("expected 1 comment, got %d", len(issue.Comments))
	}
	if issue.Comments[0].Author != "Carol" {
		t.Errorf("expected comment author 'Carol', got %q", issue.Comments[0].Author)
	}
	if issue.Comments[0].Body != "A comment" {
		t.Errorf("expected comment body 'A comment', got %q", issue.Comments[0].Body)
	}
}

func TestNewClientFromConfigMissingEnv(t *testing.T) {
	cfg := &config.JiraConfig{
		BaseURL:    "https://company.atlassian.net",
		ProjectKey: "PROJ",
		AuthEnv:    "NONEXISTENT_TOKEN_VAR",
		UserEnv:    "NONEXISTENT_EMAIL_VAR",
	}

	_, err := NewClientFromConfig(cfg)
	if err == nil {
		t.Fatal("expected error when env vars are missing")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
