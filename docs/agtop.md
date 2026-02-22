# Application: agtop

## Application Description

agtop (Agent Top) is a Terminal User Interface for orchestrating, monitoring, and managing AI agent coding workflows. It provides a lazygit/btop-inspired multi-panel interface where developers can launch, observe, and control concurrent AI agent runs against their codebase—each run executing a configurable chain of skills (route → spec → decompose → build → test → review → document) in isolated git worktrees.

The core value proposition is **visibility and control**. Today, firing off an agentic workflow means losing sight of what the agent is doing until it finishes or fails. agtop gives you a live dashboard of every run: streaming logs, token usage, cost tracking, file diffs, and interactive controls—all navigable via vim motions without leaving the terminal.

agtop is agent-runtime agnostic at the architecture level but ships with first-class Claude Code support (via the Claude Agent SDK CLI) and is designed for near-term OpenCode support. Skills follow the open Agent Skills standard (`SKILL.md`), making them portable across both runtimes.

## User Story

As a **developer running concurrent AI agent workflows**
I want to **monitor live streaming logs, control run state, and manage costs from a single TUI**
So that **I maintain full visibility and control over my agent-powered development process without context-switching between terminal tabs or losing output to scrollback**

## Problem Statement

Modern agentic coding workflows (Claude Code, OpenCode, custom skill chains) are powerful but opaque once launched. Developers face three critical gaps:

1. **No live observability.** Once a run starts, there's no unified view of what each agent is doing across concurrent runs. Logs stream to stdout of individual processes and are lost in terminal scrollback.
2. **No runtime control.** Runs can't be paused, resumed, cancelled, or redirected after launch without killing the process and manually restarting. Rate limit hits and token exhaustion cause silent failures.
3. **No cost awareness.** Token usage and dollar cost accumulate invisibly across runs. Developers don't know what a workflow costs until they check their billing dashboard hours later.

These gaps compound when running multiple concurrent workflows—the exact scenario where agentic development is most productive.

## Solution Statement

agtop solves these problems with a multi-panel TUI that acts as a control plane for AI agent workflows:

- **Live log streaming** from every active run, rendered in a dedicated panel with ANSI color support.
- **Run lifecycle management** via single-keystroke commands: launch, pause, resume, cancel, accept, reject, update, cleanup.
- **Real-time cost tracking** at the per-run and global level, with automatic pause on configurable token/cost thresholds.
- **Skill-based workflow engine** that chains skills sequentially or in parallel (via dependency graphs), with each skill executing in a fresh, hyper-targeted context.
- **Git worktree isolation** per run, with automatic dev server spin-up on hashed localhost ports for manual testing.
- **Safety hooks** that intercept dangerous commands (`rm -rf`, `git push --force`) before execution.
- **Project-level configuration** via YAML for workflows, models-per-skill, test suites, and custom skill overrides.

### Language and Framework Decision: Go + Bubble Tea

The TUI is built in **Go with Bubble Tea** (charmbracelet). The reasoning:

- **Concurrency model**: Go's goroutines and channels are purpose-built for managing dozens of concurrent agent subprocesses with streaming I/O—the core of what agtop does. Python's asyncio adds complexity for the same task.
- **Single binary distribution**: `go build` produces one static binary. No runtime dependencies, no `pip install`, no virtualenv. This matters for a personal tool you want on every machine.
- **TUI ecosystem**: Bubble Tea + Lip Gloss + Bubbles is the most mature and battle-tested TUI stack available, directly inspired by Elm architecture. lazygit is built on a similar Go TUI foundation (gocui).
- **Agent invocation**: agtop doesn't need to import the Claude Agent SDK as a library. It invokes `claude -p` (the Agent SDK CLI) as a subprocess with `--output-format stream-json`, parsing the structured JSON stream. This works identically from Go, Python, or any language. The same approach works for OpenCode (`opencode run`). The subprocess boundary is actually a feature—it provides process isolation, crash containment, and makes the runtime pluggable.
- **Performance**: A TUI that redraws on every streaming token needs to be fast. Go's compiled speed and Bubble Tea's optimized rendering pipeline handle this without frame drops.

Python (via Textual) is a credible alternative—Textual has a CSS-based styling model and good widget library—but the concurrency and distribution story is weaker for this specific use case. The Claude Agent SDK's Python package (`claude-agent-sdk`) is useful if you're building an application that needs to import the SDK as a library (e.g., for custom tools via in-process MCP servers), but agtop's interaction model is process orchestration, not library embedding.

