# Feature: Component-Level E2E Test Suite with teatest

## Metadata

type: `feat`
task_id: `teatest-component-e2e`
prompt: `a comprehensive component-level e2e test suite using teatest`

## Feature Description

The existing test suite tests UI components through manual message construction and property assertions — calling `Update()` directly, checking struct fields, and doing `strings.Contains()` on `View()` output. This works for unit-level logic but misses a class of bugs that only surface when components run inside a real `tea.Program`: rendering regressions, command sequencing, async message delivery, and visual layout breakage.

This feature adds a parallel test suite using Charmbracelet's `teatest` package (`github.com/charmbracelet/x/exp/teatest`). Teatest runs models inside a headless `tea.Program`, provides `Send()` / `Type()` for input, `Output()` for reading rendered frames, and `RequireEqualOutput` for golden file snapshot comparisons. The suite covers every panel and modal as an isolated component, plus integration scenarios that exercise the full `App` model end-to-end.

## User Story

As a developer working on agtop's TUI
I want automated snapshot tests that catch visual regressions
So that UI changes are validated against known-good output and cross-component interactions are tested through a real program loop

## Relevant Files

- `internal/ui/app.go` — Main `App` model. The top-level integration test target.
- `internal/ui/app_test.go` — Existing unit tests for `App`. Pattern reference for test helpers (`newTestApp`, `sendKey`, etc.).
- `internal/ui/panels/runlist.go` — `RunList` panel model.
- `internal/ui/panels/runlist_test.go` — Existing unit tests. Pattern reference for `testStore()` helper.
- `internal/ui/panels/detail.go` — `Detail` panel model.
- `internal/ui/panels/detail_test.go` — Existing unit tests.
- `internal/ui/panels/logview.go` — `LogView` panel model (includes `DiffView` sub-component).
- `internal/ui/panels/logview_test.go` — Existing unit tests.
- `internal/ui/panels/help.go` — `HelpOverlay` modal model.
- `internal/ui/panels/help_test.go` — Existing unit tests.
- `internal/ui/panels/newrun.go` — `NewRunModal` model.
- `internal/ui/panels/newrun_test.go` — Existing unit tests.
- `internal/ui/panels/statusbar.go` — `StatusBar` model.
- `internal/ui/panels/statusbar_test.go` — Existing unit tests.
- `internal/ui/panels/initprompt.go` — `InitPrompt` modal model (in progress).
- `internal/ui/panels/diffview.go` — `DiffView` model.
- `internal/ui/panels/diffview_test.go` — Existing unit tests.
- `internal/ui/messages.go` — Message type aliases.
- `internal/ui/keys.go` — Key binding definitions.
- `go.mod` — Dependency manifest. Needs `teatest` added.

### New Files

- `internal/ui/teatest_helpers_test.go` — Shared teatest helpers, golden file utilities, and common fixtures for the `ui` package.
- `internal/ui/app_teatest_test.go` — Teatest-based integration tests for the full `App` model.
- `internal/ui/panels/teatest_helpers_test.go` — Shared teatest helpers for the `panels` package.
- `internal/ui/panels/runlist_teatest_test.go` — Teatest snapshot and interaction tests for `RunList`.
- `internal/ui/panels/detail_teatest_test.go` — Teatest snapshot tests for `Detail`.
- `internal/ui/panels/logview_teatest_test.go` — Teatest snapshot and interaction tests for `LogView`.
- `internal/ui/panels/help_teatest_test.go` — Teatest snapshot test for `HelpOverlay`.
- `internal/ui/panels/newrun_teatest_test.go` — Teatest snapshot and workflow tests for `NewRunModal`.
- `internal/ui/panels/statusbar_teatest_test.go` — Teatest snapshot tests for `StatusBar`.
- `internal/ui/panels/diffview_teatest_test.go` — Teatest snapshot tests for `DiffView`.

## Implementation Plan

### Phase 1: Foundation

Add the `teatest` dependency and establish the test infrastructure: shared helpers, golden file conventions, `.gitattributes` for golden file line-ending safety, and a Makefile recipe for updating snapshots.

### Phase 2: Core Implementation

Write teatest-based tests for each panel and modal in isolation. Each component gets snapshot tests for its key visual states and interaction tests for multi-step user flows that are awkward to test with manual `Update()` calls.

### Phase 3: Integration

Write app-level integration tests that exercise cross-component flows through the full `App` model: navigation between panels, modal open/close cycles, store updates propagating to all panels, and key routing with focus state.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add teatest dependency

- Run `go get github.com/charmbracelet/x/exp/teatest@latest` to add the dependency to `go.mod` and `go.sum`.

