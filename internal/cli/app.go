package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"localci/internal/localci"
)

type App struct {
	Stdout            io.Writer
	Stderr            io.Writer
	Cwd               string
	OpenURL           func(string) error
	CheckRequirements func() error
	ConfigPath        string
	LoadConfig        func(string) (localci.Config, error)
	LatestCommit      func(string) (string, error)
	HeadCommit        func(string) (string, error)
}

func Run(args []string) int {
	app := App{
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Cwd:    mustGetwd(),
	}

	if err := app.Run(args); err != nil {
		fmt.Fprintf(app.Stderr, "localci: %v\n", err)
		return 1
	}

	return 0
}

func (a App) Run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}

	if isHelpCommand(args[0]) {
		a.printUsage()
		return nil
	}
	if len(args) == 2 && isHelpCommand(args[1]) {
		return a.printCommandHelp(args[0])
	}
	if err := a.checkRequirements(); err != nil {
		return err
	}

	switch args[0] {
	case "daemon":
		return a.runDaemon(args[1:])
	case "start":
		return a.runStart(args[1:])
	case "restart":
		return a.runRestart(args[1:])
	case "stop":
		return a.runStop(args[1:])
	case "postcommit":
		return a.runPostcommit(args[1:])
	case "invoke":
		return a.runInvoke(args[1:])
	case "cancel":
		return a.runCancel(args[1:])
	case "wait":
		return a.runWait(args[1:])
	case "status":
		return a.runStatus(args[1:])
	case "history":
		return a.runHistory(args[1:])
	case "artifacts":
		return a.runArtifacts(args[1:])
	case "web":
		return a.runWeb(args[1:])
	case "dash":
		return a.runDash(args[1:])
	case "install-hooks":
		return a.runInstallHooks(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], usageText())
	}
}

func isHelpCommand(arg string) bool {
	switch arg {
	case "help", "-h", "--help":
		return true
	default:
		return false
	}
}

func (a App) checkRequirements() error {
	if a.CheckRequirements != nil {
		return a.CheckRequirements()
	}
	return localci.RequirementsChecker{}.Check()
}

func (a App) runPostcommit(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "annotation": true, "json": true,
	})
	if err != nil {
		return err
	}
	repo, err := a.resolveSelectorRepo(flags.Repo)
	if err != nil {
		return err
	}
	commit, err := a.resolveRunCommit(repo, flags)
	if err != nil {
		return err
	}

	if commit == "" {
		return fmt.Errorf("commit must not be empty")
	}

	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	requestedTasks, err := a.resolveRequestedTasks(context.Background(), runner, repo, flags.Task)
	if err != nil {
		return err
	}

	client := localci.DaemonClient{Paths: runner.Paths}
	enqueued, err := client.Postcommit(context.Background(), repo, commit, mergedAnnotations(localci.GitAnnotations(context.Background(), repo), flags.Annotation), requestedTasks)
	if err != nil {
		return err
	}
	if flags.JSON {
		return writeJSON(a.Stdout, map[string]any{
			"repo":     repo,
			"commit":   commit,
			"enqueued": enqueued,
		})
	}

	fmt.Fprintf(a.Stdout, "Enqueued %d %s for %s at %s\n", len(enqueued), pluralizeTask(len(enqueued)), repo, commit)
	fmt.Fprintf(a.Stdout, "Status: localci status --repo %s --commit %s\n", shellQuote(repo), shellQuote(commit))
	if resultURL, err := a.postcommitResultURL(client, repo, commit); err == nil && resultURL != "" {
		fmt.Fprintf(a.Stdout, "Results: %s\n", resultURL)
	}
	for _, entry := range enqueued {
		fmt.Fprintf(a.Stdout, "%s\n", entry.TaskName)
	}
	fmt.Fprintln(a.Stdout, postcommitWaitInstruction(repo, commit))
	return nil
}

