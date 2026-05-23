package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"sort"
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
	case "web":
		return a.runWeb(args[1:])
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
	annotations, args, err := parseAnnotationArgs(args)
	if err != nil {
		return err
	}
	if len(args) != 2 {
		return fmt.Errorf("usage: localci postcommit [--annotation key=value] <repo> <commit>")
	}

	repo, err := a.resolveRepoArg(args[0])
	if err != nil {
		return fmt.Errorf("resolve repo: %w", err)
	}

	commit := strings.TrimSpace(args[1])
	if commit == "" {
		return fmt.Errorf("commit must not be empty")
	}

	runner, err := a.newRunner()
	if err != nil {
		return err
	}

	client := localci.DaemonClient{Paths: runner.Paths}
	enqueued, err := client.Postcommit(context.Background(), repo, commit, mergedAnnotations(localci.GitAnnotations(context.Background(), repo), annotations))
	if err != nil {
		return err
	}

	fmt.Fprintf(a.Stdout, "Enqueued %d %s for %s at %s\n", len(enqueued), pluralizeTask(len(enqueued)), repo, commit)
	fmt.Fprintf(a.Stdout, "Status: localci status %s %s\n", shellQuote(repo), shellQuote(commit))
	if resultURL, err := a.postcommitResultURL(client, repo, commit); err == nil && resultURL != "" {
		fmt.Fprintf(a.Stdout, "Results: %s\n", resultURL)
	}
	for _, entry := range enqueued {
		fmt.Fprintf(a.Stdout, "%s\n", entry.TaskName)
	}
	return nil
}

func (a App) runStatus(args []string) error {
	noClone := false
	args = filterFlag(args, "--no-clone", &noClone)
	spec, err := a.parseStatusTarget(args, noClone, "usage: localci status [--no-clone] [dir] <commit> [task]")
	if err != nil {
		return err
	}

	runner, err := a.newRunner()
	if err != nil {
		return err
	}

	client := localci.DaemonClient{Paths: runner.Paths}
	statusView, err := client.Status(context.Background(), spec.RepoDir, spec.Commit)
	if err != nil {
		return err
	}

	filtered := statusView.Tasks
	if spec.Task != "" {
		filtered = nil
		for _, task := range statusView.Tasks {
			if task.Name == spec.Task {
				filtered = append(filtered, task)
				break
			}
		}
		if len(filtered) == 0 {
			return fmt.Errorf("task %q not found", spec.Task)
		}
	}

	fmt.Fprintf(a.Stdout, "Status for %s at %s\n", statusView.RepoDir, statusView.Commit)
	a.printAnnotations(statusView.Annotations)
	for _, task := range filtered {
		if spec.Task != "" {
			a.printTaskDetail(task)
			continue
		}
		fmt.Fprintln(a.Stdout, formatTaskSummary(task))
	}
	return nil
}

func (a App) runWeb(args []string) error {
	noClone := false
	args = filterFlag(args, "--no-clone", &noClone)
	spec, err := a.parseWebTarget(args)
	if err != nil {
		return err
	}
	if noClone {
		if spec.Commit == "" {
			commit, err := a.headCommit(spec.RepoDir)
			if err != nil {
				return err
			}
			spec.Commit = commit
		}
		spec.Commit = noCloneCommitLabel(spec.Commit)
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
  localci postcommit [--annotation key=value] <repo> <commit>
  localci invoke [--wait] [--no-clone] [--annotation key=value] [dir] [commit]
  localci cancel [--no-clone]
  localci cancel [--no-clone] [dir] <commit> <task>
  localci wait [--no-clone] [dir] [commit]
  localci status [--no-clone] [dir] <commit> [task]
  localci web [--no-clone] [dir] [commit] [task]
  localci install-hooks [dir]
`
}

func commandHelpText(command string) (string, bool) {
	switch command {
	case "install-hooks":
		return `Usage:
  localci install-hooks [dir]

Install localci's Git post-commit hook entry for dir, defaulting to the
current working directory. The hook uses modern Git hook.* config and runs:

  localci postcommit "$repo" "$commit"
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
	RepoDir string
	Commit  string
	Task    string
}

func (a App) runInvoke(args []string) error {
	wait := false
	noClone := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--wait":
			wait = true
			continue
		case "--no-clone":
			noClone = true
			continue
		}
		filtered = append(filtered, arg)
	}
	args = filtered
	annotations, args, err := parseAnnotationArgs(args)
	if err != nil {
		return err
	}

	repo, commit, err := a.parseInvokeTarget(args)
	if err != nil {
		return err
	}
	if commit == "" {
		commit, err = a.headCommit(repo)
		if err != nil {
			return err
		}
	} else {
		commit, err = a.resolveCommitAlias(repo, commit)
		if err != nil {
			return err
		}
	}

	if commit == "" {
		return fmt.Errorf("commit must not be empty")
	}
	if noClone {
		commit = noCloneCommitLabel(commit)
	}

	runner, err := a.newRunner()
	if err != nil {
		return err
	}

	result, err := runner.Invoke(context.Background(), localci.InvokeRequest{
		RepoDir:     repo,
		Commit:      commit,
		Annotations: mergedAnnotations(localci.GitAnnotations(context.Background(), repo), annotations),
		NoClone:     noClone,
	})
	if err != nil {
		return err
	}

	if wait {
		view, err := a.statusViewForRun(runner, result)
		if err != nil {
			return err
		}
		return a.printWaitSummary(view)
	}

	a.printInvokeSummary(result)
	if !result.Success() {
		return fmt.Errorf("invoke finished with failing tasks")
	}

	return nil
}

