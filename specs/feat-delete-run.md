# Feature: Delete Run

## Metadata

type: `feat`
task_id: `delete-run`
prompt: `Add a way to press d and delete a run, removing the worktree and any associated branch or files. Can only apply to inactive runs.`

## Feature Description

Allow users to press `d` on a selected inactive (terminal-state) run to permanently delete it. Deletion removes the run from the store, cleans up its git worktree and branch, removes its session persistence file, clears its log buffer from the process manager, and stops any associated dev server.

## User Story

As an agtop user
I want to press `d` to delete a completed/failed/accepted/rejected run
So that I can clean up stale runs and reclaim disk space without leaving the dashboard

## Relevant Files

- `internal/ui/app.go` — Main Update loop where keybindings are handled; `d` is currently mapped to `handleDevServerToggle`. Will add `handleDelete` and reassign `d`.
- `internal/ui/keys.go` — KeyMap struct (no changes needed since action keys like `p`, `r`, `c` are handled inline).
- `internal/ui/panels/help.go` — Help overlay listing keybindings; add `d` for delete.
- `internal/run/store.go` — `Store.Remove(id)` already exists and removes a run from the store.
- `internal/run/persistence.go` — `Persistence.Remove(runID)` already exists and deletes the session JSON file.
- `internal/run/run.go` — `Run.IsTerminal()` defines which states are terminal (completed, accepted, rejected, failed). These are the only states eligible for deletion.
- `internal/git/worktree.go` — `WorktreeManager.Remove(runID)` already handles `git worktree remove --force` and `git branch -D`.
- `internal/process/manager.go` — Holds `buffers` map; needs a new `RemoveBuffer(runID)` method to clean up log buffers.
- `internal/server/devserver.go` — `DevServerManager.Stop(runID)` already exists; call it during delete to stop any running dev server.

### New Files

None — all changes are additions to existing files.

## Implementation Plan

### Phase 1: Foundation

Add a `RemoveBuffer` method to the process manager so log buffers can be cleaned up when a run is deleted.

### Phase 2: Core Implementation

Add a `handleDelete` method to `App` that:
1. Guards: only works on selected runs in a terminal state (`IsTerminal()`)
2. Stops any dev server for the run
3. Removes the run from the store (which triggers UI refresh)
4. In a goroutine: removes the worktree+branch, removes the session file, and cleans the buffer

### Phase 3: Integration

- Reassign `d` from dev server toggle to delete. Move dev server toggle to `D` (shift-d).
- Update the help overlay to show `d` for delete and `D` for dev server toggle.
- Update README keybindings table.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add `RemoveBuffer` to process Manager

- In `internal/process/manager.go`, add a method:
  ```go
  func (m *Manager) RemoveBuffer(runID string) {
      m.mu.Lock()
      defer m.mu.Unlock()
      delete(m.buffers, runID)
  }
  ```

### 2. Add `handleDelete` method to App

- In `internal/ui/app.go`, add a new method `handleDelete() (tea.Model, tea.Cmd)`:
  - Get the selected run from `a.runList.SelectedRun()`; return early if nil.
  - Guard: return early if `!selected.IsTerminal()` — only inactive runs can be deleted.
  - Capture `runID`, `branch`, and `worktree` from the selected run before removal.
  - Stop any dev server: `a.devServers.Stop(runID)`.
  - Remove from store: `a.store.Remove(runID)` — this triggers UI refresh via the change channel.
  - Remove log buffer: if `a.manager != nil`, call `a.manager.RemoveBuffer(runID)`.
  - Launch a goroutine to handle filesystem cleanup:
    - Remove worktree and branch: `a.worktrees.Remove(runID)`.
    - Remove session file: if `a.persistence != nil`, call `a.persistence.Remove(runID)`.
  - Set a status bar flash: `"Deleted run <runID>"`.
  - Return with `flashClearCmd()`.

### 3. Rebind `d` key in Update

- In `internal/ui/app.go`, in the `tea.KeyMsg` switch block:
  - Change `case "d":` to call `a.handleDelete()`.
  - Add `case "D":` to call `a.handleDevServerToggle()`.

### 4. Update help overlay

- In `internal/ui/panels/help.go`, in the Actions section:
  - Add `d` → `Delete run` entry.
  - Add `D` → `Dev server toggle` entry.

### 5. Update README keybindings

- In `README.md`, add `d` → Delete run and `D` → Toggle dev server to the Key Bindings table.

## Testing Strategy

### Unit Tests

- Add a test in `internal/ui/app_test.go` verifying:
  - Pressing `d` on a terminal-state run removes it from the store.
  - Pressing `d` on a running/queued/paused run does nothing.
  - Pressing `d` with no runs selected does nothing.
- Add a test in `internal/process/manager_test.go` for `RemoveBuffer`:
  - Inject a buffer, call `RemoveBuffer`, verify `Buffer()` returns nil.

### Edge Cases

- Deleting a run whose worktree was already removed (e.g., after accept/reject) — `worktrees.Remove` already ignores errors for missing worktrees.
- Deleting a run with no persistence layer initialized (`a.persistence == nil`) — guarded by nil check.
- Deleting the only run in the list — `store.Remove` triggers `RunStoreUpdatedMsg`, runlist `clampSelection` handles empty list.
- Deleting while a filter is active — the filtered list refreshes automatically via `applyFilter` on store update.

## Risk Assessment

- **Key reassignment**: Moving dev server toggle from `d` to `D` is a breaking change for muscle memory, but `d` is not documented in the help overlay or README, so user impact is minimal.
- **Concurrent deletion**: The goroutine for worktree/session cleanup could race with other operations on the same run, but since the run is already removed from the store, no other handler will target it.
- **Worktree removal errors**: `WorktreeManager.Remove` already ignores errors, so a partially-cleaned worktree won't cause issues.

## Validation Commands

```bash
go vet ./...
go test ./internal/ui/ ./internal/process/ ./internal/run/ ./internal/git/
go build ./cmd/agtop/
```

## Open Questions (Unresolved)

None — the feature requirements are clear and all necessary infrastructure already exists.

## Sub-Tasks

Single task — no decomposition needed.
