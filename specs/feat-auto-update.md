# Feature: Auto-Update via GitHub Releases

## Metadata

type: `feat`
task_id: `auto-update`
prompt: `Add in-app automatic update checking and binary self-replacement using GitHub Releases (goreleaser + go-selfupdate)`

## Feature Description

agtop currently embeds a version string via ldflags (`internal/ui/panels.Version`) but has no mechanism to check for or apply updates. Users deploying across multiple machines must manually run `go install` or rebuild from source to stay current. This feature adds:

1. A **`version` subcommand** that prints the current version and checks for updates
2. **Automatic update checking** on TUI startup (non-blocking goroutine)
3. **Binary self-replacement** — downloads the correct binary from GitHub Releases and atomically replaces the running executable
4. A **goreleaser configuration** and GitHub Actions workflow for building release binaries on tag push
5. A **config toggle** to disable auto-update (for offline/CI environments)

## User Story

As a developer running agtop on multiple machines
I want the app to automatically detect and install new versions
So that all my instances stay current without manual intervention

## Relevant Files

- `cmd/agtop/main.go` — Entry point, subcommand routing. Needs `version` and `update` subcommands added, and startup update check.
- `internal/ui/panels/statusbar.go` — Defines `Version` var (set via ldflags). Status bar renders version and flash messages for update notifications.
- `internal/ui/app.go` — App.Init() and App.Update() handle startup commands and messages. Needs to receive and display update-available notifications.
- `internal/ui/messages.go` — Message type aliases. Needs a new `UpdateAvailableMsg`.
- `internal/config/config.go` — Config struct. Needs an `update` section.
- `internal/config/defaults.go` — Default config values. Needs update defaults.
- `agtop.example.toml` — Example config. Needs `[update]` section documented.
- `Makefile` — Build system. Version ldflags already in place; no changes needed.
- `go.mod` — Dependencies. Needs `creativeprojects/go-selfupdate` added.

### New Files

- `internal/update/update.go` — Core update logic: check for latest release, compare versions, download and apply binary.
- `internal/update/update_test.go` — Tests for version comparison and update logic.
- `cmd/agtop/version.go` — `version` subcommand implementation.
- `.goreleaser.yaml` — goreleaser configuration for cross-platform binary builds.
- `.github/workflows/release.yml` — GitHub Actions workflow that runs goreleaser on tag push.

## Implementation Plan

### Phase 1: Foundation

Set up the `internal/update` package with version comparison and the GitHub Releases check. Add the `creativeprojects/go-selfupdate` dependency. This library handles platform detection (GOOS/GOARCH), checksum verification, and atomic binary replacement.

### Phase 2: Subcommands and Config

Add the `version` subcommand that prints the current version and optionally checks for updates. Add the `update` config section with `auto_check` and `repo` fields. Wire the update check into the TUI startup path as a non-blocking goroutine that sends a Bubble Tea message on completion.

### Phase 3: Integration

Add `UpdateAvailableMsg` to the Bubble Tea message system. When received, flash "Update available: vX.Y.Z — run `agtop update` to install" in the status bar. Add the `update` subcommand that performs the download and self-replacement.

### Phase 4: Release Infrastructure

Create `.goreleaser.yaml` for cross-platform builds (linux/amd64, linux/arm64, darwin/amd64, darwin/arm64). Create the GitHub Actions release workflow.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Add go-selfupdate dependency

- Run `go get github.com/creativeprojects/go-selfupdate` to add the dependency.

### 2. Add update config section

- In `internal/config/config.go`, add `UpdateConfig` struct with fields:
  - `AutoCheck bool` (toml: `auto_check`) — whether to check on startup
  - `Repo string` (toml: `repo`) — GitHub owner/repo, default `"justinpbarnett/agtop"`
- Add `Update UpdateConfig` field to the `Config` struct with toml tag `"update"`.
- In `internal/config/defaults.go`, add defaults: `AutoCheck: true`, `Repo: "justinpbarnett/agtop"`.
- In `agtop.example.toml`, add a commented-out `[update]` section documenting both fields.

### 3. Create internal/update package

- Create `internal/update/update.go` with:
  - `const GitHubOwnerRepo` default pulled from config.
  - `func CheckForUpdate(currentVersion, repo string) (*Release, error)` — uses `go-selfupdate` to query GitHub Releases API. Returns nil if current version is latest. Strips leading `v` for comparison. Returns early with nil if `currentVersion` is `"dev"` (untagged build).
  - `type Release struct { Version string; URL string; ReleaseNotes string }` — minimal info about available update.
  - `func Apply(repo string) error` — uses `go-selfupdate` to download the correct binary for runtime.GOOS/runtime.GOARCH, verify checksum, and atomically replace `os.Executable()`. Returns an error if any step fails.
  - `func CompareVersions(current, latest string) int` — semver comparison helper. Returns -1 if current < latest, 0 if equal, 1 if current > latest.

### 4. Create update tests

- Create `internal/update/update_test.go` with tests for:
  - `CompareVersions` — basic semver cases: equal, older, newer, with/without `v` prefix, pre-release suffixes like `v0.1.0-3-gabcdef`.
  - `CheckForUpdate` with `"dev"` version returns nil (skips check).
  - Integration test (build-tagged `//go:build integration`) that hits the real GitHub API — skipped in CI by default.

### 5. Add version subcommand

- Create `cmd/agtop/version.go` with `func runVersion()` that:
  - Prints `agtop version {Version}` using the `panels.Version` variable.
  - Calls `update.CheckForUpdate(panels.Version, defaultRepo)`.
  - If an update is available, prints `Update available: {latest}. Run "agtop update" to install.`
  - If current is latest, prints `You are up to date.`
  - If version is `"dev"`, prints `Development build — update check skipped.`