func (a App) runCancel(args []string) error {
	noClone := false
	args = filterFlag(args, "--no-clone", &noClone)
	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	client := localci.DaemonClient{Paths: runner.Paths}

	var repoDir string
	var commit string
	var taskName string
	if len(args) == 0 {
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
		spec, err := a.parseCommitTarget(args, "usage: localci cancel [--no-clone] [dir] <commit> <task>")
		if err != nil {
			return err
		}
		if spec.Task == "" {
			return fmt.Errorf("usage: localci cancel [--no-clone] [dir] <commit> <task>")
		}
		repoDir = spec.RepoDir
		commit = spec.Commit
		taskName = spec.Task
	}
	if noClone {
		commit = noCloneCommitLabel(commit)
	}

	result, err := client.Cancel(context.Background(), repoDir, commit, taskName)
	if err != nil {
		return err
	}
	if !result.Active && result.Pending == 0 {
		fmt.Fprintf(a.Stdout, "No queued or running task matched %s at %s\n", taskName, commit)
		return nil
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
	noClone := false
	args = filterFlag(args, "--no-clone", &noClone)
	spec, err := a.parseStatusTarget(args, noClone, "usage: localci wait [--no-clone] [dir] [commit]")
	if err != nil {
		return err
	}

	runner, err := a.newRunner()
	if err != nil {
		return err
	}

	client := localci.DaemonClient{Paths: runner.Paths}
	view, err := (localci.Waiter{Client: client}).Wait(context.Background(), spec.RepoDir, spec.Commit)
	if err != nil {
		return err
	}

	return a.printWaitSummary(view)
}

func (a App) parseInvokeTarget(args []string) (string, string, error) {
	switch len(args) {
	case 0:
		repo, err := a.resolveRepoArg(a.Cwd)
		if err != nil {
			return "", "", fmt.Errorf("resolve dir: %w", err)
		}
		return repo, "", nil
	case 1:
		repo, err := a.resolveRepoArg(args[0])
		if err != nil {
			return "", "", fmt.Errorf("resolve dir: %w", err)
		}
		return repo, "", nil
	case 2:
		repo, err := a.resolveRepoArg(args[0])
		if err != nil {
			return "", "", fmt.Errorf("resolve repo: %w", err)
		}
		return repo, strings.TrimSpace(args[1]), nil
	default:
		return "", "", fmt.Errorf("usage: localci invoke [--wait] [--no-clone] [--annotation key=value] [dir] [commit]")
	}
}

func (a App) parseStatusTarget(args []string, noClone bool, usage string) (commitTarget, error) {
	if noClone {
		switch {
		case len(args) == 0:
			return a.currentNoCloneTarget()
		case len(args) == 1 && looksLikePathArg(args[0]):
			repo, err := a.resolveRepoArg(args[0])
			if err != nil {
				return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
			}
			commit, err := a.headCommit(repo)
			if err != nil {
				return commitTarget{}, err
			}
			return commitTarget{RepoDir: repo, Commit: noCloneCommitLabel(commit)}, nil
		}
	}

	spec, err := a.parseCommitTarget(args, usage)
	if err != nil {
		return commitTarget{}, err
	}
	if noClone {
		spec.Commit = noCloneCommitLabel(spec.Commit)
	}
	return spec, nil
}

func (a App) currentNoCloneTarget() (commitTarget, error) {
	repo, err := a.resolveRepoArg(a.Cwd)
	if err != nil {
		return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
	}
	commit, err := a.headCommit(repo)
	if err != nil {
		return commitTarget{}, err
	}
	return commitTarget{RepoDir: repo, Commit: noCloneCommitLabel(commit)}, nil
}

func (a App) statusViewForRun(runner localci.Runner, result localci.RunRecord) (localci.CommitStatusView, error) {
	tasks, err := runner.DiscoverTasks(context.Background(), result.RepoDir)
	if err != nil {
		return localci.CommitStatusView{}, err
	}
	return localci.BuildCommitStatusView(runner.Paths, result.RepoDir, result.Commit, tasks, nil, nil)
}

func (a App) printWaitSummary(view localci.CommitStatusView) error {
	fmt.Fprintf(a.Stdout, "Completed %s at %s: %s\n", view.RepoDir, view.Commit, localci.SummarizeCommit(view))

	failed := localci.FailedTasks(view)
	if len(failed) == 0 {
		return nil
	}

	fmt.Fprintln(a.Stdout, "Failed tasks:")
	for _, task := range failed {
		fmt.Fprintf(a.Stdout, "%s\n", formatTaskSummary(task))
		if task.OutputDir != "" {
			fmt.Fprintf(a.Stdout, "  Output: %s\n", task.OutputDir)
		}
		if resultURL, err := a.taskResultURL(view.RepoDir, view.Commit, task.Name); err == nil && resultURL != "" {
			fmt.Fprintf(a.Stdout, "  Results: %s\n", resultURL)
		}
		primaryArtifact, primaryLog := localci.LoadPrimaryLog(task)
		if primaryArtifact != "" {
			fmt.Fprintf(a.Stdout, "  Primary log: %s\n", primaryArtifact)
			if primaryLogPath := artifactPathByDisplayName(task, primaryArtifact); primaryLogPath != "" {
				fmt.Fprintf(a.Stdout, "  Primary log path: %s\n", primaryLogPath)
			}
			if primaryLog != "" {
				fmt.Fprintln(a.Stdout, primaryLog)
			}
		}
	}
	return fmt.Errorf("localci run failed")
}

func (a App) printAnnotations(annotations map[string]string) {
	if len(annotations) == 0 {
		return
	}
	fmt.Fprintln(a.Stdout, "Annotations:")
	keys := make([]string, 0, len(annotations))
	for key := range annotations {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(a.Stdout, "  %s=%s\n", key, annotations[key])
	}
}

func parseAnnotationArgs(args []string) (map[string]string, []string, error) {
	annotations := map[string]string{}
	filtered := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		var value string
		switch {
		case arg == "--annotation":
			i++
			if i >= len(args) {
				return nil, nil, fmt.Errorf("--annotation requires key=value")
			}
			value = args[i]
		case strings.HasPrefix(arg, "--annotation="):
			value = strings.TrimPrefix(arg, "--annotation=")
		default:
			filtered = append(filtered, arg)
			continue
		}
		key, annotationValue, ok := strings.Cut(value, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return nil, nil, fmt.Errorf("--annotation requires key=value")
		}
		annotations[key] = annotationValue
	}
	if len(annotations) == 0 {
		return nil, filtered, nil
	}
	return annotations, filtered, nil
}