### 2. Configure golden file handling

- Add a `.gitattributes` entry to prevent git from normalizing line endings in golden files:
  ```
  **/testdata/**/*.golden binary
  ```
- Add a Makefile recipe for updating golden files:
  ```makefile
  update-golden:
  	go test ./internal/ui/... -update
  ```

### 3. Create shared test helpers for the panels package

Create `internal/ui/panels/teatest_helpers_test.go` with:

- A `newTestProgram(tb testing.TB, m tea.Model) *teatest.TestModel` helper that wraps `teatest.NewTestModel` with a fixed terminal size (120x40) to ensure deterministic output.
- A `waitForRender(tb, tm)` helper that calls `teatest.WaitFor` with a condition that checks for a non-empty frame containing expected border characters (`╭`), with a short timeout (2s).
- Re-export the existing `testStore()` fixture for use in teatest tests.

### 4. Write RunList teatest tests

Create `internal/ui/panels/runlist_teatest_test.go`:

- **`TestRunListSnapshot`** — Create a `RunList` with `testStore()`, set size, render via teatest, assert golden file with `RequireEqualOutput`. Captures the default view with 4 runs showing icons, branch names, workflow names, cost, and token counts.
- **`TestRunListEmptySnapshot`** — Empty store, assert golden file shows "No runs" message.
- **`TestRunListNavigationFlow`** — Use `Send()` to send j/j/k keys, use `WaitFor` to verify the correct run is highlighted after each step.
- **`TestRunListFilterFlow`** — Send `/`, type "feat", verify filtered output shows only matching runs. Send Esc, verify all runs restored.
- **`TestRunListScrollSnapshot`** — 20 runs in a small viewport (60x8), navigate to item 10, snapshot the scrolled state.

### 5. Write Detail teatest tests

Create `internal/ui/panels/detail_teatest_test.go`:

- **`TestDetailWithRunSnapshot`** — Set a run with all fields populated (branch, workflow, state, tokens, cost, elapsed, error), snapshot the rendered detail view.
- **`TestDetailNoRunSnapshot`** — No run selected, snapshot the "No run selected" empty state.
- **`TestDetailStateVariations`** — Table-driven test with runs in each state (running, paused, reviewing, completed, failed, cancelled), snapshot each.

### 6. Write LogView teatest tests

Create `internal/ui/panels/logview_teatest_test.go`:

- **`TestLogViewEmptySnapshot`** — No run set, snapshot the empty log panel.
- **`TestLogViewWithContentSnapshot`** — Set a run with log buffer content, snapshot the log tab.
- **`TestLogViewTabSwitching`** — Send `l` to switch tabs (log → diff → details), use `WaitFor` to verify each tab renders.
- **`TestLogViewNavigationFlow`** — Send j/k/G/gg within log content, verify viewport scrolls.
- **`TestLogViewSearchFlow`** — Send `/`, type a search term, send `n`/`N` for next/prev match, verify highlighting.

### 7. Write DiffView teatest tests

Create `internal/ui/panels/diffview_teatest_test.go`:

- **`TestDiffViewWithDiffSnapshot`** — Set diff content with added/removed lines, snapshot the rendered diff with syntax highlighting.
- **`TestDiffViewLoadingSnapshot`** — Call `SetLoading()`, snapshot the loading state.
- **`TestDiffViewEmptySnapshot`** — Call `SetEmpty()`, snapshot the empty state.
- **`TestDiffViewFileNavigation`** — Set a multi-file diff, send `]`/`[` to navigate between files, verify file indicator updates.

### 8. Write HelpOverlay teatest test

Create `internal/ui/panels/help_teatest_test.go`:

- **`TestHelpOverlaySnapshot`** — Snapshot the full help overlay. This is a static view so one golden file covers it. Any keybind additions/removals will break this test intentionally.

### 9. Write NewRunModal teatest tests

Create `internal/ui/panels/newrun_teatest_test.go`:

- **`TestNewRunModalDefaultSnapshot`** — Snapshot the default state with cursor in prompt, default workflow and model selections.
- **`TestNewRunModalWorkflowSelection`** — Send Ctrl+B, Ctrl+P, Ctrl+L to select different workflows, snapshot after each to verify the selected option is highlighted.
- **`TestNewRunModalSubmitFlow`** — Type a prompt, select a workflow, send Ctrl+S, verify the model returns `nil` (dismissed) and `SubmitNewRunMsg` is produced.

### 10. Write StatusBar teatest tests

Create `internal/ui/panels/statusbar_teatest_test.go`:

