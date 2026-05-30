<p align="center">
  <img src="logo.svg" alt="LocalCI logo" width="96">
</p>

# LocalCI

[![CI](https://github.com/irskep/localci/actions/workflows/ci.yml/badge.svg)](https://github.com/irskep/localci/actions/workflows/ci.yml)

LocalCI is an asynchronous post-commit validation runner for local development.
It runs the checks you already define as [Mise](https://mise.jdx.dev/)
tasks, stores logs and artifacts on disk, and makes results available through
one-shot CLI commands, a terminal UI, and a local web UI.

It is meant to sit between Git hooks and remote CI: local like a hook, but async like CI.

## What It Does

- Runs after each commit, so you or your coding agent can keep moving while the
  full suite runs in the background.
- Executes checks in isolated clones by default, so results are tied to a
  committed tree instead of whichever files happen to be checked out.
- Discovers every Mise task whose name starts with `localci:`, including
  monorepo tasks such as `//web:localci:test`.
- Runs a root `localci:setup` task first when one exists.
- Keeps logs and artifacts as files, with commands for finding the exact paths
  when a human or agent needs to inspect them.

LocalCI is not a GitHub Actions clone. There is no workflow DSL; mise is the contract.

## When To Use It

Use LocalCI when a repository can express its checks as Mise tasks and you want
continuous local feedback after commits. It is especially useful for agents and
larger repos where running every check before every commit would interrupt the
development loop.

LocalCI is not a replacement for remote CI, and it is not a fit for projects
that cannot define their validation commands as Mise tasks.

## Quick Start

Install LocalCI with mise, then start the daemon and install the post-commit hook:

```toml
[tools]
"github:irskep/localci" = "0.2.2"
```

```sh
mise install

localci start
localci install-hooks
```

Define checks as Mise tasks in the `localci:` namespace. A typical root
configuration uses `localci:setup` for dependencies and `localci:test` for the
actual validation:

```toml
[tasks."localci:setup"]
description = "Install dependencies for LocalCI runs"
run = "mise install"

[tasks."localci:test"]
description = "Run tests"
run = "go test ./..."
```

File tasks work too. For example, `mise-tasks/localci/test`:

```sh
#!/bin/sh
set -eu

go test ./...
```

Commit the task definitions so the post-commit hook can enqueue checks for that
revision:

```sh
git add mise.toml
git commit -m "Configure LocalCI"
```

## Daily Workflow

After each commit, LocalCI queues the eligible tasks. Inspect the latest run
from the terminal:

```sh
localci wait
localci status
```

Or open the same information in a UI:

```sh
localci web
localci dash
```

For an ad hoc daemon-managed run, use `localci run`:

```sh
localci run --task test --wait
```

When you intentionally want to test uncommitted changes through the daemon
queue, use `--no-clone`:

```sh
localci run --no-clone --task test --wait
```

No-clone runs are marked with a trailing `*` because they reflect the live
working tree instead of a committed revision.

## Artifacts

LocalCI never needs to dump a huge log into your terminal. Ask for paths instead:

```sh
localci artifacts --failed --primary
localci artifacts --task noisy-fail --primary --paths-only
```

Then open, edit, copy, or attach those files using your normal tools.

Tasks can write additional reports under `LOCALCI_TASK_OUTPUT_DIR`. LocalCI also
captures stdout and stderr into `combined.log` for every task attempt.

## Common Commands

```sh
localci start          # start the daemon
localci stop           # stop the daemon
localci restart        # restart the daemon
localci install-hooks  # install the Git post-commit hook
localci wait           # wait for the selected run or task
localci status         # print a bounded status summary
localci history        # list recent runs
localci artifacts      # print artifact paths
localci web            # open the web UI
localci dash           # open the terminal UI
localci cancel         # cancel queued or active work
```

## Documentation

The full docs cover setup, task discovery, monorepo tasks, artifact handling,
CLI usage, and the web/TUI inspection flow:

<https://steveasleep.com/localci/>
