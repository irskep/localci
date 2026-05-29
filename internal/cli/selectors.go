package cli

import (
	"fmt"
	"strings"

	"localci/internal/localci"
)

type cliFlags struct {
	Repo       string
	Commit     string
	Task       string
	Attempt    int
	Artifact   string
	NoClone    bool
	JSON       bool
	Wait       bool
	Failed     bool
	Primary    bool
	PathsOnly  bool
	Limit      int
	Statuses   []string
	Annotation map[string]string
}

func (a App) resolveSelectorRepo(repoArg string) (string, error) {
	return a.repoFromFlagOrCwd(repoArg)
}

func (a App) resolveQueryCommit(repoDir string, flags cliFlags) (string, error) {
	if flags.NoClone && strings.TrimSpace(flags.Commit) == "" {
		head, err := a.headCommit(repoDir)
		if err != nil {
			return "", err
		}
		return noCloneCommitLabel(head), nil
	}
	commit := strings.TrimSpace(flags.Commit)
	if commit == "" {
		return a.latestCommitForRepo(repoDir)
	}
	resolved, err := a.resolveCommitAlias(repoDir, commit)
	if err != nil {
		return "", err
	}
	if flags.NoClone {
		return noCloneCommitLabel(resolved), nil
	}
	return resolved, nil
}

func (a App) resolveRunCommit(repoDir string, flags cliFlags) (string, error) {
	commit := strings.TrimSpace(flags.Commit)
	if commit == "" {
		commit = "HEAD"
	}
	resolved, err := a.resolveCommitAlias(repoDir, commit)
	if err != nil {
		return "", err
	}
	if flags.NoClone {
		return noCloneCommitLabel(resolved), nil
	}
	return resolved, nil
}

func resolveTaskName(tasks []localci.TaskStatusView, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil
	}
	for _, task := range tasks {
		if task.Name == query {
			return task.Name, nil
		}
	}
	matches := []string{}
	for _, task := range tasks {
		if task.ShortName == query || trimTaskLabel(task.Name) == query {
			matches = append(matches, task.Name)
		}
	}
	return oneTaskMatch(query, matches)
}

func resolveDiscoveredTaskNames(tasks []localci.Task, query string) ([]string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		names := make([]string, 0, len(tasks))
		for _, task := range tasks {
			names = append(names, task.Name)
		}
		return names, nil
	}
	for _, task := range tasks {
		if task.Name == query {
			return []string{task.Name}, nil
		}
	}
	matches := []string{}
	for _, task := range tasks {
		if trimTaskLabel(task.Name) == query {
			matches = append(matches, task.Name)
		}
	}
	match, err := oneTaskMatch(query, matches)
	if err != nil {
		return nil, err
	}
	return []string{match}, nil
}

func oneTaskMatch(query string, matches []string) (string, error) {
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("task %q not found in selected run; use localci history --task %s to search across runs", query, shellQuote(query))
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("task %q is ambiguous; candidates: %s", query, strings.Join(matches, ", "))
	}
}

func filterStatusTasks(tasks []localci.TaskStatusView, taskName string, failedOnly bool) []localci.TaskStatusView {
	filtered := make([]localci.TaskStatusView, 0, len(tasks))
	for _, task := range tasks {
		if taskName != "" && task.Name != taskName {
			continue
		}
		if failedOnly && task.Status != localci.ExecutionStatusFailed && task.Status != localci.ExecutionStatusTimedOut {
			continue
		}
		filtered = append(filtered, task)
	}
	return filtered
}
