# Feature: JIRA Integration

## Metadata

type: `feat`
task_id: `jira-integration`
prompt: `Add JIRA integration so users can configure a JIRA project per agtop project directory, then start runs by entering a JIRA task ID — agtop fetches the issue details and expands them into a rich prompt automatically`

## Feature Description

Developers using agtop in a company environment have tasks defined in JIRA. Currently, starting a new run requires manually copying the task description from JIRA into the new run modal. This feature adds project-level JIRA configuration to `agtop.yaml` and automatic prompt expansion — when a user types a JIRA issue key (e.g., `PROJ-1234`) as their prompt, agtop detects the pattern, fetches the issue via the JIRA REST API, and constructs a rich markdown prompt from the issue's title, description, type, priority, and acceptance criteria. The expanded prompt is then passed through the normal workflow pipeline.

The JIRA task ID is also stored on the `Run` struct and displayed in the detail panel and run list, giving visibility into which JIRA issue each run is working on.

## User Story

As a developer using agtop at a company with JIRA
I want to start a run by typing a JIRA issue key instead of rewriting the task description
So that I can go from JIRA ticket to AI agent execution in seconds with full task context

## Relevant Files

- `internal/config/config.go` — Config struct definitions; needs new `Integrations` section with `JiraConfig`
- `internal/config/loader.go` — Config loading and merge logic; needs merge support for the new `Integrations` field
- `internal/config/validate.go` — Config validation; needs JIRA config validation rules
- `internal/config/defaults.go` — Default config values; needs default `Integrations` field (nil/empty)
- `internal/run/run.go` — Run struct; needs `TaskID` field to store the JIRA issue key
- `internal/ui/app.go` — Run creation flow (`StartRunMsg` handler at line 237); needs JIRA expansion hook between `SubmitNewRunMsg` and `StartRunMsg`
- `internal/ui/panels/detail.go` — Detail panel rendering; needs to display `TaskID` field
- `internal/ui/panels/runlist.go` — Run list rendering and filtering; needs to display task ID and include it in filter matching
- `internal/ui/panels/messages.go` — Panel message types; `SubmitNewRunMsg` may carry task ID
- `internal/run/persistence.go` — Session persistence; `TaskID` auto-included via JSON tags
- `agtop.example.yaml` — Example config; needs `integrations.jira` section

### New Files

- `internal/jira/client.go` — JIRA REST API client. Handles authentication, issue fetching, and HTTP communication.
- `internal/jira/client_test.go` — Tests for the JIRA client using `httptest.NewServer` for mock API responses.
- `internal/jira/formatter.go` — Converts a JIRA issue into a structured markdown prompt string.
- `internal/jira/formatter_test.go` — Tests for prompt formatting with various issue shapes.
- `internal/jira/expander.go` — Prompt expansion logic: detects JIRA issue key patterns in user input, fetches the issue, and returns the expanded prompt. This is the integration entry point called from the UI layer.
- `internal/jira/expander_test.go` — Tests for pattern detection and expansion.

## Implementation Plan

### Phase 1: Configuration

Add JIRA configuration types to the config system. Extend `Config` with an `Integrations` section containing `JiraConfig`. Add merge, validation, and example config. Auth credentials come from environment variables named in the config — never stored in YAML.

### Phase 2: JIRA Client and Formatter

Build the HTTP client that calls the JIRA REST API v3 (`/rest/api/3/issue/{issueKey}`). Parse the response into a local `Issue` struct. Build a formatter that renders the issue into a markdown prompt suitable for AI agent consumption.

### Phase 3: Prompt Expansion

Implement the expansion logic that detects JIRA issue key patterns in user prompt text, fetches the issue, and returns an expanded prompt. Wire this into the run creation flow in `app.go` so expansion happens transparently between modal submission and run start.

### Phase 4: Run Metadata and UI

Add `TaskID` to the `Run` struct. Display it in the detail panel and run list. Include it in the filter search.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add JIRA Config Types

In `internal/config/config.go`:

- Add `Integrations IntegrationsConfig` field to `Config` struct with yaml tag `integrations`
- Define `IntegrationsConfig` struct with a single field: `Jira *JiraConfig` (pointer, so nil = not configured) with yaml tag `jira,omitempty`
- Define `JiraConfig` struct:
  ```go
  type JiraConfig struct {
      BaseURL    string `yaml:"base_url"`     // e.g. "https://company.atlassian.net"
      ProjectKey string `yaml:"project_key"`  // e.g. "PROJ"
      AuthEnv    string `yaml:"auth_env"`     // env var name for API token
      UserEnv    string `yaml:"user_env"`     // env var name for user email
  }
  ```

