# Feature: Git Worktree and Dev Server Management

## Metadata

type: `feat`
task_id: `git-worktree-devserver`
prompt: `Implement step 10 of docs/agtop.md — Git worktree isolation per run and dev server management with hashed ports`

## Feature Description

Every agtop run needs process isolation at the filesystem level. This feature implements git worktree lifecycle management (create on run start, remove on reject/cleanup) and dev server management (launch on run completion with deterministic hashed ports). Together they let a developer launch multiple concurrent agent runs, each operating in its own branch and directory, and immediately test results via a local dev server — all without manual git or server juggling.

## User Story

As a developer running concurrent AI agent workflows
I want each run to execute in an isolated git worktree with its own branch, and completed runs to auto-launch a dev server on a predictable port
So that runs never conflict with each other or my working tree, and I can immediately test results without manual setup

## Problem Statement

Currently, agtop stubs out worktree and dev server management. The `StartRunMsg` handler in `app.go` sets `Worktree` to the project root and `Branch` to a hardcoded `"agtop/run"`, meaning all runs share the same directory and branch. This makes concurrent runs impossible — file writes collide, and there's no way to review or test individual run outputs in isolation. There is also no mechanism to launch dev servers for testing completed work.

## Solution Statement

Implement two managers that integrate with the existing executor and TUI:

1. **WorktreeManager** (`internal/git/worktree.go`) — creates/removes git worktrees under `.agtop/worktrees/<run-id>` with branches named `agtop/<run-id>`. Shells out to `git worktree add/remove` and `git branch -D`.

2. **DevServerManager** (`internal/server/devserver.go`) — launches the configured dev server command inside a completed run's worktree on a deterministic port computed as `base_port + (hash(run_id) % 100)`. Tracks running servers and kills them on cleanup.

3. **Integration** — the `App.StartRunMsg` handler calls `WorktreeManager.Create()` before starting the executor. The executor's `terminalState` path and TUI keybinds (`a` accept, `x` reject) trigger dev server start/stop and worktree cleanup. A new `Run.DevServerPort` field tracks the assigned port.

## Relevant Files

Use these files to implement the feature:

- `internal/git/worktree.go` — stub to replace with full WorktreeManager implementation
- `internal/git/diff.go` — stub to replace with DiffGenerator that produces diffs against main
- `internal/server/devserver.go` — stub to replace with full DevServerManager implementation
- `internal/run/run.go` — add `DevServerPort` and `DevServerURL` fields to Run struct
- `internal/ui/app.go` — update `StartRunMsg` to create worktrees, add accept/reject/cleanup keybinds
- `internal/engine/executor.go` — no changes needed (already reads `Run.Worktree` from store)
- `internal/config/config.go` — already has `DevServerConfig` with Command, PortStrategy, BasePort
- `internal/config/defaults.go` — verify dev server defaults exist

### New Files

- `internal/git/worktree_test.go` — unit tests for WorktreeManager
- `internal/server/devserver_test.go` — unit tests for DevServerManager

## Implementation Plan

### Phase 1: WorktreeManager

Build the git worktree manager that creates isolated directories and branches for each run. This is the foundation — nothing else works without filesystem isolation.

### Phase 2: DevServerManager

Build the dev server manager that launches/kills dev servers in worktree directories on hashed ports. This depends on worktrees existing.

### Phase 3: Integration

Wire both managers into the TUI app and run lifecycle. Update `StartRunMsg` to create worktrees, add accept/reject keybinds, and trigger dev server start on completion.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Implement WorktreeManager

Replace the stub in `internal/git/worktree.go` with a full implementation:

- Define `WorktreeManager` struct with fields: `repoRoot string` (project root where `.git` lives), `worktreeDir string` (`.agtop/worktrees`), and `mu sync.Mutex`
- `NewWorktreeManager(repoRoot string) *WorktreeManager` — constructor. Sets `worktreeDir` to `filepath.Join(repoRoot, ".agtop", "worktrees")`
- `Create(runID string) (worktreePath string, branch string, err error)`:
  - Ensure `.agtop/worktrees` directory exists (`os.MkdirAll`)
  - Compute `branch = "agtop/" + runID` and `worktreePath = filepath.Join(worktreeDir, runID)`
  - Execute: `git worktree add <worktreePath> -b <branch>` with `Cmd.Dir` set to `repoRoot`
  - Return the worktree path and branch name
- `Remove(runID string) error`:
  - Compute worktree path
  - Execute: `git worktree remove <worktreePath> --force`
  - Execute: `git branch -D agtop/<runID>` (ignore error if branch already gone)
