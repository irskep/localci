# Overview

LocalCI is an asynchronous post-commit validation runner for local development. It fills the gap between Git hooks and remote CI: checks run locally, but they run after the commit so the person or coding agent making the change does not have to block on the full suite before moving on.

LocalCI is deliberately small. It discovers mise tasks, queues them in a daemon, runs them against an isolated clone, and makes the results available from the CLI and a local web UI.

## What LocalCI is for

<div class="grid cards" markdown>

-   :lucide-check:{ .lg .middle } **Run all checks even if you forget**

    You or your agent might forget to keep one of your test suites green. LocalCI makes sure everything runs all the time.

-   :lucide-check:{ .lg .middle } **Browse results like GitHub Actions (but better)**

    Results, logs, and history are all available via one-shot commands, a web UI, and a terminal UI.

</div>

## What LocalCI is not for

<div class="grid cards" markdown>

-   :lucide-x:{ .lg .middle } **GitHub Actions compatibility**

    LocalCI remains simple by sticking to running Mise tasks.

-   :lucide-x:{ .lg .middle } **Projects without Mise**

    If a project cannot express its checks as mise tasks, it is not a good fit.

</div>

## How it works

The normal path is:

1. The daemon is running.
2. A Git post-commit hook calls `localci postcommit --repo <repo> --commit <commit>`.
3. The LocalCI daemon clones your project to a unique place and checks out the commit.
4. The daemon discovers every mise task whose task name starts with `localci:`, including monorepo tasks addressed with mise's `//path:task` syntax, and puts them in a queue.
5. One by one, the daemon runs each task.
6. Results can be inspected with `localci wait`, `localci status`, `localci web`, or `localci dash`.

By default, LocalCI runs everything in isolated clones. Use `localci run --no-clone --wait` when you intentionally want a daemon-managed run against your working tree.