## Implementation Plan

### Phase 1: Foundation

**Goal**: Core TUI shell with run list, log viewer, and subprocess management.

1. **Project scaffolding**: Go module, Bubble Tea app skeleton, Lip Gloss theme system, config loading (YAML via `gopkg.in/yaml.v3`).
2. **Layout engine**: Multi-panel layout with resizable panes. Primary panels: Run List (left), Detail Panel (right, tabbed: Details / Logs / Diff), Status Bar (bottom). Implement vim-motion navigation (`j`/`k` for list, `tab` for panel cycling, `h`/`l` for detail tabs).
3. **Process manager**: Goroutine-based subprocess orchestrator. Launches `claude -p` processes, captures stdout/stderr via pipes, parses `stream-json` output format, routes messages to the correct run's log buffer. Handles process lifecycle (spawn, signal, wait).
4. **Run state machine**: Each run tracks its state: `queued` → `routing` → `running` → `paused` → `completed` → `reviewing` → `accepted`/`rejected`/`failed`. State transitions triggered by skill completions, user commands, or error conditions.
5. **Git worktree manager**: Creates/destroys worktrees per run. Shells out to `git worktree add`/`remove`. Tracks worktree paths and branch names.

### Phase 2: Core Implementation

**Goal**: Full skill execution engine, cost tracking, safety hooks, and all run lifecycle commands.

1. **Skill engine**: Reads `SKILL.md` files from project and user directories. Parses YAML frontmatter for name, description, and custom agtop fields (`model`, `parallel-group`, `timeout`). Chains skills into workflows defined in config.
2. **Workflow executor**: Executes a workflow as a sequence of skill invocations. Each skill is a separate `claude -p` call with a hyper-specific prompt assembled from the skill's `SKILL.md` content plus run context. Supports parallel execution for skills in the same parallel group (from decompose output). Passes only the minimal required context between skills (the sniper pattern).
3. **Cost tracker**: Parses `usage` and `total_cost_usd` fields from the Agent SDK's stream-json output. Aggregates per-run and global totals. Auto-pauses runs that exceed configurable token or cost thresholds.
4. **Safety hooks**: Three-layer interception (prompt injection, Claude Code PreToolUse hooks, `--allowedTools` restriction). Configurable via YAML.
5. **Run commands**: All lifecycle keybinds: `n` (new), `p` (pause), `r` (resume), `c` (cancel), `a` (accept → auto-PR), `x` (reject → cleanup), `u` (update → new run on same worktree), `R` (resume from checkpoint).
6. **Dev server manager**: After run completion, launches dev server on a deterministic hashed port.
7. **Diff viewer**: Syntax-highlighted git diff in the Detail Panel's Diff tab.

### Phase 3: Integration

**Goal**: OpenCode support, persistence, and polish.

1. **Runtime abstraction**: `Runtime` interface with `ClaudeCodeRuntime` and `OpenCodeRuntime` implementations. Both parse their streaming output into a common `RunEvent` type.
2. **Custom skill authoring**: Project-local skills in `.agtop/skills/` override built-ins by name. Standard `SKILL.md` format with optional agtop frontmatter extensions.
3. **Notification system**: Desktop notifications via `osascript`/`notify-send` on run completion, failure, or permission-needed.
4. **Session persistence**: Serialize run state to disk so runs survive agtop restarts. Reconnect to live subprocesses on startup.
5. **Config hot-reload**: Watch `agtop.yaml` for changes, apply non-destructive updates without restart.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Project Initialization

- Initialize Go module: `go mod init github.com/<user>/agtop`
- Add dependencies: `github.com/charmbracelet/bubbletea`, `github.com/charmbracelet/lipgloss`, `github.com/charmbracelet/bubbles`, `gopkg.in/yaml.v3`
- Create directory structure:

```
agtop/
├── cmd/agtop/main.go          # Entry point
├── internal/
│   ├── tui/                    # All Bubble Tea models and views
│   │   ├── app.go              # Root model, layout, panel routing
│   │   ├── runlist.go          # Run list panel (left)
│   │   ├── detail.go           # Detail panel (right, tabbed)
│   │   ├── logs.go             # Log viewer component
│   │   ├── diff.go             # Diff viewer component
│   │   ├── statusbar.go        # Global status bar
│   │   ├── modal.go            # Simple modal system (lazygit-style)
│   │   ├── input.go            # Text input for new run prompt
│   │   └── theme.go            # Lip Gloss styles, colors, borders
│   ├── engine/                 # Workflow and skill execution
│   │   ├── skill.go            # SKILL.md parser and registry
│   │   ├── workflow.go         # Workflow definition and sequencing
│   │   ├── executor.go         # Skill execution orchestrator
│   │   └── decompose.go        # Dependency graph / parallel group logic
│   ├── runtime/                # Agent runtime abstraction
│   │   ├── runtime.go          # Runtime interface
│   │   ├── claude.go           # Claude Code runtime (claude -p)
│   │   └── opencode.go         # OpenCode runtime (opencode run)
│   ├── process/                # Subprocess management
│   │   ├── manager.go          # Process pool, lifecycle management
│   │   ├── stream.go           # stream-json parser
│   │   └── pipe.go             # Stdout/stderr pipe handling
│   ├── run/                    # Run state management
│   │   ├── run.go              # Run struct, state machine
│   │   ├── store.go            # In-memory run store
│   │   └── persistence.go      # Session serialization/deserialization
│   ├── git/                    # Git operations
│   │   ├── worktree.go         # Worktree create/remove/list
│   │   └── diff.go             # Diff generation and parsing
│   ├── safety/                 # Safety hooks
│   │   ├── hooks.go            # Hook engine
│   │   └── patterns.go         # Dangerous command patterns
│   ├── cost/                   # Cost tracking
│   │   ├── tracker.go          # Per-run and global cost aggregation
│   │   └── limits.go           # Threshold-based auto-pause
│   ├── server/                 # Dev server management
│   │   └── devserver.go        # Spin up/tear down dev servers
│   └── config/                 # Configuration
│       ├── config.go           # YAML config struct and loader
│       └── defaults.go         # Default config values
├── skills/                     # Built-in generic skills
│   ├── route/SKILL.md
│   ├── spec/SKILL.md
│   ├── decompose/SKILL.md
│   ├── build/SKILL.md
│   ├── test/SKILL.md
│   ├── review/SKILL.md
│   ├── document/SKILL.md
│   ├── commit/SKILL.md
│   └── pr/SKILL.md
└── agtop.example.yaml          # Example config file
```

- Set up Makefile with `build`, `run`, `install`, `lint` targets

### 2. Configuration System

- Define `agtop.yaml` schema:

```yaml
# agtop.yaml — project-level configuration
project:
  name: my-project
  root: .
  test_command: "npm test"
  dev_server:
    command: "npm run dev"
    port_strategy: hash # hash | sequential | fixed
    base_port: 3100

runtime:
  default: claude # claude | opencode
  claude:
    model: sonnet # Default model for skills
    permission_mode: acceptEdits # acceptEdits | acceptAll | manual
    max_turns: 50
    allowed_tools:
      - Read
      - Write
      - Edit
      - MultiEdit
      - Bash
      - Grep
      - Glob
  opencode:
    model: anthropic/claude-sonnet-4-5
    agent: build

workflows:
  build:
    skills: [route, build, test]
  plan-build:
    skills: [route, spec, build, test]
  sdlc:
    skills: [route, spec, decompose, build, test, review, document]
  quick-fix:
    skills: [build, test, commit]

skills:
  route:
    model: haiku
    timeout: 60
  spec:
    model: opus
  decompose:
    model: opus
  build:
    model: sonnet
    timeout: 300
    parallel: true
  test:
    model: sonnet
    timeout: 120
  review:
    model: opus
  document:
    model: haiku
  commit:
    model: haiku
    timeout: 30
  pr:
    model: haiku
    timeout: 30

safety:
  blocked_patterns:
    - 'rm\s+-[rf]+\s+/'
    - 'git\s+push.*--force'
    - 'DROP\s+TABLE'
    - '(curl|wget).*\|\s*(sh|bash)'
    - 'chmod\s+777'
    - ":(){.*};" # Fork bomb
  allow_overrides: false

limits:
  max_tokens_per_run: 500000
  max_cost_per_run: 5.00
  max_concurrent_runs: 5
  rate_limit_backoff: 60 # Seconds

ui:
  theme: default
  show_token_count: true
  show_cost: true
  log_scroll_speed: 5
```