### 2. Add Config Merge and Validation

In `internal/config/loader.go`, add merge logic in the `merge()` function:
- If `override.Integrations.Jira` is non-nil, set `base.Integrations.Jira` to `override.Integrations.Jira`

In `internal/config/validate.go`, add JIRA validation at the end (only when `cfg.Integrations.Jira != nil`):
- `BaseURL` must be non-empty
- `ProjectKey` must be non-empty
- `AuthEnv` must be non-empty
- `UserEnv` must be non-empty
- If `AuthEnv` is set, warn (to stderr, non-fatal) if the named env var is actually empty at runtime — helps users catch misconfiguration

### 3. Update Example Config

In `agtop.example.yaml`, add an `integrations` section at the end:
```yaml
# integrations:
#   jira:
#     base_url: "https://yourcompany.atlassian.net"
#     project_key: "PROJ"
#     auth_env: "JIRA_API_TOKEN"    # env var containing your API token
#     user_env: "JIRA_USER_EMAIL"   # env var containing your Atlassian email
```

Keep it commented out so the example works without JIRA configured.

### 4. Implement JIRA Client

Create `internal/jira/client.go`:

- Define `Issue` struct with fields: `Key`, `Summary`, `Description` (string — already converted from ADF), `IssueType`, `Priority`, `Status`, `Labels`, `AcceptanceCriteria` (string)
- Define `Client` struct with fields: `baseURL`, `email`, `token`, `httpClient *http.Client`
- `NewClient(baseURL, email, token string) *Client` — constructor, creates `http.Client` with 10s timeout
- `NewClientFromConfig(cfg *config.JiraConfig) (*Client, error)` — reads env vars named in config, returns error if env vars are empty
- `FetchIssue(key string) (*Issue, error)`:
  - GET `{baseURL}/rest/api/3/issue/{key}?fields=summary,description,issuetype,priority,status,labels,customfield_*`
  - Auth: Basic auth with `email:token` (base64 encoded) — this is Atlassian Cloud's standard API auth
  - Parse JSON response, extract fields into `Issue` struct
  - Handle the `description` field: JIRA v3 returns Atlassian Document Format (ADF). Walk the ADF tree and extract plain text. For MVP, a simple recursive text extraction is sufficient (walk `content` arrays, concatenate `text` fields from `text` nodes, add newlines for `paragraph`/`heading` nodes)
  - Look for acceptance criteria in `customfield_*` fields — common field names vary by JIRA instance, so check the description text for "acceptance criteria" headings as a fallback
  - Return `nil, err` for non-200 responses with the status code and body in the error message

### 5. Implement JIRA Client Tests

Create `internal/jira/client_test.go`:

- Use `httptest.NewServer` to mock the JIRA API
- **TestFetchIssue**: mock returns valid issue JSON → verify all fields parsed correctly
- **TestFetchIssueNotFound**: mock returns 404 → verify error contains "404"
- **TestFetchIssueUnauthorized**: mock returns 401 → verify error contains "401"
- **TestFetchIssueADFDescription**: mock returns ADF-format description → verify plain text extraction
- **TestNewClientFromConfig**: verify env vars are read correctly, verify error when env vars missing

### 6. Implement Prompt Formatter

Create `internal/jira/formatter.go`:

- `FormatPrompt(issue *Issue) string` — renders the issue into a markdown prompt:
  ```
  ## {KEY}: {Summary}

  **Type:** {IssueType}  |  **Priority:** {Priority}  |  **Status:** {Status}

  ### Description
  {Description text}

  ### Acceptance Criteria
  {Acceptance criteria if present}

  ### Labels
  {comma-separated labels if present}
  ```
- Omit sections that are empty (e.g., no acceptance criteria → skip that section)
- Keep formatting clean — this becomes the prompt the AI agent reads

### 7. Implement Prompt Formatter Tests

Create `internal/jira/formatter_test.go`:

- **TestFormatPromptFull**: issue with all fields → verify all sections present
- **TestFormatPromptMinimal**: issue with only key/summary → verify only those sections render
- **TestFormatPromptNoAcceptanceCriteria**: verify section is omitted
- **TestFormatPromptNoLabels**: verify section is omitted

