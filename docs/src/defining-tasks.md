# Defining Tasks

LocalCI runs checks that are defined as Mise tasks. A task is eligible when its task name starts with `localci:`.

LocalCI discovers tasks with:

```sh
mise tasks --json --all
```

It then runs each matching task with:

```sh
mise run <task-name>
```

## TOML tasks

Use TOML tasks when a check is short and belongs near the rest of your Mise configuration.

```toml title="mise.toml"
[tasks."localci:test"]
description = "Run tests"
run = "go test ./..."
```

```toml title="mise.toml"
[tasks."localci:fmt"]
description = "Format Go files"
run = "gofmt -w ./cmd ./internal"
```

Mise documents this format in [TOML tasks](https://mise.en.dev/tasks/toml-tasks.html).

## File tasks

Use file tasks when a check is easier to maintain as a script.

```sh
mkdir -p mise-tasks/localci
```

```sh title="mise-tasks/localci/test"
go test ./...
```

```sh
chmod +x mise-tasks/localci/test
```

Mise documents this format in [file tasks](https://mise.en.dev/tasks/file-tasks.html).

## Setup tasks

Use a root `localci:setup` task for dependency installation shared by all checks.

```toml title="mise.toml"
[tasks."localci:setup"]
description = "Install dependencies for LocalCI runs"
run = "mise install"
```

LocalCI runs `localci:setup` first when it exists. It is setup for the run, not an ordinary validation check.

## Monorepos

Mise can expose tasks from child config roots. For example, a task named `localci:test` in a child package can appear as a path-addressed Mise task.

LocalCI follows Mise's [monorepo task syntax](https://mise.jdx.dev/tasks/monorepo.html) and checks the task portion after the path prefix for the `localci:` namespace.
