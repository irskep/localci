package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
