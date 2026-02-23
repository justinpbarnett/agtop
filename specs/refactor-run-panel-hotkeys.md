# Refactor: Run panel hotkey bindings

## Metadata

type: `refactor`
task_id: `run-panel-hotkeys`
prompt: `The 'r' key on the runs panel doesn't seem to work. Refactor hotkeys: r should restart, space should suspend or continue the run. Verify all hotkeys for the run panel work.`

## Refactor Description

The run panel action hotkeys have a usability problem: the `r` key is bound to `handleResume()`, which combines two behaviors — SIGCONT resume (for paused processes) and workflow resume (for failed runs) — but neither works intuitively. The user expects `r` to mean "restart" (re-run the same prompt from scratch). Additionally, there is no hotkey to toggle suspend/resume (SIGSTOP/SIGCONT), which is a natural fit for the spacebar.

Several hotkey actions also have state-guard issues that silently swallow key presses with no feedback, making them appear broken.

## Current State

### Hotkey mapping (`internal/ui/app.go:443-519`)

| Key | Handler | Allowed States | Behavior |
|-----|---------|----------------|----------|
| `n` | open new-run modal | any | works |
| `a` | `handleAccept()` | completed, reviewing, failed+merge | works |
| `x` | `handleReject()` | completed, reviewing | works |
| `p` | `handlePause()` | running | sends SIGSTOP |
| `r` | `handleResume()` | paused → SIGCONT; failed/paused → executor.Resume | two-path logic, confusing |
| `c` | `handleCancel()` | running, paused, queued | sends SIGTERM + cancel executor |
| `d` | `handleDelete()` | terminal states only | works |
| `D` | `handleDevServerToggle()` | completed, reviewing | works |
| `u` | `handleFollowUp()` | completed, reviewing | works |

### Problems

1. **`r` (resume) is confusing** — The user expects "restart" but gets "resume from where it left off". For a paused run it sends SIGCONT. For a failed run it resumes from the failed skill. Neither is "restart from scratch".

2. **No suspend/continue toggle** — Pausing (`p`) and resuming (`r`) are separate keys. Spacebar is the natural toggle for play/pause and is not currently bound.

3. **No restart capability** — There is no way to re-run a failed/completed run with the same prompt and workflow from the beginning.

4. **Silent failures** — When a hotkey's state guard doesn't match (e.g., pressing `p` on a completed run), nothing happens and no flash message is shown, making it look broken.

5. **`handleResume()` has overlapping logic** — It first checks `StatePaused` + manager for SIGCONT, then checks `StateFailed || StatePaused` + executor for workflow resume. The paused case hits the first branch, so the second branch's `StatePaused` is dead code for SIGCONT but alive for executor resume. This is confusing.

## Target State

### New hotkey mapping

| Key | Handler | Allowed States | Behavior |
|-----|---------|----------------|----------|
| `n` | open new-run modal | any | unchanged |
| `a` | `handleAccept()` | completed, reviewing, failed+merge | unchanged |
| `x` | `handleReject()` | completed, reviewing | unchanged |
| `space` | `handleTogglePause()` | running → pause; paused → resume (SIGCONT); failed → executor.Resume | new — replaces `p` and SIGCONT path of `r` |
| `r` | `handleRestart()` | failed, completed, reviewing, rejected | new — re-runs the same prompt/workflow from scratch |
| `c` | `handleCancel()` | running, paused, queued | unchanged |
| `d` | `handleDelete()` | terminal states only | unchanged |
| `D` | `handleDevServerToggle()` | completed, reviewing | unchanged |
| `u` | `handleFollowUp()` | completed, reviewing | unchanged |

### Behavioral changes

- **`space`** toggles between running ↔ paused via SIGSTOP/SIGCONT. For failed runs, it resumes the workflow from the failed skill (current `executor.Resume` behavior). Flash message on state mismatch.
- **`r`** restarts a terminal-state run: creates a new run with the same prompt, workflow, and model, in a new worktree. Flash message on state mismatch.
- **`p`** is removed as a hotkey (replaced by `space`).
- All action hotkeys that fail their state guard show a flash message explaining why (e.g., "Cannot restart: run is still active").

## Relevant Files

- `internal/ui/app.go` — Global key handler switch statement (line 443), `handlePause()`, `handleResume()`, and the new `handleTogglePause()` and `handleRestart()` methods
- `internal/ui/panels/help.go` — Help overlay keybind descriptions (line 51-60), must be updated
- `internal/ui/keys.go` — `KeyMap` struct (not directly used for action keys, but documents conventions)
- `internal/ui/app_test.go` — Tests for key handling
- `internal/engine/executor.go` — `Resume()` method used by the toggle-pause handler for failed runs

## Migration Strategy

