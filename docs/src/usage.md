# Usage

These commands cover the common ways to run and inspect LocalCI checks.

## Follow the latest run

The normal workflow is post-commit: the hook queues work after each commit, and you inspect the daemon's latest run.

```sh
localci wait
```

Show a bounded status summary without waiting:

```sh
localci status
```

Open the same run in a browser or terminal UI:

```sh
localci web
localci dash
```

After `localci postcommit` enqueues tasks, use the wait command it prints:

```sh
localci wait --repo <repo> --commit <commit>
```

For a no-clone run:

```sh
localci wait --commit 'HEAD*'
```

## Run an ad hoc check

Use `invoke` when you explicitly want to kick off a manual run outside the post-commit path.

```sh
localci invoke --task test --wait
```

To test uncommitted changes, use `--no-clone`:

```sh
localci invoke --no-clone --task test --wait
```

No-clone runs are labeled with a trailing `*`. They intentionally see unstaged files and local edits.

## Check status

Show the latest run for the current repository:

```sh
localci status
```

Show one task:

```sh
localci status --task noisy-fail
```

Show a specific run:

```sh
localci status --commit 6bd8d389f3f550da3fccb11bac380d59268fe320
```

Non-interactive commands print bounded summaries. They do not dump full logs.

## Find artifact paths

Print artifact paths for the latest run:

```sh
localci artifacts
```

Print the primary artifact for failed tasks:

```sh
localci artifacts --failed --primary
```

For shell scripts and agents, print only paths:

```sh
localci artifacts --task noisy-fail --primary --paths-only
```

Then read the file directly:

```sh
cat "$(localci artifacts --task noisy-fail --primary --paths-only)"
```

## Browse history

Show recent runs:

```sh
localci history
```

Show recent results for a task across commits:

```sh
localci history --task noisy-fail
```

Show recent failed runs:

```sh
localci history --failed
```

## Open the web UI

Open the latest run for the current repository:

```sh
localci web
```

Open a specific task:

```sh
localci web --task noisy-fail
```

Task artifact pages show full artifact paths and include copy/open actions.

## Use the terminal UI

Open the all-repos dashboard:

```sh
localci dash
```

Open a specific task:

```sh
localci dash --task noisy-fail
```

The terminal UI supports the same run, task, retry, cancel, and artifact views as the web UI. On artifact views, use `e` to edit the selected artifact with `$VISUAL` or `$EDITOR`, `o` to open it with the platform opener, and `f` on macOS to reveal it in Finder through the daemon.

## Cancel work

Cancel the active task:

```sh
localci cancel
```

Cancel a specific task:

```sh
localci cancel --commit HEAD --task test
```
