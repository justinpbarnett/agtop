# Feature: Init Prompt Modal on Startup

## Metadata

type: `feat`
task_id: `init-prompt-modal`
prompt: `if the user opens up agtop and no config file has been created, prompt the user via a modal asking yes (enter) no (esc) to run agtop init and create a project-specific config via init. this modal should pop-up every time the user opens agtop in a dir that has no config file in it.`

## Feature Description

When agtop starts in a directory without an `agtop.yaml` config file (and no user-level config at `~/.config/agtop/config.yaml`), the app currently proceeds silently with built-in defaults. There is no signal to the user that initialization is available or recommended.

This feature adds a lightweight confirmation modal that appears on startup when no config file is detected. The modal asks whether to run `agtop init`, which creates safety hooks (`.agtop/hooks/safety-guard.sh`), wires Claude Code settings (`.claude/settings.json` PreToolUse hook), and copies `agtop.example.yaml` to `agtop.yaml`. Pressing Enter/Y accepts and runs init inline; Esc/N dismisses the modal and continues with defaults.

The modal appears every time agtop launches in a directory without a config file — there is no "don't ask again" persistence.

## User Story

As a developer launching agtop in a new project
I want to be prompted to run initialization when no config exists
So that I discover and set up safety hooks and project config without memorizing subcommands

## Relevant Files

- `internal/config/loader.go` — Config discovery logic. `discoverConfigPath()` (line 51) returns `""` when no config file exists. Need to export a function to check for local config presence.
- `internal/ui/app.go` — App struct (line 47), `NewApp()` (line 73), `Init()` (line 199), `Update()` (line 203), `View()` (line 454). The init prompt modal integrates here following the same pattern as `helpOverlay` and `newRunModal`.
- `internal/ui/panels/help.go` — The simplest modal pattern in the codebase (71 lines). The init prompt modal should follow this structure: minimal state, key handling, `border.RenderPanel()` for rendering.
- `internal/ui/panels/messages.go` — Message definitions. Needs new `InitAcceptedMsg` type.
- `internal/ui/messages.go` — Type aliases from `panels` to `ui` package. Needs alias for the new message.
- `cmd/agtop/init.go` — `runInit()` function (line 13). The init logic lives here but is tied to `main` package and writes to stdout. The TUI needs to run equivalent logic without stdout prints.
- `internal/ui/border/panel.go` — `RenderPanel()` used by all modals for bordered rendering.
- `internal/ui/styles/theme.go` — Existing styles (`TitleStyle`, `TextPrimaryStyle`, `SelectedOptionStyle`) are sufficient for the modal.

### New Files

- `internal/ui/panels/initprompt.go` — Init prompt modal component (~60 lines). Simple overlay following the `HelpOverlay` pattern.
- `internal/ui/panels/initprompt_test.go` — Tests for the init prompt modal.

## Implementation Plan

### Phase 1: Config Detection

Export a function from the config package to check whether a local project config file exists. This is distinct from `Load()` which silently falls back to defaults — the UI needs to know specifically whether the local file is missing so it can prompt.

### Phase 2: Init Logic Extraction

Extract the core init logic from `cmd/agtop/init.go` into an internal package so it can be called from both the CLI subcommand and the TUI modal handler. The extracted function should return structured results instead of printing to stdout.

### Phase 3: Modal Component

Create the init prompt modal following the `HelpOverlay` pattern. Fixed-size centered overlay with a message and two keybinds. No text input, no complex state — just render and handle Enter/Esc.

### Phase 4: App Integration

Wire the modal into the App lifecycle: show it on startup when config is missing, route keys to it when active, handle the accept message by running init, dismiss on decline.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `LocalConfigExists` to config package

- In `internal/config/loader.go`, add an exported function:
  ```go
  func LocalConfigExists() bool {
      cwd, err := os.Getwd()
      if err != nil {
          return false
      }
      _, err = os.Stat(filepath.Join(cwd, "agtop.yaml"))
      return err == nil
  }
  ```
- This checks only the local `agtop.yaml` — the user-level config at `~/.config/agtop/config.yaml` should not suppress the prompt since it doesn't indicate the project has been initialized.

### 2. Extract init logic into internal package

- Create a new exported function in a suitable location (e.g., add `RunInit` to `internal/config/` or create a small init helper in `cmd/agtop/init.go` that returns results).
- Alternatively, since the init logic touches safety, settings, and file I/O across packages, the simplest approach is to keep `runInit` in `cmd/agtop/init.go` and have the TUI shell out to `agtop init` via `os/exec`. This avoids extracting cross-package logic and reuses the existing CLI path.
- The TUI will run `exec.Command("agtop", "init")` in a goroutine and report success/failure via a message.

### 3. Add `InitAcceptedMsg` message type

- In `internal/ui/panels/messages.go`, add:
  ```go
  // InitAcceptedMsg signals the user accepted the init prompt.
  type InitAcceptedMsg struct{}
  ```
- In `internal/ui/messages.go`, add the type alias:
  ```go
  type InitAcceptedMsg = panels.InitAcceptedMsg
  ```

### 4. Create init prompt modal component

- Create `internal/ui/panels/initprompt.go` following the `HelpOverlay` pattern:
  ```go
  type InitPrompt struct {
      width  int
      height int
  }
  ```
- `NewInitPrompt()` returns a pointer with fixed dimensions (width ~50, height ~7).
- `Update(msg tea.Msg)` handles:
  - `enter`, `y`, `Y` → return `InitAcceptedMsg{}`
  - `esc`, `n`, `N` → return `CloseModalMsg{}`
