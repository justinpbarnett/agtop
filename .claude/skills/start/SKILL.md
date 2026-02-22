---
name: start
description: >
  Start your application by discovering and running the development server
  and any required background services. Auto-detects the start command from
  the project's task runner or package manager. Use when a user wants to
  start, run, launch, or boot the application. Triggers on "start the app",
  "run the dev server", "launch the application", "boot it up", "start the
  server", "spin up the app". Do NOT use for running tests, linting, or
  type checking (use the test skill or run commands directly). Do NOT use
  for deploying to production or staging environments.
---

# Purpose

Starts the project's development server and any required background services so the application is available for local development.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Discover the Start Command

Detect how to start the project by checking these sources in priority order:

1. **justfile** — Look for a `start` or `dev` recipe
2. **package.json** — Look for `start`, `dev`, or `serve` scripts
3. **Makefile** — Look for a `start`, `dev`, or `run` target
4. **docker-compose.yml** — If present, use `docker compose up`
5. **pyproject.toml / manage.py** — For Python projects, look for `runserver` or similar

If no start command is found, report this and ask the user how to start their project.

### Step 2: Start the Development Server

Run the discovered start command. Common examples:

- `just start` or `just dev`
- `npm run dev` or `pnpm dev`
- `make dev`
- `docker compose up`
- `python manage.py runserver`

### Step 3: Confirm Startup

Watch the output for successful startup indicators:

- A URL like `http://localhost:<port>` confirms the server is live
- Any startup errors (missing dependencies, database connection failures) will appear in the output

If startup fails, see the Cookbook section below.

## Workflow

1. **Discover** — Detect the start command from justfile, package.json, Makefile, etc.
2. **Start** — Run the discovered command
3. **Confirm** — Verify the server starts without errors

## Cookbook

<If: missing dependencies or ModuleNotFoundError>
<Then: install dependencies first (e.g., `npm install`, `pnpm install`, `pip install -r requirements.txt`, `bundle install`) then retry the start command>

<If: port already in use>
<Then: find and stop the conflicting process with `lsof -i :<port>`, or kill it with `kill $(lsof -ti :<port>)`, then retry>

<If: database connection failure>
<Then: check that the database is running and that environment files (.env, .env.local) have correct connection settings. Run any pending migrations.>

<If: no start command discovered>
<Then: ask the user how to start their project. Common patterns: framework CLIs (next dev, rails server, flask run), docker compose, or custom scripts.>

<If: server runs in the foreground and user needs a terminal>
<Then: the dev server watches for file changes and hot-reloads — use a separate terminal for other commands.>

## Validation

- The server process starts without errors
- A local URL is reachable

## Examples

### Example 1: Node.js Project with justfile

**User says:** "Start the app"
**Discovery:** justfile has a `start` recipe
**Action:** Run `just start`

### Example 2: Python Django Project

**User says:** "Run the dev server"
**Discovery:** manage.py found
**Action:** Run `python manage.py runserver`

### Example 3: Docker-based Project

**User says:** "Boot it up"
**Discovery:** docker-compose.yml found
**Action:** Run `docker compose up`
