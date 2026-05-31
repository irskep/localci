# Tasks

LocalCI runs checks that are defined as Mise tasks. A task is eligible when its task name starts with `localci:`.

LocalCI discovers tasks with:

```sh
mise tasks --json --all
```

It then runs each matching task with:

```sh
mise run <task-name>
```

## Defining tasks

### TOML tasks

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

#### Nested `mise.toml` files

TOML tasks also work from nested Mise config roots. Tell Mise which child directories are config roots from the repo-level `mise.toml`:

```toml title="mise.toml"
experimental_monorepo_root = true

[monorepo]
config_roots = ["web", "docs"]
```

Then define LocalCI tasks inside those child configs:

```toml title="web/mise.toml"
[tasks."localci:test"]
description = "Run Vue unit tests"
run = "pnpm exec vitest --run"
```

```toml title="docs/mise.toml"
[tasks."localci:build"]
description = "Build documentation"
run = "zensical build --strict"
```

LocalCI uses Mise's task list, so nested tasks keep their path prefix. In the UI, LocalCI removes the `localci:` namespace from task labels: a root task like `localci:test` shows as `test`, and a nested task like `//web:localci:test` shows as `//web:test`.

Mise documents the path syntax in [monorepo tasks](https://mise.jdx.dev/tasks/monorepo.html).

### File tasks

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

#### File tasks in monorepos

File tasks work the same way inside nested config roots. For example, this file:

```sh title="web/mise-tasks/localci/test"
pnpm exec vitest --run
```

is exposed by Mise as `//web:localci:test`, and LocalCI shows it in the UI as `//web:test`.

### Setup tasks

Use a root `localci:setup` task for dependency installation shared by all checks.

```toml title="mise.toml"
[tasks."localci:setup"]
description = "Install dependencies for LocalCI runs"
run = "mise install"
```

LocalCI runs `localci:setup` first when it exists. It is setup for the run, not an ordinary validation check.

Only the root setup task is used. LocalCI ignores setup tasks from nested config roots, such as `//web:localci:setup`. If you need package-specific setup, use [task dependencies](https://mise.en.dev/tasks/architecture.html#task-dependency-system).

## Task execution environment

Tasks run either in shallow clones of a specific commit, or on your working directory when you pass `--no-clone`. Runs with `--no-clone` are marked with an asterisk `*` in the UI because their state won’t match what’s in git.

### Environment variables

Every task receives these environment variables:

| Variable | Meaning |
| --- | --- |
| `LOCALCI_TASK_OUTPUT_DIR` | Directory for artifacts from this task attempt. |
| `LOCALCI_TASK_ARTIFACTS_FILE` | JSON manifest path for named artifact actions from this task attempt. |
| `LOCALCI_TASK_CACHE_DIR` | Cache directory for this task. |
| `LOCALCI_CACHE_DIR` | Cache directory shared by LocalCI tasks. |

Write reports and inspectable artifacts under `LOCALCI_TASK_OUTPUT_DIR`. LocalCI exposes those files in the web and terminal UIs.

### Task output

LocalCI captures each task's stdout and stderr into `combined.log`. For ordinary checks, let the task print normally:

```sh title="mise-tasks/localci/test"
go test ./...
```

For tasks with multiple outputs, write them under `LOCALCI_TASK_OUTPUT_DIR`:

```sh title="mise-tasks/localci/test-with-coverage"
mkdir -p "$LOCALCI_TASK_OUTPUT_DIR/coverage"
pytest --cov --cov-report=html:"$LOCALCI_TASK_OUTPUT_DIR/coverage"
```

If a task has an output people should open from higher-level views, write a manifest to `LOCALCI_TASK_ARTIFACTS_FILE`. Manifest paths are relative to `LOCALCI_TASK_OUTPUT_DIR`, and marked artifacts sort before ordinary artifacts:

```sh title="mise-tasks/localci/docs"
mkdir -p "$LOCALCI_TASK_OUTPUT_DIR/site"
cp -R site/. "$LOCALCI_TASK_OUTPUT_DIR/site/"
cat >"$LOCALCI_TASK_ARTIFACTS_FILE" <<'JSON'
{
  "version": 1,
  "artifacts": [
    {"name": "docs html", "path": "site/index.html", "action": "open"}
  ]
}
JSON
```

Valid actions are `open`, `download`, `reveal`, and `view`.

Here‘s how you might configure a browser testing tool like Playwright to put its filesystem output in the right place:

```ts title="playwright.config.ts"
import { defineConfig } from '@playwright/test'

export default defineConfig({
  outputDir: process.env.LOCALCI_TASK_OUTPUT_DIR
    ? `${process.env.LOCALCI_TASK_OUTPUT_DIR}/playwright-artifacts`
    : 'test-results',
  reporter: [
    ['list'],
    [
      'html',
      {
        outputFolder: process.env.LOCALCI_TASK_OUTPUT_DIR
          ? `${process.env.LOCALCI_TASK_OUTPUT_DIR}/playwright-report`
          : 'playwright-report',
        open: 'never',
      },
    ],
  ],
})
```

```sh title="mise-tasks/localci/playwright"
pnpm exec playwright test
```