- Implement config loader with defaults, validation, and env var overrides (`AGTOP_RUNTIME`, `AGTOP_MODEL`, etc.)
- Config file discovery: `./agtop.yaml` → `~/.config/agtop/config.yaml` → built-in defaults

### 3. TUI Shell and Layout

- Implement root Bubble Tea model (`app.go`) with three regions:
  - **Left panel** (~30% width): Run list with status indicators
  - **Right panel** (~70% width): Tabbed detail view (Details / Logs / Diff)
  - **Bottom bar** (1 line): Global status + keybind hints
- Lip Gloss theme (`theme.go`):
  - Thin borders with rounded corners (`lipgloss.RoundedBorder()`)
  - Muted color palette: borders in dim gray, active panel border highlighted
  - Status colors: running=blue, paused=yellow, completed=green, failed=red, reviewing=magenta
- Vim-motion navigation:
  - `j`/`k`: Move selection in run list
  - `h`/`l`: Cycle detail tabs
  - `G`/`gg`: Jump to bottom/top
  - `/`: Filter runs
  - `Tab`: Cycle panel focus
  - `?`: Help modal
  - `q`: Quit (confirm if active runs)
- Modal system (`modal.go`): Centered overlay, keybind-driven actions, no field tabbing

### 4. Run List Panel

- Each run rendered as a row:

```
● #001  feat/add-auth    sdlc       building [3/7]   12.4k tok  $0.42
◐ #002  fix/nav-bug      quick-fix  paused   [1/3]    3.1k tok  $0.08
✓ #003  feat/dashboard   plan-build reviewing        45.2k tok  $1.23
✗ #004  fix/css-overflow build      failed   [2/3]    8.7k tok  $0.31
```

- Fields: status icon, run ID, branch name, workflow, state + progress, tokens, cost
- Highlighted row = active selection; dimmed rows for terminal states

### 5. Process Manager and Stream Parser

- Subprocess spawning via `os/exec` with stdout/stderr pipes
- Claude Code runtime CLI invocation:

```bash
claude -p "<skill prompt>" \
  --output-format stream-json \
  --model <model> \
  --allowedTools "<tools>" \
  --max-turns <turns> \
  --cwd <worktree-path>
```

- Parse `stream-json` line by line. Key types:
  - `assistant` messages with `content` blocks (`text`, `tool_use`)
  - `result` messages with `usage` and `total_cost_usd`
  - Stream events for partial rendering
- Route events into per-run ring buffers via Go channels
- Graceful shutdown: SIGTERM → 5s grace → SIGKILL
- Each subprocess in its own goroutine with cancellable `context.Context`

### 6. Skill Engine

- Scan for skills at startup (precedence order, highest first):
  1. `.agtop/skills/*/SKILL.md` — project agtop overrides
  2. `.claude/skills/*/SKILL.md` — project Claude Code
  3. `.agents/skills/*/SKILL.md` — project OpenCode/standard
  4. `~/.config/agtop/skills/*/SKILL.md` — user agtop
  5. `~/.claude/skills/*/SKILL.md` — user Claude Code
  6. `<binary-dir>/skills/*/SKILL.md` — built-in defaults
- Parse frontmatter: `name`, `description`, plus agtop extensions (`model`, `timeout`, `parallel-group`, `allowed-tools`)
- Same-name resolution: highest precedence wins
- **Skill injection model**: agtop does NOT use native skill discovery. It reads SKILL.md and injects content directly as the system prompt for each `claude -p` call:

```
System: {SKILL.md markdown body}

Context:
- Working directory: {worktree}
- Branch: {branch}
- Previous skill output: {summary}

Task: {user prompt}
```

- Custom skills work immediately—just SKILL.md files that agtop reads and injects

### 7. Workflow Executor

- Workflow = ordered list of skill names (from config)
- Sequential execution, passing minimal context forward:
  - Route → determines workflow
  - Spec → writes `SPEC.md` to worktree
  - Decompose → dependency graph JSON (parsed for parallel groups)
  - Build/test/review/document → operate on worktree
- Parallel execution: decompose output parsed for independent nodes, dispatched as concurrent `claude -p` subprocesses
- Checkpoint between skills. Resume restarts from last incomplete skill
- Failure (non-zero exit, timeout, rate limit) → `paused` state with reason

