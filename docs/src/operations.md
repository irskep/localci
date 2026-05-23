# How-to Guides

These commands cover the common ways to run and inspect LocalCI checks.

## Run checks for the latest commit

To enqueue tasks for the repository's current `HEAD` and wait for the result:

```sh title="Terminal"
localci invoke --wait
```

This uses the normal clone-based execution path.

## Run checks against the live working tree

To test uncommitted changes, use `--no-clone`:

```sh title="Terminal"
localci invoke --no-clone --wait
```

No-clone runs are labeled with a trailing `*`. They intentionally see unstaged files and local edits.

## Open the web UI

Open the latest run for the current repository:

```sh title="Terminal"
localci web
```

Open a specific commit:

```sh title="Terminal"
localci web . <commit>
```

Open a specific task:

```sh title="Terminal"
localci web . <commit> <task>
```

Use `--no-clone` when viewing a no-clone run:

```sh title="Terminal"
localci web --no-clone . HEAD
```

## Wait for a post-commit run

After `localci postcommit` enqueues tasks, use the wait command it prints:

```sh title="Terminal"
localci wait <repo> <commit>
```

For the current repository's latest run:

```sh title="Terminal"
localci wait
```

For a no-clone run:

```sh title="Terminal"
localci wait --no-clone
```

## Cancel work

Cancel queued or running work for the current repository:

```sh title="Terminal"
localci cancel
```

Cancel a specific task:

```sh title="Terminal"
localci cancel <repo> <commit> <task>
```

Use `--no-clone` for no-clone runs:

```sh title="Terminal"
localci cancel --no-clone
```
