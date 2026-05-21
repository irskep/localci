# localci tasks

Fields:

- `id`
- `deps`
- `name`
- `description`
- `status`
- `validation`

Statuses:

- `unstarted`
- `inprogress`
- `done`

Validation steps are always: `lint`, `typecheck`, `test`, `commit`.

Current order:

- `LC-001` repo scaffold and mise-managed tooling
- `LC-002` self-hosting `localci:*` tasks
- `LC-003` direct `invoke` runner
- `LC-019` checkpoint after local execution baseline
- `LC-004` stabilize result model
- `LC-005` filesystem-backed status reader
- `LC-006` queue store and in-progress markers
- `LC-007` single-worker scheduler
- `LC-008` daemon lifecycle commands
- `LC-009` unix socket control API
- `LC-020` checkpoint after daemon core
- `LC-010` `postcommit` enqueue path
- `LC-011` `status` command
- `LC-012` localhost HTTP server and template views
- `LC-013` WebSocket live updates
- `LC-014` `web` command
- `LC-021` checkpoint after operator surfaces
- `LC-015` per-task attempts and retry flow
- `LC-016` daemon timeout watcher reuse
- `LC-017` crash recovery
- `LC-018` modern git and mise checks
- `LC-023` repo-local daemon mise tasks
- `LC-024` git hook install and self-hosting loop
- `LC-022` checkpoint before final hardening

## LC-001

- `id`: `LC-001`
- `deps`: `[]`
- `name`: Repo scaffold and mise-managed tooling
- `description`: Establish the Go project scaffold, pinned tools, and baseline developer tasks.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-002

- `id`: `LC-002`
- `deps`: `[LC-001]`
- `name`: Self-hosting `localci:*` tasks
- `description`: Define `localci:*` mise tasks for localci itself with artifact output wiring.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-003

- `id`: `LC-003`
- `deps`: `[LC-001, LC-002]`
- `name`: Direct invoke runner
- `description`: Implement serial task discovery and execution plus persisted invoke metadata.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-019

- `id`: `LC-019`
- `deps`: `[LC-003]`
- `name`: Checkpoint after local execution baseline
- `description`: Review invoke and artifact design for assumptions, duplication, refactor opportunities, and unnecessary defensive code.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-004

- `id`: `LC-004`
- `deps`: `[LC-019]`
- `name`: Stabilize result model
- `description`: Tighten persisted run and task metadata so later daemon and UI layers share one model.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-005

- `id`: `LC-005`
- `deps`: `[LC-004]`
- `name`: Filesystem-backed status reader
- `description`: Read current commit and task state directly from persisted artifacts.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-006

- `id`: `LC-006`
- `deps`: `[LC-004]`
- `name`: Queue store and in-progress markers
- `description`: Persist queued work and active-task markers for daemon ownership.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-007

- `id`: `LC-007`
- `deps`: `[LC-006]`
- `name`: Single-worker scheduler
- `description`: Add the one-at-a-time scheduler for machine-wide serial execution.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-008

- `id`: `LC-008`
- `deps`: `[LC-006]`
- `name`: Daemon lifecycle commands
- `description`: Implement idempotent `start` and clean `stop` around one daemon process.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-009

- `id`: `LC-009`
- `deps`: `[LC-008]`
- `name`: Unix socket control API
- `description`: Expose daemon control and status operations over a local socket.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-020

- `id`: `LC-020`
- `deps`: `[LC-007, LC-008, LC-009]`
- `name`: Checkpoint after daemon core
- `description`: Reassess daemon boundaries, scheduler shape, shared abstractions, Go quality, and any avoidable migration-style baggage.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-010

- `id`: `LC-010`
- `deps`: `[LC-007, LC-009, LC-020]`
- `name`: Postcommit enqueue path
- `description`: Implement `postcommit <repo> <commit>` against the daemon queue.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-011

- `id`: `LC-011`
- `deps`: `[LC-005, LC-009]`
- `name`: Status command
- `description`: Implement `status [dir] <commit> [task]` with daemon-backed reads and task filtering.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-012

- `id`: `LC-012`
- `deps`: `[LC-004, LC-008]`
- `name`: HTTP server and template views
- `description`: Add the localhost server plus commit and task pages with stdlib templates.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-013

- `id`: `LC-013`
- `deps`: `[LC-012]`
- `name`: WebSocket live updates
- `description`: Stream status and output file changes into the browser.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-014

- `id`: `LC-014`
- `deps`: `[LC-009, LC-012, LC-013]`
- `name`: Web command
- `description`: Implement `web [dir] <commit> [task]` for direct browser links.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-021

- `id`: `LC-021`
- `deps`: `[LC-011, LC-012, LC-013, LC-014]`
- `name`: Checkpoint after operator surfaces
- `description`: Review CLI and web layers for duplicated logic, weak APIs, stale assumptions, and code that is too defensive for a pre-user system.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-015

- `id`: `LC-015`
- `deps`: `[LC-007, LC-013, LC-021]`
- `name`: Per-task attempts and retry flow
- `description`: Add attempt-aware task output layout and single-task retry behavior.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-016

- `id`: `LC-016`
- `deps`: `[LC-007]`
- `name`: Daemon timeout watcher reuse
- `description`: Reuse the invoke timeout and output-activity semantics in daemon execution.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-017

- `id`: `LC-017`
- `deps`: `[LC-008, LC-010, LC-016]`
- `name`: Crash recovery
- `description`: Recover partially executed runs after daemon restart.
- `status`: `done`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-018

- `id`: `LC-018`
- `deps`: `[LC-001]`
- `name`: Modern git and mise checks
- `description`: Fail fast when required git or mise capabilities are missing.
- `status`: `unstarted`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-022

- `id`: `LC-022`
- `deps`: `[LC-015, LC-016, LC-017, LC-018, LC-023, LC-024]`
- `name`: Checkpoint before final hardening
- `description`: Audit the near-finished design for exemplary Go quality, simplification opportunities, and anything that should be cut before users exist.
- `status`: `unstarted`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-023

- `id`: `LC-023`
- `deps`: `[LC-008]`
- `name`: Repo-local daemon mise tasks
- `description`: Add `mise` tasks in this repo to start and stop the localci daemon for local development.
- `status`: `unstarted`
- `validation`: `lint`, `typecheck`, `test`, `commit`

## LC-024

- `id`: `LC-024`
- `deps`: `[LC-010, LC-023]`
- `name`: Git hook install and self-hosting loop
- `description`: Wire post-commit hook installation so localci can run on itself end to end in this repo.
- `status`: `unstarted`
- `validation`: `lint`, `typecheck`, `test`, `commit`
