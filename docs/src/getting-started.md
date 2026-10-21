# Getting Started

This guide gets a repository to the point where LocalCI runs checks after each commit.

## Set up a repository

### Install LocalCI

LocalCI runs [Mise](https://mise.en.dev) tasks, so make sure Mise is installed before you configure a repository.

#### With brew

```sh
brew tap irskep/tap
brew install localci
```

#### With mise

If you want everyone working in a repository to use the same LocalCI toolchain, install LocalCI through mise's GitHub backend:

```toml title="mise.toml"
[tools]
"github:irskep/localci" = "0.3.0"
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

Print the primary artifact for a task directly to stdout:

```sh
localci cat --task noisy-fail
```

Print a named artifact:

```sh
localci cat report.txt --task noisy-fail
```

Use raw mode for binary artifacts:

```sh
localci cat dist/app.tar.gz --task build --raw
```

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

## Large logs and task history

The web UI previews the first 2 MiB of a task log. For the complete output, use the
log's download action or `localci cat --raw`. Text artifact previews also have a 2 MiB
limit. Task history in the web UI shows 50 runs per page; use **Older** and
**Newer** to browse. The terminal dashboard shows the latest 50 matching runs;
`localci history` can retrieve older runs.

## Prune logs and artifacts

LocalCI keeps outputs until you remove them. Preview pruning across all repositories:

```sh
localci prune --dry-run
```

Run `localci prune` to see the removal summary and confirm with `y`. The default
limits are 30 days and 10 GiB: remove outputs from runs last active more than
30 days ago, then remove the oldest remaining run outputs until their combined
size is at most 10 GiB. All attempts of a selected run are included.

Override the limits for one invocation, or pass `--yes` to skip confirmation:

```sh
localci prune --max-age 7d --max-size 5GiB --yes
```

Set either limit to `0` to disable it. No settings are saved, and pruning never
runs automatically.

To remove individual large files regardless of age, use `--larger-than` instead
of the age and total-size limits:

```sh
localci prune --larger-than 100MiB
```

This lists and removes files strictly larger than 100 MiB, leaving smaller files
in the same run. Sizes accept bytes, decimal `KB`/`MB`/`GB`, or binary
`KiB`/`MiB`/`GiB`. For example, `1GB` is 1,000,000,000 bytes and `1GiB` is
1,073,741,824 bytes. Ages accept whole days (`30d`) or durations such as `24h`.

Active and queued runs are protected, even if that prevents reaching the size
limit. Runs or files that change after the preview are skipped. Task results,
exit codes, failure messages, and attempt history remain; logs and artifact
contents are deleted. Task and shared caches are untouched. Symlink targets are
never followed or deleted.

Text output from `localci status` suggests pruning when history contains completed
runs older than 30 days. This is an age-based hint, not a disk-space measurement;
only `prune` scans output files to calculate their sizes.
