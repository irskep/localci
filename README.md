localci is a minimalistic postcommit validation check runner. You're supposed to call it from a postcommit hook. From there, it waits in a queue, runs all configured checks with output going to files, and make the results available in a web browser or over a cli. Logs can be tailed from anywhere on the system, or via the cli, or tailed live in the browser.

Workflows are defined as mise tasks under the 'localci' prefix. localci runs 'mise tasks' to see them all.

# Configuration

Postcommit hook:
1. define git global hooks (latest version of git has a git hooks feature; we DO NOT need a separate git hook manager. local git might be out of date; localci will make sure you have a recent version.)
2. /path/to/localci postcommit --repo <path-to-repo> <commit>

## How localci discovers "jobs"

Tasks are mise tasks prefixed with "localci:". That means you can put them under mise-tasks/localci/ as shell scripts if you want.

## Where to put output

Each task is called with env vars set:

- `LOCALCI_TASK_OUTPUT_DIR`: put task output files here. name them well! send stderr to stdout as a habit.
- `LOCALCI_TASK_CACHE_DIR`: task-local cache shared across multiple invocations of this task
- `LOCALCI_CACHE_DIR`: cache shared across all tasks and all invocations

There is no cache busting built in at the moment.

There is no special "workflow configuration," only files, so place and name your files well.
