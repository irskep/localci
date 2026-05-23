package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"localci/internal/localci"
)

type cliArtifactRow struct {
	Task      string                  `json:"task"`
	ShortTask string                  `json:"short_task"`
	Status    localci.ExecutionStatus `json:"status"`
	Attempt   int                     `json:"attempt"`
	Artifact  string                  `json:"artifact"`
	Path      string                  `json:"path"`
	Primary   bool                    `json:"primary"`
}

type cliArtifactsOutput struct {
	Repo      string           `json:"repo"`
	Commit    string           `json:"commit"`
	Artifacts []cliArtifactRow `json:"artifacts"`
}

func (a App) runArtifacts(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "attempt": true, "failed": true, "primary": true, "paths-only": true, "json": true,
	})
	if err != nil {
		return err
	}
	if flags.JSON && flags.PathsOnly {
		return fmt.Errorf("--json and --paths-only cannot be used together")
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
	view, err := client.Status(context.Background(), repo, commit)
	if err != nil {
		return err
	}
	taskName, err := resolveTaskName(view.Tasks, flags.Task)
	if err != nil {
		return err
	}
	tasks := filterStatusTasks(view.Tasks, taskName, flags.Failed)
	rows := artifactRows(runner.Paths, repo, commit, tasks, flags)
	output := cliArtifactsOutput{Repo: repo, Commit: commit, Artifacts: rows}
	if flags.JSON {
		return writeJSON(a.Stdout, output)
	}
	if flags.PathsOnly {
		for _, row := range rows {
			fmt.Fprintln(a.Stdout, row.Path)
		}
		return nil
	}
	printArtifactsOutput(a.Stdout, output)
	return nil
}

func artifactRows(paths localci.Paths, repo string, commit string, tasks []localci.TaskStatusView, flags cliFlags) []cliArtifactRow {
	rows := []cliArtifactRow{}
	for _, task := range tasks {
		task = localci.ApplySelectedAttempt(paths, repo, commit, task, flags.Attempt)
		primaryArtifact, hasPrimary := localci.PrimaryArtifact(task)
		artifacts := task.Artifacts
		if flags.Primary {
			if !hasPrimary {
				continue
			}
			artifacts = []localci.ArtifactView{primaryArtifact}
		}
		for _, artifact := range artifacts {
			rows = append(rows, cliArtifactRow{
				Task:      task.Name,
				ShortTask: task.ShortName,
				Status:    task.Status,
				Attempt:   task.Attempt,
				Artifact:  artifact.DisplayName,
				Path:      artifact.Path,
				Primary:   hasPrimary && artifact.DisplayName == primaryArtifact.DisplayName,
			})
		}
	}
	return rows
}

func printArtifactsOutput(w io.Writer, output cliArtifactsOutput) {
	fmt.Fprintln(w, cliTitleStyle.Render("Artifacts"))
	printKeyValues(w, [][2]string{
		{"repo", output.Repo},
		{"commit", output.Commit},
	})
	if len(output.Artifacts) == 0 {
		fmt.Fprintln(w, cliMutedStyle.Render("No artifacts."))
		return
	}
	fmt.Fprintln(w)
	headers := []string{"status", "task", "attempt", "artifact", "path"}
	widths := []int{len(headers[0]), len(headers[1]), len(headers[2]), len(headers[3]), len(headers[4])}
	for _, row := range output.Artifacts {
		values := artifactRowValues(row)
		for i, value := range values {
			widths[i] = max(widths[i], len(value))
		}
	}
	renderedHeaders := make([]string, len(headers))
	for i, header := range headers {
		renderedHeaders[i] = cliHeaderStyle.Render(padRight(header, widths[i]))
	}
	fmt.Fprintln(w, strings.Join(renderedHeaders, "  "))
	for _, row := range output.Artifacts {
		values := artifactRowValues(row)
		rendered := make([]string, len(values))
		for i, value := range values {
			value = padRight(value, widths[i])
			if i == 0 {
				rendered[i] = styleStatus(row.Status, value)
			} else {
				rendered[i] = value
			}
		}
		fmt.Fprintln(w, strings.Join(rendered, "  "))
	}
}

