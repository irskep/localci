# Getting Started

This guide gets a repository to the point where LocalCI runs checks after each commit.

## Install LocalCI

LocalCI requires mise. Install mise first, then add LocalCI as a pinned GitHub release tool:

```toml title="mise.toml"
[tools]
"github:irskep/localci" = "VERSION"
```

Use an exact released version rather than a floating version.

## Define setup

Add a `localci:setup` task if the repository needs dependencies installed before checks run:

```toml title="mise.toml"
[tasks."localci:setup"]
description = "Install dependencies for LocalCI runs"
run = [
  "printf 'cwd: %s\\n' \"$(pwd)\"",
  "mise trust",
  "mise install",
]
```

Setup runs before validation tasks. Keep it deterministic and pinned.

## Define checks

LocalCI discovers mise tasks in the `localci:` namespace. For simple shell checks, use script tasks:

```sh title="Terminal"
mkdir -p mise-tasks/localci
```

```sh title="mise-tasks/localci/test"
#!/bin/sh
set -eu

printf 'cwd: %s\n' "$(pwd)"

go test ./...
```

Save that as `mise-tasks/localci/test` and make it executable.

Monorepo tasks work too. A task named `//web:localci:test` is discovered the same way as `localci:test`.

## Start the daemon

```sh title="Terminal"
localci start
```

## Install the hook

From the repository root:

```sh title="Terminal"
localci install-hooks
```

After each commit, the hook enqueues checks and prints commands for status and waiting.

## Check results

Wait in the terminal:

```sh title="Terminal"
localci wait
```

Or open the web UI:

```sh title="Terminal"
localci web
```