func postcommitWaitInstruction(repo string, commit string) string {
	return fmt.Sprintf("Wait: localci wait --repo %s --commit %s", shellQuote(repo), shellQuote(commit))
}

func (a App) runStatus(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "attempt": true, "no-clone": true, "json": true,
	})
	if err != nil {
		return err
	}
	runner, err := a.newRunner()
	if err != nil {
		return err
	}

	client := localci.DaemonClient{Paths: runner.Paths}
	repo, err := a.resolveSelectorRepo(flags.Repo)
	if err != nil {
		return err
	}
	commit, err := a.resolveQueryCommit(repo, flags)
	if err != nil {
		return err
	}
	statusView, err := client.Status(context.Background(), repo, commit)
	if err != nil {
		return err
	}

	taskName, err := resolveTaskName(statusView.Tasks, flags.Task)
	if err != nil {
		return err
	}
	filtered := filterStatusTasks(statusView.Tasks, taskName, false)
	for i := range filtered {
		filtered[i] = localci.ApplySelectedAttempt(runner.Paths, repo, commit, filtered[i], flags.Attempt)
	}
	if flags.JSON {
		return writeJSON(a.Stdout, localci.CommitStatusView{
			RepoDir:     statusView.RepoDir,
			Commit:      statusView.Commit,
			Annotations: statusView.Annotations,
			Tasks:       filtered,
		})
	}

	if taskName != "" {
		a.printCommitHeader("Status", statusView)
		fmt.Fprintln(a.Stdout)
		a.printTaskDetail(filtered[0])
		return nil
	}
	a.printStatusSummary(statusView, filtered)
	return nil
}

func (a App) runWeb(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "attempt": true, "artifact": true, "no-clone": true,
	})
	if err != nil {
		return err
	}
	repo, err := a.resolveSelectorRepo(flags.Repo)
	if err != nil {
		return err
	}
	commit := ""
	if flags.Commit != "" || flags.Task != "" || flags.Artifact != "" || flags.NoClone {
		commit, err = a.resolveQueryCommit(repo, flags)
		if err != nil {
			return err
		}
	}
	runner, err := a.newRunner()
	if err != nil {
		return err
	}

	client := localci.DaemonClient{Paths: runner.Paths}
	state, err := client.Ping(context.Background())
	if err != nil {
		return err
	}
	task := flags.Task
	attempt := flags.Attempt
	if commit != "" && task != "" {
		view, err := client.Status(context.Background(), repo, commit)
		if err != nil {
			return err
		}
		task, err = resolveTaskName(view.Tasks, task)
		if err != nil {
			return err
		}
		if attempt <= 0 {
			for _, candidate := range view.Tasks {
				if candidate.Name == task {
					attempt = candidate.Attempt
					break
				}
			}
		}
	}
	spec := commitTarget{RepoDir: repo, Commit: commit, Task: task, Attempt: attempt, Artifact: flags.Artifact}

	return a.openWeb(spec, state)
}

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

func mustGetwd() string {
	wd, err := os.Getwd()
	if err != nil {
		return "."
	}

	return wd
}

type commitTarget struct {
	RepoDir  string
	Commit   string
	Task     string
	Attempt  int
	Artifact string
}

func (a App) runInvoke(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "wait": true, "no-clone": true, "annotation": true, "json": true,
	})
	if err != nil {
		return err
	}
	repo, err := a.resolveSelectorRepo(flags.Repo)
	if err != nil {
		return err
	}
	commit, err := a.resolveRunCommit(repo, flags)
	if err != nil {
		return err
	}
	if commit == "" {
		return fmt.Errorf("commit must not be empty")
	}

	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	requestedTasks, err := a.resolveRequestedTasks(context.Background(), runner, repo, flags.Task)
	if err != nil {
		return err
	}

	result, err := runner.Invoke(context.Background(), localci.InvokeRequest{
		RepoDir:        repo,
		Commit:         commit,
		Annotations:    mergedAnnotations(localci.GitAnnotations(context.Background(), repo), flags.Annotation),
		RequestedTasks: requestedTasks,
		NoClone:        flags.NoClone,
	})
	if err != nil {
		return err
	}

	if flags.Wait {
		view, err := a.statusViewForRun(runner, result)
		if err != nil {
			return err
		}
		return a.printWaitSummary(view)
	}

	if flags.JSON {
		return writeJSON(a.Stdout, result)
	}
	a.printInvokeSummary(result)
	if !result.Success() {
		return fmt.Errorf("invoke finished with failing tasks")
	}

	return nil
}

