# Feature: First-Run Onboarding Flow

## Metadata

type: `feat`
task_id: `onboarding-flow`
prompt: `An onboarding flow that triggers on first startup when no agtop.toml exists. Instead of writing both .claude/settings.json and opencode.json unconditionally, it should guide the user through selecting their runtime (claude or opencode), then only set up permissions for the chosen runtime. Should replace the current agtop init behavior and integrate into the normal startup path.`

## Feature Description

Currently `agtop init` unconditionally writes permissions for **both** Claude Code (`.claude/settings.json`) and OpenCode (`opencode.json`), even though a user only ever uses one runtime. This is wasteful and clutters projects with unnecessary config files.

The onboarding flow replaces the current simple yes/no `InitPrompt` modal with a multi-step setup wizard that:
1. Detects available runtimes and project context (static detection)
2. Asks the user to pick their runtime (claude or opencode)
3. Writes **only** the permissions config for the chosen runtime
4. Generates `agtop.toml` with the selected runtime as default

This runs inline in the TUI on first startup — no more shelling out to `agtop init` as a subprocess. The `agtop init` CLI subcommand remains as a manual re-init path but is updated to accept the same runtime selection.

## User Story

As a developer launching agtop for the first time
I want a guided setup that asks which runtime I use
So that only the relevant permissions and config files are created

## Relevant Files

- `internal/ui/panels/initprompt.go` — Current simple yes/no modal. Will be replaced with a multi-step onboarding modal.
- `internal/ui/panels/initprompt_test.go` — Tests for the current modal. Must be rewritten to cover the new flow.
- `internal/ui/panels/messages.go` — Message types. `InitAcceptedMsg` will carry runtime selection data.
- `internal/ui/app.go` — Handles `InitAcceptedMsg` and `InitResultMsg`. Must be updated for the new in-process init (no more subprocess).
- `cmd/agtop/init.go` — The `runInit()` function and config/settings merge helpers. Needs to be refactored so the core logic is callable from the TUI (not just the CLI).
- `internal/safety/hooks.go` — `GenerateSettings()` (Claude) and `GenerateOpenCodeSettings()` (OpenCode). Called conditionally based on runtime choice.
- `internal/detect/detect.go` — Static project detection (runtime, language, test command, dev server). Already used by init; will be called from the onboarding flow.
- `internal/config/loader.go` — `LocalConfigExists()` check that triggers the modal.

### New Files

- `internal/init/init.go` — Extracted init logic as a library package so both the CLI subcommand and TUI onboarding can call it. Contains `Run(opts Options)` that takes a runtime choice and performs setup.

## Implementation Plan

### Phase 1: Extract init logic into a shared package

Move the core init logic out of `cmd/agtop/init.go` into `internal/init/init.go` so it can be called from both the CLI and the TUI without shelling out to a subprocess. The function takes an `Options` struct with the selected runtime, whether to use AI detection, and the project root.

### Phase 2: Onboarding modal

Replace `InitPrompt` with a new multi-step `OnboardingModal` component:
- **Step 1: Runtime selection** — Show detected runtimes (auto-detected via `exec.LookPath`), let user pick with j/k + Enter. If only one runtime is found, pre-select it.
- **Step 2: Confirm** — Show a summary of what will be created (hooks, permissions file, agtop.toml) and the detected project info (name, language, test command). Enter to proceed, Esc to go back.

### Phase 3: Wire into the app

Update `App` to:
- Create the new `OnboardingModal` instead of `InitPrompt` when no config exists
- Handle the new `InitAcceptedMsg` (now carrying runtime choice) by calling `init.Run()` in-process
- Remove `runInitCmd()` subprocess invocation

### Phase 4: Update CLI init

Update `cmd/agtop/init.go` to call `init.Run()` from the shared package. Add a `--runtime` flag so CLI users can also specify their runtime without being prompted.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Create `internal/init/init.go` — extract init logic

- Create the `init` package with an `Options` struct:
  ```go
  type Options struct {
      Runtime string // "claude" or "opencode"
      UseAI   bool
      Root    string // project root directory
  }
  ```
- Extract the core logic from `cmd/agtop/init.go:runInit()` into `func Run(cfg *config.Config, opts Options) error`
- The function should:
  - Always create `.agtop/hooks/` and `safety-guard.sh` (shared across runtimes)
  - If `opts.Runtime == "claude"`: merge `.claude/settings.json` only
  - If `opts.Runtime == "opencode"`: merge `opencode.json` only
  - Create `agtop.toml` with the selected runtime as default
- Move `mergeSettings()`, `mergeOpenCodeConfig()`, `renderConfig()`, and related helpers into this package
- Keep `cmd/agtop/init.go` as a thin wrapper that calls `init.Run()`

### 2. Build the `OnboardingModal` component

- Replace `internal/ui/panels/initprompt.go` with a new `OnboardingModal` struct:
  ```go
  type OnboardingModal struct {
      step     int           // 0 = runtime selection, 1 = confirm
      runtimes []string      // detected available runtimes
      selected int           // cursor index in runtime list
      detected *detect.Result // static detection results
      width    int
      height   int
  }
  ```
