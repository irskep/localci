# LocalCI documentation plan

This plan uses hk's documentation as the quality bar, not as a structure to copy
verbatim. hk's docs work because they separate the reader's jobs clearly: a
landing page explains the value, getting started gets a project to first use,
configuration/reference pages answer exact questions, and deeper explanation
pages justify the design.

## What hk does well

hk's public docs have a clear information architecture:

- A landing page with a one-line product description, direct calls to action,
  quick install snippet, and compact feature cards.
- Top-level navigation for the first three high-intent destinations: Getting
  Started, Configuration, and CLI Reference.
- A sidebar that separates introductory/explanatory pages from a large reference
  section.
- A Getting Started page that is operationally complete: install, install hooks,
  alternative hook setup, project setup, an example config, and how to run the
  tool manually.
- A Configuration page that starts with precedence and config discovery before
  listing individual options with defaults, examples, and notes.
- Reference pages for built-ins, examples, glossary, environment variables,
  hooks, logging/debugging, integration with mise, and generated CLI command
  pages.
- Explanation pages that do real work, especially Why hk?, which explains the
  design constraints behind file locking and smart stashing instead of merely
  repeating marketing claims.

The important lesson is density with orientation. hk does not make every page a
tutorial, but most pages answer one reader question completely.

## LocalCI target structure

LocalCI should keep the docs smaller than hk because the product is smaller, but
the same content classes apply.

Recommended public navigation:

```text
Overview
Getting Started
How-to Guides
Reference
Explanation
Troubleshooting
```

Concrete page set:

```text
index.md
getting-started.md
how-to/
  define-tasks.md
  install-hooks.md
  run-checks-manually.md
  inspect-results.md
  run-without-clones.md
reference/
  cli.md
  tasks.md
  environment.md
  storage.md
  web-ui.md
explanation/
  why-localci.md
  execution-model.md
  clones-and-no-clone.md
troubleshooting.md
```

Do not add tutorials yet. The MVP audience is someone willing to read a compact
guide or reference page and apply it to their own repo.

## Page responsibilities

### Overview

Keep the landing page short and product-shaped:

- What LocalCI is.
- What it is for and not for.
- The core execution model.
- Two primary next links: Getting Started and CLI Reference.

The current overview is close. It should eventually grow a compact "Why not
GitHub Actions/post-commit hooks?" section, but only if it stays explanatory
rather than cute.

### Getting Started

This should replace the current split between Installation and Setup for the
first-use path. It should include:

- Install LocalCI with mise using `github:irskep/localci`.
- Start the daemon.
- Add one `localci:setup` task.
- Add one validation task.
- Install hooks.
- Commit.
- Run `localci wait`.
- Open `localci web`.

This page can be a guide, not a tutorial: give commands and expected outcomes,
but avoid fake sample projects.

### How-to guides

Each how-to should answer one operational question:

- Define tasks: root tasks, monorepo tasks, script tasks, naming rules.
- Install hooks: Git config hook behavior, per-repo setup, future global setup if
  LocalCI supports it.
- Run checks manually: `invoke`, `invoke --wait`, annotations, commit selection.
- Inspect results: `status`, `wait`, `web`, task pages, artifacts, logs.
- Run without clones: when to use `--no-clone`, how labels work, what risks it
  has.

These pages should be command-forward and short.

### Reference

Reference should be exhaustive and boring.

Needed pages:

- CLI: generated or mechanically maintained from `localci --help`; every command
  should list usage, arguments, flags, behavior, and examples.
- Tasks: discovery, setup task semantics, task ordering, retry/cancel behavior,
  artifact rules, and examples.
- Environment: every `LOCALCI_*` variable, when it exists, what paths are stable,
  and what can be written.
- Storage: `.localci` layout, clones, output directories, cache directories,
  cleanup expectations.
- Web UI: route model, live updates, daemon connection states, artifacts, retry,
  cancel.

The CLI reference should eventually be generated from the code or at least
checked against tests. Hand-maintained command docs will drift.

### Explanation

Explanation should cover why the system behaves the way it does:

- Why LocalCI exists: local async checks versus remote CI and synchronous hooks.
- Execution model: daemon, queue, setup, task discovery, task execution, result
  storage.
- Clones and no-clone: why clones are the default, why no-clone exists, and how
  commit labels communicate the distinction.

This is where correctness-sensitive design decisions belong.

### Troubleshooting

hk has a useful logging/debugging page. LocalCI needs the equivalent:

- Daemon is not running.
- Hook fired but no tasks appeared.
- A task works in shell but fails in LocalCI.
- A clone run does not see local changes.
- `--no-clone` sees too much local state.
- mise trust/install failures.
- pnpm/node/tool cache problems.
- Web UI disconnected states.
- Where to find raw logs and artifacts on disk.

This page should prefer symptoms and direct commands over prose.

## Content we should write next

Priority order:

1. Replace Installation + Setup with a single Getting Started page.
2. Expand CLI Reference to cover every command and flag accurately.
3. Add Task Reference and Environment Reference as separate pages.
4. Add Execution Model and Clones vs No-clone explanation pages.
5. Add Troubleshooting.
6. Add Web UI reference after the UI stabilizes.

## Quality rules

- Every command in docs should be runnable as written from an obvious directory.
- Every page should start by saying what question it answers.
- Reference pages should include defaults, side effects, output locations, and
  failure modes.
- Avoid vague categories like "workflow DSL"; name the concrete thing, such as
  GitHub Actions YAML.
- Keep LocalCI's docs honest about scope: it is not remote CI, not a scheduler
  platform, and not a generic workflow engine.
- Prefer mise-native examples because LocalCI requires mise.
- Do not document planned behavior as current behavior. Mark future work
  explicitly or leave it out.

## Current gaps against hk's bar

- There is no single first-use path comparable to hk's Getting Started page.
- CLI docs are not generated and do not yet include enough behavior detail.
- Task environment variables are documented, but not with enough precision about
  lifecycle and stability.
- The clone/no-clone model is central but only lightly explained.
- There is no troubleshooting/logging page.
- There is no storage/layout reference for `.localci`.
- There is no documentation of daemon lifecycle or web UI states.

Filling those gaps would bring LocalCI's docs close to hk's level while keeping
the total page count appropriate for a smaller project.
