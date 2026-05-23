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

Task artifact pages show the full artifact path. On macOS, use the page's Show in Finder action to reveal that file through the daemon.

## Use the terminal UI

Open the all-repos dashboard:

```sh
localci dash
```

Open a specific task:

```sh
localci dash <commit> <task>
```

The terminal UI supports the same run, task, retry, cancel, and artifact views as the web UI. On artifact views, use `e` to edit the selected artifact with `$VISUAL` or `$EDITOR`, `o` to open it with the platform opener, and `f` on macOS to reveal it in Finder through the daemon.

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