- Constructor `NewOnboardingModal()` runs static detection (`detect.Detect(".")`) and `exec.LookPath` for claude/opencode to populate the runtime list
- **Step 0 — Runtime selection view:**
  - Title: "Setup agtop"
  - Body: "Select your AI coding runtime:" with j/k navigation, Enter to confirm
  - Show each runtime with a selection indicator (e.g., `> claude   opencode`)
- **Step 1 — Confirm view:**
  - Title: "Setup agtop"
  - Body: Summary of what will be created:
    - Selected runtime
    - Detected project name, language, test command
    - Files to be created: `.agtop/hooks/safety-guard.sh`, permissions file, `agtop.toml`
  - Keybinds: Enter to proceed, Esc/Backspace to go back
- Update `Update()` to handle step transitions and emit `InitAcceptedMsg{Runtime: selected}` on final confirm
- Update `View()` to render the current step

### 3. Update message types

- In `internal/ui/panels/messages.go`, update `InitAcceptedMsg` to carry the runtime selection:
  ```go
  type InitAcceptedMsg struct {
      Runtime string
  }
  ```

### 4. Wire into App

- In `internal/ui/app.go`:
  - Replace `initPrompt *panels.InitPrompt` with `onboarding *panels.OnboardingModal`
  - At line 260-262: create `OnboardingModal` instead of `InitPrompt`
  - Update the `InitAcceptedMsg` handler (line 308-311) to call `init.Run()` in-process using the runtime from the message, instead of `runInitCmd()` subprocess
  - After successful init, reload config with `config.Load()` so the app picks up the new settings
  - Remove `runInitCmd()` function (line 1289-1299)
  - Update `CloseModalMsg` handler to clear `onboarding` instead of `initPrompt`
  - Update the view rendering to use `onboarding` instead of `initPrompt`

### 5. Update CLI init subcommand

- In `cmd/agtop/init.go`:
  - Replace `runInit()` body with a call to `init.Run(cfg, opts)`
  - Add `--runtime` flag (string, default empty — meaning auto-detect/prompt)
  - If `--runtime` not specified on CLI, default to detected runtime or "claude"
  - Remove duplicated helper functions that moved to `internal/init/`

### 6. Update tests

- Rewrite `internal/ui/panels/initprompt_test.go` for the new `OnboardingModal`:
  - Test runtime selection with j/k navigation
  - Test Enter advances from step 0 to step 1
  - Test Enter on step 1 emits `InitAcceptedMsg` with selected runtime
  - Test Esc on step 0 emits `CloseModalMsg`
  - Test Esc/Backspace on step 1 goes back to step 0
  - Test view contains expected text for each step
- Add unit tests for `internal/init/init.go`:
  - Test that claude runtime only creates `.claude/settings.json`
  - Test that opencode runtime only creates `opencode.json`
  - Test that `agtop.toml` uses the selected runtime as default
  - Test merge functions preserve existing user config

## Testing Strategy

### Unit Tests

- `internal/init/init_test.go` — Test `Run()` with each runtime option in a temp directory. Verify only the correct permissions file is created.
- `internal/ui/panels/initprompt_test.go` (renamed or replaced) — Test `OnboardingModal` step transitions, key handling, and message emission.

### Edge Cases

- Only one runtime binary available — pre-select it, skip step 0 if only one option
- Neither runtime available — show error message in the modal instead of runtime selection
- Existing `opencode.json` or `.claude/settings.json` — merge functions must preserve user entries (already tested)
- User declines onboarding (Esc on step 0) — app continues with defaults, no files written

## Risk Assessment

- **Behavioral change**: The init prompt is no longer a simple yes/no — users who muscle-memory press Enter will now land on a runtime selection screen. Low risk since this only fires once per project.
- **Package naming**: `internal/init` shadows the Go builtin `init`. Using a different name like `internal/setup` or `internal/bootstrap` may be necessary to avoid confusion. **Recommendation: use `internal/setup`.**
- **Subprocess removal**: Removing `runInitCmd()` means init no longer runs in a separate process. This is safer (no PATH issues with finding the `agtop` binary) but means init errors surface differently (in-process vs exit code).

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go build ./...
go vet ./...
go test ./...
```

## Open Questions (Unresolved)

1. **Package name for extracted init logic** — `internal/init` shadows Go's `init` builtin. Recommend using `internal/setup` instead. Functionally identical, avoids confusion.

2. **Should we skip step 0 when only one runtime is detected?** — Recommend yes: if only `claude` is in PATH, pre-select it and start on the confirm step. Still show which runtime was auto-detected so the user knows.

3. **Should `agtop init` CLI also become interactive?** — Recommend no: keep CLI non-interactive with the `--runtime` flag. Default to auto-detected runtime when flag is omitted. Interactive onboarding stays in the TUI only.

## Sub-Tasks

Single task — no decomposition needed.
