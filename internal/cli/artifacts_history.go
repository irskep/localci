package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"localci/internal/localci"
)

type cliArtifactRow struct {
	Task      string                  `json:"task"`
	ShortTask string                  `json:"short_task"`
	Status    localci.ExecutionStatus `json:"status"`
	Attempt   int                     `json:"attempt"`
	Artifact  string                  `json:"artifact"`
	Label     string                  `json:"label,omitempty"`
	Action    localci.ArtifactAction  `json:"action,omitempty"`
	Path      string                  `json:"path"`
	Primary   bool                    `json:"primary"`
}

type cliArtifactsOutput struct {
	Repo      string           `json:"repo"`
	Commit    string           `json:"commit"`
	Artifacts []cliArtifactRow `json:"artifacts"`
}

type cliCatArtifactResponse struct {
	Artifact localci.ArtifactView `json:"artifact"`
	Content  string               `json:"content"`
}

type cliCatCandidate struct {
	Task     localci.TaskStatusView
	Artifact localci.ArtifactView
	Primary  bool
}

func (a App) runArtifacts(flags cliFlags) error {
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

func (a App) runCat(flags cliFlags) error {
	if flags.Artifact != "" && flags.Primary {
		return fmt.Errorf("[artifact-name] and --primary cannot be used together")
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
	candidate, err := selectCatArtifact(runner.Paths, repo, commit, tasks, flags)
	if err != nil {
		return err
	}
	state, err := client.Ping(context.Background())
	if err != nil {
		return err
	}
	if strings.TrimSpace(state.HTTPBaseURL) == "" {
		return fmt.Errorf("daemon did not report an HTTP URL")
	}
	if flags.Raw {
		return streamRawCatArtifact(a.Stdout, state.HTTPBaseURL, repo, commit, candidate.Task, candidate.Artifact)
	}
	return printTextCatArtifact(a.Stdout, state.HTTPBaseURL, repo, commit, candidate.Task, candidate.Artifact)
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
				Label:     artifact.MarkedName,
				Action:    artifact.Action,
				Path:      artifact.Path,
				Primary:   hasPrimary && artifact.DisplayName == primaryArtifact.DisplayName,
			})
		}
	}
	return rows
}

func selectCatArtifact(paths localci.Paths, repo string, commit string, tasks []localci.TaskStatusView, flags cliFlags) (cliCatCandidate, error) {
	matches := catArtifactCandidates(paths, repo, commit, tasks, flags)
	switch len(matches) {
	case 0:
		if flags.Artifact != "" {
			return cliCatCandidate{}, fmt.Errorf("artifact %q not found in selected run", flags.Artifact)
		}
		return cliCatCandidate{}, fmt.Errorf("primary artifact not found in selected run")
	case 1:
		return matches[0], nil
	default:
		return cliCatCandidate{}, fmt.Errorf("artifact selection is ambiguous; candidates: %s; add --task or an artifact name to narrow the selection", catCandidateLabels(matches))
	}
}

func catArtifactCandidates(paths localci.Paths, repo string, commit string, tasks []localci.TaskStatusView, flags cliFlags) []cliCatCandidate {
	candidates := []cliCatCandidate{}
	for _, task := range tasks {
		task = localci.ApplySelectedAttempt(paths, repo, commit, task, flags.Attempt)
		primaryArtifact, hasPrimary := localci.PrimaryArtifact(task)
		if flags.Artifact == "" {
			if hasPrimary {
				candidates = append(candidates, cliCatCandidate{Task: task, Artifact: primaryArtifact, Primary: true})
			}
			continue
		}
		for _, artifact := range task.Artifacts {
			if artifact.DisplayName == flags.Artifact {
				candidates = append(candidates, cliCatCandidate{
					Task:     task,
					Artifact: artifact,
					Primary:  hasPrimary && artifact.DisplayName == primaryArtifact.DisplayName,
				})
			}
		}
	}
	return candidates
}

func catCandidateLabels(candidates []cliCatCandidate) string {
	labels := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		labels = append(labels, fmt.Sprintf("%s attempt %d %s", candidate.Task.ShortName, candidate.Task.Attempt, candidate.Artifact.DisplayName))
	}
	return strings.Join(labels, ", ")
}

func printTextCatArtifact(w io.Writer, baseURL string, repo string, commit string, task localci.TaskStatusView, artifact localci.ArtifactView) error {
	apiPath, err := localci.ArtifactRoutePath(repo, commit, task.Name, task.Attempt, artifact.DisplayName)
	if err != nil {
		return err
	}
	var response cliCatArtifactResponse
	if err := getDaemonJSON(baseURL, "/api"+apiPath, &response); err != nil {
		return err
	}
	if !response.Artifact.IsText {
		return fmt.Errorf("artifact %q is not displayable text; pass --raw to print raw bytes", artifact.DisplayName)
	}
	_, err = io.WriteString(w, response.Content)
	return err
}

