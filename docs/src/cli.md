# Command line reference

## `localci start`

Start the daemon.

## `localci restart`

Restart the daemon.

## `localci stop`

Stop the daemon.

## `localci postcommit [--annotation key=value] <repo> <commit>`

Enqueue LocalCI tasks for a committed revision. This is the command the Git post-commit hook calls.

Annotations are stored with the run and displayed in the UI. Git annotations such as branch are added automatically when available.

The command prints:

- number of enqueued tasks
- a `localci status` command
- a web results URL when the daemon exposes one
- a `localci wait` command

## `localci invoke [--wait] [--no-clone] [--annotation key=value] [dir] [commit]`

Discover and enqueue tasks manually.

With no args, `dir` defaults to the current directory and `commit` defaults to `HEAD`.

`--wait` blocks until the run completes and prints failed task details.

`--no-clone` runs against the live working tree and labels the commit as `<commit>*`.

## `localci wait [--no-clone] [dir] [commit]`

Wait for a run to complete. With no args, waits for the latest run for the current directory.

## `localci status [--no-clone] [dir] <commit> [task]`

Print status for a run. When `task` is provided, prints detail for that task.

## `localci web [--no-clone] [dir] [commit] [task]`

Open the web UI for a repo, commit, or task.

## `localci cancel [--no-clone]`

Cancel queued and active work for the current repository's latest run.

## `localci cancel [--no-clone] [dir] <commit> <task>`

Cancel a specific task.

## `localci install-hooks [dir]`

Install LocalCI's Git post-commit hook entry for a repository. `dir` defaults to the current working directory.
