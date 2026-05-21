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

	repo, err := resolveDir(args[0], a.Cwd)
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

	fmt.Fprintf(a.Stdout, "Enqueued %d task(s) for %s at %s\n", len(enqueued), repo, commit)
	for _, entry := range enqueued {
		fmt.Fprintf(a.Stdout, "%s\n", entry.TaskName)
	}
	return nil
}

func (a App) runStatus(args []string) error {
	spec, err := parseCommitTarget(args, a.Cwd, "usage: localci status [dir] <commit> [task]")
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
		fmt.Fprintf(a.Stdout, "%s\t%s\t%s\n", task.Status, task.Name, task.OutputDir)
		for _, file := range task.OutputFiles {
			fmt.Fprintf(a.Stdout, "  %s\n", file)
		}
	}
	return nil
}

func (a App) runWeb(args []string) error {
	spec, err := parseCommitTarget(args, a.Cwd, "usage: localci web [dir] <commit> [task]")
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
  localci stop
	localci postcommit <repo> <commit>
  localci invoke <repo> <commit>
  localci status [dir] <commit> [task]
  localci web [dir] <commit> [task]
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

	repo, err := resolveDir(args[0], a.Cwd)
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

func parseCommitTarget(args []string, cwd string, usage string) (commitTarget, error) {
	switch len(args) {
	case 1:
		return commitTarget{
			RepoDir: cwd,
			Commit:  strings.TrimSpace(args[0]),
		}, nil
	case 2:
		repo, err := resolveDir(args[0], cwd)
		if err != nil {
			return commitTarget{}, fmt.Errorf("resolve dir: %w", err)
		}

		return commitTarget{
			RepoDir: repo,
			Commit:  strings.TrimSpace(args[1]),
		}, nil
	case 3:
		repo, err := resolveDir(args[0], cwd)
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

func buildWebURL(baseURL string, spec commitTarget) (string, error) {
	root, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse daemon url: %w", err)
	}

	if spec.Task == "" {
		root.Path = "/commit"
	} else {
		root.Path = "/task"
	}

	query := root.Query()
	query.Set("repo", spec.RepoDir)
	query.Set("commit", spec.Commit)
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

func resolveDir(path string, cwd string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("directory must not be empty")
	}

	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	return filepath.Clean(abs), nil
}
