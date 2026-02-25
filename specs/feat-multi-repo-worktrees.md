# Feature: Multi-Repo Worktree Support

## Metadata

type: `feat`
task_id: `multi-repo-worktrees`
prompt: `Add the ability for agtop to be used in folders with sub-repos. When a run starts, create a worktree for each sub-repo (not the root repo). When creating PRs, create separate PRs per sub-repo with the same title.`

## Feature Description

agtop currently assumes the project root is a single git repository and creates worktrees directly from it. This fails for projects that use a **poly-repo** (or "meta-repo") layout — a root directory that orchestrates multiple independent git repositories in subdirectories.

For example, a project like `passioncamp` has:
- `app/client` — its own git repo (Nuxt frontend)
- `app/server` — its own git repo (Laravel backend)
- `app/db` — its own git repo (Docker Compose)
- Root directory — either a separate meta-repo or no `.git` at all

When agtop creates a worktree, it should detect sub-repos and create a worktree **per sub-repo** using the same branch name. Skills execute against the assembled worktree root (which mirrors the original project structure). When creating PRs, separate PRs are created per sub-repo with the same title for consistency.

This is modeled after the working prototype in the `adws` system at `~/dev/passion/passioncamp/adws/`.

## User Story

As a developer working on a poly-repo project
I want agtop to create worktrees for each sub-repo and PRs per sub-repo
So that I can use agtop's full workflow orchestration with multi-repo projects

## Relevant Files

- `internal/git/worktree.go` — Current `WorktreeManager` that creates/removes/merges single-repo worktrees. Core file to extend.
- `internal/config/config.go` — Config structs. Needs a new `[repos]` section for declaring sub-repos.
- `internal/config/loader.go` — Config loading logic. Needs to parse the new `[repos]` section.
- `internal/ui/app.go` — Calls `a.worktrees.Create(runID)` on `StartRunMsg`. Needs to handle multi-repo returns.
- `internal/run/run.go` — `Run` struct has single `Worktree` and `Branch` fields. Needs to accommodate multiple worktrees/branches.
- `internal/engine/executor.go` — Uses `r.Worktree` to set `opts.WorkDir` for skills. Must use the assembled root path for multi-repo.
- `skills/pr/SKILL.md` — Built-in PR skill assumes single repo. Needs multi-repo awareness.
- `agtop.example.toml` — Example config. Needs multi-repo example.

### New Files

- None — all changes fit within existing files and structures.

## Implementation Plan

### Phase 1: Configuration

Add a `[repos]` config section that declares sub-repos. When present, agtop operates in multi-repo mode. When absent, behavior is unchanged (single-repo mode).

### Phase 2: Worktree Manager

Extend `WorktreeManager` to detect repo topology and handle multi-repo worktree creation. In multi-repo mode: create a worktree per sub-repo under an assembled root directory, all using the same branch name.

### Phase 3: Run Model & Executor Integration

Update the `Run` struct to store sub-repo worktree info. Update the executor to pass the assembled root path as WorkDir. Skills run against the assembled root and see the same directory structure as the original project.

### Phase 4: PR Skill

Update the built-in PR skill to detect multi-repo projects and create a separate PR per sub-repo with the same title.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add Repos Config

In `internal/config/config.go`:

- Add a `RepoConfig` struct:
  ```go
  type RepoConfig struct {
      Path string `toml:"path"` // relative path from project root, e.g. "app/client"
      Name string `toml:"name"` // short label, e.g. "client"
  }
  ```
- Add `Repos []RepoConfig` field to the `Config` struct with tag `toml:"repos"`

In `internal/config/loader.go`:

- Ensure the TOML parser correctly deserializes `[[repos]]` array-of-tables into `[]RepoConfig`

In `agtop.example.toml`:

- Add a commented-out example showing multi-repo configuration:
  ```toml
  # ── Multi-Repo (Poly-Repo) ────────────────────────────────────
  # For projects with multiple independent git repos in subdirectories.
  # When configured, agtop creates a worktree per sub-repo instead of
  # one worktree for the root. PRs are created per sub-repo.
  #
  # [[repos]]
  # name = "client"
  # path = "app/client"
  #
  # [[repos]]
  # name = "server"
  # path = "app/server"
  ```

### 2. Add Topology Detection to WorktreeManager

In `internal/git/worktree.go`:

- Add a `SubRepoWorktree` struct to hold per-repo worktree info:
  ```go
  type SubRepoWorktree struct {
      Name     string // e.g. "client"
      Path     string // worktree path for this sub-repo
      Branch   string // branch name (same for all)
      RepoRoot string // original git root for this sub-repo
  }
  ```