func artifactRowValues(row cliArtifactRow) []string {
	return []string{
		statusLabel(row.Status),
		row.ShortTask,
		fmt.Sprintf("%d", row.Attempt),
		row.Artifact,
		row.Path,
	}
}

type cliHistoryRow struct {
	Commit    string                  `json:"commit"`
	Task      string                  `json:"task,omitempty"`
	ShortTask string                  `json:"short_task,omitempty"`
	Status    localci.ExecutionStatus `json:"status,omitempty"`
	Attempt   int                     `json:"attempt,omitempty"`
	Duration  string                  `json:"duration,omitempty"`
	Failure   string                  `json:"failure,omitempty"`
	Summary   string                  `json:"summary,omitempty"`
	Updated   string                  `json:"updated"`
}

type cliHistoryOutput struct {
	Repo    string          `json:"repo"`
	Task    string          `json:"task,omitempty"`
	History []cliHistoryRow `json:"history"`
}

func (a App) runHistory(args []string) error {
	flags, err := parseCLIFlags(args, flagSpec{
		"repo": true, "commit": true, "task": true, "status": true, "failed": true, "limit": true, "json": true,
	})
	if err != nil {
		return err
	}
	repo, err := a.resolveSelectorRepo(flags.Repo)
	if err != nil {
		return err
	}
	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	commits, err := (localci.HistoryReader{Paths: runner.Paths}).ListRepoCommits(repo)
	if err != nil {
		return err
	}
	statuses := selectedHistoryStatuses(flags)
	rows := make([]cliHistoryRow, 0, min(flags.Limit, len(commits)))
	for _, run := range commits {
		if flags.Commit != "" && run.Commit != flags.Commit {
			continue
		}
		view, err := localci.BuildCommitStatusView(runner.Paths, repo, run.Commit, run.DiscoveredTasks, nil, nil)
		if err != nil {
			return err
		}
		if flags.Task == "" {
			if !runMatchesStatuses(view.Tasks, statuses) {
				continue
			}
			rows = append(rows, cliHistoryRow{
				Commit:  run.Commit,
				Summary: localci.SummarizeCommit(view),
				Updated: timeAgo(localci.RunActivityAt(run)),
			})
		} else {
			taskName, err := resolveTaskName(view.Tasks, flags.Task)
			if err != nil {
				continue
			}
			for _, task := range filterStatusTasks(view.Tasks, taskName, false) {
				if !statusSelected(task.Status, statuses) {
					continue
				}
				rows = append(rows, cliHistoryRow{
					Commit:    run.Commit,
					Task:      task.Name,
					ShortTask: task.ShortName,
					Status:    task.Status,
					Attempt:   task.Attempt,
					Duration:  durationLabel(task.DurationMilliseconds),
					Failure:   task.Failure,
					Updated:   timeAgo(localci.RunActivityAt(run)),
				})
			}
		}
		if len(rows) >= flags.Limit {
			break
		}
	}
	output := cliHistoryOutput{Repo: repo, Task: flags.Task, History: rows}
	if flags.JSON {
		return writeJSON(a.Stdout, output)
	}
	printHistoryOutput(a.Stdout, output)
	return nil
}

