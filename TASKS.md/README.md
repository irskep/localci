# localci tasks

One task per file.

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
- `LC-022` checkpoint before final hardening