- `List() ([]WorktreeInfo, error)`:
  - Execute: `git worktree list --porcelain` with `Cmd.Dir` set to `repoRoot`
  - Parse output into `[]WorktreeInfo` structs (Path, Branch, HEAD)
  - Filter to only worktrees under `.agtop/worktrees/`
- `Exists(runID string) bool` — check if the worktree directory exists on disk
- Define `WorktreeInfo` struct: `Path string`, `Branch string`, `HEAD string`

### 2. Implement DiffGenerator

Replace the stub in `internal/git/diff.go`:

- Define `DiffGenerator` struct with `repoRoot string`
- `NewDiffGenerator(repoRoot string) *DiffGenerator`
- `Diff(branch string) (string, error)`:
  - Execute: `git diff main...<branch>` with `Cmd.Dir` set to `repoRoot`
  - Return raw diff output as string
- `DiffStat(branch string) (string, error)`:
  - Execute: `git diff --stat main...<branch>`
  - Return stat summary

### 3. Implement DevServerManager

Replace the stub in `internal/server/devserver.go`:

- Define `DevServerManager` struct with fields: `command string`, `basePort int`, `portStrategy string`, `mu sync.Mutex`, `servers map[string]*RunningServer`
- Define `RunningServer` struct: `Port int`, `Cmd *exec.Cmd`, `WorkDir string`, `cancel context.CancelFunc`
- `NewDevServerManager(cfg config.DevServerConfig) *DevServerManager`
- `Start(runID string, workDir string) (port int, err error)`:
  - If `command` is empty, return 0 and nil (no dev server configured)
  - Compute port via `computePort(runID)`
  - Split command string into executable and args (use `strings.Fields` or shell-style parsing)
  - Set `PORT` environment variable to the computed port
  - Start subprocess with `Cmd.Dir` set to `workDir`
  - Store in `servers` map
  - Return port
- `Stop(runID string) error`:
  - Look up server in map
  - Send SIGTERM, wait 5s, then SIGKILL if still alive
  - Remove from map
- `StopAll()` — stop all running servers (for cleanup on exit)
- `computePort(runID string) int`:
  - Use FNV-1a hash of runID
  - Return `basePort + (hash % 100)`
- `Port(runID string) int` — return port for a running server, 0 if not running

### 4. Add DevServer Fields to Run

Edit `internal/run/run.go`:

- Add `DevServerPort int` field to the `Run` struct
- Add `DevServerURL string` field to the `Run` struct (e.g., `http://localhost:3142`)

### 5. Integrate WorktreeManager into App

Edit `internal/ui/app.go`:

- Add `worktrees *git.WorktreeManager` and `devServers *server.DevServerManager` fields to the `App` struct
- In `NewApp()`:
  - Create `WorktreeManager` with project root
  - Create `DevServerManager` with `cfg.Project.DevServer`
