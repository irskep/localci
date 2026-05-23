# CLI Reference

This is a short command index. The CLI is still changing, so use `localci --help` as the source of truth for exact usage.

## `localci start`

Start the daemon.

## `localci restart`

Restart the daemon.

## `localci stop`

Stop the daemon.

## `localci postcommit [--repo dir] [--annotation key=value] <commit>`

Enqueue tasks for a committed revision. This is the command the Git post-commit hook calls.

When `--repo` is omitted, commands that operate on a repository use the nearest ancestor of the current directory that contains `.git`.

## `localci invoke [--repo dir] [--wait] [--no-clone] [--annotation key=value] [commit]`

Discover and enqueue tasks manually. Use `--wait` to block for results and `--no-clone` to run against the live working tree.

## `localci wait [--repo dir] [--no-clone] [commit]`

Wait for a run to complete.

## `localci status [--repo dir] [--no-clone] <commit> [task]`

Print status for a run.

## `localci web [--repo dir] [--no-clone] [commit] [task]`

Open the web UI. Artifact pages show full artifact paths and can reveal files in Finder on macOS.

## `localci dash [--repo dir] [--no-clone] [commit] [task]`

Open the terminal UI. It uses the daemon's REST and websocket APIs, supports the same run, task, artifact, retry, and cancel workflows as the web UI, and defaults to the all-repos home view when no target is provided. Artifact views support `e` for `$VISUAL` or `$EDITOR`, `o` for the platform opener, and `f` for Finder on macOS.

## `localci cancel [--no-clone]`

Cancel queued and active work for the current repository's latest run.

## `localci cancel [--repo dir] [--no-clone] <commit> <task>`

Cancel a specific task.

## `localci install-hooks [--repo dir]`

Install LocalCI's Git post-commit hook entry for a repository.