### 8. Implement Prompt Expander

Create `internal/jira/expander.go`:

- `Expander` struct with fields: `client *Client`, `projectKey string`
- `NewExpander(client *Client, projectKey string) *Expander`
- `issueKeyPattern` — compiled regex: `^(?i)({projectKey}-\d+)\s*(.*)$` where `{projectKey}` is the configured project key. This matches the key at the start of the prompt, with optional trailing text.
- `Expand(prompt string) (expanded string, taskID string, err error)`:
  - Check if prompt matches `issueKeyPattern`
  - If no match, return `prompt, "", nil` unchanged
  - If match, extract the issue key and any trailing user text
  - Call `client.FetchIssue(key)`
  - Format the issue with `FormatPrompt(issue)`
  - If user added trailing text, append it under a `### Additional Context` section
  - Return the expanded prompt, the issue key as `taskID`, and nil error
- `IsIssueKey(prompt string) bool` — quick check without fetching, useful for UI hints

### 9. Implement Expander Tests

Create `internal/jira/expander_test.go`:

- **TestExpandMatchesIssueKey**: prompt `"PROJ-123"` → fetches and expands
- **TestExpandWithTrailingText**: prompt `"PROJ-123 focus on the API layer"` → expands with additional context section
- **TestExpandNoMatch**: prompt `"refactor the auth module"` → returns unchanged
- **TestExpandCaseInsensitive**: prompt `"proj-123"` → matches
- **TestExpandFetchError**: client returns error → returns error (prompt unchanged)

### 10. Add TaskID to Run Struct

In `internal/run/run.go`:

- Add `TaskID string` field with json tag `"task_id"` — placed after `Prompt` for logical grouping
- This auto-persists via the existing JSON serialization in `internal/run/persistence.go`

### 11. Wire Expansion into Run Creation Flow

In `internal/ui/app.go`:

- Add a `jiraExpander *jira.Expander` field to the `App` struct (nil when JIRA not configured)
- In `NewApp()`, if `cfg.Integrations.Jira != nil`:
  - Call `jira.NewClientFromConfig(cfg.Integrations.Jira)` — log warning and leave expander nil if env vars missing (non-fatal; app works without JIRA)
  - Create `jira.NewExpander(client, cfg.Integrations.Jira.ProjectKey)`
  - Store on `App`
- In the `SubmitNewRunMsg` handler (line 228), modify the tea.Cmd function to perform expansion before returning `StartRunMsg`:
  ```go
  case SubmitNewRunMsg:
      return a, func() tea.Msg {
          prompt := msg.Prompt
          taskID := ""
          if a.jiraExpander != nil {
              expanded, tid, err := a.jiraExpander.Expand(prompt)
              if err != nil {
                  // Log error but fall through with original prompt
              } else {
                  prompt = expanded
                  taskID = tid
              }
          }
          return StartRunMsg{
              Prompt:   prompt,
              Workflow: msg.Workflow,
              Model:    msg.Model,
              TaskID:   taskID,
          }
      }
  ```
- Add `TaskID string` field to `StartRunMsg` (in `internal/ui/panels/messages.go` on `SubmitNewRunMsg` or create a dedicated `StartRunMsg` type in `internal/ui/app.go`)
- In the `StartRunMsg` handler (line 237), set `newRun.TaskID = msg.TaskID`

### 12. Display TaskID in Detail Panel

In `internal/ui/panels/detail.go`, in `renderDetails()`:

- After the Prompt row (line 100), add a TaskID row if non-empty:
  ```go
  if r.TaskID != "" {
      fmt.Fprintf(&b, "  %s\n", row("Task", r.TaskID))
  }
  ```

### 13. Display TaskID in Run List and Filter

In `internal/ui/panels/runlist.go`:

- In `renderContent()` (line 201-227): when `rn.TaskID != ""`, display the task ID in place of or alongside the branch in the run list row. Replace the branch column with a combined display: if TaskID is set, show TaskID; otherwise show branch. This keeps the compact layout.
- In `applyFilter()` (line 262-280): add `rn.TaskID` to the filter matching:
  ```go
  strings.Contains(strings.ToLower(rn.TaskID), query)
  ```

### 14. Write Config Integration Tests

In `internal/config/loader_test.go`, add:

