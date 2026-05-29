# Getting Started

This guide gets a repository to the point where LocalCI runs checks after each commit.

## Install LocalCI

LocalCI requires [Mise](https://mise.en.dev). Install Mise first, then add LocalCI as a pinned GitHub release tool:

```toml title="mise.toml"
[tools]
"github:irskep/localci" = "0.1.5"
```

```sh
mise install
```

## Define tasks

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
