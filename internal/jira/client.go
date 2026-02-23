package jira

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/justinpbarnett/agtop/internal/config"
)

type Issue struct {
	Key                string
	Summary            string
	Description        string
	IssueType          string
	Priority           string
	Status             string
	Labels             []string
	AcceptanceCriteria string
}

type Client struct {
	baseURL    string
	email      string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, email, token string) *Client {
	return &Client{
		baseURL:    strings.TrimRight(baseURL, "/"),
		email:      email,
		token:      token,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func NewClientFromConfig(cfg *config.JiraConfig) (*Client, error) {
	email := os.Getenv(cfg.UserEnv)
	if email == "" {
		return nil, fmt.Errorf("jira: env var %s is not set", cfg.UserEnv)
	}
	token := os.Getenv(cfg.AuthEnv)
	if token == "" {
		return nil, fmt.Errorf("jira: env var %s is not set", cfg.AuthEnv)
	}
	return NewClient(cfg.BaseURL, email, token), nil
}

func (c *Client) FetchIssue(key string) (*Issue, error) {
	url := fmt.Sprintf("%s/rest/api/3/issue/%s?fields=summary,description,issuetype,priority,status,labels", c.baseURL, key)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("jira: creating request: %w", err)
	}

	auth := base64.StdEncoding.EncodeToString([]byte(c.email + ":" + c.token))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("jira: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jira: %s returned %d: %s", key, resp.StatusCode, truncate(string(body), 200))
	}

	var raw apiResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("jira: parsing response: %w", err)
	}

	issue := &Issue{
		Key:     raw.Key,
		Summary: raw.Fields.Summary,
		Status:  raw.Fields.Status.Name,
		Labels:  raw.Fields.Labels,
	}

	if raw.Fields.IssueType.Name != "" {
		issue.IssueType = raw.Fields.IssueType.Name
	}
	if raw.Fields.Priority.Name != "" {
		issue.Priority = raw.Fields.Priority.Name
	}
	if raw.Fields.Description != nil {
		fullText := extractADFText(raw.Fields.Description)
		desc, ac := splitAcceptanceCriteria(fullText)
		issue.Description = desc
		if ac != "" {
			issue.AcceptanceCriteria = ac
		}
	}

	return issue, nil
}

// apiResponse maps the JIRA REST API v3 issue response.
type apiResponse struct {
	Key    string    `json:"key"`
	Fields apiFields `json:"fields"`
}

type apiFields struct {
	Summary     string          `json:"summary"`
	Description json.RawMessage `json:"description"`
	IssueType   apiName         `json:"issuetype"`
	Priority    apiName         `json:"priority"`
	Status      apiName         `json:"status"`
	Labels      []string        `json:"labels"`
}

type apiName struct {
	Name string `json:"name"`
}

// extractADFText walks an Atlassian Document Format tree and extracts plain text.
func extractADFText(raw json.RawMessage) string {
	var node adfNode
	if err := json.Unmarshal(raw, &node); err != nil {
		return ""
	}
	var b strings.Builder
	walkADF(&node, &b)
	return strings.TrimSpace(b.String())
}

type adfNode struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Content json.RawMessage `json:"content"`
}

func walkADF(node *adfNode, b *strings.Builder) {
	if node.Type == "text" {
		b.WriteString(node.Text)
		return
	}
	if node.Type == "hardBreak" {
		b.WriteString("\n")
		return
	}

	var children []adfNode
	if node.Content != nil {
		if err := json.Unmarshal(node.Content, &children); err != nil {
			return
		}
	}

	for i := range children {
		walkADF(&children[i], b)
	}

	switch node.Type {
	case "paragraph", "heading", "bulletList", "orderedList":
		b.WriteString("\n")
	case "listItem":
		// handled by parent list types
	}
}

// splitAcceptanceCriteria scans the description text for an "Acceptance Criteria"
// heading and splits it into (description, acceptanceCriteria). If no heading is
// found, returns the full text as description with empty acceptance criteria.
func splitAcceptanceCriteria(text string) (string, string) {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(strings.TrimSpace(line)), "acceptance criteria") {
			desc := strings.TrimSpace(strings.Join(lines[:i], "\n"))
			ac := strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
			return desc, ac
		}
	}
	return text, ""
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
