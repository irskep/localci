package cli

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"localci/internal/localci"
)

type commitTarget struct {
	RepoDir  string
	Commit   string
	Task     string
	Attempt  int
	Artifact string
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

	switch {
	case spec.Commit == "":
		repoPath, err := localci.RouteRepoPath(spec.RepoDir)
		if err != nil {
			return "", err
		}
		root.Path = path.Join("/repo", repoPath)
	case spec.Task == "":
		commitPath, err := localci.CommitRoutePath(spec.RepoDir, spec.Commit)
		if err != nil {
			return "", err
		}
		if err := setEscapedURLPath(root, commitPath); err != nil {
			return "", err
		}
	case spec.Artifact == "":
		taskPath, err := localci.TaskRoutePath(spec.RepoDir, spec.Commit, spec.Task)
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
		artifactPath, err := localci.ArtifactRoutePath(spec.RepoDir, spec.Commit, spec.Task, attempt, spec.Artifact)
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
