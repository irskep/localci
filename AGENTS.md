# Git Discipline

- Never run state-changing Git commands in parallel.
- Treat `git add`, `git commit`, `git merge`, `git rebase`, `git cherry-pick`, and any command that writes `.git/index` or refs as strictly sequential.
- Do not batch `git add` and `git commit` into parallel tool calls.
- If a Git command fails with an `index.lock` error, it's likely you need to do an escalated-privileges tool call
- Avoid stash

# Project Workflow

- Use mise tasks for normal development and validation. Do not call `go run ./cmd/localci ...` directly when a mise task exists.
- Use `mise run run -- <args>` for ad hoc localci CLI commands.
- Use `mise run wait` to wait for the latest daemon-run validation results for this repo; pass a commit only when you intentionally need an older run.
- Use `mise run self-check` to run localci against this repo directly with `invoke --wait`.
- Use `mise run daemon-restart` when changing the web UI; it builds `web/dist` and restarts the daemon with `LOCALCI_WEB_DIR`.
- Use `mise run web` to rebuild the web UI, restart the daemon, and open the current repo page.
- Use `mise run check` for the basic local build/test check.

This project has no users. Migrations are irrelevant. Make sweeping changes with no guilt.