func selectedHistoryStatuses(flags cliFlags) map[localci.ExecutionStatus]bool {
	statuses := map[localci.ExecutionStatus]bool{}
	if flags.Failed {
		statuses[localci.ExecutionStatusFailed] = true
		statuses[localci.ExecutionStatusTimedOut] = true
	}
	for _, raw := range flags.Statuses {
		switch strings.TrimSpace(raw) {
		case "ok", "succeeded", "passed":
			statuses[localci.ExecutionStatusSucceeded] = true
		case "failed":
			statuses[localci.ExecutionStatusFailed] = true
		case "timed-out", "timeout":
			statuses[localci.ExecutionStatusTimedOut] = true
		case "running":
			statuses[localci.ExecutionStatusRunning] = true
		case "queued":
			statuses[localci.ExecutionStatusQueued] = true
		case "not-run":
			statuses[localci.ExecutionStatusNotRun] = true
		}
	}
	return statuses
}

func runMatchesStatuses(tasks []localci.TaskStatusView, statuses map[localci.ExecutionStatus]bool) bool {
	if len(statuses) == 0 {
		return true
	}
	for _, task := range tasks {
		if statuses[task.Status] {
			return true
		}
	}
	return false
}

func statusSelected(status localci.ExecutionStatus, statuses map[localci.ExecutionStatus]bool) bool {
	return len(statuses) == 0 || statuses[status]
}

func printHistoryOutput(w io.Writer, output cliHistoryOutput) {
	fmt.Fprintln(w, cliTitleStyle.Render("History"))
	rows := [][2]string{{"repo", output.Repo}}
	if output.Task != "" {
		rows = append(rows, [2]string{"task", output.Task})
	}
	printKeyValues(w, rows)
	if len(output.History) == 0 {
		fmt.Fprintln(w, cliMutedStyle.Render("No history."))
		return
	}
	fmt.Fprintln(w)
	if output.Task == "" {
		printHistoryRunRows(w, output.History)
		return
	}
	printHistoryTaskRows(w, output.History)
}

func printHistoryRunRows(w io.Writer, rows []cliHistoryRow) {
	headers := []string{"commit", "summary", "updated"}
	widths := []int{len(headers[0]), len(headers[1]), len(headers[2])}
	for _, row := range rows {
		values := []string{shortCommit(row.Commit), row.Summary, row.Updated}
		for i, value := range values {
			widths[i] = max(widths[i], len(value))
		}
	}
	fmt.Fprintln(w, cliHeaderStyle.Render(padRight(headers[0], widths[0]))+"  "+cliHeaderStyle.Render(padRight(headers[1], widths[1]))+"  "+cliHeaderStyle.Render(padRight(headers[2], widths[2])))
	for _, row := range rows {
		fmt.Fprintf(w, "%s  %s  %s\n", padRight(shortCommit(row.Commit), widths[0]), padRight(row.Summary, widths[1]), padRight(row.Updated, widths[2]))
	}
}

func printHistoryTaskRows(w io.Writer, rows []cliHistoryRow) {
	headers := []string{"commit", "status", "attempt", "duration", "failure", "updated"}
	widths := []int{len(headers[0]), len(headers[1]), len(headers[2]), len(headers[3]), len(headers[4]), len(headers[5])}
	for _, row := range rows {
		values := historyTaskValues(row)
		for i, value := range values {
			widths[i] = max(widths[i], len(value))
		}
	}
	renderedHeaders := make([]string, len(headers))
	for i, header := range headers {
		renderedHeaders[i] = cliHeaderStyle.Render(padRight(header, widths[i]))
	}
	fmt.Fprintln(w, strings.Join(renderedHeaders, "  "))
	for _, row := range rows {
		values := historyTaskValues(row)
		rendered := make([]string, len(values))
		for i, value := range values {
			value = padRight(value, widths[i])
			if i == 1 {
				rendered[i] = styleStatus(row.Status, value)
			} else {
				rendered[i] = value
			}
		}
		fmt.Fprintln(w, strings.Join(rendered, "  "))
	}
}

func historyTaskValues(row cliHistoryRow) []string {
	failure := row.Failure
	if failure == "" {
		failure = "-"
	}
	return []string{shortCommit(row.Commit), statusLabel(row.Status), fmt.Sprintf("%d", row.Attempt), row.Duration, failure, row.Updated}
}
