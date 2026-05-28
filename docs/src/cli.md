# CLI Reference

This is a short command index. The CLI is still changing, so use `localci --help` as the source of truth for exact usage.

## `localci start`

Start the daemon.

## `localci restart`

Restart the daemon.

## `localci stop`

Stop the daemon.

## `localci postcommit [--repo dir] [--commit ref] [--task task] [--annotation key=value]`

Enqueue tasks for a committed revision. This is the command the Git post-commit hook calls.

When `--repo` is omitted, commands that operate on a repository use the nearest ancestor of the current directory that contains `.git`.

## `localci run [--repo dir] [--commit ref] [--task task] [--wait] [--no-clone] [--annotation key=value]`

Queue a daemon-managed run manually. Use `--no-clone` to run against the live working tree.

## `localci status [--repo dir] [--commit ref] [--task task] [--attempt n] [--no-clone]`

Print a bounded status summary for the selected run.

## `localci wait [--repo dir] [--commit ref] [--task task] [--no-clone]`

Wait for the selected run or task to complete.

## `localci artifacts [--repo dir] [--commit ref] [--task task] [--attempt n] [--failed] [--primary] [--paths-only]`

Print filesystem paths for task artifacts without dumping log contents.

## `localci history [--repo dir] [--commit ref] [--task task] [--status status] [--failed] [--limit n]`

Print recent runs, optionally filtered to a task or status.

## `localci web [--repo dir] [--commit ref] [--task task] [--attempt n] [--artifact artifact] [--no-clone]`

Open the web UI. Artifact pages show full artifact paths and can reveal files in Finder on macOS.

## `localci dash [--repo dir] [--commit ref] [--task task] [--attempt n] [--artifact artifact] [--no-clone]`

Open the terminal UI. It uses the daemon's REST and websocket APIs, supports the same run, task, artifact, retry, and cancel workflows as the web UI, and defaults to the all-repos home view when no target is provided. Artifact views support `e` for `$VISUAL` or `$EDITOR`, `o` for the platform opener, and `f` for Finder on macOS.

## `localci cancel [--repo dir] [--commit ref] [--task task] [--no-clone]`

Cancel queued and active work.

## `localci invoke [--repo dir] [--commit ref] [--task task] [--wait] [--no-clone] [--annotation key=value]`

Run an ad hoc check directly in the current terminal. Use `run` when you want daemon-managed queueing.

## `localci install-hooks [--repo dir]`

Install LocalCI's Git post-commit hook entry for a repository.