func (a App) runCancel(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "no-clone": true, "json": true,
	})
	if err != nil {
		return err
	}
	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	client := localci.DaemonClient{Paths: runner.Paths}

	var repoDir string
	var commit string
	var taskName string
	if flags.Repo == "" && flags.Commit == "" && flags.Task == "" {
		active, err := client.ActiveTask(context.Background())
		if err != nil {
			return err
		}
		if active == nil || active.Kind != localci.QueueEntryKindTask {
			return fmt.Errorf("no active task to cancel")
		}
		repoDir = active.RepoDir
		commit = active.Commit
		taskName = active.TaskName
	} else {
		repoDir, err = a.resolveSelectorRepo(flags.Repo)
		if err != nil {
			return err
		}
		commit, err = a.resolveQueryCommit(repoDir, flags)
		if err != nil {
			return err
		}
		view, err := client.Status(context.Background(), repoDir, commit)
		if err != nil {
			return err
		}
		taskName, err = resolveTaskName(view.Tasks, flags.Task)
		if err != nil {
			return err
		}
		if taskName == "" {
			return fmt.Errorf("--task is required when canceling by selector")
		}
	}

	result, err := client.Cancel(context.Background(), repoDir, commit, taskName)
	if err != nil {
		return err
	}
	if !result.Active && result.Pending == 0 {
		fmt.Fprintf(a.Stdout, "No queued or running task matched %s at %s\n", taskName, commit)
		return nil
	}
	if flags.JSON {
		return writeJSON(a.Stdout, result)
	}
	fmt.Fprintf(a.Stdout, "Canceled %s at %s", taskName, commit)
	if result.Active {
		fmt.Fprint(a.Stdout, " (running)")
	}
	if result.Pending > 0 {
		fmt.Fprintf(a.Stdout, " (%d queued)", result.Pending)
	}
	fmt.Fprintln(a.Stdout)
	return nil
}

func (a App) runWait(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "no-clone": true, "json": true,
	})
	if err != nil {
		return err
	}
	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	repo, err := a.resolveSelectorRepo(flags.Repo)
	if err != nil {
		return err
	}
	commit, err := a.resolveQueryCommit(repo, flags)
	if err != nil {
		return err
	}

	client := localci.DaemonClient{Paths: runner.Paths}
	view, err := (localci.Waiter{Client: client}).Wait(context.Background(), repo, commit)
	if err != nil {
		return err
	}
	if flags.Task != "" {
		taskName, err := resolveTaskName(view.Tasks, flags.Task)
		if err != nil {
			return err
		}
		view.Tasks = filterStatusTasks(view.Tasks, taskName, false)
	}
	if flags.JSON {
		return writeJSON(a.Stdout, view)
	}

	return a.printWaitSummary(view)
}

func (a App) statusViewForRun(runner localci.Runner, result localci.RunRecord) (localci.CommitStatusView, error) {
	tasks := result.DiscoveredTasks
	if len(tasks) == 0 {
		var err error
		tasks, err = runner.DiscoverTasks(context.Background(), result.RepoDir)
		if err != nil {
			return localci.CommitStatusView{}, err
		}
	}
	return localci.BuildCommitStatusView(runner.Paths, result.RepoDir, result.Commit, tasks, nil, nil)
}

