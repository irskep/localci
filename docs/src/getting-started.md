# Getting Started

This guide gets a repository to the point where LocalCI runs checks after each commit.

## Set up a repository

### Install LocalCI

LocalCI requires [Mise](https://mise.en.dev). Install Mise first, then add LocalCI as a pinned GitHub release tool:

```toml title="mise.toml"
[tools]
"github:irskep/localci" = "0.2.2"
```

```sh
mise install
```

### Start the daemon

```sh
localci start
```

### Read docs in the terminal

```sh
localci docs
```

Use `localci docs --plain` when you want plain text for scripts, pagers, or agents.

### Install the hook

From the repository root:

```sh
localci install-hooks
```

After each commit, the hook enqueues checks and prints commands for status and waiting.

### Define tasks

LocalCI executes all tasks with `localci:` in their name, and always executes `localci:setup` first.


```toml title="mise.toml"
[tasks."localci:setup"]
description = "Install dependencies for LocalCI runs"
run = [
  # 'mise trust' is run automatically, so you can usually just install dependencies immediately
  "mise install",
]

[tasks."localci:test"]
description = "go test"
run = "go test ./..."
```

### Commit the setup

Commit the LocalCI task definitions so the post-commit hook can enqueue checks for the new revision:

```sh
git add mise.toml
git commit -m "Configure LocalCI"
```

## Run checks

### Follow the latest run

The normal workflow is post-commit: the hook queues work after each commit, and you inspect the daemon's latest run.

Wait in the terminal:

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

To inspect an existing no-clone run:

```sh
localci wait --commit 'HEAD*'
```

### Run an ad hoc daemon check

Use `run` when you explicitly want to queue a manual daemon-managed run outside the post-commit path.

```sh
localci run --task test --wait
```

To test uncommitted changes through the daemon queue, use `--no-clone`:

```sh
localci run --no-clone --task test --wait
```

No-clone runs are labeled with a trailing `*`. They intentionally see unstaged files and local edits.

Use `invoke` only when you want to run checks directly in the current terminal instead of through the daemon.

## Inspect results

### Check status

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

### Find artifact paths

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

### Browse history

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

## Open UIs

### Open the web UI

Open the latest run for the current repository:

```sh
localci web
```

Open a specific task:

```sh
localci web --task noisy-fail
```

Task artifact pages show full artifact paths and include copy/open actions.

### Use the terminal UI

Open the all-repos dashboard:

```sh
localci dash
```

Open a specific task:

```sh
localci dash --task noisy-fail
```

The terminal UI supports the same run, task, retry, cancel, and artifact views as the web UI. On artifact views, use `e` to edit the selected artifact with `$VISUAL` or `$EDITOR`, `o` to open it with the platform opener, and `f` on macOS to reveal it in Finder through the daemon.

## Manage queued work

### Cancel work

Cancel the active task:

```sh
localci cancel
```

Cancel a specific task:

```sh
localci cancel --commit HEAD --task test
```