func streamRawCatArtifact(w io.Writer, baseURL string, repo string, commit string, task localci.TaskStatusView, artifact localci.ArtifactView) error {
	rawPath, err := localci.RawArtifactRoutePath(repo, commit, task.Name, task.Attempt, artifact.DisplayName)
	if err != nil {
		return err
	}
	resp, err := http.Get(daemonURL(baseURL, rawPath))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artifact request failed: %s", resp.Status)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}

func getDaemonJSON(baseURL string, apiPath string, target any) error {
	resp, err := http.Get(daemonURL(baseURL, apiPath))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("artifact request failed: %s", resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(target)
}

func daemonURL(baseURL string, routePath string) string {
	return strings.TrimRight(baseURL, "/") + routePath
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
	headers := []string{"status", "task", "attempt", "artifact", "action", "path"}
	widths := []int{len(headers[0]), len(headers[1]), len(headers[2]), len(headers[3]), len(headers[4]), len(headers[5])}
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
		artifactLabel(row),
		string(row.Action),
		row.Path,
	}
}

func artifactLabel(row cliArtifactRow) string {
	if row.Label != "" {
		return row.Label + " (" + row.Artifact + ")"
	}
	return row.Artifact
}

type cliHistoryRow struct {
	Commit       string                  `json:"commit"`
	Annotations  map[string]string       `json:"annotations,omitempty"`
	Task         string                  `json:"task,omitempty"`
	ShortTask    string                  `json:"short_task,omitempty"`
	Status       localci.ExecutionStatus `json:"status,omitempty"`
	Attempt      int                     `json:"attempt,omitempty"`
	AttemptCount int                     `json:"attempt_count,omitempty"`
	Duration     string                  `json:"duration,omitempty"`
	Failure      string                  `json:"failure,omitempty"`
	Summary      string                  `json:"summary,omitempty"`
	Updated      string                  `json:"updated"`
}

type cliHistoryOutput struct {
	Repo    string          `json:"repo"`
	Task    string          `json:"task,omitempty"`
	History []cliHistoryRow `json:"history"`
}

func (a App) runHistory(flags cliFlags) error {
	repo, err := a.resolveSelectorRepo(flags.Repo)
	if err != nil {
		return err
	}
	commitFilter := ""
	if flags.Commit != "" {
		commitFilter, err = a.resolveCommitAlias(repo, flags.Commit)
		if err != nil {
			return err
		}
	}
	runner, err := a.newRunner()
	if err != nil {
		return err
	}
	commits, err := (localci.HistoryReader{Paths: runner.Paths}).ListRepoCommits(repo)
	if err != nil {
		return err
	}
	statuses, err := selectedHistoryStatuses(flags)
	if err != nil {
		return err
	}
	rows := make([]cliHistoryRow, 0, min(flags.Limit, len(commits)))
	for _, run := range commits {
		if commitFilter != "" && run.Commit != commitFilter {
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
				Commit:      run.Commit,
				Annotations: cloneStringMap(run.Annotations),
				Summary:     localci.SummarizeCommit(view),
				Updated:     timeAgo(localci.RunActivityAt(run)),
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
					Commit:       run.Commit,
					Annotations:  cloneStringMap(run.Annotations),
					Task:         task.Name,
					ShortTask:    task.ShortName,
					Status:       task.Status,
					Attempt:      task.Attempt,
					AttemptCount: task.AttemptCount,
					Duration:     durationLabel(task.DurationMilliseconds),
					Failure:      task.Failure,
					Updated:      timeAgo(localci.RunActivityAt(run)),
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

func selectedHistoryStatuses(flags cliFlags) (map[localci.ExecutionStatus]bool, error) {
	statuses := map[localci.ExecutionStatus]bool{}
	if flags.Failed {
		statuses[localci.ExecutionStatusFailed] = true
		statuses[localci.ExecutionStatusTimedOut] = true
	}
	for _, raw := range flags.Statuses {
		status := strings.TrimSpace(raw)
		switch status {
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
		default:
			return nil, fmt.Errorf("unknown status %q", status)
		}
	}
	return statuses, nil
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
	headers := []string{"commit", "message", "status", "attempt", "duration", "failure", "updated"}
	widths := []int{len(headers[0]), len(headers[1]), len(headers[2]), len(headers[3]), len(headers[4]), len(headers[5]), len(headers[6])}
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
			if i == 2 {
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
	subject := row.Annotations[localci.AnnotationCommitSubject]
	if subject == "" {
		subject = "-"
	}
	return []string{shortCommit(row.Commit), subject, statusLabel(row.Status), attemptLabel(row.Attempt, row.AttemptCount), row.Duration, failure, row.Updated}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}