- Add a `MultiWorktreeResult` struct returned by multi-repo create:
  ```go
  type MultiWorktreeResult struct {
      RootPath     string            // assembled root (e.g. /tmp/agtop-worktrees/001)
      Branch       string            // shared branch name
      SubWorktrees []SubRepoWorktree // one per configured sub-repo
  }
  ```

- Add a helper `isGitRepo(path string) bool` that runs `git rev-parse --git-dir` in the given path and checks the return code.

### 3. Implement Multi-Repo Create

In `internal/git/worktree.go`:

- Add method `CreateMulti(runID string, repos []config.RepoConfig) (*MultiWorktreeResult, error)`:
  1. Compute the assembled root path: `filepath.Join(w.worktreeDir, runID)`
  2. Use shared branch name: `agtop/<runID>`
  3. For each repo in `repos`:
     - Resolve the sub-repo's git root: `filepath.Join(w.repoRoot, repo.Path)`
     - Verify it is a git repo via `isGitRepo`
     - Compute the sub-repo worktree path: `filepath.Join(assembledRoot, repo.Path)`
     - Create intermediate directories: `os.MkdirAll`
     - Run `git worktree add <worktree-path> -b <branch>` with `cmd.Dir` set to the sub-repo git root
     - On failure, roll back all previously created worktrees and return error
  4. Return the `MultiWorktreeResult`

- The existing `Create(runID)` method remains unchanged for single-repo mode.

### 4. Implement Multi-Repo Remove

In `internal/git/worktree.go`:

- Add method `RemoveMulti(runID string, repos []config.RepoConfig) error`:
  1. For each repo: run `git worktree remove` and `git branch -D` against the sub-repo's git root
  2. Remove the assembled root directory if empty
  3. Ignore errors (same pattern as single-repo `Remove`)

### 5. Implement Multi-Repo Merge

In `internal/git/worktree.go`:

- Add method `MergeMulti(runID string, repos []config.RepoConfig) error`:
  1. For each repo: merge the `agtop/<runID>` branch in each sub-repo's git root
  2. Reuse the existing rebase-then-merge pattern from `MergeWithOptions`
  3. If any sub-repo merge fails, return an error indicating which one failed (but do not roll back successful merges — let the user decide)

### 6. Implement Multi-Repo List and Exists

In `internal/git/worktree.go`:

- Add method `ListMulti(repos []config.RepoConfig) ([]WorktreeInfo, error)`:
  - Collects worktrees from each sub-repo and deduplicates by run ID
- Add method `ExistsMulti(runID string, repos []config.RepoConfig) bool`:
  - Returns true only if ALL sub-repo worktrees exist for the run ID

### 7. Update Run Struct

In `internal/run/run.go`:

- Add a `SubWorktrees` field to the `Run` struct:
  ```go
  SubWorktrees []SubWorktreeInfo `json:"sub_worktrees,omitempty"`
  ```
- Add the `SubWorktreeInfo` struct:
  ```go
  type SubWorktreeInfo struct {
      Name     string `json:"name"`
      Path     string `json:"path"`
      RepoRoot string `json:"repo_root"`
  }
  ```

The existing `Worktree` field stores the assembled root path. The existing `Branch` field stores the shared branch name. `SubWorktrees` provides per-repo detail when needed (e.g., for the PR skill).

### 8. Update App to Use Multi-Repo Create

In `internal/ui/app.go`:

- In the `StartRunMsg` handler (around line 396-426):
  - Check if `a.cfg.Repos` is non-empty
  - If multi-repo: call `a.worktrees.CreateMulti(runID, a.cfg.Repos)` and populate `r.Worktree` with `result.RootPath`, `r.Branch` with `result.Branch`, and `r.SubWorktrees` from `result.SubWorktrees`
  - If single-repo: keep existing `a.worktrees.Create(runID)` call unchanged

- In the accept handler (wherever merge is called):
  - If `r.SubWorktrees` is non-empty, call `MergeMulti` and `RemoveMulti`
  - Otherwise keep existing `Merge` and `Remove`

### 9. Update Executor WorkDir Passthrough

In `internal/engine/executor.go`:

- No changes needed. The executor already reads `r.Worktree` and sets `opts.WorkDir`. Since `r.Worktree` is now the assembled root path (which mirrors the original project structure), skills execute in the correct directory.

### 10. Update the Built-in PR Skill

In `skills/pr/SKILL.md`:

- Add a "Multi-Repo Support" section that describes the behavior when the working directory contains multiple independent git repos:
  1. Detect if the working directory root is NOT a git repo but contains subdirectories that ARE git repos
  2. For each sub-repo with a feature branch and commits ahead of base:
     - Generate a PR title (same title for all repos for consistency)
     - Push and create a PR independently
  3. Report all PR URLs grouped by sub-repo name

