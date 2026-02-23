# Feature: Auto-Route Workflow Selection

## Metadata

type: `feat`
task_id: `auto-route-workflow`
prompt: `New run workflow selection should default to "auto" which hits the route skill first to decide what workflow to run. If the user defines a specific workflow, route skill should be skipped.`

## Feature Description

The new run modal currently defaults to the `build` workflow, and every non-quick-fix workflow (`build`, `plan-build`, `sdlc`) redundantly includes `route` as its first skill. This feature introduces an `auto` workflow option that becomes the default selection, cleanly separating routing from execution workflows. When a user explicitly picks a workflow, the route skill is skipped entirely.

## User Story

As an agtop user
I want new runs to auto-select the best workflow for my task
So that I don't have to manually guess which workflow complexity is appropriate

## Relevant Files

- `internal/ui/panels/newrun.go` — Workflow option list, default selection, keybindings, and view rendering
- `internal/ui/panels/newrun_test.go` — Tests for the new run modal
- `internal/engine/executor.go` — Route skill handling in `executeWorkflow()`, `Execute()` entry point
- `internal/engine/workflow.go` — `ResolveWorkflow()` function
- `internal/config/config.go` — `WorkflowConfig` struct
- `agtop.example.yaml` — Example workflow definitions
- `skills/route/SKILL.md` — Route skill definition

## Implementation Plan

### Phase 1: Config — Remove route from explicit workflows, add auto

Update the example config and default workflow definitions so that `build`, `plan-build`, and `sdlc` no longer include `route` as their first skill. Add a new `auto` workflow with `skills: [route]`.

### Phase 2: UI — Add auto option, make it default

Add `auto` as a workflow option in the new run modal with a keybind (`^a`), and change the default from `build` to `auto`.

### Phase 3: Executor — No changes needed

The executor's existing route-skill handling already works correctly for this feature. When the `auto` workflow runs, its only skill is `route`. The route handler (`executor.go:202-216`) parses the route result and replaces the skill list with the resolved workflow's skills. No executor changes required.

## Step by Step Tasks

IMPORTANT: Execute every step in order, top to bottom.

### 1. Update example config workflow definitions

- In `agtop.example.yaml`, add a new `auto` workflow: `auto: { skills: [route] }`
- Remove `route` from the `build` workflow skills: `build: { skills: [build, test] }`
- Remove `route` from the `plan-build` workflow skills: `plan-build: { skills: [spec, build, test] }`
- Remove `route` from the `sdlc` workflow skills: `sdlc: { skills: [spec, decompose, build, test, review, document] }`
- Leave `quick-fix` unchanged: `quick-fix: { skills: [build, test, commit] }`

### 2. Add auto option to the new run modal UI

- In `internal/ui/panels/newrun.go`, add `auto` as the first entry in the `workflows` slice: `{key: "^a", name: "auto", workflow: "auto"}`
- Change the default `workflow` field in `NewNewRunModal` from `"build"` to `"auto"`
- Add a `case "ctrl+a":` handler in `Update()` that sets `m.workflow = "auto"`

### 3. Update tests

- In `internal/ui/panels/newrun_test.go`, update any tests that assert the default workflow is `"build"` to expect `"auto"` instead
- Add a test case for the `ctrl+a` keybind selecting `auto`

## Testing Strategy

### Unit Tests

- Verify `NewNewRunModal` defaults to `workflow: "auto"`
- Verify `ctrl+a` sets workflow to `"auto"`
- Verify existing keybinds (`ctrl+b`, `ctrl+p`, `ctrl+l`, `ctrl+q`) still work and override auto
- Verify submitting with `auto` sends `SubmitNewRunMsg{Workflow: "auto"}`

### Edge Cases

- User opens modal, types prompt, submits without changing workflow — should submit with `"auto"`
- User switches to `build` then back to `auto` — should submit with `"auto"`
- Route skill returns an invalid workflow name — executor already handles this gracefully (falls through without overriding)

## Risk Assessment

- **Low risk**: The executor's route-skill handling is already battle-tested. The only net-new behavior is the `auto` workflow entry and UI default change.
- **Config migration**: Users with existing `agtop.yaml` files that include `route` in their workflow definitions will continue to work — the route skill will simply execute as before. The change is only to the example/default config.
- **Backwards compatibility**: Users who relied on the default being `build` will now get `auto` instead, which runs `route` first. This is intentional and strictly better behavior.

## Validation Commands

```bash
make build
go vet ./...
go test ./internal/ui/panels/...
go test ./internal/engine/...
```

## Open Questions (Unresolved)

None — the design is straightforward and requires no architectural decisions.

## Sub-Tasks

Single task — no decomposition needed.