This is a pure refactor of the key handler layer. No data structures, persistence, or process management changes. The executor's `Resume()` method is reused as-is for the space-on-failed path.

1. Replace `handlePause()` and `handleResume()` with a single `handleTogglePause()` method
2. Add `handleRestart()` method
3. Update the key switch to bind `space` → toggle pause, `r` → restart, remove `p`
4. Add flash messages for invalid-state key presses across all action handlers
5. Update help overlay text

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `handleTogglePause` method

In `internal/ui/app.go`, add a new method that replaces both `handlePause()` and `handleResume()`:

- If no run is selected, flash "No run selected" and return
- If state is `running` and manager is non-nil, call `manager.Pause(id)` — flash on error
- If state is `paused` and manager is non-nil, call `manager.Resume(id)` — flash on error
- If state is `failed` and executor is non-nil, call `executor.Resume(id, prompt)` — flash on error
- Otherwise, flash "Cannot pause/resume: run is {state}"

### 2. Add `handleRestart` method

In `internal/ui/app.go`, add a new method:

- If no run is selected, flash "No run selected" and return
- If the run is not in a terminal state (`IsTerminal()`) and not `reviewing`, flash "Cannot restart: run is still active" and return
- If executor is nil, return
- Create a `StartRunMsg` with the selected run's `Prompt`, `Workflow`, and `Model`, then dispatch it through the update loop (return `a, func() tea.Msg { return StartRunMsg{...} }`)

### 3. Update key switch in `Update()`

In the `tea.KeyMsg` switch at `internal/ui/app.go:443`:

- Remove the `"p"` case entirely
- Change `"r"` to call `a.handleRestart()`
- Add `" "` (space) case to call `a.handleTogglePause()`

### 4. Remove old `handlePause` and `handleResume` methods

Delete `handlePause()` (line 868) and `handleResume()` (line 883) from `internal/ui/app.go` since they are replaced by `handleTogglePause()`.

### 5. Add flash messages to remaining action handlers

Add state-mismatch flash messages to handlers that currently silently return:

- `handleAccept()` — flash "Cannot accept: run is {state}" when state doesn't match
- `handleReject()` — flash "Cannot reject: run is {state}" when state doesn't match
- `handleCancel()` — flash "Cannot cancel: run is {state}" when state doesn't match
- `handleDelete()` — flash "Cannot delete: run is still active" when not terminal
- `handleDevServerToggle()` — flash "Dev server: run is {state}" when state doesn't match
- `handleFollowUp()` — flash "Cannot follow up: run is {state}" when state doesn't match

Each flash should return `a, flashClearCmd()` so the message auto-clears.

### 6. Update help overlay

In `internal/ui/panels/help.go`, update the Actions section:

- Change `kv("p", "Pause")` to `kv("Space", "Pause / Resume")`
- Change `kv("r", "Resume / Retry")` to `kv("r", "Restart")`

### 7. Update run list panel keybind bar

In `internal/ui/panels/runlist.go`, the bottom keybind bar (line 143-148) should remain unchanged since it only shows `n`, `y`, `/` — none of the affected keys.

### 8. Update README hotkey table

In `README.md`, update the key bindings table:

- Change `p` row to `Space` with description "Pause / resume run"
- Change `r` row to description "Restart run"
- Remove the separate `p` row for "Pause run"

## Testing Strategy

### Existing tests to verify

- `go test ./internal/ui/...` — All existing UI tests must pass
- `go test ./internal/engine/...` — Executor tests must pass (Resume behavior unchanged)
- `go vet ./...` — No lint errors

### Manual verification

- Start a run → press `space` → verify it pauses (SIGSTOP)
- On a paused run → press `space` → verify it resumes (SIGCONT)
- On a failed run → press `space` → verify it resumes from failed skill
- On a completed run → press `r` → verify new run starts with same prompt/workflow
- On a running run → press `r` → verify flash message "Cannot restart: run is still active"
- On a running run → press `d` → verify flash message about not being able to delete
- Press `?` → verify updated help text shows `Space` for pause/resume and `r` for restart

### Golden file updates

The help overlay golden file will need updating:

```bash
make update-golden
```

## Risk Assessment

- **Low risk** — This is a UI-layer-only refactor. No changes to process management, executor logic, or persistence.
- **Key conflict** — `space` is not used by any other handler. The run list panel does not handle `space` in its `Update()` method, so there is no conflict.
- **`p` removal** — Users accustomed to `p` for pause will need to learn `space`. The help overlay will guide them.

## Validation Commands

```bash
go vet ./...
go test ./internal/ui/... -count=1
go test ./internal/engine/... -count=1
make update-golden
go test ./internal/ui/... -count=1
```

## Open Questions (Unresolved)

None — the scope and behavior are well-defined.

## Sub-Tasks

Single task — no decomposition needed.
