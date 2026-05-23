# Setup

## Add LocalCI tasks

LocalCI discovers mise tasks whose names end in the `localci:` namespace. A root task can be written as `localci:test`; a monorepo task can be written as `//web:localci:test`.

For shell-heavy tasks, prefer script tasks under `mise-tasks/localci/`:

```sh
mise-tasks/localci/test
mise-tasks/localci/build
mise-tasks/localci/fmt
```

Each script should be executable and should print the working directory at startup:

```sh
#!/bin/sh
set -eu

printf 'cwd: %s\n' "$(pwd)"

go test ./...
```

## Add setup

`localci:setup` is special. If it exists, LocalCI runs it before the other LocalCI tasks. Use it for repository-wide dependency setup rather than duplicating install steps in every task.

For a mise-based project with a web workspace, setup commonly looks like:

```toml
[tasks."localci:setup"]
description = "Install dependencies for cloned runs"
run = [
  "printf 'cwd: %s\\n' \"$(pwd)\"",
  "mise trust",
  "mise install",
  "pnpm --dir web install --frozen-lockfile",
]
```

Keep setup deterministic. It should install pinned tools and locked dependencies, not make broad changes to the checkout.

## Install the post-commit hook

From the repository root:

```sh
localci install-hooks
```

The hook calls:

```sh
localci postcommit "$repo" "$commit"
```

After each commit, the postcommit command prints commands for checking status and waiting for results.

## Start the daemon

Run the daemon before relying on the hook:

```sh
localci start
```

Use `localci restart` after upgrading LocalCI or changing daemon-served assets.