### 8. Cost Tracker

- Parse `total_cost_usd` and `usage` from `result` messages
- Per-run accumulators: total tokens, total cost
- Global accumulators across session
- Status bar: `Runs: 5 (2 active) │ Tokens: 142.3k │ Cost: $4.87`
- Detail view: per-skill token/cost breakdown
- **Auto-pause**: SIGSTOP on threshold breach + modal notification
- **Rate limit**: Parse 429 errors → auto-pause → countdown timer → auto-resume after backoff

### 9. Safety Hooks

Three layers of defense:

**Layer 1 — Prompt injection**: Every skill prompt includes safety instructions ("NEVER execute: rm -rf, git push --force, ..."). Soft guardrail, effective with well-behaved models.

**Layer 2 — Claude Code PreToolUse hook**: `agtop init` deploys a hook to `.claude/settings.json`:

```json
{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": ".agtop/hooks/safety-guard.sh"
          }
        ]
      }
    ]
  }
}
```

The hook script checks commands against blocked patterns and exits with code 2 to block execution. This runs inside Claude Code's process, intercepting tool calls before execution regardless of permission mode.

**Layer 3 — Tool restriction**: `--allowedTools` on `claude -p` per-skill. A `review` skill gets `Read,Grep`; a `build` skill gets the full set. Configured in `agtop.yaml` under `skills.<name>.allowed-tools`.

### 10. Git Worktree and Dev Server Management

- New run: `git worktree add .agtop/worktrees/<run-id> -b agtop/<run-id>`
- Completion → launch dev server: `base_port + (hash(run_id) % 100)`, display URL in details
- Accept: push branch, `gh pr create` (configurable), cleanup
- Reject: `git worktree remove`, delete branch
- Cleanup: kill dev server, remove worktree, delete branch

### 11. Log Viewer

- Ring buffer per run (default 10k lines)
- ANSI color support
- Vim motions: `j`/`k` scroll, `G` follow, `gg` top, `/` search
- Auto-scroll on active runs; user scroll-up disables follow; `G` re-enables
- Prefix: `[14:32:05 build] Editing src/auth.ts...`

### 12. Diff Viewer

- `git diff main...<worktree-branch>`
- File list (navigable), unified diff with color, stats summary
- Auto-refresh on state changes

### 13. New Run Modal

- `n` keybind triggers modal:
  1. Text input: task description
  2. Hotkey: workflow (`b`=build, `p`=plan-build, `s`=sdlc, `q`=quick-fix)
  3. Hotkey: model override (optional)
  4. `Enter` confirm, `Esc` cancel
- On confirm: create worktree → init run → start executor

### 14. OpenCode Runtime Support

- `OpenCodeRuntime` behind `Runtime` interface
- Invocation: `opencode run "<prompt>"` with flags
- Parse output into common `RunEvent` type
- Skill injection identical to Claude Code path

### 15. Session Persistence and Recovery

- Serialize run state on every transition to `~/.agtop/sessions/<project-hash>/<run-id>.json`
- State: run ID, branch, worktree, workflow, skill index, cost/tokens, log tail, PID
- Startup: rehydrate, reconnect live PIDs, mark dead non-terminal runs as failed
- `agtop cleanup`: remove stale sessions and orphaned worktrees

## Edge Cases

- **Rate limit mid-skill**: Detect 429 → auto-pause → backoff timer → auto-resume. Stagger multiple run resumes to avoid thundering herd
- **Agent crash/hang**: No output for configurable timeout (default 120s) → mark stalled. `K` to force-kill
- **Worktree conflicts**: Existing branch → prompt reuse or fresh. `agtop cleanup` handles orphans
- **Disk space**: Track worktree count, warn at threshold (default 10)
- **Terminal resize**: Bubble Tea native resize handling; ensure panel reflow
- **Long-running skills**: Show per-skill elapsed time and total run duration
- **Concurrent file writes**: Parallel decomposed tasks touching same file → last writer wins. Mitigate via decompose skill marking file-level deps
- **Config changes mid-run**: Active runs snapshot config at launch. New runs get latest. Visual indicator for stale config
- **Missing `claude` binary**: Startup check, clear error with install link
- **Missing `opencode` binary**: Fall back to claude if available
- **SKILL.md parse errors**: Skip with warning, don't crash
- **Empty stream output**: Show spinner with elapsed time
- **Network disconnect**: Pause, exponential backoff retry (3 attempts)
- **Multiple agtop instances**: Lockfile per project (`~/.agtop/<hash>.lock`)
- **Zombie processes**: On startup, scan session files for orphaned PIDs, offer cleanup
- **Context window exhaustion**: Truncate previous skill output summary, warn in logs

