package cli

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"localci/internal/localci"
)

func (a App) runTUI(args []string) error {
	noClone := false
	args = filterFlag(args, "--no-clone", &noClone)
	repoArg, args, err := extractRepoFlag(args)
	if err != nil {
		return err
	}

	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	state, err := (localci.DaemonClient{Paths: runner.Paths}).Ping(context.Background())
	if err != nil {
		return err
	}
	if strings.TrimSpace(state.HTTPBaseURL) == "" {
		return fmt.Errorf("daemon did not publish an HTTP base URL")
	}
	client, err := newTUIClient(state.HTTPBaseURL)
	if err != nil {
		return err
	}

	route := tuiRoute{view: tuiViewHome, apiPath: "/api", title: "Home"}
	if repoArg != "" || len(args) > 0 {
		spec, err := a.parseWebTarget(repoArg, args)
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
		route, err = a.tuiRouteForTarget(spec)
		if err != nil {
			return err
		}
	}

	model := newTUIModel(client, route)
	program := tea.NewProgram(model, tea.WithAltScreen())
	_, err = program.Run()
	return err
}

func (a App) tuiRouteForTarget(spec commitTarget) (tuiRoute, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return tuiRoute{}, err
	}
	return tuiRouteForTarget(cfg.Root, spec)
}

func tuiRouteForTarget(root string, spec commitTarget) (tuiRoute, error) {
	switch {
	case spec.RepoDir == "":
		return tuiRoute{view: tuiViewHome, apiPath: "/api", title: "Home"}, nil
	case spec.Commit == "":
		repoPath, err := localci.RouteRepoPath(root, spec.RepoDir)
		if err != nil {
			return tuiRoute{}, err
		}
		return tuiRoute{
			view:     tuiViewRepo,
			apiPath:  "/api/repo/" + strings.TrimPrefix(repoPath, "/"),
			repoPath: canonicalPathLabel(root, spec.RepoDir),
			repoDir:  spec.RepoDir,
			title:    canonicalPathLabel(root, spec.RepoDir),
		}, nil
	case spec.Task == "":
		commitPath, err := localci.CommitRoutePath(root, spec.RepoDir, spec.Commit)
		if err != nil {
			return tuiRoute{}, err
		}
		apiPath := "/api" + commitPath
		return tuiRoute{
			view:     tuiViewCommit,
			apiPath:  apiPath,
			repoPath: canonicalPathLabel(root, spec.RepoDir),
			repoDir:  spec.RepoDir,
			commit:   spec.Commit,
			title:    shortCommit(spec.Commit),
		}, nil
	default:
		taskPath, err := localci.TaskRoutePath(root, spec.RepoDir, spec.Commit, spec.Task)
		if err != nil {
			return tuiRoute{}, err
		}
		apiPath := "/api" + taskPath
		return tuiRoute{
			view:     tuiViewTask,
			apiPath:  apiPath,
			repoPath: canonicalPathLabel(root, spec.RepoDir),
			repoDir:  spec.RepoDir,
			commit:   spec.Commit,
			task:     spec.Task,
			title:    trimTaskLabel(spec.Task),
		}, nil
	}
}

func tuiCommitRoute(repo tuiRepoSummary, commit string) tuiRoute {
	apiPath := path.Join("/api/repo", repo.RepoPath, "commit", commit)
	return tuiRoute{
		view:     tuiViewCommit,
		apiPath:  apiPath,
		repoPath: repo.RepoPath,
		repoDir:  repo.RepoDir,
		commit:   commit,
		title:    shortCommit(commit),
	}
}

func tuiRepoRoute(repo tuiRepoSummary) tuiRoute {
	return tuiRoute{
		view:     tuiViewRepo,
		apiPath:  path.Join("/api/repo", repo.RepoPath),
		repoPath: repo.RepoPath,
		repoDir:  repo.RepoDir,
		title:    repo.RepoPath,
	}
}

func tuiTaskRoute(repo tuiRepoSummary, commit string, task string) tuiRoute {
	return tuiRoute{
		view:     tuiViewTask,
		apiPath:  path.Join("/api/repo", repo.RepoPath, "commit", commit, "task", url.PathEscape(task)),
		repoPath: repo.RepoPath,
		repoDir:  repo.RepoDir,
		commit:   commit,
		task:     task,
		title:    trimTaskLabel(task),
	}
}

func tuiAttemptRoute(route tuiRoute, attempt int) tuiRoute {
	route.view = tuiViewTask
	route.attempt = attempt
	route.apiPath = path.Join("/api/repo", route.repoPath, "commit", route.commit, "task", url.PathEscape(route.task), "attempt", strconv.Itoa(attempt))
	route.title = trimTaskLabel(route.task) + " attempt " + strconv.Itoa(attempt)
	return route
}

func tuiArtifactListRoute(route tuiRoute, attempt int) tuiRoute {
	route.view = tuiViewArtifacts
	route.attempt = attempt
	route.apiPath = path.Join("/api/repo", route.repoPath, "commit", route.commit, "task", url.PathEscape(route.task), "attempt", strconv.Itoa(attempt), "artifact")
	route.title = "Artifacts"
	return route
}

func tuiArtifactRoute(route tuiRoute, artifact string) tuiRoute {
	route.view = tuiViewArtifact
	route.artifact = artifact
	route.apiPath = path.Join("/api/repo", route.repoPath, "commit", route.commit, "task", url.PathEscape(route.task), "attempt", strconv.Itoa(route.attempt), "artifact", artifact)
	route.title = artifact
	return route
}

func canonicalPathLabel(root string, repoDir string) string {
	label, err := localci.CanonicalRepoPath(root, repoDir)
	if err != nil || label == "" {
		return repoDir
	}
	return label
}
