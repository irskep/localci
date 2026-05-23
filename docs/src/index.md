# LocalCI

LocalCI is an asynchronous post-commit validation runner for local development. It fills the gap between Git hooks and remote CI: checks run locally, but they run after the commit so the person or coding agent making the change does not have to block on the full suite before moving on.

LocalCI is deliberately small. It discovers mise tasks, queues them in a daemon, runs them against an isolated checkout by default, and makes the results available from the CLI and a local web UI.

## What LocalCI is for

- Running a comprehensive local validation suite after every commit.
- Keeping coding agents honest by making the expected checks automatic.
- Viewing task logs without rerunning commands manually.
- Trying a run against the current working tree with `--no-clone` when fast feedback matters.

## What LocalCI is not for

- Replacing remote CI for release gates, protected branches, or cross-platform checks.
- Managing arbitrary workflow syntax. LocalCI intentionally uses mise tasks as the workflow surface.
- Running without mise. If a project cannot express its checks as mise tasks, it is not a good fit yet.

## Model

The normal path is:

1. The daemon is running.
2. A Git post-commit hook calls `localci postcommit <repo> <commit>`.
3. LocalCI discovers every mise task named `localci:*`, including monorepo tasks such as `//web:localci:test`.
4. The daemon runs `localci:setup`, when present, before other tasks.
5. Each task receives output and cache directories through environment variables.
6. Results can be inspected with `localci wait`, `localci status`, or `localci web`.

By default, committed runs execute in an independent clone under LocalCI's data directory. `--no-clone` commands are available for explicitly running against the live working tree.
