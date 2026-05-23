package cli

import (
	"fmt"
	"strconv"
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

type flagSpec map[string]bool

func parseCLIFlags(args []string, allowed flagSpec) (cliFlags, error) {
	flags := cliFlags{Limit: 20}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "--") {
			return flags, fmt.Errorf("unexpected positional argument %q; use explicit flags like --repo, --commit, and --task", arg)
		}
		name, value, hasValue := strings.Cut(strings.TrimPrefix(arg, "--"), "=")
		if !allowed[name] {
			return flags, fmt.Errorf("unknown or unsupported flag --%s", name)
		}
		takeValue := func() (string, error) {
			if hasValue {
				return value, nil
			}
			i++
			if i >= len(args) {
				return "", fmt.Errorf("--%s requires a value", name)
			}
			return args[i], nil
		}
		switch name {
		case "repo":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			flags.Repo = v
		case "commit":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			flags.Commit = v
		case "task":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			flags.Task = v
		case "artifact":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			flags.Artifact = v
		case "attempt":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			attempt, err := strconv.Atoi(v)
			if err != nil || attempt <= 0 {
				return flags, fmt.Errorf("--attempt must be a positive integer")
			}
			flags.Attempt = attempt
		case "limit":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			limit, err := strconv.Atoi(v)
			if err != nil || limit <= 0 {
				return flags, fmt.Errorf("--limit must be a positive integer")
			}
			flags.Limit = limit
		case "status":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			flags.Statuses = append(flags.Statuses, v)
		case "annotation":
			v, err := takeValue()
			if err != nil {
				return flags, err
			}
			key, annotationValue, ok := strings.Cut(v, "=")
			key = strings.TrimSpace(key)
			if !ok || key == "" {
				return flags, fmt.Errorf("--annotation requires key=value")
			}
			if flags.Annotation == nil {
				flags.Annotation = map[string]string{}
			}
			flags.Annotation[key] = annotationValue
		case "no-clone":
			if hasValue {
				return flags, fmt.Errorf("--no-clone does not take a value")
			}
			flags.NoClone = true
		case "json":
			if hasValue {
				return flags, fmt.Errorf("--json does not take a value")
			}
			flags.JSON = true
		case "wait":
			if hasValue {
				return flags, fmt.Errorf("--wait does not take a value")
			}
			flags.Wait = true
		case "failed":
			if hasValue {
				return flags, fmt.Errorf("--failed does not take a value")
			}
			flags.Failed = true
		case "primary":
			if hasValue {
				return flags, fmt.Errorf("--primary does not take a value")
			}
			flags.Primary = true
		case "paths-only":
			if hasValue {
				return flags, fmt.Errorf("--paths-only does not take a value")
			}
			flags.PathsOnly = true
		}
	}
	return flags, nil
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
