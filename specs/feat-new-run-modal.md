# Feature: New Run Modal

## Metadata

type: `feat`
task_id: `new-run-modal`
prompt: `Implement the new run modal (step 13 of agtop.md): n keybind triggers a lazygit-style modal with text input for task description, hotkey-driven workflow selection (b=build, p=plan-build, s=sdlc, q=quick-fix), optional model override hotkey, Enter to confirm, Esc to cancel. On confirm: create worktree, init run, start executor.`

## Feature Description

The new run modal is a centered, keyboard-driven overlay that lets developers launch a new AI agent workflow in a single fluid interaction. Pressing `n` opens the modal, the user types a task description, presses a single hotkey to select a workflow, optionally presses a hotkey to override the model, then `Enter` to launch. The modal follows the lazygit design language: simple, centered, no field tabbing, entirely driven by hotkeys and a single text input. On confirm, the modal emits a `StartRunMsg` that triggers the existing worktree creation → run initialization → executor start pipeline.

Currently, the `n` key fires a hardcoded `StartRunMsg{Prompt: "placeholder task", Workflow: "build"}` stub. This feature replaces that stub with a proper interactive modal.

## User Story

As a developer managing concurrent AI agent workflows
I want to launch a new run from a quick keyboard-driven modal
So that I can specify a task and workflow without leaving the TUI or navigating complex forms

## Problem Statement

The `n` keybind in `internal/ui/app.go` (line 287) currently emits a hardcoded `StartRunMsg` with `"placeholder task"` as the prompt and `"build"` as the workflow. There is no way for the user to:

1. **Type a task description** — the prompt that drives the entire agent run is hardcoded.
2. **Choose a workflow** — every run starts with the `build` workflow regardless of the task.
3. **Override the model** — the default model from config is always used, with no per-run override.

The existing `HelpOverlay` in `internal/ui/panels/help.go` demonstrates the modal pattern (pointer field on `App`, key event interception, `CloseModalMsg`, centered rendering via `lipgloss.Place`), but there is no modal that accepts user input.

## Solution Statement

1. **Create a `NewRunModal` component** (`internal/ui/panels/newrun.go`) with a `textinput.Model` for the task prompt, hotkey-driven workflow selection, optional model override, and `Enter`/`Esc` handling.

2. **Wire it into `App`** alongside the existing `helpOverlay` pattern: store as a `*panels.NewRunModal` field, intercept all key events when open, render as a centered overlay.

3. **Replace the `n` stub** with opening the modal. On `Enter`, the modal emits a `StartRunMsg` with the user's prompt, selected workflow, and model override. The existing `StartRunMsg` handler creates the worktree, initializes the run, and starts the executor — no changes needed there.

4. **Add `Prompt` field to `Run`** so the task description persists on the run for display in the detail panel and potential resume.

## Relevant Files

Use these files to implement the feature:

- `internal/ui/app.go` — Add `newRunModal *panels.NewRunModal` field. Change `n` handler from stub to opening the modal. Add key routing when modal is open (same pattern as `helpOverlay`). Render modal overlay in `View()`. Update `StartRunMsg` to include `Model` field and pass it to the run/executor.
- `internal/ui/panels/help.go` — Reference for modal pattern (struct, Update, View, CloseModalMsg). No changes needed.
- `internal/ui/panels/messages.go` — Add `SubmitNewRunMsg` for the modal to signal run creation with prompt, workflow, and model.
- `internal/ui/messages.go` — Add type alias for `SubmitNewRunMsg`.
- `internal/ui/border/panel.go` — Reference for `RenderPanel()` API used to render the modal frame. No changes needed.
- `internal/ui/border/keybind.go` — Reference for `Keybind` struct. No changes needed.
- `internal/ui/styles/colors.go` — Add a `SelectedOption` color for highlighting the active workflow/model selection.
- `internal/ui/styles/theme.go` — Add `SelectedOptionStyle` for the active workflow/model highlight.
- `internal/run/run.go` — Add `Prompt string` field to the `Run` struct.
- `internal/ui/panels/detail.go` — Display `Run.Prompt` in the details tab.
- `internal/config/config.go` — Reference for `WorkflowConfig` and `ClaudeConfig.Model`. No changes needed.
- `internal/config/defaults.go` — Reference for default workflows (build, plan-build, sdlc, quick-fix) and default model. No changes needed.

### New Files

- `internal/ui/panels/newrun.go` — New run modal component with text input, workflow selection, model override, and submit/cancel logic.
- `internal/ui/panels/newrun_test.go` — Unit tests for the modal: workflow selection, model override, submit, cancel, validation, edge cases.

## Implementation Plan

### Phase 1: Modal Component

