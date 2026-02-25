package jira

import (
	"fmt"
	"strings"
)

// FormatPrompt renders a JIRA issue into a structured markdown prompt
// suitable for AI agent consumption.
func FormatPrompt(issue *Issue) string {
	var b strings.Builder

	fmt.Fprintf(&b, "## %s: %s\n", issue.Key, issue.Summary)

	var meta []string
	if issue.IssueType != "" {
		meta = append(meta, fmt.Sprintf("**Type:** %s", issue.IssueType))
	}
	if issue.Priority != "" {
		meta = append(meta, fmt.Sprintf("**Priority:** %s", issue.Priority))
	}
	if issue.Status != "" {
		meta = append(meta, fmt.Sprintf("**Status:** %s", issue.Status))
	}
	if len(meta) > 0 {
		b.WriteString("\n")
		b.WriteString(strings.Join(meta, "  |  "))
		b.WriteString("\n")
	}

	if issue.Description != "" {
		b.WriteString("\n### Description\n\n")
		b.WriteString(issue.Description)
		b.WriteString("\n")
	}

	if issue.AcceptanceCriteria != "" {
		b.WriteString("\n### Acceptance Criteria\n\n")
		b.WriteString(issue.AcceptanceCriteria)
		b.WriteString("\n")
	}

	if len(issue.Labels) > 0 {
		b.WriteString("\n### Labels\n\n")
		b.WriteString(strings.Join(issue.Labels, ", "))
		b.WriteString("\n")
	}

	if len(issue.Comments) > 0 {
		b.WriteString("\n### Comments\n")
		for _, c := range issue.Comments {
			fmt.Fprintf(&b, "\n**%s** (%s):\n%s\n", c.Author, c.Created, c.Body)
		}
	}

	return b.String()
}
