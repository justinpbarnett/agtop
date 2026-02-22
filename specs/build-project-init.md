# Build: Project Initialization

## Metadata

type: `build`
task_id: `project-init`
prompt: `Initialize the agtop Go project: Go module, core dependencies (Bubble Tea, Lip Gloss, Bubbles, yaml.v3), full directory structure with package files, entry point, and Makefile with build/run/install/lint targets`

## Description

Scaffold the agtop project from scratch. This is a greenfield Go project — the repo currently contains only `docs/agtop.md` (the design document) and `.claude/skills/` (skill definitions). This task creates the Go module, pulls in core dependencies, lays out the full package directory structure with minimal placeholder files so every package compiles, wires up the entry point (`cmd/agtop/main.go`), and provides a Makefile for day-to-day development.

The goal is a project that runs `go build ./...` and `go vet ./...` cleanly after this step, producing a binary that launches and immediately exits (or prints a version string). No real TUI logic, no business logic — just the skeleton that later steps build on.

## Relevant Files

### New Files

- `go.mod` — Go module definition (`github.com/jpb/agtop`, Go 1.23+)
- `cmd/agtop/main.go` — Entry point; initializes Bubble Tea app and runs it
- `internal/tui/app.go` — Root Bubble Tea model (stub `Init`, `Update`, `View`)
- `internal/tui/runlist.go` — Run list panel model (stub)
- `internal/tui/detail.go` — Detail panel model (stub)
- `internal/tui/logs.go` — Log viewer component (stub)
- `internal/tui/diff.go` — Diff viewer component (stub)
- `internal/tui/statusbar.go` — Status bar component (stub)
- `internal/tui/modal.go` — Modal system (stub)
- `internal/tui/input.go` — Text input component (stub)
- `internal/tui/theme.go` — Lip Gloss styles and color definitions (stub)
- `internal/engine/skill.go` — SKILL.md parser and registry (stub)
- `internal/engine/workflow.go` — Workflow definition (stub)
- `internal/engine/executor.go` — Skill execution orchestrator (stub)
- `internal/engine/decompose.go` — Dependency graph logic (stub)
- `internal/runtime/runtime.go` — Runtime interface definition (stub)
- `internal/runtime/claude.go` — Claude Code runtime (stub)
- `internal/runtime/opencode.go` — OpenCode runtime (stub)
- `internal/process/manager.go` — Process pool and lifecycle (stub)
- `internal/process/stream.go` — stream-json parser (stub)
- `internal/process/pipe.go` — Stdout/stderr pipe handling (stub)
- `internal/run/run.go` — Run struct and state machine (stub)
- `internal/run/store.go` — In-memory run store (stub)
- `internal/run/persistence.go` — Session serialization (stub)
- `internal/git/worktree.go` — Worktree management (stub)
- `internal/git/diff.go` — Diff generation (stub)
- `internal/safety/hooks.go` — Hook engine (stub)
- `internal/safety/patterns.go` — Dangerous command patterns (stub)
- `internal/cost/tracker.go` — Cost aggregation (stub)
- `internal/cost/limits.go` — Threshold-based auto-pause (stub)
- `internal/server/devserver.go` — Dev server management (stub)
- `internal/config/config.go` — YAML config struct and loader (stub)
- `internal/config/defaults.go` — Default config values (stub)
- `agtop.example.yaml` — Example configuration file
- `Makefile` — Build, run, install, lint targets
- `.gitignore` — Ignore build artifacts, editor files, OS files

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Initialize Go Module

- Run `go mod init github.com/jpb/agtop`
- Require Go 1.23 or later in `go.mod`

### 2. Create Directory Structure

- Create all directories:
  - `cmd/agtop/`
  - `internal/tui/`
  - `internal/engine/`
  - `internal/runtime/`
  - `internal/process/`
  - `internal/run/`
  - `internal/git/`
  - `internal/safety/`
  - `internal/cost/`
  - `internal/server/`
  - `internal/config/`
  - `skills/` (for built-in skill SKILL.md files — leave empty for now, populated in a later step)

### 3. Create Entry Point