- `View()` renders a bordered panel with:
  - Title: `"Initialize Project"`
  - Body: `"No agtop.yaml found in this directory.\n\nRun agtop init to set up hooks and config?"`
  - Bottom keybinds: `[Enter] Yes` and `[Esc] No`
- Use `border.RenderPanel()` with `focused: true` for the border.
- Use `styles.TitleStyle` for emphasis, `styles.TextPrimaryStyle` for body text.

### 5. Wire modal into App

- In `internal/ui/app.go`, add field to `App` struct:
  ```go
  initPrompt *panels.InitPrompt
  ```
- In `NewApp()` (after line 196, before the return), check config:
  ```go
  if !config.LocalConfigExists() {
      app.initPrompt = panels.NewInitPrompt()
  }
  ```
  (Use a local variable for the app, then return it.)
- In `Update()`, add key routing for the init prompt before the helpOverlay check (line 345):
  ```go
  if a.initPrompt != nil {
      var cmd tea.Cmd
      *a.initPrompt, cmd = a.initPrompt.Update(msg)
      return a, cmd
  }
  ```
- Handle `InitAcceptedMsg` in the `Update()` switch:
  ```go
  case InitAcceptedMsg:
      a.initPrompt = nil
      a.statusBar.SetFlash("Running agtop init...")
      return a, tea.Batch(a.runInitCmd(), flashClearCmd())
  ```
- Handle `CloseModalMsg` — the existing handler (line 216) already sets `nil` on modals. Add `a.initPrompt = nil` there too.
- In `View()`, add rendering for the init prompt (before the helpOverlay block, line 478):
  ```go
  if a.initPrompt != nil {
      modalView := a.initPrompt.View()
      fullLayout = lipgloss.Place(a.width, a.height,
          lipgloss.Center, lipgloss.Center, modalView,
          lipgloss.WithWhitespaceChars(" "),
          lipgloss.WithWhitespaceForeground(styles.TextDim),
      )
  }
  ```
- In `WindowSizeMsg` handler, propagate size to init prompt if non-nil.

### 6. Implement async init execution

- Add a method to `App` that runs `agtop init` via `os/exec`:
  ```go
  type InitResultMsg struct {
      Err error
  }

  func (a App) runInitCmd() tea.Cmd {
      return func() tea.Msg {
          cmd := exec.Command("agtop", "init")
          cmd.Dir, _ = os.Getwd()
          output, err := cmd.CombinedOutput()
          if err != nil {
              return InitResultMsg{Err: fmt.Errorf("%v: %s", err, output)}
          }
          return InitResultMsg{}
      }
  }
  ```
- Handle `InitResultMsg` in `Update()`:
  ```go
  case InitResultMsg:
      if msg.Err != nil {
          a.statusBar.SetFlash(fmt.Sprintf("Init failed: %v", msg.Err))
      } else {
          a.statusBar.SetFlash("agtop init complete")
      }
      return a, flashClearCmd()
  ```

### 7. Write tests for init prompt modal

- Create `internal/ui/panels/initprompt_test.go`:
  - Test that Enter key returns `InitAcceptedMsg`
  - Test that `y` key returns `InitAcceptedMsg`
  - Test that Esc key returns `CloseModalMsg`
  - Test that `n` key returns `CloseModalMsg`
  - Test that other keys produce no command
  - Test that `View()` returns non-empty string containing expected text

### 8. Verify integration

- Run `go vet ./...` to check for compile errors
- Run `go test ./...` to verify all tests pass
- Manual test: remove `agtop.yaml` from a project dir, run `agtop`, confirm modal appears
- Manual test: press Esc — modal dismisses, app continues normally
- Manual test: press Enter — init runs, flash shows success, `agtop.yaml` is created

## Testing Strategy

### Unit Tests

- `internal/ui/panels/initprompt_test.go` — Key handling (Enter→accept, Esc→close, y→accept, n→close, other→noop), view rendering contains expected text
- `internal/config/loader_test.go` — Add test for `LocalConfigExists()`: returns false when no file, true when file exists

### Edge Cases

- Terminal too small for modal: `border.RenderPanel` already handles small dimensions by returning empty string
- `agtop` binary not in PATH when running init from TUI: `InitResultMsg.Err` will be set, flash shows error
- Config file created between app start and modal accept: `agtop init` is idempotent, so double-init is safe
- User-level config exists but no local config: modal still shows (intentional — local init creates safety hooks)

## Risk Assessment

- **Low risk**: The modal is a self-contained overlay with no impact on existing functionality. Dismissing it (Esc) has zero side effects.
- **Init is idempotent**: Running `agtop init` when already initialized is safe — it checks for existing files before creating.
- **os/exec dependency**: Shelling out to `agtop init` requires the binary to be in PATH. If run via `go run`, the `agtop` binary won't exist. This is acceptable since the modal is for installed usage.

## Validation Commands

```bash
go vet ./...
go test ./internal/ui/panels/ -run TestInitPrompt
go test ./internal/config/ -run TestLocalConfigExists
go test ./...
```

## Open Questions (Unresolved)

1. **Should the modal suppress when user-level config exists?** Currently the spec checks only for local `agtop.yaml`. A user-level `~/.config/agtop/config.yaml` provides runtime config but not safety hooks or `.claude/settings.json`. Recommendation: always show the modal when local `agtop.yaml` is missing, since init creates project-level artifacts beyond just config.

2. **Should init be run in-process instead of via os/exec?** Extracting `runInit()` into an internal package avoids the PATH dependency but requires refactoring cross-package init logic. Recommendation: start with `os/exec` for simplicity; refactor to in-process later if the PATH issue causes friction.

## Sub-Tasks

Single task — no decomposition needed.