## Acceptance Criteria

- [ ] Multi-panel TUI: run list, tabbed detail view (details/logs/diff), status bar
- [ ] Vim-motion navigation everywhere: `j`/`k`, `h`/`l`, `G`/`gg`, `/`, `?`
- [ ] New run via `n` modal with hotkey-driven workflow selection (no field tabbing)
- [ ] Runs execute in isolated git worktrees from main branch
- [ ] Each skill = separate `claude -p` subprocess with SKILL.md injected as prompt
- [ ] Real-time streaming logs with ANSI color in Logs tab
- [ ] Token count and cost in run list, detail view, and status bar — real-time
- [ ] Auto-pause on token/cost threshold with notification
- [ ] Auto-detect rate limits, pause with countdown, auto-resume
- [ ] Safety hooks block dangerous commands (rm -rf, force push, etc.)
- [ ] Single-keystroke run controls: `p` pause, `r` resume, `c` cancel, `a` accept, `x` reject, `u` update
- [ ] Accept → push + PR creation via configurable command
- [ ] Reject → worktree + branch cleanup
- [ ] Completed runs auto-launch dev server on hashed port
- [ ] `.agtop/skills/` overrides built-in skills by name
- [ ] `agtop.yaml` configures workflows, models, tests, safety, limits
- [ ] Lazygit-style modals: simple, centered, keyboard-driven
- [ ] Status bar: run count, active, total tokens, total cost
- [ ] Single Go binary, no runtime dependencies
- [ ] `agtop init` scaffolds config, skills dir, safety hooks

## Notes

### Skill Portability and the Injection Model

The Agent Skills standard (`SKILL.md` with YAML frontmatter) is supported by Claude Code, OpenCode, and OpenAI Codex. agtop embraces this standard fully but uses a different execution model:

- **Native skill discovery** (Claude Code/OpenCode): The runtime discovers skills automatically and the agent decides when to invoke them. Great for interactive use, wrong for deterministic workflow chains.
- **agtop's injection model**: agtop reads SKILL.md and injects it directly into the prompt for each `claude -p` invocation. The agent never "discovers" the skill—it receives the instructions as its system prompt. This guarantees deterministic ordering with exactly the context agtop provides.

A user can write a skill once and use it both interactively in Claude Code (`/skill-name`) and in an agtop workflow. The file is the same; the invocation mechanism differs.

### Key Architecture Decisions

| Decision         | Choice                                      | Rationale                                                             |
| ---------------- | ------------------------------------------- | --------------------------------------------------------------------- |
| Language         | Go                                          | Goroutines for concurrent subprocess I/O, single binary, fast redraws |
| TUI framework    | Bubble Tea + Lip Gloss                      | Elm architecture, battle-tested, lazygit-adjacent                     |
| Agent invocation | Subprocess (`claude -p`)                    | Process isolation, crash containment, runtime-agnostic                |
| Streaming format | `--output-format stream-json`               | Structured JSON with token/cost data, line-by-line parseable          |
| Skill execution  | Prompt injection                            | Deterministic ordering, fresh context per skill, runtime-agnostic     |
| Config format    | YAML                                        | Human-readable, good Go support                                       |
| Safety model     | Three-layer (prompt + hook + tool restrict) | Defense in depth                                                      |

### Future Considerations

- **Web dashboard**: Companion UI via local WebSocket for richer rendering
- **Team sharing**: Export/import workflow configs and skill sets as packages
- **Metrics**: Persist cost/token data across sessions for trend analysis
- **Multi-repo**: First-class config for projects spanning multiple repositories
- **MCP integration**: Pass `--mcp-config` to agent runtime for external tools
- **AI-powered routing**: Auto-select or dynamically compose workflows
- **Daemon mode**: Background agtop, submit via CLI (`agtop run "..." --workflow sdlc`), reconnect TUI later
- **Replay mode**: Re-render completed run logs at configurable speed for review/demo
