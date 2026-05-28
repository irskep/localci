# LocalCI

[![CI](https://github.com/irskep/localci/actions/workflows/ci.yml/badge.svg)](https://github.com/irskep/localci/actions/workflows/ci.yml)

LocalCI is an asynchronous post-commit validation runner for local development. It runs the checks you already define as [mise](https://mise.jdx.dev/) tasks, stores logs on disk, and gives you results through one-shot CLI commands, a terminal UI, and a local web UI.

It is meant to sit between Git hooks and remote CI: local like a hook, but async like CI.

## What It Does

- Runs after each commit, so the author or agent does not have to block on the full suite before moving on.
- Executes checks in isolated clones by default, so results are tied to a committed tree instead of whatever happens to be checked out.
- Discovers `localci:` mise tasks, including monorepo tasks such as `//web:localci:test`.
- Keeps logs and artifacts as files, with commands for finding paths when a human or agent needs to inspect them.

LocalCI is not a GitHub Actions clone. There is no workflow DSL; mise is the contract.

## Quick Start

Install LocalCI with mise, then start the daemon and install the post-commit hook:

```toml
[tools]
"github:irskep/localci" = "0.1.3"
```

```sh
mise install

localci start
localci install-hooks
```

Define checks as mise tasks in the `localci:` namespace. For example, `mise-tasks/localci/test`:

```sh
#!/bin/sh
set -eu

go test ./...
```

After a commit, inspect the latest run:

```sh
localci wait
localci status
localci web
localci dash
```

For an ad hoc working-tree check, use `invoke` explicitly:

```sh
localci invoke --no-clone --task test --wait
```

## Artifacts

LocalCI never needs to dump a huge log into your terminal. Ask for paths instead:

```sh
localci artifacts --failed --primary
localci artifacts --task noisy-fail --primary --paths-only
```

Then open, edit, copy, or attach those files using your normal tools.

## Documentation

The docs cover setup, task discovery, CLI usage, and the web/TUI inspection flow:

<https://steveasleep.com/localci/>
