package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"localci/internal/localci"
)

type App struct {
	Stdout io.Writer
	Stderr io.Writer
	Cwd    string
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

	switch args[0] {
	case "help", "-h", "--help":
		a.printUsage()
		return nil
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

	return errors.New("not implemented: postcommit enqueue for repo " + repo + " and commit " + commit)
}

func (a App) runStatus(args []string) error {
	spec, err := parseCommitTarget(args, a.Cwd)
	if err != nil {
		return err
	}

	return errors.New("not implemented: status for repo " + spec.RepoDir + ", commit " + spec.Commit + maybeTaskSuffix(spec.Task))
}

func (a App) runWeb(args []string) error {
	spec, err := parseCommitTarget(args, a.Cwd)
	if err != nil {
		return err
	}

	return errors.New("not implemented: web for repo " + spec.RepoDir + ", commit " + spec.Commit + maybeTaskSuffix(spec.Task))
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

func parseCommitTarget(args []string, cwd string) (commitTarget, error) {
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
		return commitTarget{}, fmt.Errorf("usage: localci status [dir] <commit> [task]")
	}
}

func maybeTaskSuffix(task string) string {
	if task == "" {
		return ""
	}

	return ", task " + task
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