func filterFlag(args []string, flag string, found *bool) []string {
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == flag {
			*found = true
			continue
		}
		filtered = append(filtered, arg)
	}
	return filtered
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

func (a App) printInvokeSummary(result localci.RunRecord) {
	if len(result.TaskResults) == 0 {
		fmt.Fprintf(a.Stdout, "No localci tasks discovered for %s at %s\n", result.RepoDir, result.Commit)
		return
	}

	fmt.Fprintf(a.Stdout, "Invoked %d task(s) for %s at %s\n", len(result.TaskResults), result.RepoDir, result.Commit)
	for _, task := range result.TaskResults {
		fmt.Fprintf(a.Stdout, "%s\t%s\t%s\n", task.Status, task.Name, task.OutputDir)
	}
}

func defaultLocalCIRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}

	return filepath.Join(home, ".localci"), nil
}

func (a App) parseCommitTarget(args []string, usage string) (commitTarget, error) {
	switch len(args) {
	case 0:
		repo, err := a.resolveRepoArg(a.Cwd)
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}
		commit, err := a.latestCommitForRepo(repo)
		if err != nil {
			return commitTarget{}, err
		}
		return commitTarget{
			RepoDir: repo,
			Commit:  commit,
		}, nil
	case 1:
		repo, err := a.resolveRepoArg(a.Cwd)
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}
		commit, err := a.resolveCommitAlias(repo, args[0])
		if err != nil {
			return commitTarget{}, err
		}
		return commitTarget{
			RepoDir: repo,
			Commit:  commit,
		}, nil
	case 2:
		repo, err := a.tryResolveExistingRepoArg(args[0])
		if err == nil {
			commit, err := a.resolveCommitAlias(repo, args[1])
			if err != nil {
				return commitTarget{}, err
			}

			return commitTarget{
				RepoDir: repo,
				Commit:  commit,
			}, nil
		}
		if looksLikePathArg(args[0]) {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}
		defaultRepo, defaultErr := a.resolveRepoArg(a.Cwd)
		if defaultErr != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", defaultErr)
		}
		commit, err := a.resolveCommitAlias(defaultRepo, args[0])
		if err != nil {
			return commitTarget{}, err
		}

		return commitTarget{
			RepoDir: defaultRepo,
			Commit:  commit,
			Task:    strings.TrimSpace(args[1]),
		}, nil
	case 3:
		repo, err := a.resolveRepoArg(args[0])
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}
		commit, err := a.resolveCommitAlias(repo, args[1])
		if err != nil {
			return commitTarget{}, err
		}

		return commitTarget{
			RepoDir: repo,
			Commit:  commit,
			Task:    strings.TrimSpace(args[2]),
		}, nil
	default:
		return commitTarget{}, fmt.Errorf("%s", usage)
	}
}

