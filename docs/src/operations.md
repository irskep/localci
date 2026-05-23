# Usage

These commands cover the common ways to run and inspect LocalCI checks.

## Run checks for the latest commit

To enqueue tasks for the repository's current `HEAD` and wait for the result:

```sh
localci invoke --wait
```

This uses the normal clone-based execution path.

## Run checks against the live working tree

To test uncommitted changes, use `--no-clone`:

```sh
localci invoke --no-clone --wait
```

No-clone runs are labeled with a trailing `*`. They intentionally see unstaged files and local edits.

## Open the web UI

Open the latest run for the current repository:

```sh
localci web
```

Open the terminal UI:

```sh
localci dash
```

Open a specific commit:

```sh
localci web <commit>
```

Open a specific task:

```sh
localci web <commit> <task>
```

Use `--no-clone` when viewing a no-clone run:

```sh
localci web --no-clone HEAD
```

## Wait for a post-commit run

After `localci postcommit` enqueues tasks, use the wait command it prints:

```sh
localci wait --repo <repo> <commit>
```

For the current repository's latest run:

```sh
localci wait
```

For a no-clone run:

```sh
localci wait --no-clone
```

## Cancel work

Cancel queued or running work for the current repository:

```sh
localci cancel
```

Cancel a specific task:

```sh
localci cancel --repo <repo> <commit> <task>
```

Use `--no-clone` for no-clone runs:

```sh
localci cancel --no-clone
```
