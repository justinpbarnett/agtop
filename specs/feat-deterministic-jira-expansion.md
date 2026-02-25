# Feature: Deterministic JIRA Expansion with Comments

## Metadata

type: `feat`
task_id: `deterministic-jira-expansion`
prompt: `Make JIRA task retrieval deterministic — detect PROJECT-ID patterns anywhere in a prompt, fetch the task info plus comments via the Go client, and inject the result into the prompt replacing the task ID reference`

## Feature Description

The current JIRA integration has two gaps:

1. **Pattern matching is positional** — the `Expander` regex (`^(PROJ-\d+)\s*(.*)$`) only matches when the JIRA key appears at the very start of the prompt. If a user writes `"implement PROJ-123 focusing on the API"`, the key is not detected and the prompt passes through unexpanded. This forces users to always lead with the issue key.

2. **Comments are not fetched** — JIRA issue comments often contain critical context: clarifications from product, design decisions, scope changes, or technical notes. The current client only fetches issue fields (`summary`, `description`, `issuetype`, `priority`, `status`, `labels`). Comments are ignored.

3. **Non-deterministic fallback** — when expansion doesn't fire (due to gap #1), the raw `PROJ-123` string reaches the AI agent as-is. The agent may then attempt to fetch the JIRA data itself via bash/curl calls, which is unreliable, inconsistent, and may fail due to missing credentials in the subprocess environment.

This feature fixes all three by:
- Matching JIRA keys anywhere in the prompt (not just at the start)
- Fetching issue comments alongside issue fields
- Replacing the JIRA key in-place within the prompt text with the full formatted context

## User Story

As a developer using agtop with JIRA
I want JIRA issue keys detected and expanded wherever they appear in my prompt
So that every run gets consistent, complete task context — including comments — without relying on the AI agent to fetch it

## Relevant Files

- `internal/jira/client.go` — JIRA REST API client; needs a new `FetchComments` method
- `internal/jira/client_test.go` — Client tests; needs tests for comment fetching
- `internal/jira/expander.go` — Prompt expansion logic; needs pattern change from start-anchored to inline matching, and in-place replacement logic
- `internal/jira/expander_test.go` — Expander tests; needs updated tests for inline matching and comment inclusion
- `internal/jira/formatter.go` — Issue-to-markdown rendering; needs a `Comments` section
- `internal/jira/formatter_test.go` — Formatter tests; needs tests for comment rendering
- `internal/ui/app.go:363-394` — `SubmitNewRunMsg` handler where expansion is invoked; no changes needed (already calls `expander.Expand()`)

### New Files

None — all changes are within existing files.

## Implementation Plan

### Phase 1: Add Comment Fetching to Client

Extend the JIRA client to fetch issue comments via the `/rest/api/3/issue/{key}/comment` endpoint. Define a `Comment` struct and add the comments to the `Issue` struct.

### Phase 2: Update Formatter to Render Comments

Add a `### Comments` section to the formatted prompt output. Each comment should show the author name, timestamp, and body text (extracted from ADF).

### Phase 3: Update Expander for Inline Matching

Change the expansion pattern from start-anchored (`^PROJ-\d+`) to an inline match that finds JIRA keys anywhere in the prompt. Replace the matched key in-place with the formatted issue block, preserving the surrounding prompt text.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add Comment Struct and FetchComments to Client

In `internal/jira/client.go`:

- Add a `Comment` struct:
  ```go
  type Comment struct {
      Author    string
      Body      string
      Created   string
  }
  ```
- Add a `Comments []Comment` field to the `Issue` struct
- Add API response types for the comments endpoint:
  ```go
  type apiCommentResponse struct {
      Comments []apiComment `json:"comments"`
  }
  type apiComment struct {
      Author apiCommentAuthor `json:"author"`
      Body   json.RawMessage  `json:"body"`
      Created string          `json:"created"`
  }
  type apiCommentAuthor struct {
      DisplayName string `json:"displayName"`
  }
  ```
- Add `FetchComments(key string) ([]Comment, error)` method to `Client`:
  - GET `{baseURL}/rest/api/3/issue/{key}/comment?orderBy=created`
  - Parse response, extract each comment's author display name, created timestamp, and body text (using the existing `extractADFText` helper for the ADF body)
  - Return the slice of comments
- Update `FetchIssue` to also call `FetchComments` and attach the result to `issue.Comments`. If comment fetching fails, log a warning but still return the issue (comments are supplementary, not critical).

### 2. Add Comment Fetching Tests

In `internal/jira/client_test.go`:

- **TestFetchComments**: mock server returns a comment list JSON → verify comments parsed with correct author, body, and created fields
- **TestFetchCommentsEmpty**: mock returns empty comments array → verify empty slice, no error
- **TestFetchCommentsError**: mock returns 500 → verify error returned
- **TestFetchIssueIncludesComments**: update or add a test verifying that `FetchIssue` populates `issue.Comments`
- Update the mock server in existing tests to also handle the `/comment` path (return empty comments so existing tests don't break)

### 3. Update Formatter to Render Comments

In `internal/jira/formatter.go`:

- After the Labels section in `FormatPrompt`, add a Comments section:
  ```go
  if len(issue.Comments) > 0 {
      b.WriteString("\n### Comments\n")
      for _, c := range issue.Comments {
          fmt.Fprintf(&b, "\n**%s** (%s):\n%s\n", c.Author, c.Created, c.Body)
      }
  }
  ```
- Omit the section when there are no comments

### 4. Add Formatter Tests for Comments

In `internal/jira/formatter_test.go`:

- **TestFormatPromptWithComments**: issue with 2 comments → verify `### Comments` section appears with both authors and bodies
- **TestFormatPromptNoComments**: issue with empty `Comments` slice → verify no Comments section rendered

### 5. Update Expander for Inline Pattern Matching

In `internal/jira/expander.go`:

- Change the regex pattern from `^(PROJ-\d+)\s*(.*)$` (start-anchored, captures trailing text) to an inline pattern: `(?i)(PROJ-\d+)` (finds the key anywhere in the prompt)
- Update `Expand` to:
  1. Find the first match of the JIRA key pattern in the prompt
  2. If no match, return prompt unchanged
  3. Fetch the issue (which now includes comments)
  4. Format the issue with `FormatPrompt`
  5. Replace the matched key in the prompt with the formatted block. Preserve surrounding text naturally — text before the key stays before the block, text after stays after.
  6. Return the expanded prompt and the issue key as `taskID`
- Update `IsIssueKey` to use the new pattern (no longer requires start-of-string match)

### 6. Update Expander Tests

In `internal/jira/expander_test.go`:

- **TestExpandMatchesIssueKey**: keep — `"PROJ-123"` alone still works
- **TestExpandWithTrailingText**: update — `"PROJ-456 focus on the API layer only"` should now have the trailing text appear after the formatted block (no separate `### Additional Context` section needed since the text naturally follows)
- **TestExpandNoMatch**: keep — `"refactor the auth module"` returns unchanged
- **TestExpandCaseInsensitive**: keep — `"proj-10"` still matches
- **TestExpandFetchError**: keep — error handling unchanged
- **TestExpandInlineMatch**: new — `"implement PROJ-123 focusing on the API"` → key is replaced inline, surrounding text preserved
- **TestExpandMidSentence**: new — `"please look at PROJ-42 and implement it"` → verify key is replaced and both prefix/suffix text are preserved
- **TestIsIssueKey**: update cases — `"implement PROJ-123"` should now return `true`
- Update `newTestServer` to also handle the `/comment` endpoint (return empty comments array)

## Testing Strategy

### Unit Tests

- **Client tests** (`jira/client_test.go`): Existing tests updated to handle comment endpoint; new tests for `FetchComments` with mock HTTP server
- **Formatter tests** (`jira/formatter_test.go`): New tests for comment rendering in formatted output
- **Expander tests** (`jira/expander_test.go`): Updated tests for inline matching behavior; new tests for mid-prompt key detection

### Edge Cases

- Prompt contains multiple JIRA keys (e.g., `"PROJ-1 and PROJ-2"`) — only the first should be expanded (keeps behavior predictable; avoids multiple API calls)
- Prompt contains a string that looks like a key but isn't the configured project (`"OTHER-123"`) — no match, no expansion
- Issue has many comments — all are included (no pagination needed for MVP; JIRA default page size is 50)
- Comment body is in ADF format — reuse `extractADFText` to get plain text
- JIRA key appears inside a URL or code block — the simple regex will still match; acceptable trade-off for MVP
- Comment author has no display name — fall back to empty string
- Comment fetching fails but issue fetch succeeds — issue is still returned with empty comments

## Risk Assessment

- **Inline matching is a behavior change** for users who relied on the start-of-prompt pattern. Since the new pattern is strictly more permissive (it matches everything the old one did plus more), this is backwards-compatible. The only edge case is if a user's prompt started with a JIRA key but they did NOT want expansion — this is unlikely and was already "working" before.
- **Comment fetching adds a second API call** per expansion. This slightly increases latency (~100-200ms). Since expansion runs in a goroutine (the `SubmitNewRunMsg` tea.Cmd), the UI remains responsive. The added context value outweighs the latency cost.
- **No changes to `app.go`** — the expansion integration point stays the same. `Expand()` still returns `(expanded, taskID, err)` with the same signature.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
# All packages compile
go build ./...

# No vet issues
go vet ./...

# JIRA package tests pass
go test ./internal/jira/... -v

# All tests pass
go test ./...
```

## Open Questions (Unresolved)

1. **Multiple JIRA keys in a single prompt**: Should we expand all matched keys or just the first? **Recommendation**: Expand only the first match for MVP. Multiple expansions would make the prompt very long and require multiple API calls. Users can always provide additional keys as separate context.

2. **Comment pagination**: JIRA's comment endpoint returns up to 50 comments by default. Should we paginate for issues with many comments? **Recommendation**: No pagination for MVP. 50 comments is more than enough context. If needed later, add `maxResults` parameter support.

3. **Comment ordering**: Should comments be newest-first or oldest-first? **Recommendation**: Oldest-first (chronological) via `orderBy=created` — this preserves the conversation flow and gives the AI agent context in the order decisions were made.

## Sub-Tasks

Single task — no decomposition needed. The changes are confined to the `internal/jira` package (client, formatter, expander) with no architectural changes to the rest of the system.