Create the `NewRunModal` struct with a `textinput.Model`, workflow selection state, and model override state. Implement `Update()` for key routing (text input, workflow hotkeys, model hotkeys, Enter, Esc) and `View()` for rendering the modal content. The modal is self-contained and testable independently.

### Phase 2: App Integration

Wire the modal into `App`: add the pointer field, change the `n` handler to open the modal, add key interception when the modal is open, render the overlay in `View()`. Update `StartRunMsg` to carry a `Model` field and update the `StartRunMsg` handler to store the prompt on the run and pass the model override through.

### Phase 3: Run Prompt Persistence

Add `Prompt` field to `Run`, set it in the `StartRunMsg` handler, and display it in the detail panel's Details tab.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add SelectedOption Color and Style

- Add to `internal/ui/styles/colors.go`:
  - `SelectedOption` — green adaptive color (Light: `"#1a7f37"`, Dark: `"#9ece6a"`) to indicate the active workflow/model selection
- Add to `internal/ui/styles/theme.go`:
  - `SelectedOptionStyle` — `lipgloss.NewStyle().Foreground(SelectedOption).Bold(true)`

### 2. Add SubmitNewRunMsg

- Add to `internal/ui/panels/messages.go`:
  ```go
  // SubmitNewRunMsg is sent when the user confirms the new run modal.
  type SubmitNewRunMsg struct {
      Prompt   string
      Workflow string
      Model    string
  }
  ```
- Add type alias to `internal/ui/messages.go`:
  ```go
  type SubmitNewRunMsg = panels.SubmitNewRunMsg
  ```

### 3. Create NewRunModal Component

- Create `internal/ui/panels/newrun.go` with:
  - `NewRunModal` struct fields:
    - `promptInput textinput.Model` — task description input
    - `workflow string` — selected workflow, default `"build"`
    - `model string` — selected model override, default `""` (use config default)
    - `width int` — modal width (56)
    - `height int` — modal height (18)
  - `NewNewRunModal() *NewRunModal`:
    - Initialize `textinput.New()` with placeholder `"Describe the task..."`, char limit 256, width fitting inside the modal (width - 6 for borders + padding)
    - Focus the text input immediately
    - Set `workflow` to `"build"` as default
    - Return pointer
  - `Update(msg tea.Msg) (*NewRunModal, tea.Cmd)`:
    - On `tea.KeyMsg`:
      - `esc`: return `nil, closeModalCmd()` — returning nil signals the modal is dismissed
      - `enter`: validate prompt is non-empty, then return `nil, submitCmd()` where `submitCmd` sends `SubmitNewRunMsg{Prompt, Workflow, Model}`
      - `ctrl+b`: set `workflow = "build"`
      - `ctrl+p`: set `workflow = "plan-build"`
      - `ctrl+s`: set `workflow = "sdlc"`
      - `ctrl+q`: set `workflow = "quick-fix"`
      - `ctrl+h`: set `model = "haiku"`
      - `ctrl+o`: set `model = "opus"`
      - `ctrl+n`: set `model = "sonnet"`
      - `ctrl+x`: set `model = ""` (clear override, use default)
      - All other keys: route to `promptInput.Update(msg)`
    - On other `tea.Msg` types: route to `promptInput.Update(msg)` (for blink timer, etc.)
  - `View() string`:
    - Build content with sections:
      - **Prompt line**: render `promptInput.View()`
      - **Blank separator line**
      - **Workflow line**: `"Workflow: "` followed by the four options, each rendered as `[^b] build` with the selected one in `SelectedOptionStyle` and others in `TextDimStyle`. Format: `[^b] build  [^p] plan  [^s] sdlc  [^q] quick`
      - **Model line**: `"Model:    "` followed by model options: `[^h] haiku  [^o] opus  [^n] sonnet  [^x] default`. Selected in `SelectedOptionStyle`, others in `TextDimStyle`. If model is `""`, highlight "default".
    - Bottom keybinds: `{Key: "Enter", Label: " submit"}`, `{Key: "Esc", Label: " cancel"}`
    - Return `border.RenderPanel("New Run", content, keybinds, width, height, true)`
  - Helper `closeModalCmd() tea.Cmd` — returns a `tea.Cmd` that sends `CloseModalMsg{}`
  - Helper `submitCmd(prompt, workflow, model string) tea.Cmd` — returns a `tea.Cmd` that sends `SubmitNewRunMsg{Prompt, Workflow, Model}`

### 4. Wire Modal into App

- Add field to `App` struct: `newRunModal *panels.NewRunModal`
- Update `Update()` key routing — add a check for `newRunModal` **after** the `helpOverlay` check and **before** the search-mode check:
  ```go
  if a.newRunModal != nil {
      var cmd tea.Cmd
      a.newRunModal, cmd = a.newRunModal.Update(msg)
      if a.newRunModal == nil {
          // Modal was dismissed (nil return)
      }
      return a, cmd
  }
  ```
  Note: The modal's `Update` returns `nil` for the modal pointer on both Esc and Enter, signaling dismissal. On Enter it also sends `SubmitNewRunMsg`. On Esc it sends `CloseModalMsg`.