- Write `cmd/agtop/main.go`:
  - `package main`
  - Import `bubbletea` and `internal/tui`
  - Create the root `tui.App` model
  - Run `tea.NewProgram(model, tea.WithAltScreen()).Run()`
  - Exit on error with `os.Exit(1)`

### 4. Create TUI Package Stubs

- Write `internal/tui/app.go`:
  - `package tui`
  - Define `App` struct implementing `tea.Model`
  - Stub `Init() tea.Cmd` returning `nil`
  - Stub `Update(tea.Msg) (tea.Model, tea.Cmd)` handling `tea.KeyMsg` for `q`/`ctrl+c` to quit
  - Stub `View() string` returning a placeholder string (e.g., `"agtop v0.1.0 — press q to quit"`)
  - Export `NewApp() App` constructor
- Write stub files for remaining TUI components (`runlist.go`, `detail.go`, `logs.go`, `diff.go`, `statusbar.go`, `modal.go`, `input.go`, `theme.go`):
  - Each file: `package tui`, empty struct definition, no methods yet
  - `theme.go`: define placeholder Lip Gloss style variables

### 5. Create Internal Package Stubs

- Write stub files for each internal package. Every file must:
  - Declare the correct `package` name
  - Define at least one exported type or variable (so the package compiles and is importable)
  - Include no business logic
- Packages and their anchor types:
  - `internal/engine`: `Skill`, `Workflow`, `Executor`, `DecomposeResult`
  - `internal/runtime`: `Runtime` (interface with `Start`, `Stop` methods), `ClaudeRuntime`, `OpenCodeRuntime`
  - `internal/process`: `Manager`, `StreamParser`, `Pipe`
  - `internal/run`: `Run`, `Store`, `Persistence`
  - `internal/git`: `WorktreeManager`, `DiffGenerator`
  - `internal/safety`: `HookEngine`, `PatternSet`
  - `internal/cost`: `Tracker`, `LimitChecker`
  - `internal/server`: `DevServer`
  - `internal/config`: `Config` (struct with YAML tags), `Defaults` (function returning default `Config`)

### 6. Create Config Struct and Example YAML

- Write `internal/config/config.go` with the `Config` struct matching the schema from `docs/agtop.md` lines 157-245 (project, runtime, workflows, skills, safety, limits, ui sections), with `yaml` struct tags
- Write `internal/config/defaults.go` with a `DefaultConfig()` function returning sensible defaults
- Write `agtop.example.yaml` — copy the example config from `docs/agtop.md`

### 7. Add Dependencies

- Run `go get github.com/charmbracelet/bubbletea`
- Run `go get github.com/charmbracelet/lipgloss`
- Run `go get github.com/charmbracelet/bubbles`
- Run `go get gopkg.in/yaml.v3`
- Run `go mod tidy` to clean up

### 8. Create Makefile

- Write `Makefile` with these targets:
  - `build`: `go build -o bin/agtop ./cmd/agtop`
  - `run`: `go run ./cmd/agtop`
  - `install`: `go install ./cmd/agtop`
  - `lint`: `go vet ./...`
  - `clean`: `rm -rf bin/`
  - `.PHONY` declarations for all targets

### 9. Create .gitignore

- Write `.gitignore` ignoring:
  - `bin/`
  - `*.exe`
  - `.agtop/worktrees/`
  - `.DS_Store`
  - `*.swp`
  - `*.swo`

## Validation Commands

Execute these commands to validate the task is complete:

```bash
# Module and dependencies resolve
go mod tidy

# All packages compile
go build ./...

# No vet issues
go vet ./...

# Binary builds and runs (should exit cleanly or show placeholder TUI)
go build -o bin/agtop ./cmd/agtop && bin/agtop

# Makefile works
make build
make lint
make clean
```

## Notes

- Stub files should compile but contain no real logic. The purpose is to establish the package structure so later steps can fill in implementations without needing to create directories or resolve import cycles.
- The `Runtime` interface in `internal/runtime/runtime.go` is the key abstraction — define it early so `claude.go` and `opencode.go` can implement it in later steps.
- Go 1.23+ is specified to ensure access to modern standard library features (iterators, slices/maps packages).
- The `skills/` directory at the project root is for built-in generic skills (SKILL.md files). These are populated in a later task step, not this one.
