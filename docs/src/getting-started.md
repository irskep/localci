# Getting Started

This guide gets a repository to the point where LocalCI runs checks after each commit.

## Install LocalCI

LocalCI requires [Mise](https://mise.en.dev). Install Mise first, then add LocalCI as a pinned GitHub release tool:

```toml title="mise.toml"
[tools]
"github:irskep/localci" = "0.1.4"
```

```sh
mise install
```

## Define setup

Add a `localci:setup` task if the repository needs dependencies installed before checks run:

```toml title="mise.toml"
[tasks."localci:setup"]
description = "Install dependencies for LocalCI runs"
run = [
  # 'mise trust' is run automatically, so you can usually just install dependencies immediately
  "mise install",
]
```

The root `localci:setup` task is always run before all other tasks. It runs once, not before every task.

## Define checks

LocalCI discovers mise tasks in the `localci:` namespace. [File tasks](https://mise.en.dev/tasks/file-tasks.html) are usually a good fit:

```sh
mkdir -p mise-tasks/localci
```

```sh title="mise-tasks/localci/test"
go test ./...
```

Save that as `mise-tasks/localci/test` and make it executable.

[Monorepo tasks](https://mise.en.dev/tasks/monorepo.html) work too. Mise addresses child tasks as `//path:task`; LocalCI treats the task portion after the path as eligible when it starts with `localci:`.

See [Defining Tasks](defining-tasks.md) for more examples.

## Start the daemon

```sh
localci start
```

## Install the hook

From the repository root:

```sh
localci install-hooks
```

After each commit, the hook enqueues checks and prints commands for status and waiting.

## Check results

Wait in the terminal:

```sh
localci wait
```

Or open the web UI:

```sh
localci web
```

Or open the TUI:

```sh
localci dash
```
