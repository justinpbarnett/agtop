# Deterministic JIRA Expansion with Comments

## Overview

This feature enhances agtop's JIRA integration to detect issue keys anywhere in a prompt and expand them with complete context, including comments. Previously, JIRA keys had to appear at the very start of the prompt, and comments were not fetched. Now keys are detected inline, comment context is automatically included, and the expanded content replaces the key in-place while preserving surrounding text.

## What Was Built

### Phase 1: Comment Fetching

Extended the JIRA client (`internal/jira/client.go`) to fetch issue comments:

- Added `Comment` struct with author name, body text, and timestamp
- Implemented `FetchComments(key)` method that calls the JIRA REST API v3 `/rest/api/3/issue/{key}/comment` endpoint with `orderBy=created` for chronological ordering
- Updated `FetchIssue()` to automatically fetch and attach comments; gracefully degrades if comment fetching fails
- Handles Atlassian Document Format (ADF) comment bodies via the existing `extractADFText()` helper

### Phase 2: Comment Rendering

Updated the formatter (`internal/jira/formatter.go`) to display comments:

- Added `### Comments` section to the formatted output after Labels
- Section only appears if comments exist (omitted for empty lists)
- Each comment shows author name, creation timestamp, and extracted plain text

### Phase 3: Inline Pattern Matching

Changed the expander (`internal/jira/expander.go`) from start-anchored to flexible matching:

- Pattern changed from `^PROJ-\d+` (start-of-line) to inline `(?i)(PROJ-\d+)` to detect keys anywhere in the prompt
- Updated `Expand()` to locate the first matching key, fetch the issue with comments, and replace the key in-place
- Text before and after the key is preserved, creating natural flow
- `IsIssueKey()` no longer requires the key to be at the start

## Technical Implementation

### Client Changes

The `FetchComments()` method:
1. Makes an authenticated GET request to `{baseURL}/rest/api/3/issue/{key}/comment?orderBy=created`
2. Parses the JSON response into `apiCommentResponse` containing a slice of comments
3. For each comment, extracts the author display name, creation timestamp, and plain text body
4. Returns a slice of `Comment` structs ordered chronologically

Comment fetching errors are logged to stderr as warnings but don't prevent issue fetching — comments are supplementary context.

### Expander Changes

The `Expand()` method now:
1. Searches for the first occurrence of a JIRA key pattern anywhere in the trimmed prompt
2. If no match is found, returns the prompt unchanged with empty taskID
3. If a match is found, extracts the text before the key (prefix) and after the key (suffix)
4. Fetches the issue (which now includes comments via `FetchIssue`)
5. Formats the issue with `FormatPrompt` to produce the markdown block
6. Combines: `prefix + formattedIssue + suffix` and returns with the issue key as taskID

### Test Coverage

Added comprehensive tests across three packages:

**Client tests** (`internal/jira/client_test.go`):
- `TestFetchComments`: Verifies comment parsing with correct author, body, and timestamp fields
- `TestFetchCommentsEmpty`: Ensures empty comment lists are handled gracefully
- `TestFetchCommentsError`: Validates error handling when API returns non-200
- `TestFetchIssueIncludesComments`: Confirms `FetchIssue` populates the `Comments` field

**Formatter tests** (`internal/jira/formatter_test.go`):
- `TestFormatPromptWithComments`: Verifies the `### Comments` section renders with author and timestamp
- `TestFormatPromptNoComments`: Ensures no Comments section appears when list is empty

**Expander tests** (`internal/jira/expander_test.go`):
- `TestExpandInlineMatch`: Tests key replacement when key appears mid-sentence (e.g., `"implement PROJ-123 focusing on..."`)
- `TestExpandMidSentence`: Verifies both prefix and suffix text are preserved around the formatted block
- Updated existing tests: `TestExpandMatchesIssueKey`, `TestExpandWithTrailingText`, `TestExpandCaseInsensitive`, `TestExpandFetchError`, and `TestIsIssueKey` to work with inline matching

## How to Use

### From the User's Perspective

Write a prompt with a JIRA key anywhere in the text:

```
implement PROJ-123 focusing on the API layer
```

Instead of:
```
PROJ-123 implement focusing on the API layer
```

When you submit a run, agtop will:
1. Detect `PROJ-123` in the prompt
2. Fetch the issue details and its comments from JIRA
3. Replace the key with a formatted markdown block containing:
   - Issue key and summary
   - Type, priority, and status
   - Description and acceptance criteria
   - All comments in chronological order (oldest to newest)
4. Pass the expanded prompt to the AI agent with full context

### Example Output

Input prompt:
```
Can you help me with PROJ-42 and ensure we follow the acceptance criteria?
```

Expands to:
```
Can you help me with ## PROJ-42: Implement user authentication

**Type:** Feature | **Priority:** High | **Status:** In Progress

### Description

Add OAuth 2.0 integration with GitHub provider for user login.

### Acceptance Criteria

- Users can log in via GitHub
- Sessions persist across page reloads
- Logout clears session state

### Labels

security, auth, github

### Comments

**Alice** (2025-02-10T10:15:00Z):
Please also support GitHub token refresh automatically.

**Bob** (2025-02-12T14:30:00Z):
Design doc is ready in the wiki — check section 3 for token flow details.
 and ensure we follow the acceptance criteria?
```

## Edge Cases and Behavior

- **Multiple keys in one prompt**: Only the first match is expanded. This keeps behavior predictable and avoids multiple API calls.
- **Key in URL or code**: The simple regex may match keys embedded in URLs or code blocks. This is an acceptable trade-off for MVP simplicity.
- **Missing author display name**: Falls back to an empty string gracefully.
- **Comment fetch failure**: Issue is still returned with an empty comments list; a warning is logged to stderr.
- **Large comment counts**: All comments are included (JIRA default page size is 50); no pagination logic is implemented.
- **Case insensitivity**: Keys are matched case-insensitively (e.g., `proj-10` matches) but normalized to uppercase in the API call.

## Configuration

No new configuration is required. The feature uses the existing JIRA configuration (base URL, email, API token) already set up in the project.

## Known Limitations

1. **Only the first key is expanded** — to keep prompt expansion deterministic and efficient
2. **No comment pagination** — assumes fewer than 50 comments per issue (JIRA default)
3. **ADF parsing is basic** — extracts plain text but doesn't preserve formatting like bold or links

## Additional Changes

This branch also includes two bug fixes unrelated to the JIRA feature:

- **Symlink resolution** (`internal/git/worktree.go`): Fixed worktree list filtering to properly resolve symbolic links before comparison
- **Shell portability** (`internal/engine/pipeline_test.go`): Replaced `echo -n` with `printf` for better POSIX shell compatibility in tests

## Testing

All tests pass:
```bash
go test ./...
```

JIRA-specific tests:
```bash
go test ./internal/jira/... -v
```

## Backwards Compatibility

This change is backwards-compatible. The new inline pattern matches everything the old start-anchored pattern did plus more. Users who relied on keys at the beginning of prompts will continue to see the same behavior.
