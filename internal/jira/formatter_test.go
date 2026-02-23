package jira

import (
	"strings"
	"testing"
)

func TestFormatPromptFull(t *testing.T) {
	t.Parallel()
	issue := &Issue{
		Key:                "PROJ-100",
		Summary:            "Add user avatar upload",
		Description:        "Users should be able to upload a custom avatar image.",
		IssueType:          "Story",
		Priority:           "High",
		Status:             "To Do",
		Labels:             []string{"backend", "api"},
		AcceptanceCriteria: "- Upload accepts PNG/JPG\n- Max 5MB",
	}

	got := FormatPrompt(issue)

	if !strings.Contains(got, "## PROJ-100: Add user avatar upload") {
		t.Error("expected header with key and summary")
	}
	if !strings.Contains(got, "**Type:** Story") {
		t.Error("expected issue type")
	}
	if !strings.Contains(got, "**Priority:** High") {
		t.Error("expected priority")
	}
	if !strings.Contains(got, "**Status:** To Do") {
		t.Error("expected status")
	}
	if !strings.Contains(got, "### Description") {
		t.Error("expected description section")
	}
	if !strings.Contains(got, "Users should be able to upload") {
		t.Error("expected description content")
	}
	if !strings.Contains(got, "### Acceptance Criteria") {
		t.Error("expected acceptance criteria section")
	}
	if !strings.Contains(got, "Upload accepts PNG/JPG") {
		t.Error("expected acceptance criteria content")
	}
	if !strings.Contains(got, "### Labels") {
		t.Error("expected labels section")
	}
	if !strings.Contains(got, "backend, api") {
		t.Error("expected labels content")
	}
}

func TestFormatPromptMinimal(t *testing.T) {
	t.Parallel()
	issue := &Issue{
		Key:     "PROJ-1",
		Summary: "Quick task",
	}

	got := FormatPrompt(issue)

	if !strings.Contains(got, "## PROJ-1: Quick task") {
		t.Error("expected header")
	}
	if strings.Contains(got, "### Description") {
		t.Error("expected no description section when empty")
	}
	if strings.Contains(got, "### Acceptance Criteria") {
		t.Error("expected no acceptance criteria section when empty")
	}
	if strings.Contains(got, "### Labels") {
		t.Error("expected no labels section when empty")
	}
}

func TestFormatPromptNoAcceptanceCriteria(t *testing.T) {
	t.Parallel()
	issue := &Issue{
		Key:         "PROJ-50",
		Summary:     "Fix bug",
		Description: "Something is broken",
		IssueType:   "Bug",
		Priority:    "Critical",
		Status:      "In Progress",
	}

	got := FormatPrompt(issue)

	if !strings.Contains(got, "### Description") {
		t.Error("expected description section")
	}
	if strings.Contains(got, "### Acceptance Criteria") {
		t.Error("expected no acceptance criteria section")
	}
}

func TestFormatPromptNoLabels(t *testing.T) {
	t.Parallel()
	issue := &Issue{
		Key:     "PROJ-2",
		Summary: "No labels task",
		Labels:  []string{},
	}

	got := FormatPrompt(issue)

	if strings.Contains(got, "### Labels") {
		t.Error("expected no labels section for empty labels")
	}
}
