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

func (a App) runDash(flags cliFlags) error {
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
	if flags.Repo != "" || flags.Commit != "" || flags.Task != "" || flags.Artifact != "" || flags.NoClone {
		repo, err := a.resolveSelectorRepo(flags.Repo)
		if err != nil {
			return err
		}
		commit, err := a.resolveQueryCommit(repo, flags)
		if err != nil {
			return err
		}
		task := flags.Task
		attempt := flags.Attempt
		if task != "" {
			var view localci.CommitStatusView
			view, err = (localci.DaemonClient{Paths: runner.Paths}).Status(context.Background(), repo, commit)
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
		route, err = a.tuiRouteForTarget(spec)
		if err != nil {
			return err
		}
	}

	model := newTUIModel(client, route)
	program := tea.NewProgram(model, tea.WithAltScreen(), tea.WithMouseCellMotion())
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
	case spec.Artifact == "":
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
	default:
		attempt := spec.Attempt
		if attempt <= 0 {
			attempt = 1
		}
		base := tuiRoute{
			view:     tuiViewArtifacts,
			repoPath: canonicalPathLabel(root, spec.RepoDir),
			repoDir:  spec.RepoDir,
			commit:   spec.Commit,
			task:     spec.Task,
			attempt:  attempt,
		}
		base.apiPath = path.Join("/api/repo", base.repoPath, "commit", base.commit, "task", url.PathEscape(base.task), "attempt", strconv.Itoa(attempt), "artifact")
		return tuiArtifactRoute(base, spec.Artifact), nil
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
