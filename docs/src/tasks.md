# Task Reference

## Discovery

LocalCI discovers tasks by running:

```sh
mise tasks --json --all
```

Any discovered task containing the `localci:` namespace is eligible to run. This includes root tasks like `localci:test` and monorepo tasks like `//web:localci:test`.

`localci:setup` is reserved for setup and is not treated as an ordinary validation task.

## Execution order

LocalCI runs `localci:setup` first when it exists. Validation tasks run after setup.

Each task runs through mise:

```sh
mise run <task-name>
```

## Environment

Every task receives these environment variables:

| Variable | Meaning |
| --- | --- |
| `LOCALCI_TASK_OUTPUT_DIR` | Directory for artifacts from this task attempt. |
| `LOCALCI_TASK_CACHE_DIR` | Cache directory shared by attempts of the same task. |
| `LOCALCI_CACHE_DIR` | Cache directory shared across all tasks for the repo. |

Write logs, build outputs, reports, and other inspectable artifacts under `LOCALCI_TASK_OUTPUT_DIR`.

## Logs and artifacts

LocalCI captures task output into `combined.log`. Files written under `LOCALCI_TASK_OUTPUT_DIR` are exposed as task artifacts in the web UI.

Prefer stable, descriptive artifact names:

```sh
mkdir -p "$LOCALCI_TASK_OUTPUT_DIR"
go test ./... >"$LOCALCI_TASK_OUTPUT_DIR/test.log" 2>&1
```

## Clones

Committed runs execute in independent clones under LocalCI's data directory. This keeps asynchronous runs isolated from later edits in the live checkout.

No-clone runs are explicit exceptions. They execute in the live working directory and are labeled with a trailing `*`.