func (a App) resolveRequestedTasks(ctx context.Context, runner localci.Runner, repo string, taskQuery string) ([]string, error) {
	if strings.TrimSpace(taskQuery) == "" {
		return nil, nil
	}
	tasks, err := runner.DiscoverTasks(ctx, repo)
	if err != nil {
		return nil, err
	}
	return resolveDiscoveredTaskNames(tasks, taskQuery)
}

func (a App) printWaitSummary(view localci.CommitStatusView) error {
	a.printCommitHeader("Completed", view)

	failed := localci.FailedTasks(view)
	if len(failed) == 0 {
		return nil
	}

	fmt.Fprintln(a.Stdout)
	fmt.Fprintln(a.Stdout, cliTitleStyle.Render("Failed Tasks"))
	printTaskRows(a.Stdout, taskRows(failed), false)
	for _, task := range failed {
		if task.OutputDir != "" {
			fmt.Fprintf(a.Stdout, "  Output: %s\n", task.OutputDir)
		}
		if resultURL, err := a.taskResultURL(view.RepoDir, view.Commit, task.Name); err == nil && resultURL != "" {
			fmt.Fprintf(a.Stdout, "  Results: %s\n", resultURL)
		}
		if primaryArtifact, ok := localci.PrimaryArtifact(task); ok {
			fmt.Fprintf(a.Stdout, "  Primary artifact: %s\n", primaryArtifact.DisplayName)
			if primaryLogPath := artifactPathByDisplayName(task, primaryArtifact.DisplayName); primaryLogPath != "" {
				fmt.Fprintf(a.Stdout, "  Primary log path: %s\n", primaryLogPath)
			}
		}
	}
	return fmt.Errorf("localci run failed")
}

func mergedAnnotations(base map[string]string, override map[string]string) map[string]string {
	if len(base) == 0 && len(override) == 0 {
		return nil
	}
	merged := map[string]string{}
	for key, value := range base {
		merged[key] = value
	}
	for key, value := range override {
		merged[key] = value
	}
	return merged
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func noCloneCommitLabel(commit string) string {
	commit = strings.TrimSpace(commit)
	if strings.HasSuffix(commit, "*") {
		return commit
	}
	return commit + "*"
}

func (a App) newRunner() (localci.Runner, error) {
	root, err := defaultLocalCIRoot()
	if err != nil {
		return localci.Runner{}, err
	}

	return localci.Runner{
		Paths:  localci.Paths{Root: root},
		Stdout: a.Stdout,
		Stderr: a.Stderr,
	}, nil
}

func defaultLocalCIRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}

	return filepath.Join(home, ".localci"), nil
}

func (a App) openWeb(spec commitTarget, state localci.DaemonState) error {
	if strings.TrimSpace(state.HTTPBaseURL) == "" {
		return fmt.Errorf("daemon did not publish an HTTP base URL")
	}

	targetURL, err := a.buildWebURL(state.HTTPBaseURL, spec)
	if err != nil {
		return err
	}

	if err := a.openURL(targetURL); err != nil {
		return err
	}

	fmt.Fprintln(a.Stdout, targetURL)
	return nil
}

func (a App) postcommitResultURL(client localci.DaemonClient, repo string, commit string) (string, error) {
	state, err := client.Ping(context.Background())
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(state.HTTPBaseURL) == "" {
		return "", nil
	}
	return a.buildWebURL(state.HTTPBaseURL, commitTarget{
		RepoDir: repo,
		Commit:  commit,
	})
}

func (a App) taskResultURL(repo string, commit string, task string) (string, error) {
	runner, err := a.newRunner()
	if err != nil {
		return "", err
	}
	state, err := (localci.DaemonClient{Paths: runner.Paths}).Ping(context.Background())
	if err != nil {
		return "", nil
	}
	if strings.TrimSpace(state.HTTPBaseURL) == "" {
		return "", nil
	}
	return a.buildWebURL(state.HTTPBaseURL, commitTarget{
		RepoDir: repo,
		Commit:  commit,
		Task:    task,
	})
}

