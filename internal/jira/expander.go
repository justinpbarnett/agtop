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
	// Match the project key followed by a dash and digits anywhere in the prompt.
	pat := fmt.Sprintf(`(?i)(%s-\d+)`, regexp.QuoteMeta(projectKey))
	return &Expander{
		client:     client,
		projectKey: projectKey,
		pattern:    regexp.MustCompile(pat),
	}
}

// Expand searches for a JIRA issue key anywhere in the prompt. If found, it
// fetches the issue (including comments) and replaces the key in-place with
// the full formatted issue block, preserving any surrounding text. If no key
// is found, the prompt is returned unchanged with an empty taskID.
func (e *Expander) Expand(prompt string) (expanded string, taskID string, err error) {
	trimmed := strings.TrimSpace(prompt)
	loc := e.pattern.FindStringSubmatchIndex(trimmed)
	if loc == nil {
		return prompt, "", nil
	}

	issueKey := strings.ToUpper(trimmed[loc[2]:loc[3]])
	prefix := trimmed[:loc[0]]
	suffix := trimmed[loc[1]:]

	issue, err := e.client.FetchIssue(issueKey)
	if err != nil {
		return prompt, "", fmt.Errorf("jira expand: %w", err)
	}

	return prefix + FormatPrompt(issue) + suffix, issueKey, nil
}

// IsIssueKey returns true if the prompt contains a JIRA issue key pattern
// for this expander's project anywhere in the text. Does not make any network calls.
func (e *Expander) IsIssueKey(prompt string) bool {
	return e.pattern.MatchString(strings.TrimSpace(prompt))
}
