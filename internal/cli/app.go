package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
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
	if len(args) != 2 {
		return fmt.Errorf("usage: localci postcommit <repo> <commit>")
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
	enqueued, err := client.Postcommit(context.Background(), repo, commit)
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
	spec, err := a.parseCommitTarget(args, "usage: localci status [dir] <commit> [task]")
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
	for _, task := range filtered {
		fmt.Fprintf(a.Stdout, "%s\t%s\t%s", task.Status, task.Name, task.OutputDir)
		if task.Attempt > 0 {
			fmt.Fprintf(a.Stdout, "\tattempt %d", task.Attempt)
		}
		if task.Failure != "" {
			fmt.Fprintf(a.Stdout, "\tfailure=%s", task.Failure)
		}
		if task.DurationMilliseconds > 0 {
			fmt.Fprintf(a.Stdout, "\t%dms", task.DurationMilliseconds)
		}
		fmt.Fprintln(a.Stdout)
		if spec.Task != "" {
			primaryArtifact, primaryLog := localci.LoadPrimaryLog(task)
			if primaryArtifact != "" {
				fmt.Fprintf(a.Stdout, "Primary log: %s\n", primaryArtifact)
				if primaryLog != "" {
					fmt.Fprintln(a.Stdout, primaryLog)
				}
			}
		}
		for _, file := range task.OutputFiles {
			fmt.Fprintf(a.Stdout, "  %s\n", file)
		}
	}
	return nil
}

func (a App) runWeb(args []string) error {
	spec, err := a.parseWebTarget(args)
	if err != nil {
		return err
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

func usageText() string {
	return `localci is a local post-commit validation runner.

Usage:
  localci start
  localci restart
  localci stop
	localci postcommit <repo> <commit>
  localci invoke <repo> <commit>
  localci status [dir] <commit> [task]
  localci web [dir] [commit] [task]
  localci install-hooks [dir]
`
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
	if len(args) != 2 {
		return fmt.Errorf("usage: localci invoke <repo> <commit>")
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

	result, err := runner.Invoke(context.Background(), localci.InvokeRequest{
		RepoDir: repo,
		Commit:  commit,
	})
	if err != nil {
		return err
	}

	a.printInvokeSummary(result)
	if !result.Success() {
		return fmt.Errorf("invoke finished with failing tasks")
	}

	return nil
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
	case 1:
		repo, err := a.resolveRepoArg(a.Cwd)
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}
		return commitTarget{
			RepoDir: repo,
			Commit:  strings.TrimSpace(args[0]),
		}, nil
	case 2:
		repo, err := a.resolveRepoArg(args[0])
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}

		return commitTarget{
			RepoDir: repo,
			Commit:  strings.TrimSpace(args[1]),
		}, nil
	case 3:
		repo, err := a.resolveRepoArg(args[0])
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}

		return commitTarget{
			RepoDir: repo,
			Commit:  strings.TrimSpace(args[1]),
			Task:    strings.TrimSpace(args[2]),
		}, nil
	default:
		return commitTarget{}, fmt.Errorf("%s", usage)
	}
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
		return commitTarget{
			RepoDir: defaultRepo,
			Commit:  strings.TrimSpace(args[0]),
		}, nil
	case 2, 3:
		return a.parseCommitTarget(args, "usage: localci web [dir] [commit] [task]")
	default:
		return commitTarget{}, fmt.Errorf("usage: localci web [dir] [commit] [task]")
	}
}

func (a App) openWeb(spec commitTarget, state localci.DaemonState) error {
	if strings.TrimSpace(state.HTTPBaseURL) == "" {
		return fmt.Errorf("daemon did not publish an HTTP base URL")
	}

	targetURL, err := buildWebURL(state.HTTPBaseURL, spec)
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
	return buildWebURL(state.HTTPBaseURL, commitTarget{
		RepoDir: repo,
		Commit:  commit,
	})
}

func buildWebURL(baseURL string, spec commitTarget) (string, error) {
	root, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse daemon url: %w", err)
	}

	switch {
	case spec.RepoDir == "" && spec.Commit == "" && spec.Task == "":
		root.Path = "/"
	case spec.Commit == "":
		root.Path = "/repo"
	case spec.Task == "":
		root.Path = "/commit"
	default:
		root.Path = "/task"
	}

	query := root.Query()
	if spec.RepoDir != "" {
		query.Set("repo", spec.RepoDir)
	}
	if spec.Commit != "" {
		query.Set("commit", spec.Commit)
	}
	if spec.Task != "" {
		query.Set("task", spec.Task)
	}
	root.RawQuery = query.Encode()
	return root.String(), nil
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

	cfg, err := localci.LoadConfig(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return localci.Config{Root: string(filepath.Separator)}, nil
		}
		return localci.Config{}, err
	}
	return cfg, nil
}