func (a App) buildWebURL(baseURL string, spec commitTarget) (string, error) {
	root, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse daemon url: %w", err)
	}

	if spec.RepoDir == "" && spec.Commit == "" && spec.Task == "" {
		root.Path = "/"
		root.RawQuery = ""
		return root.String(), nil
	}

	cfg, err := a.loadConfig()
	if err != nil {
		return "", err
	}

	switch {
	case spec.Commit == "":
		repoPath, err := localci.RouteRepoPath(cfg.Root, spec.RepoDir)
		if err != nil {
			return "", err
		}
		root.Path = path.Join("/repo", repoPath)
	case spec.Task == "":
		commitPath, err := localci.CommitRoutePath(cfg.Root, spec.RepoDir, spec.Commit)
		if err != nil {
			return "", err
		}
		if err := setEscapedURLPath(root, commitPath); err != nil {
			return "", err
		}
	case spec.Artifact == "":
		taskPath, err := localci.TaskRoutePath(cfg.Root, spec.RepoDir, spec.Commit, spec.Task)
		if err != nil {
			return "", err
		}
		if err := setEscapedURLPath(root, taskPath); err != nil {
			return "", err
		}
	default:
		attempt := spec.Attempt
		if attempt <= 0 {
			attempt = 1
		}
		artifactPath, err := localci.ArtifactRoutePath(cfg.Root, spec.RepoDir, spec.Commit, spec.Task, attempt, spec.Artifact)
		if err != nil {
			return "", err
		}
		if err := setEscapedURLPath(root, artifactPath); err != nil {
			return "", err
		}
	}
	root.RawQuery = ""
	return root.String(), nil
}

func setEscapedURLPath(target *url.URL, escapedPath string) error {
	decodedPath, err := url.PathUnescape(escapedPath)
	if err != nil {
		return err
	}
	target.Path = decodedPath
	target.RawPath = escapedPath
	return nil
}

func (a App) openURL(targetURL string) error {
	if a.OpenURL != nil {
		return a.OpenURL(targetURL)
	}
	return defaultOpenURL(targetURL)
}

func defaultOpenURL(targetURL string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", targetURL)
	case "linux":
		cmd = exec.Command("xdg-open", targetURL)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", targetURL)
	default:
		return fmt.Errorf("unsupported platform %q for browser launching", runtime.GOOS)
	}

	if err := cmd.Start(); err != nil {
		return err
	}
	if cmd.Process != nil {
		_ = cmd.Process.Release()
	}
	return nil
}

func pluralizeTask(count int) string {
	if count == 1 {
		return "task"
	}
	return "tasks"
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"\\$`!#&*()[]{}|;<>?~") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func formatTaskSummary(task localci.TaskStatusView) string {
	parts := []string{
		string(task.Status),
		task.ShortName,
	}
	if task.Attempt > 0 {
		attempt := fmt.Sprintf("attempt %d", task.Attempt)
		if task.AttemptCount > 1 {
			attempt += fmt.Sprintf(" of %d", task.AttemptCount)
		}
		parts = append(parts, attempt)
	}
	if task.DurationMilliseconds > 0 {
		parts = append(parts, fmt.Sprintf("%dms", task.DurationMilliseconds))
	}
	if task.Failure != "" {
		parts = append(parts, "failure="+task.Failure)
	}
	return strings.Join(parts, "  ")
}

func (a App) printTaskDetail(task localci.TaskStatusView) {
	fmt.Fprintln(a.Stdout, formatTaskSummary(task))
	if task.OutputDir != "" {
		fmt.Fprintf(a.Stdout, "Output: %s\n", task.OutputDir)
	}
	if len(task.Artifacts) > 0 {
		fmt.Fprintln(a.Stdout, "Artifacts:")
		for _, artifact := range task.Artifacts {
			fmt.Fprintf(a.Stdout, "  %s\t%s\n", artifact.DisplayName, artifact.Path)
		}
	}
	if primaryArtifact, ok := localci.PrimaryArtifact(task); ok {
		fmt.Fprintf(a.Stdout, "Primary artifact: %s\n", primaryArtifact.DisplayName)
		if primaryLogPath := artifactPathByDisplayName(task, primaryArtifact.DisplayName); primaryLogPath != "" {
			fmt.Fprintf(a.Stdout, "Primary log path: %s\n", primaryLogPath)
		}
	}
}