func looksLikePathArg(arg string) bool {
	arg = strings.TrimSpace(arg)
	return arg == "." || arg == ".." || strings.HasPrefix(arg, "~") || strings.ContainsAny(arg, `/\`)
}

func (a App) parseWebTarget(args []string) (commitTarget, error) {
	switch len(args) {
	case 0:
		repo, err := a.resolveRepoArg(a.Cwd)
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}
		return commitTarget{RepoDir: repo}, nil
	case 1:
		repo, err := a.tryResolveExistingRepoArg(args[0])
		if err == nil {
			return commitTarget{RepoDir: repo}, nil
		}
		defaultRepo, defaultErr := a.resolveRepoArg(a.Cwd)
		if defaultErr != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", defaultErr)
		}
		commit, err := a.resolveCommitAlias(defaultRepo, args[0])
		if err != nil {
			return commitTarget{}, err
		}
		return commitTarget{
			RepoDir: defaultRepo,
			Commit:  commit,
		}, nil
	case 2:
		repo, err := a.tryResolveExistingRepoArg(args[0])
		if err == nil {
			commit, err := a.resolveCommitAlias(repo, args[1])
			if err != nil {
				return commitTarget{}, err
			}
			return commitTarget{
				RepoDir: repo,
				Commit:  commit,
			}, nil
		}
		defaultRepo, defaultErr := a.resolveRepoArg(a.Cwd)
		if defaultErr != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", defaultErr)
		}
		commit, err := a.resolveCommitAlias(defaultRepo, args[0])
		if err != nil {
			return commitTarget{}, err
		}
		return commitTarget{
			RepoDir: defaultRepo,
			Commit:  commit,
			Task:    strings.TrimSpace(args[1]),
		}, nil
	case 3:
		return a.parseCommitTarget(args, "usage: localci web [dir] [commit] [task]")
	default:
		return commitTarget{}, fmt.Errorf("usage: localci web [dir] [commit] [task]")
	}
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
	default:
		taskPath, err := localci.TaskRoutePath(cfg.Root, spec.RepoDir, spec.Commit, spec.Task)
		if err != nil {
			return "", err
		}
		if err := setEscapedURLPath(root, taskPath); err != nil {
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
	primaryArtifact, primaryLog := localci.LoadPrimaryLog(task)
	if primaryArtifact != "" {
		fmt.Fprintf(a.Stdout, "Primary log: %s\n", primaryArtifact)
		if primaryLogPath := artifactPathByDisplayName(task, primaryArtifact); primaryLogPath != "" {
			fmt.Fprintf(a.Stdout, "Primary log path: %s\n", primaryLogPath)
		}
		if primaryLog != "" {
			fmt.Fprintln(a.Stdout, primaryLog)
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
	if len(args) > 1 {
		return fmt.Errorf("usage: localci install-hooks [dir]")
	}

	repoDir := a.Cwd
	if len(args) == 1 {
		var err error
		repoDir, err = a.resolveRepoArg(args[0])
		if err != nil {
			return fmt.Errorf("resolve dir: %w", err)
		}
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

func (a App) tryResolveExistingRepoArg(path string) (string, error) {
	resolved, err := a.resolveRepoArg(path)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%s is not a directory", resolved)
	}
	return resolved, nil
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
