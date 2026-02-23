# agtop

A terminal UI for orchestrating and monitoring AI agent coding workflows. Launch, observe, and control concurrent agent runs from a lazygit/btop-inspired dashboard — with streaming logs, token usage, cost tracking, and vim-style navigation.

## Why agtop

Running AI coding agents in the background means losing visibility into what they're doing. agtop gives you a live multi-panel dashboard so you can watch multiple agent runs in parallel, see their logs and diffs in real time, track costs, and intervene when needed — all without leaving your terminal.

## Features

- **Multi-panel TUI** — Run list, tabbed detail view (details/logs/diffs), status bar, and help overlay in a responsive terminal layout
- **Vim-style navigation** — `j`/`k` to move, `l`/`h` to switch tabs, `G`/`gg` to jump, `/` to filter, `?` for help
- **Run controls** — Pause, resume, cancel, accept, and reject runs directly from the dashboard
- **Skill-based workflows** — Configurable chains of skills (route, spec, decompose, build, test, review, document, commit, PR)
- **Multiple runtimes** — Supports Claude Code (`claude -p`) and OpenCode (`opencode run`)
- **Git worktree isolation** — Each agent run operates in its own worktree
- **Cost and token tracking** — Per-run and session-wide aggregation with auto-pause thresholds
- **Safety guardrails** — Blocked command patterns, tool restrictions, and hook-based filtering
- **Session persistence** — Run state saved to disk and recovered on restart
- **Dev server management** — Auto-detection and port allocation for dev servers
- **TOML configuration** — Project, runtime, workflow, and UI settings with sensible defaults

## Getting Started

### Prerequisites

- Go 1.25+
- One of: [Claude Code](https://github.com/anthropics/claude-code) or [OpenCode](https://github.com/opencode-ai/opencode)

### Install

```bash
go install github.com/justinpbarnett/agtop/cmd/agtop@latest
```

Or build from source:

```bash
git clone https://github.com/justinpbarnett/agtop.git
cd agtop
make build       # outputs to bin/agtop
make install     # installs to $GOPATH/bin
```

### Usage

Run `agtop` from within a project directory:

```bash
agtop
```

#### Subcommands

| Command               | Description                                            |
| --------------------- | ------------------------------------------------------ |
| `agtop`               | Start the interactive dashboard                        |
| `agtop init`          | Initialize project (hooks, config, safety guard)       |
| `agtop cleanup`       | Remove stale sessions and orphaned worktrees           |
| `agtop cleanup --dry-run` | Preview cleanup without deleting anything          |

`agtop init` creates `.agtop/hooks/` with a safety guard script, wires it into `.claude/settings.json` as a PreToolUse hook, and copies `agtop.example.toml` to `agtop.toml` if one doesn't exist.

### Configuration

agtop looks for configuration in this order:

1. `./agtop.toml` (project root)
2. `~/.config/agtop/config.toml` (user config)
3. Built-in defaults

See [`agtop.example.toml`](agtop.example.toml) for all available options.

```toml
[project]
name = "my-project"
test_command = "npm test"

[project.dev_server]
command = "npm run dev"
port_strategy = "hash"   # hash | sequential | fixed
base_port = 3100

[runtime]
default = "claude"       # claude | opencode

[runtime.claude]
model = "opus"
permission_mode = "acceptEdits"
max_turns = 50

[runtime.opencode]
model = "anthropic/claude-sonnet-4-5"
agent = "build"

[workflows.build]
skills = ["build", "test"]

[workflows.plan-build]
skills = ["spec", "build", "test"]

[workflows.sdlc]
skills = ["spec", "decompose", "build", "test", "review", "document"]

[workflows.quick-fix]
skills = ["build", "test", "commit"]

[limits]
max_cost_per_run = 5.00
max_concurrent_runs = 5
```

### Key Bindings

| Key            | Action                     |
| -------------- | -------------------------- |
| `j` / `k`      | Navigate run list          |
| `l` / `h`      | Next / previous detail tab |
| `G` / `gg`     | Jump to bottom / top       |
| `Tab`          | Cycle panel focus          |
| `/`            | Filter runs                |
| `n`            | New run                    |
| `Space`        | Pause / resume run         |
| `r`            | Restart run                |
| `c`            | Cancel run                 |
| `d`            | Delete run                 |
| `a`            | Accept run outcome         |
| `x`            | Reject run outcome         |
| `D`            | Toggle dev server          |
| `?`            | Toggle help                |
| `q` / `Ctrl+C` | Quit                       |

## Project Structure

```
cmd/agtop/         Entry point and subcommands (init, cleanup)
internal/
  config/          YAML config loading and validation
  ui/              Bubble Tea UI components
    panels/        Run list, logs, details, diffs, status bar, help, modals
    layout/        Terminal layout management
    styles/        Theme and style definitions
  engine/          Skill registry and workflow execution
  run/             Run state management and persistence
  runtime/         Agent runtime abstraction (Claude, OpenCode)
  process/         Subprocess management and streaming
  git/             Worktree and diff operations
  cost/            Token and cost tracking
  safety/          Command pattern filtering and hooks
  server/          Dev server management
skills/            Built-in skill definitions (SKILL.md files)
```

## Development

```bash
make build    # compile to bin/agtop
make run      # go run
make install  # install to $GOPATH/bin
make lint     # go vet
make clean    # remove build artifacts
```

## License

See [LICENSE](LICENSE) for details.