func artifactPathByDisplayName(task localci.TaskStatusView, displayName string) string {
	for _, artifact := range task.Artifacts {
		if artifact.DisplayName == displayName {
			return artifact.Path
		}
	}
	return ""
}

func (a App) runInstallHooks(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{"repo": true})
	if err != nil {
		return err
	}

	repoDir, err := a.repoFromFlagOrCwd(flags.Repo)
	if err != nil {
		return err
	}

	if err := (localci.HookInstaller{RepoDir: repoDir}).Install(); err != nil {
		return err
	}

	fmt.Fprintf(a.Stdout, "Installed localci post-commit hook in %s\n", repoDir)
	return nil
}

func (a App) resolveRepoArg(path string) (string, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return "", err
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(a.Cwd, path)
	}

	return localci.ResolveRepoDir(cfg.Root, path)
}

func (a App) repoFromFlagOrCwd(repoArg string) (string, error) {
	if strings.TrimSpace(repoArg) != "" {
		repo, err := a.resolveRepoArg(repoArg)
		if err != nil {
			return "", fmt.Errorf("resolve --repo: %w", err)
		}
		return repo, nil
	}

	repo, err := a.discoverRepoFromCwd()
	if err != nil {
		return "", err
	}
	return repo, nil
}

func (a App) discoverRepoFromCwd() (string, error) {
	start := a.Cwd
	if strings.TrimSpace(start) == "" {
		start = mustGetwd()
	}
	if !filepath.IsAbs(start) {
		absStart, err := filepath.Abs(start)
		if err != nil {
			return "", fmt.Errorf("resolve cwd: %w", err)
		}
		start = absStart
	}

	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
			return a.resolveRepoArg(dir)
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("inspect git metadata: %w", err)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find a git repository from %s; pass --repo <path>", start)
}

func (a App) loadConfig() (localci.Config, error) {
	path := a.ConfigPath
	if path == "" {
		root, err := defaultLocalCIRoot()
		if err != nil {
			return localci.Config{}, err
		}
		path = filepath.Join(root, "config.toml")
	}

	if a.LoadConfig != nil {
		return a.LoadConfig(path)
	}
	return localci.LoadConfigOrDefault(path)
}

func (a App) latestCommitForRepo(repoDir string) (string, error) {
	if a.LatestCommit != nil {
		return a.LatestCommit(repoDir)
	}

	root, err := defaultLocalCIRoot()
	if err != nil {
		return "", err
	}

	commits, err := (localci.HistoryReader{Paths: localci.Paths{Root: root}}).ListRepoCommits(repoDir)
	if err != nil {
		if errors.Is(err, localci.ErrRecordNotFound) {
			return "", fmt.Errorf("no localci runs found for %s", repoDir)
		}
		return "", err
	}
	if len(commits) == 0 {
		return "", fmt.Errorf("no localci runs found for %s", repoDir)
	}
	return commits[0].Commit, nil
}

func (a App) resolveCommitAlias(repoDir string, commit string) (string, error) {
	commit = strings.TrimSpace(commit)
	if commit == "HEAD*" {
		head, err := a.headCommit(repoDir)
		if err != nil {
			return "", err
		}
		return noCloneCommitLabel(head), nil
	}
	if commit != "HEAD" {
		return commit, nil
	}
	return a.headCommit(repoDir)
}

func (a App) headCommit(repoDir string) (string, error) {
	if a.HeadCommit != nil {
		return a.HeadCommit(repoDir)
	}
	return localci.GitHeadCommit(context.Background(), repoDir)
}