- This mirrors the approach in `~/dev/passion/passioncamp/.claude/skills/pr/SKILL.md` which uses `git -C <repo-path>` and `cd <repo-path> && gh pr create` for each sub-repo.

### 11. Update the Built-in Commit Skill

In `skills/commit/SKILL.md` (if it exists as built-in):

- Add multi-repo awareness: when the working directory is not a git repo, find sub-repos and commit in each one that has changes.

### 12. Update Session Persistence

In `internal/run/persistence.go`:

- Ensure `SubWorktrees` is serialized/deserialized with the run state so that recovered runs retain multi-repo info.

## Testing Strategy

### Unit Tests

- `internal/git/worktree_test.go`:
  - `TestCreateMulti_Success` — Create worktrees for 2 sub-repos, verify paths and branches
  - `TestCreateMulti_RollbackOnFailure` — Second sub-repo fails, verify first is cleaned up
  - `TestRemoveMulti` — Remove worktrees for 2 sub-repos
  - `TestExistsMulti` — Returns false if any sub-repo worktree is missing
  - `TestIsGitRepo` — Correctly detects git and non-git directories
  - `TestCreateMulti_SingleRepoFallback` — Verify single-repo mode still works when repos config is empty

- `internal/config/loader_test.go`:
  - Verify `[[repos]]` TOML array deserializes correctly
  - Verify empty repos config results in nil/empty slice

### Edge Cases

- Project root IS a git repo AND has sub-repos configured — sub-repo config takes precedence, root repo is ignored for worktree purposes
- Only some sub-repos have changes — PR skill creates PRs only for repos with feature branches
- Sub-repo path does not exist or is not a git repo — `CreateMulti` returns clear error
- One sub-repo worktree creation fails mid-way — all previously created worktrees are rolled back
- Session recovery with multi-repo runs — `SubWorktrees` is preserved in persistence

## Risk Assessment

- **Backward compatibility**: Single-repo mode is completely unchanged. Multi-repo only activates when `[[repos]]` is configured. No existing users are affected.
- **Merge conflicts in multi-repo mode**: If merge fails for one sub-repo but succeeds for another, the user needs to manually resolve. The spec proposes failing with a clear message rather than attempting rollback of successful merges.
- **Skill compatibility**: Skills that run git commands (commit, pr) need multi-repo awareness. Skills that only read/write files (build, test, review) work unchanged because they operate on the assembled root which has the same file layout as the original project.
- **Dev server**: The dev server integration may need updating if it relies on the worktree being a single git repo. This is out of scope for the initial implementation — multi-repo dev server support can be added later.

## Validation Commands

NOTE: These commands are run by the **test skill**, not the build skill. The build skill should only compile-check.

```bash
go build ./...
go test ./internal/git/... -run TestCreateMulti
go test ./internal/git/... -run TestRemoveMulti
go test ./internal/git/... -run TestIsGitRepo
go test ./internal/config/... -run TestRepos
go vet ./...
```

## Open Questions (Unresolved)

1. **Symlink support**: The adws prototype symlinks `specs/`, `.claude/`, `justfile`, `references`, and dependency directories (`node_modules`, `vendor`) from the original project root into the assembled worktree root. Should agtop support configurable symlinks? **Recommendation**: Add an optional `[worktree.symlinks]` config for declaring paths to symlink from the project root into assembled worktree roots. This is important for multi-repo projects where shared config lives at the project root level.

2. **Sub-repo auto-detection vs explicit config**: Should agtop auto-detect sub-repos (by scanning for `.git` directories) or require explicit `[[repos]]` configuration? **Recommendation**: Require explicit config. Auto-detection is fragile (could pick up vendored repos, submodules, etc.) and the user knows which repos matter.

3. **Merge behavior**: When accepting a multi-repo run, should agtop merge all sub-repos atomically (fail all if one fails) or independently? **Recommendation**: Merge independently and report results per sub-repo. This matches how the repos are independent in practice.

## Sub-Tasks

This feature is large enough to warrant decomposition:

1. **Config layer** (Steps 1) — Add `[[repos]]` config support
2. **Worktree manager** (Steps 2-6) — Multi-repo create/remove/merge/list/exists
3. **Run model & app integration** (Steps 7-9) — Wire multi-repo into the run lifecycle
4. **Skill updates** (Steps 10-11) — Update PR and commit skills for multi-repo
5. **Persistence** (Step 12) — Ensure multi-repo runs survive restarts