- Change `n` handler (line 287) from stub to:
  ```go
  case "n":
      a.newRunModal = panels.NewNewRunModal()
      return a, a.newRunModal.Init()
  ```
  Where `Init()` returns `textinput.Blink` to start the cursor blinking.
- Add `Init() tea.Cmd` method to `NewRunModal` that returns `a.promptInput.Blink()`
- Handle `SubmitNewRunMsg` in `Update()`:
  ```go
  case SubmitNewRunMsg:
      return a, func() tea.Msg {
          return StartRunMsg{
              Prompt:   msg.Prompt,
              Workflow: msg.Workflow,
              Model:    msg.Model,
          }
      }
  ```
- Update `View()` — render modal overlay (same pattern as `helpOverlay`):
  ```go
  if a.newRunModal != nil {
      modalView := a.newRunModal.View()
      fullLayout = lipgloss.Place(a.width, a.height,
          lipgloss.Center, lipgloss.Center, modalView,
          lipgloss.WithWhitespaceChars(" "),
          lipgloss.WithWhitespaceForeground(styles.TextDim),
      )
  }
  ```
  Place this check **after** the helpOverlay check (helpOverlay takes priority if both are somehow open).

### 5. Update StartRunMsg and Handler

- Add `Model string` field to `StartRunMsg`:
  ```go
  type StartRunMsg struct {
      Prompt   string
      Workflow string
      Model    string
  }
  ```
- In the `StartRunMsg` handler, set `Prompt` on the new run:
  ```go
  newRun := &run.Run{
      Workflow:  msg.Workflow,
      State:     run.StateQueued,
      CreatedAt: time.Now(),
      Prompt:    msg.Prompt,
  }
  ```
- If `msg.Model != ""`, set it on the run:
  ```go
  if msg.Model != "" {
      a.store.Update(runID, func(r *run.Run) {
          r.Model = msg.Model
      })
  }
  ```

### 6. Add Prompt Field to Run

- Add `Prompt string` field to the `Run` struct in `internal/run/run.go`
- In `internal/ui/panels/detail.go`, add a row to the details view that displays the prompt:
  - After the existing "Workflow" row, add: `kv("Prompt", truncate(r.Prompt, innerWidth-12))` where `truncate` limits the display to one line
  - If `Prompt` is empty, show `"-"` or omit the row

### 7. Update Help Overlay

- In `internal/ui/panels/help.go`, update the "New run" entry from `kv("n", "New run")` to reflect that it opens a modal (the text can stay as `"New run"` since the modal is self-documenting)
- No functional changes needed — just confirm the help text is accurate

### 8. Handle CloseModalMsg for NewRunModal

- The existing `CloseModalMsg` handler in `app.go` sets `a.helpOverlay = nil`. Extend it to also set `a.newRunModal = nil`:
  ```go
  case CloseModalMsg:
      a.helpOverlay = nil
      a.newRunModal = nil
      return a, nil
  ```
  This handles the Esc path. The Enter path returns a nil pointer directly from `Update`, so the field is already nil after the modal's Update returns.

### 9. Write Tests

- Create `internal/ui/panels/newrun_test.go`:
  - `TestNewRunModalDefaults` — create modal, verify default workflow is `"build"`, model is `""`, prompt input is focused
  - `TestNewRunModalWorkflowSelection` — send `ctrl+s` key, verify workflow changes to `"sdlc"`; send `ctrl+q`, verify `"quick-fix"`; send `ctrl+b`, verify `"build"`; send `ctrl+p`, verify `"plan-build"`
  - `TestNewRunModalModelOverride` — send `ctrl+o`, verify model is `"opus"`; send `ctrl+h`, verify `"haiku"`; send `ctrl+n`, verify `"sonnet"`; send `ctrl+x`, verify `""` (default)
  - `TestNewRunModalSubmit` — type a prompt, press `enter`, verify a `SubmitNewRunMsg` is produced with correct prompt, workflow, and model
  - `TestNewRunModalEmptyPromptNoSubmit` — press `enter` with empty prompt, verify no `SubmitNewRunMsg` is produced (modal stays open)
  - `TestNewRunModalCancel` — press `esc`, verify `CloseModalMsg` is produced and modal pointer is nil
  - `TestNewRunModalTextInput` — type characters, verify they appear in the prompt input value
  - `TestNewRunModalView` — create modal, call `View()`, verify it contains "New Run" title, workflow options, model options, and keybind hints

## Testing Strategy

### Unit Tests