- Update the `StartRunMsg` handler:
  - Call `worktrees.Create(runID)` to get worktree path and branch
  - Set `newRun.Worktree` and `newRun.Branch` from the result
  - Generate a unique run ID before adding to store (use the store's auto-generated ID by adding first, then creating worktree, then updating the run)
  - On worktree creation failure, set run to `StateFailed` with error

### 6. Add Accept, Reject, and Cleanup Keybinds

Edit `internal/ui/app.go` key handling:

- `a` (accept): On the selected run in `StateCompleted` or `StateReviewing`:
  - Transition to `StateAccepted`
  - Stop dev server if running
  - Execute `git push origin <branch>` in background
  - Optionally run `gh pr create` (if configured; defer to future work)
  - Remove worktree and branch
- `x` (reject): On the selected run in `StateCompleted` or `StateReviewing`:
  - Transition to `StateRejected`
  - Stop dev server if running
  - Remove worktree and branch via `worktrees.Remove(runID)`
- `d` (dev server): On the selected run in `StateCompleted` or `StateReviewing`:
  - Start dev server if not running, stop if running (toggle)
  - Update `Run.DevServerPort` and `Run.DevServerURL`

### 7. Auto-Launch Dev Server on Completion

Edit `internal/ui/app.go`:

- In the `RunStoreUpdatedMsg` handler, check if the selected run just transitioned to `StateCompleted` or `StateReviewing`
- If dev server config has a command and the run has a worktree, auto-start the dev server
- Update the run's `DevServerPort` and `DevServerURL` fields
- Show the dev server URL in the status bar flash

### 8. Display Dev Server Info in Detail Panel

- The detail panel (`internal/ui/panels/detail.go`) should display `DevServerPort` and `DevServerURL` when set on the run
- This should work automatically if the detail panel already renders Run fields — verify and add if missing

### 9. Write Tests

- `internal/git/worktree_test.go`:
  - Test `Create` — verifies directory and branch creation (use `git init` in a temp dir)
  - Test `Remove` — verifies directory and branch cleanup
  - Test `List` — verifies filtering to `.agtop/worktrees/` entries
  - Test `Exists` — verifies path checking
  - Test idempotent remove (already removed)
- `internal/server/devserver_test.go`:
  - Test `computePort` — verify deterministic hash-based port allocation
  - Test `Start`/`Stop` with a simple command like `sleep 60`
  - Test `StopAll` — verify all servers are cleaned up
  - Test empty command returns 0 port and no error

## Testing Strategy

### Unit Tests

- **WorktreeManager**: Create a temporary git repo with `git init`, test create/remove/list/exists operations. Verify the worktree directory exists after create and is gone after remove. Verify the branch is created and deleted.
- **DevServerManager**: Test port computation is deterministic (same runID always produces same port). Test start/stop lifecycle with a trivial subprocess (`sleep 60`). Test that `StopAll` kills all tracked servers.
- **DiffGenerator**: Create a temp repo, make a commit on a branch, verify diff output is non-empty and contains the changed file.

### Edge Cases

- **Worktree already exists**: `Create` should return a clear error if the directory or branch already exists. The caller should handle this (e.g., reuse or fail).
- **Worktree remove on non-existent path**: `Remove` should not error if the worktree was already removed (idempotent).
- **Port collision**: Two different run IDs could hash to the same port. The dev server start will fail with "address in use" — surface this error to the user via the status bar.
- **No dev server configured**: If `DevServerConfig.Command` is empty, skip dev server launch silently.
- **Run rejected while dev server running**: The reject handler must stop the dev server before removing the worktree, otherwise the server holds file locks.
- **agtop exit with running dev servers**: `StopAll()` must be called during app teardown.
- **Dirty worktree on remove**: Use `--force` flag on `git worktree remove` since agent may leave uncommitted changes.
- **Disk space**: Track worktree count in `List()`, warn if count exceeds 10 (configurable in future).
- **Branch name conflicts**: Run IDs are auto-generated sequential (`001`, `002`, ...) so branch names are unique per session. Cross-session conflicts handled by checking `Exists()` first.

## Acceptance Criteria

- [ ] `WorktreeManager.Create()` creates a worktree at `.agtop/worktrees/<run-id>` with branch `agtop/<run-id>`
- [ ] `WorktreeManager.Remove()` removes the worktree directory and deletes the branch
- [ ] `WorktreeManager.List()` returns only agtop-managed worktrees
- [ ] `DiffGenerator.Diff()` produces a diff between main and the run's branch
- [ ] `DevServerManager.Start()` launches the configured command on a hashed port
- [ ] `DevServerManager.Stop()` kills the server process
- [ ] `DevServerManager.computePort()` is deterministic for the same run ID
- [ ] New runs create an isolated worktree before executing (not using project root)
- [ ] New runs have unique branches (`agtop/<run-id>`)
- [ ] `a` keybind accepts a completed/reviewing run: pushes branch, removes worktree
- [ ] `x` keybind rejects a completed/reviewing run: stops dev server, removes worktree and branch
- [ ] Dev server auto-launches on run completion if configured
- [ ] `Run.DevServerPort` and `Run.DevServerURL` are set when dev server starts
- [ ] Dev server info is visible in the detail panel
- [ ] All dev servers are stopped on app exit
- [ ] Unit tests pass for WorktreeManager, DevServerManager, and DiffGenerator

## Validation Commands

Execute these commands to validate the feature is complete:

```bash
# Run all tests
make lint
go test ./internal/git/... -v
go test ./internal/server/... -v
go test ./... -count=1

# Build successfully
make build
```

## Notes

- The `gh pr create` integration for accept is mentioned in the spec doc but should be kept minimal here — just push the branch. Full PR creation with title/body generation can be a follow-up feature.
- The executor already reads `Run.Worktree` from the store and passes it as `opts.WorkDir` to `StartSkill`, so no executor changes are needed — worktree integration is purely at the app/TUI layer.
- Dev server `PORT` env var is a convention used by most frameworks (Next.js, Vite, etc.). For frameworks that use different env vars, users can configure the command to include the port directly (e.g., `"npm run dev -- --port $PORT"`).
- The `.agtop/worktrees/` directory should be added to `.gitignore` by `agtop init` (future task).
- Worktree creation shells out to git rather than using a Go git library — this keeps the dependency footprint small and matches the project's existing pattern of subprocess invocation.