- **TestMergeJiraConfig**: verify JIRA config merges correctly from YAML
- **TestValidateJiraConfigValid**: fully populated JIRA config passes validation
- **TestValidateJiraConfigMissingBaseURL**: JIRA configured but base_url empty → validation error
- **TestValidateJiraConfigMissingProjectKey**: → validation error

In `internal/config/validate_test.go`, add:

- **TestValidateJiraRequiredFields**: each required field missing in turn → error

## Testing Strategy

### Unit Tests

- **Client tests** (`jira/client_test.go`): HTTP mock server tests for fetch, auth, error handling, ADF parsing
- **Formatter tests** (`jira/formatter_test.go`): prompt rendering with various field combinations
- **Expander tests** (`jira/expander_test.go`): pattern detection, expansion, trailing text, error handling
- **Config tests** (`config/loader_test.go`, `config/validate_test.go`): merge and validation for JIRA config section

### Edge Cases

- JIRA not configured (nil `Integrations.Jira`) — entire feature is dormant, no behavior change
- JIRA configured but env vars not set — warning at startup, expander not created, runs work normally
- Prompt matches issue key pattern but JIRA API is unreachable — error logged, original prompt used as-is
- Prompt matches issue key pattern but issue doesn't exist (404) — error logged, original prompt used as-is
- Issue has empty description — formatter renders key + summary only
- Issue description is in ADF format — text extracted from ADF nodes
- User types issue key with extra text: `PROJ-123 only do the backend` — both issue details and user text included
- User types a prompt that looks like an issue key but isn't configured project: `OTHER-123` — no match, passed through unchanged
- Case insensitivity: `proj-123` matches when project_key is `PROJ`
- Multiple issue keys in prompt — only the first (at start of prompt) is expanded
- JIRA API rate limiting (429) — error returned, original prompt used

## Risk Assessment

- **No risk to existing functionality**: JIRA integration is entirely additive. When `Integrations.Jira` is nil (default), zero code paths change. The expander is nil-checked before use.
- **Network dependency at run creation time**: The JIRA API call happens in the `SubmitNewRunMsg` → `StartRunMsg` tea.Cmd function, which runs in a goroutine. This means the UI won't block, but there will be a brief delay before the run appears. If the API is slow, the delay is noticeable but not blocking.
- **API token security**: Tokens are never stored in config files — only env var names are stored. The client reads env vars at initialization time.
- **ADF parsing**: JIRA v3 uses Atlassian Document Format for descriptions, which is a complex nested JSON structure. The MVP text extractor handles common nodes (paragraph, heading, text, hardBreak, listItem) but may miss formatting from rare node types. This is acceptable — the extracted text is still useful for AI agents.

## Validation Commands

```bash
# All packages compile
go build ./...

# No vet issues
go vet ./...

# JIRA package tests pass
go test ./internal/jira/... -v

# Config tests pass (including new JIRA merge/validation tests)
go test ./internal/config/... -v

# All tests pass
go test ./...

# Binary builds
just build
```

## Open Questions (Unresolved)

1. **Custom fields for acceptance criteria**: JIRA instances use different custom field IDs for acceptance criteria. Should we add a config option like `acceptance_criteria_field: "customfield_10001"`, or just rely on parsing the description text? **Recommendation**: Start with description-text parsing (look for "Acceptance Criteria" heading). Add the custom field config option later if users need it.

2. **JIRA status updates**: Should agtop update the JIRA issue when a run completes (e.g., add a comment, transition status)? **Recommendation**: Not in Phase 1. This is a natural Phase 2 follow-up — post a comment with run results, diff summary, and cost.

3. **JIRA Server vs Cloud**: The spec targets Atlassian Cloud (basic auth with email + API token). JIRA Server/Data Center uses different auth (personal access tokens, OAuth). **Recommendation**: Target Cloud only for Phase 1. JIRA Server support can be added later by abstracting the auth strategy in the client.

4. **Branch naming from JIRA**: Should the git branch name incorporate the JIRA issue key (e.g., `agtop/PROJ-123-001` instead of `agtop/001`)? **Recommendation**: Yes, this would be a nice touch — implement if straightforward, otherwise defer. The worktree manager's `Create(runID)` would need the task ID passed in.

## Sub-Tasks

Single task — no decomposition needed. The feature touches config, one new package, and light UI changes — all within scope for a single implementation pass.
