# agtop

A terminal UI for orchestrating and monitoring AI agent coding workflows. Launch, observe, and control concurrent agent runs from a lazygit/btop-inspired dashboard — with streaming logs, token usage, cost tracking, and vim-style navigation.

## Why agtop

Running AI coding agents in the background means losing visibility into what they're doing. agtop gives you a live multi-panel dashboard so you can watch multiple agent runs in parallel, see their logs and diffs in real time, track costs, and intervene when needed — all without leaving your terminal.

## Features

- **Multi-panel TUI** — Run list, tabbed detail view (details/logs/diffs), and status bar in a responsive terminal layout
- **Vim-style navigation** — `j`/`k` to move, `l`/`h` to switch tabs, `Tab` to cycle panels, `?` for help
- **Skill-based workflows** — Configurable chains of skills (route, spec, decompose, build, test, review, document, commit, PR)
- **Multiple runtimes** — Supports Claude Code (`claude -p`) and OpenCode (`opencode run`)
- **Git worktree isolation** — Each agent run operates in its own worktree
- **Cost and token tracking** — Per-run and global aggregation with auto-pause thresholds
- **Safety guardrails** — Blocked command patterns, tool restrictions, and prompt-level safeguards
- **YAML configuration** — Project, runtime, workflow, and UI settings with sensible defaults

## Getting Started

### Prerequisites

- Go 1.25+
- One of: [Claude Code](https://github.com/anthropics/claude-code) or [OpenCode](https://github.com/opencode-ai/opencode)

### Install

```bash
go install github.com/jpb/agtop/cmd/agtop@latest
```

Or build from source:

```bash
git clone https://github.com/jpb/agtop.git
cd agtop
make build       # outputs to bin/agtop
make install     # installs to $GOPATH/bin
```

### Usage

Run `agtop` from within a project directory:

```bash
agtop
```

agtop looks for configuration in this order:

1. `./agtop.yaml` (project root)
2. `~/.config/agtop/config.yaml` (user config)
3. Built-in defaults

See [`agtop.example.yaml`](agtop.example.yaml) for all available options.

### Key Bindings

| Key | Action |
|-----|--------|
| `j` / `k` | Navigate run list |
| `l` / `h` | Next / previous detail tab |
| `G` / `gg` | Jump to bottom / top |
| `Tab` | Cycle panel focus |
| `?` | Toggle help |
| `q` / `Ctrl+C` | Quit |

## Configuration

```yaml
project:
  name: my-project
  test_command: "npm test"

runtime:
  default: claude
  claude:
    model: sonnet
    max_turns: 50

workflows:
  build:      { skills: [route, build, test] }
  plan-build: { skills: [route, spec, build, test] }
  sdlc:       { skills: [route, spec, decompose, build, test, review, document] }
  quick-fix:  { skills: [build, test, commit] }

limits:
  max_cost_per_run: 5.00
  max_concurrent_runs: 5
```

## Project Structure

```
cmd/agtop/         Entry point
internal/
  config/          YAML config loading and validation
  tui/             Bubble Tea UI components
  run/             Run state management
  engine/          Skill parsing and workflow execution
  runtime/         Agent runtime abstraction (Claude, OpenCode)
  process/         Subprocess management and streaming
  git/             Worktree and diff operations
  cost/            Token and cost tracking
  safety/          Command pattern filtering
skills/            Built-in skill definitions (SKILL.md files)
```

## Development

```bash
make build    # compile to bin/agtop
make run      # go run
make lint     # go vet
make clean    # remove build artifacts
```

## License

See [LICENSE](LICENSE) for details.
