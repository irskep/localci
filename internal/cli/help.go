package cli

import (
	"errors"
	"fmt"
	"strings"
)

func (a App) printUsage() {
	fmt.Fprint(a.Stdout, usageText())
}

func (a App) printCommandHelp(command string) error {
	text, ok := commandHelpText(command)
	if !ok {
		return fmt.Errorf("unknown command %q\n\n%s", command, usageText())
	}
	fmt.Fprint(a.Stdout, text)
	return nil
}

func usageText() string {
	return `localci is a local post-commit validation runner.

Usage:
  localci start
  localci restart
  localci stop
  localci postcommit [--repo DIR] [--commit REF] [--task TASK] [--annotation KEY=VALUE] [--json]
  localci run [--repo DIR] [--commit REF] [--task TASK] [--wait] [--no-clone] [--annotation KEY=VALUE] [--json]
  localci status [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--no-clone] [--json]
  localci wait [--repo DIR] [--commit REF] [--task TASK] [--no-clone] [--json]
  localci history [--repo DIR] [--commit REF] [--task TASK] [--status STATUS] [--failed] [--limit N] [--json]
  localci artifacts [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--failed] [--primary] [--paths-only] [--json]
  localci web [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--artifact ARTIFACT] [--no-clone]
  localci dash [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--artifact ARTIFACT] [--no-clone]
  localci cancel [--repo DIR] [--commit REF] [--task TASK] [--no-clone] [--json]
  localci invoke [--repo DIR] [--commit REF] [--task TASK] [--wait] [--no-clone] [--annotation KEY=VALUE] [--json]
  localci install-hooks [--repo DIR]

Selectors:
  --repo DIR       Defaults to the nearest Git repo ancestor.
  --commit REF     Defaults to the latest run for query commands and HEAD for run commands.
  --task TASK      Full task name or unambiguous short task name.
  --attempt N      Defaults to the latest attempt.
`
}

func commandHelpText(command string) (string, bool) {
	switch command {
	case "status":
		return `Usage:
  localci status [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--no-clone] [--json]

Print a bounded status summary for a LocalCI run.

Defaults:
  --repo      nearest Git repo ancestor
  --commit    latest LocalCI run for the repo
  --task      all tasks
  --attempt   latest attempt

Examples:
  localci status
  localci status --task noisy-fail
  localci status --commit HEAD
  localci status --commit 'HEAD*' --task test
`, true
	case "run":
		return `Usage:
  localci run [--repo DIR] [--commit REF] [--task TASK] [--wait] [--no-clone] [--annotation KEY=VALUE] [--json]

Queue a daemon-managed LocalCI run. Use --no-clone for a working-tree run
against the current checkout.

Defaults:
  --repo      nearest Git repo ancestor
  --commit    HEAD
  --task      all tasks

Examples:
  localci run --wait
  localci run --no-clone --wait
  localci run --no-clone --task test --wait
`, true
	case "artifacts":
		return `Usage:
  localci artifacts [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--failed] [--primary] [--paths-only] [--json]

Print filesystem paths for task artifacts.

Defaults:
  --repo      nearest Git repo ancestor
  --commit    latest LocalCI run for the repo
  --task      all tasks
  --attempt   latest attempt for each task

Examples:
  localci artifacts
  localci artifacts --task noisy-fail
  localci artifacts --failed --primary
  localci artifacts --task noisy-fail --primary --paths-only
`, true
	case "history":
		return `Usage:
  localci history [--repo DIR] [--commit REF] [--task TASK] [--status STATUS] [--failed] [--limit N] [--json]

Print recent LocalCI runs, optionally filtered to a task or status.

Defaults:
  --repo    nearest Git repo ancestor
  --limit   20

Examples:
  localci history
  localci history --task noisy-fail
  localci history --failed
  localci history --task //web:localci:test --limit 50
`, true
	case "wait":
		return `Usage:
  localci wait [--repo DIR] [--commit REF] [--task TASK] [--no-clone] [--json]

Wait for the selected run or task to complete.

Examples:
  localci wait
  localci wait --task noisy-fail
  localci wait --commit 'HEAD*'
`, true
	case "invoke":
		return `Usage:
  localci invoke [--repo DIR] [--commit REF] [--task TASK] [--wait] [--no-clone] [--annotation KEY=VALUE] [--json]

Run an ad hoc LocalCI check for a commit or working tree.

Defaults:
  --repo      nearest Git repo ancestor
  --commit    HEAD
  --task      all tasks

Examples:
  localci invoke --task test --wait
  localci invoke --no-clone --task test --wait
  localci invoke --commit HEAD --annotation branch=main
`, true
	case "postcommit":
		return `Usage:
  localci postcommit [--repo DIR] [--commit REF] [--task TASK] [--annotation KEY=VALUE] [--json]

Queue LocalCI checks for a commit. Intended for Git hooks.

Defaults:
  --repo      nearest Git repo ancestor
  --commit    HEAD
  --task      all tasks

Examples:
  localci postcommit --commit HEAD
  localci postcommit --commit HEAD --task test
  localci postcommit --repo "$repo" --commit "$commit"
`, true
	case "cancel":
		return `Usage:
  localci cancel [--repo DIR] [--commit REF] [--task TASK] [--no-clone] [--json]

Cancel active or queued LocalCI tasks.

Examples:
  localci cancel
  localci cancel --task noisy-fail
  localci cancel --commit 'HEAD*' --task test
`, true
	case "web":
		return `Usage:
  localci web [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--artifact ARTIFACT] [--no-clone]

Open the local web UI at the selected page.

Examples:
  localci web
  localci web --task noisy-fail
  localci web --task noisy-fail --artifact combined.log
`, true
	case "dash":
		return `Usage:
  localci dash [--repo DIR] [--commit REF] [--task TASK] [--attempt N] [--artifact ARTIFACT] [--no-clone]

Open the terminal dashboard at the selected page.

Examples:
  localci dash
  localci dash --task noisy-fail
`, true
	case "install-hooks":
		return `Usage:
  localci install-hooks [--repo DIR]

Install localci's Git post-commit hook entry for a repo, defaulting to the
nearest ancestor of the current working directory that contains .git. The hook
uses modern Git hook.* config and runs:

  localci postcommit --repo "$repo" --commit "$commit"
`, true
	default:
		return "", false
	}
}

func usageError() error {
	return errors.New(strings.TrimSuffix(usageText(), "\n"))
}