- Modal creation: verify defaults (workflow `"build"`, model `""`, prompt focused)
- Workflow hotkeys: each of `ctrl+b`, `ctrl+p`, `ctrl+s`, `ctrl+q` sets the correct workflow
- Model hotkeys: each of `ctrl+h`, `ctrl+o`, `ctrl+n`, `ctrl+x` sets the correct model (or clears it)
- Submit: Enter with non-empty prompt produces `SubmitNewRunMsg` with all fields
- Cancel: Esc produces `CloseModalMsg`
- Validation: Enter with empty prompt is a no-op
- Text input: regular characters are routed to the text input
- View rendering: verify all UI elements are present

### Edge Cases

- **Empty prompt + Enter**: modal stays open, no run created. Optionally flash the input border in error color.
- **`q` key while modal is open**: routed to text input as a character, NOT the global quit handler. This is guaranteed by the key interception check in `App.Update()`.
- **`?` key while modal is open**: routed to text input, does NOT open help overlay.
- **Terminal resize while modal is open**: modal dimensions are fixed (56x18), centered via `lipgloss.Place` which handles resize naturally.
- **Rapid `n` presses**: if modal is already open, `n` is captured by the text input as a character. No double-open possible.
- **No executor configured** (claude binary not found): `StartRunMsg` handler already checks `a.executor != nil` and no-ops. The modal itself doesn't need to know.
- **Workflow hotkeys that conflict with text input**: using `ctrl+` modifier avoids conflicts with regular typing. `b`, `p`, `s`, `q` without ctrl are typed into the prompt.
- **Very long prompt**: char limit of 256 on the text input prevents overflow. The detail panel truncates on display.

## Acceptance Criteria

- [ ] `n` opens a centered modal overlay (replaces the hardcoded stub)
- [ ] Modal has a text input for task description, focused on open
- [ ] Workflow selection via `ctrl+b` (build), `ctrl+p` (plan-build), `ctrl+s` (sdlc), `ctrl+q` (quick-fix)
- [ ] Default workflow is `build`, visually highlighted
- [ ] Model override via `ctrl+h` (haiku), `ctrl+o` (opus), `ctrl+n` (sonnet), `ctrl+x` (clear/default)
- [ ] Default model is "default" (config value), visually highlighted
- [ ] Selected workflow and model are highlighted in green, unselected dimmed
- [ ] `Enter` with non-empty prompt creates a new run with the specified prompt, workflow, and model
- [ ] `Enter` with empty prompt is a no-op (modal stays open)
- [ ] `Esc` closes the modal without creating a run
- [ ] Regular typing goes to the text input (no hotkey conflicts)
- [ ] All key events are captured by the modal when open (`q`, `?`, etc. don't leak to global handlers)
- [ ] Modal renders with rounded border, "New Run" title, and bottom keybinds (Enter/Esc)
- [ ] `Run.Prompt` field stores the task description
- [ ] Detail panel displays the prompt in the Details tab
- [ ] Lazygit-style: simple, centered, no field tabbing, keyboard-driven
- [ ] All new behavior has unit tests

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Lint
go vet ./...

# Run all tests
go test ./...

# Run new run modal tests specifically
go test ./internal/ui/panels/ -run TestNewRunModal -v

# Build to verify compilation
go build -o bin/agtop ./cmd/agtop
```

## Notes

- The workflow hotkeys use `ctrl+` modifiers rather than bare letters (`b`, `p`, `s`, `q`) to avoid conflicts with text input. The original spec suggests bare hotkeys, but since the modal has a text input that needs all letter keys, ctrl-modified hotkeys are the correct design. This is the standard pattern for TUI modals with text input — lazygit uses the same approach.
- The `q` key conflict (quit vs quick-fix workflow) is resolved by the modal intercepting all keys. Inside the modal, `q` is a text input character. `ctrl+q` selects the quick-fix workflow.
- The `SubmitNewRunMsg` → `StartRunMsg` indirection keeps concerns separate: the modal component knows about `SubmitNewRunMsg` (its own message), and the app translates it to `StartRunMsg` (the executor's message). This allows the `StartRunMsg` handler to remain unchanged except for the new `Model` field.
- Model override is optional. When empty, the executor uses the per-skill model from config (the existing behavior). When set, it should be passed through to the process manager to override the skill-level model. The executor/process manager wiring for model override is a follow-up if not already supported — the modal and `Run.Model` field are ready for it.
- The modal dimensions (56x18) are chosen to comfortably fit the prompt input (50 chars visible), workflow options, model options, and keybind bar. For terminals at the minimum size (80x24), the modal occupies ~70% width and ~75% height, leaving visible background for context.
- The `NewRunModal.Update` returns a `*NewRunModal` (pointer) so that returning `nil` naturally signals dismissal to the caller in `App.Update`. This matches how a nil `helpOverlay` means "modal closed".
