package jira

import (
	"fmt"
	"regexp"
	"strings"
)

// Expander detects JIRA issue keys in user prompts and expands them
// into rich markdown prompts by fetching the issue details.
type Expander struct {
	client     *Client
	projectKey string
	pattern    *regexp.Regexp
}

// NewExpander creates an Expander for the given project key.
func NewExpander(client *Client, projectKey string) *Expander {
	// Match the project key followed by a dash and digits at the start of the prompt,
	// optionally followed by whitespace and additional user text.
	pat := fmt.Sprintf(`(?i)^(%s-\d+)\s*(.*)$`, regexp.QuoteMeta(projectKey))
	return &Expander{
		client:     client,
		projectKey: projectKey,
		pattern:    regexp.MustCompile(pat),
	}
}

// Expand checks if the prompt starts with a JIRA issue key. If so, it fetches
// the issue and returns an expanded prompt with full issue details. Any trailing
// text after the issue key is preserved as additional context. If the prompt does
// not match, it is returned unchanged with an empty taskID.
func (e *Expander) Expand(prompt string) (expanded string, taskID string, err error) {
	matches := e.pattern.FindStringSubmatch(strings.TrimSpace(prompt))
	if matches == nil {
		return prompt, "", nil
	}

	issueKey := strings.ToUpper(matches[1])
	extraText := strings.TrimSpace(matches[2])

	issue, err := e.client.FetchIssue(issueKey)
	if err != nil {
		return prompt, "", fmt.Errorf("jira expand: %w", err)
	}

	result := FormatPrompt(issue)

	if extraText != "" {
		result += "\n### Additional Context\n\n" + extraText + "\n"
	}

	return result, issueKey, nil
}

// IsIssueKey returns true if the prompt starts with a JIRA issue key pattern
// for this expander's project. Does not make any network calls.
func (e *Expander) IsIssueKey(prompt string) bool {
	return e.pattern.MatchString(strings.TrimSpace(prompt))
}