- **`TestStatusBarSnapshot`** — Snapshot the status bar with a store containing runs with various costs/tokens to verify aggregation display.
- **`TestStatusBarFlashSnapshot`** — Set a flash message, snapshot to verify it appears.

### 11. Create shared test helpers for the ui package

Create `internal/ui/teatest_helpers_test.go` with:

- A `newTestAppModel(tb testing.TB) *teatest.TestModel` helper that creates an `App` with `DefaultConfig`, dismisses `initPrompt`, and wraps it in `teatest.NewTestModel` with fixed terminal size.

### 12. Write App integration teatest tests

Create `internal/ui/app_teatest_test.go`:

- **`TestAppInitialRenderSnapshot`** — Snapshot the full app after receiving `WindowSizeMsg`. Captures the three-panel layout with status bar.
- **`TestAppHelpModalFlow`** — Send `?`, wait for help overlay to render, snapshot. Send Esc, wait for overlay to disappear, snapshot the restored state.
- **`TestAppNewRunModalFlow`** — Send `n`, wait for modal, snapshot. Send Esc, verify modal dismissed.
- **`TestAppFocusCycleVisual`** — Send Tab three times, snapshot after each to verify focus indicator moves between panels.
- **`TestAppNavigationWithRuns`** — Add runs to store, send `RunStoreUpdatedMsg`, navigate with j/k, verify detail panel updates to reflect selected run.
- **`TestAppStoreUpdatePropagation`** — Add a run, send `RunStoreUpdatedMsg`, verify all three panels reflect the new run (run list shows it, detail shows its info, log view is ready).

### 13. Generate initial golden files

- Run `go test ./internal/ui/... -update` to generate all golden files in `testdata/` directories.
- Review each golden file manually to verify correctness before committing.
- Commit the golden files as binary (per `.gitattributes`).

### 14. Verify full test suite passes

- Run `go test ./internal/ui/...` without `-update` to confirm all tests pass against the golden files.
- Run `go vet ./...` to verify no lint issues.

## Testing Strategy

### Unit Tests

Existing unit tests in `*_test.go` files remain unchanged. They continue to test model logic, state transitions, and message handling at the struct level. The new teatest files (`*_teatest_test.go`) run alongside them.

### Snapshot Tests

Each component gets golden file snapshots for its key visual states. Golden files live in `testdata/` directories adjacent to the test files (Go test convention). The `-update` flag regenerates them. CI runs tests without `-update` to catch regressions.

### Integration Tests

App-level tests exercise multi-component flows: focus changes, modal lifecycles, store updates propagating across panels. These use `Send()` for input and `WaitFor()` for output assertions rather than golden files, since full-app output is large and sensitive to minor changes.

### Edge Cases

- Terminal size too small (below minimum threshold) — verify "too small" message renders
- Empty store — verify all panels handle zero runs gracefully
- Rapid key sequences — verify no panics or state corruption
- Large run counts — verify scrolling and viewport management

## Risk Assessment

- **Golden file maintenance burden** — Intentional UI changes require running `-update` and reviewing diffs. Mitigated by keeping golden files granular (per-component, per-state) so changes are isolated.
- **Environment sensitivity** — Terminal color profiles can change ANSI output. Mitigated by teatest running in a headless mode with no real terminal. If flakes appear, set `TERM=dumb` in CI.
- **teatest is experimental** — The package lives under `x/exp/`. API could change. Mitigated by isolating all teatest usage behind helper functions in `*_helpers_test.go` so migration is localized.
- **Existing tests unaffected** — New files use `_teatest_test.go` suffix and don't modify any existing test files.

## Validation Commands

```bash
go get github.com/charmbracelet/x/exp/teatest@latest
go test ./internal/ui/... -update       # generate golden files
go test ./internal/ui/...               # verify against golden files
go test ./...                           # full suite
go vet ./...                            # lint
```

## Open Questions (Unresolved)

- **Color profile for golden files** — Should golden files be generated with `TERM=dumb` (no ANSI colors) for maximum portability, or with a specific color profile to also catch styling regressions? Recommendation: start with `TERM=dumb` for stability, add a separate color-aware test target later if needed.
- **Panel-level vs app-level teatest** — Panels don't implement `Init()` and aren't designed to run as standalone `tea.Program`s. They'll need thin wrapper models that implement `Init()` (returning `nil`) and delegate `Update`/`View`. This is a small amount of test-only boilerplate. Recommendation: create a generic `panelWrapper[T tea.Model]` in the helpers file.

## Sub-Tasks

Single task — no decomposition needed.