### 6. Add update subcommand

- In `cmd/agtop/main.go`, add a `"version"` case that calls `runVersion()`.
- Add an `"update"` case that:
  - Calls `update.CheckForUpdate` to find the latest release.
  - If no update available, prints "Already up to date" and returns.
  - Prints `Updating agtop to {latest}...`
  - Calls `update.Apply(repo)`.
  - On success, prints `Updated to {latest}. Restart agtop to use the new version.`
  - On failure, prints the error and exits with code 1.

### 7. Add startup update check to TUI

- In `internal/ui/messages.go`, add `UpdateAvailableMsg` type alias (not an alias — a new struct in `panels` package with `Version string` field).
- In `internal/ui/panels/messages.go` (or wherever `RunStoreUpdatedMsg` etc. are defined), add `type UpdateAvailableMsg struct { Version string }`.
- In `internal/ui/app.go`:
  - In `NewApp()`, after building the App struct, if `cfg.Update.AutoCheck` is true, store the repo string on the App.
  - In `App.Init()`, add a `tea.Cmd` that runs the update check in a goroutine: call `update.CheckForUpdate(panels.Version, repo)`, and if a newer version exists, return `UpdateAvailableMsg{Version: release.Version}`.
  - In `App.Update()`, handle `UpdateAvailableMsg`: call `a.statusBar.SetFlashWithLevel(fmt.Sprintf("Update available: v%s — run `agtop update`", msg.Version), panels.FlashInfo)` and return `flashClearCmd()`. Use a longer flash duration (e.g., 10 seconds) or persist the message in the status bar permanently since this is important but not urgent.

### 8. Create goreleaser config

- Create `.goreleaser.yaml` at project root with:
  - `builds` targeting `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`.
  - Entry point: `./cmd/agtop`.
  - Ldflags: `-s -w -X github.com/justinpbarnett/agtop/internal/ui/panels.Version={{.Version}}`.
  - `archives` with tar.gz for Linux, zip for Darwin.
  - `checksum` enabled (sha256).
  - `release` targeting GitHub.

### 9. Create GitHub Actions release workflow

- Create `.github/workflows/release.yml` that:
  - Triggers on tag push matching `v*`.
  - Checks out code with `fetch-depth: 0`.
  - Sets up Go 1.25.
  - Runs goreleaser with `goreleaser release --clean`.
  - Uses the `goreleaser/goreleaser-action` action.

### 10. Validate

- Run `make check` to verify all tests pass and `go vet` is clean.
- Run `go build ./...` to verify compilation.
- Verify the `version` subcommand works: `go run ./cmd/agtop version` should print `agtop version dev` and skip the update check.

## Testing Strategy

### Unit Tests

- `internal/update/update_test.go`:
  - `TestCompareVersions` — table-driven: `("v0.1.0", "v0.2.0", -1)`, `("v1.0.0", "v1.0.0", 0)`, `("v2.0.0", "v1.0.0", 1)`, `("0.1.0", "v0.1.0", 0)`, `("v0.1.0-3-gabcdef", "v0.1.0", -1)`, `("dev", "v1.0.0", -1)`.
  - `TestCheckForUpdateDevVersion` — returns nil, no error.
  - `TestCheckForUpdateSameVersion` — mock or skip; verifies no update returned when versions match.

### Edge Cases

- Version is `"dev"` (untagged build) — skip update check entirely, don't panic or error.
- No network connectivity — `CheckForUpdate` should return a descriptive error; TUI startup should silently swallow it (log only).
- Binary is not writable (permission denied) — `Apply` should return a clear error suggesting `sudo` or reinstall.
- GitHub API rate limit — `go-selfupdate` handles this; surface the error message.
- Running from `go run` (no real binary to replace) — `Apply` should fail gracefully.

## Risk Assessment

- **Network dependency on startup** — The update check runs in a goroutine and must never block TUI startup. If the check fails or times out, the app proceeds normally. Add a 10-second timeout on the HTTP call.
- **Binary replacement safety** — `go-selfupdate` uses atomic rename, which is safe on Linux and macOS. The old binary is only removed after the new one is verified. If the process crashes mid-update, the old binary remains.
- **goreleaser config correctness** — An incorrect `.goreleaser.yaml` will fail the release workflow but won't affect existing builds. Test locally with `goreleaser release --snapshot --clean` before the first real release.
- **Existing installs via `go install`** — Users who installed via `go install` will have a binary in `$GOPATH/bin`. Self-update will replace that binary in-place. If the binary was installed via a package manager, self-update may fail due to permissions — the error message should be clear.

## Validation Commands

```bash
make check        # go vet ./... && go test ./... in parallel
go build ./...    # verify compilation
go run ./cmd/agtop version   # verify version subcommand
```

## Open Questions (Unresolved)

- **Update frequency**: Should the startup check happen every launch, or cache the last check time and only re-check after N hours? Recommendation: check every launch — the GitHub API call is fast (~100ms) and doesn't count against rate limits for public repos.
- **Auto-apply vs prompt**: Should the app auto-apply updates or just notify? Recommendation: notify only on startup (flash message), require explicit `agtop update` to apply. Auto-applying a running binary is jarring and could interrupt active runs.
- **Windows support**: goreleaser can target `windows/amd64` but the project doesn't appear to target Windows currently. Recommendation: skip Windows in the initial goreleaser config; add it later if needed.

## Sub-Tasks

Single task — no decomposition needed. All steps are sequential and tightly coupled (the update package is needed by the subcommands, which are needed by the TUI integration, which requires the release infrastructure for end-to-end testing).
