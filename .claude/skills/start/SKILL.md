---
name: start
description: >
  Start your applications by running the development server and any
  required background services. Use when a user wants to start, run,
  launch, or boot the application. Triggers on "start the app",
  "run the dev server", "launch the application", "boot it up",
  "start the server", "spin up the app". Do NOT use for running tests,
  linting, or type checking (use just commands directly). Do NOT use
  for deploying to production or staging environments.
---

# Purpose

Starts the Keep development server and any required background services so the application is available for local development.

## Variables

This skill requires no additional input.

## Instructions

### Step 1: Start the Development Server

Run the development server using the justfile recipe:

```
just start
```

This launches the Next.js development server:

- Serves the application on `http://localhost:3000`
- Enables hot-reload so code changes take effect immediately

### Step 2: Confirm Startup

Watch the output for successful startup indicators:

- `Ready on http://localhost:3000` or `Local: http://localhost:3000` confirms the server is live
- Any startup errors (missing dependencies, database connection failures) will appear in the output

If startup fails, see the Cookbook section below.

## Workflow

1. **Start** — Run `just start` to launch the development server
2. **Confirm** — Verify the server starts without errors and is reachable at `http://localhost:3000`

## Cookbook

<If: ModuleNotFoundError or missing dependencies>
<Then: run `pnpm install` to install/update dependencies, then retry `just start`>

<If: port 3000 already in use>
<Then: find and stop the conflicting process with `lsof -i :3000`, or kill it with `kill $(lsof -ti :3000)`, then retry `just start`>

<If: database connection failure>
<Then: check that the database is running and that `.env.local` has correct connection settings. Run `just db-upgrade` if migrations are pending.>

<If: server runs in the foreground and user needs a terminal>
<Then: Next.js watches for file changes and hot-reloads automatically — use a separate terminal for other commands.>

## Validation

- The server process starts without errors
- `http://localhost:3000` is reachable

## Examples

### Example 1: Direct Start Request

**User says:** "Start the app"

**Actions:**

1. Run `just start`
2. Confirm server starts successfully

### Example 2: Run the Server

**User says:** "Run the dev server"

**Actions:**

1. Run `just start`
2. Confirm server starts successfully
