# LocalCI

LocalCI is an asynchronous post-commit validation runner for local development. It fills the gap between Git hooks and remote CI: checks run locally, but they run after the commit so the person or coding agent making the change does not have to block on the full suite before moving on.

LocalCI is deliberately small. It discovers mise tasks, queues them in a daemon, runs them against an isolated checkout by default, and makes the results available from the CLI and a local web UI.

## What LocalCI is for

<div class="grid cards" markdown>

- :lucide-check:{ .lg .middle } __Post-commit validation__
- :lucide-check:{ .lg .middle } __Agent accountability__
- :lucide-check:{ .lg .middle } __Local task logs__
- :lucide-check:{ .lg .middle } __Working-tree runs__

</div>

## What LocalCI is not for

<div class="grid cards" markdown>

- :lucide-x:{ .lg .middle } __Release gates__
- :lucide-x:{ .lg .middle } __Workflow DSLs__
- :lucide-x:{ .lg .middle } __Non-mise projects__

</div>

## Model

The normal path is:

1. The daemon is running.
2. A Git post-commit hook calls `localci postcommit <repo> <commit>`.
3. LocalCI discovers every mise task named `localci:*`, including monorepo tasks such as `//web:localci:test`.
4. The daemon runs `localci:setup`, when present, before other tasks.
5. Each task receives output and cache directories through environment variables.
6. Results can be inspected with `localci wait`, `localci status`, or `localci web`.

By default, committed runs execute in an independent clone under LocalCI's data directory. `--no-clone` commands are available for explicitly running against the live working tree.
