# CLI Reference

This is a short command index. The CLI is still changing, so use `localci --help` as the source of truth for exact usage.

## `localci start`

Start the daemon.

## `localci restart`

Restart the daemon.

## `localci stop`

Stop the daemon.

## `localci postcommit [--annotation key=value] <repo> <commit>`

Enqueue tasks for a committed revision. This is the command the Git post-commit hook calls.

## `localci invoke [--wait] [--no-clone] [--annotation key=value] [dir] [commit]`

Discover and enqueue tasks manually. Use `--wait` to block for results and `--no-clone` to run against the live working tree.

## `localci wait [--no-clone] [dir] [commit]`

Wait for a run to complete.

## `localci status [--no-clone] [dir] <commit> [task]`

Print status for a run.

## `localci web [--no-clone] [dir] [commit] [task]`

Open the web UI.

## `localci cancel [--no-clone]`

Cancel queued and active work for the current repository's latest run.

## `localci cancel [--no-clone] [dir] <commit> <task>`

Cancel a specific task.

## `localci install-hooks [dir]`

Install LocalCI's Git post-commit hook entry for a repository.
