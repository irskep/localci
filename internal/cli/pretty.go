package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"localci/internal/localci"
)

var (
	cliTitleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c4b5fd"))
	cliKeyStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#a1a1aa"))
	cliHeaderStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#a78bfa"))
	cliMutedStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#71717a"))
	cliOKStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#86efac"))
	cliBadStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#f87171"))
	cliWarnStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#fbbf24"))
	cliRunStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#67e8f9"))
)

type cliTaskRow struct {
	Status   localci.ExecutionStatus
	Task     string
	Attempt  string
	Duration string
	Failure  string
	Output   string
}

func (a App) printStatusSummary(view localci.CommitStatusView, tasks []localci.TaskStatusView) {
	a.printCommitHeader("Status", view)
	if len(tasks) == 0 {
		fmt.Fprintln(a.Stdout, cliMutedStyle.Render("No tasks."))
		return
	}
	fmt.Fprintln(a.Stdout)
	printTaskRows(a.Stdout, taskRows(tasks), false)
}

func (a App) printCommitHeader(title string, view localci.CommitStatusView) {
	fmt.Fprintln(a.Stdout, cliTitleStyle.Render(title))
	rows := [][2]string{
		{"repo", view.RepoDir},
		{"commit", view.Commit},
		{"summary", localci.SummarizeCommit(view)},
	}
	if len(view.Annotations) > 0 {
		keys := make([]string, 0, len(view.Annotations))
		for key := range view.Annotations {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			rows = append(rows, [2]string{key, view.Annotations[key]})
		}
	}
	printKeyValues(a.Stdout, rows)
}

func (a App) printInvokeSummary(result localci.RunRecord) {
	if len(result.TaskResults) == 0 {
		fmt.Fprintf(a.Stdout, "No localci tasks discovered for %s at %s\n", result.RepoDir, result.Commit)
		return
	}

	view := localci.CommitStatusView{
		RepoDir:     result.RepoDir,
		Commit:      result.Commit,
		Annotations: result.Annotations,
		Tasks:       taskViewsFromRecords(result.TaskResults),
	}
	a.printCommitHeader(fmt.Sprintf("Invoked %d %s", len(result.TaskResults), pluralizeTask(len(result.TaskResults))), view)
	fmt.Fprintln(a.Stdout)
	printTaskRows(a.Stdout, taskRowsWithOutput(view.Tasks), true)
}

func printKeyValues(w io.Writer, rows [][2]string) {
	keyWidth := 0
	for _, row := range rows {
		keyWidth = max(keyWidth, lipgloss.Width(row[0]))
	}
	for _, row := range rows {
		key := padRight(row[0], keyWidth)
		fmt.Fprintf(w, "%s  %s\n", cliKeyStyle.Render(key), row[1])
	}
}

func printTaskRows(w io.Writer, rows []cliTaskRow, includeOutput bool) {
	headers := []string{"status", "task", "attempt", "duration", "failure"}
	if includeOutput {
		headers = append(headers, "output")
	}
	widths := []int{
		lipgloss.Width(headers[0]),
		lipgloss.Width(headers[1]),
		lipgloss.Width(headers[2]),
		lipgloss.Width(headers[3]),
		lipgloss.Width(headers[4]),
	}
	if includeOutput {
		widths = append(widths, lipgloss.Width(headers[5]))
	}
	for _, row := range rows {
		values := row.values(includeOutput)
		for i, value := range values {
			widths[i] = max(widths[i], lipgloss.Width(value))
		}
	}
	renderedHeaders := make([]string, len(headers))
	for i, header := range headers {
		renderedHeaders[i] = cliHeaderStyle.Render(padRight(header, widths[i]))
	}
	fmt.Fprintln(w, strings.Join(renderedHeaders, "  "))
	for _, row := range rows {
		values := row.values(includeOutput)
		rendered := make([]string, len(values))
		for i, value := range values {
			if value == "" {
				rendered[i] = cliMutedStyle.Render(padRight("-", widths[i]))
				continue
			}
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

func (r cliTaskRow) values(includeOutput bool) []string {
	values := []string{
		statusLabel(r.Status),
		r.Task,
		r.Attempt,
		r.Duration,
		r.Failure,
	}
	if includeOutput {
		values = append(values, r.Output)
	}
	return values
}

func taskRows(tasks []localci.TaskStatusView) []cliTaskRow {
	rows := make([]cliTaskRow, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, taskRow(task, false))
	}
	return rows
}

func taskRowsWithOutput(tasks []localci.TaskStatusView) []cliTaskRow {
	rows := make([]cliTaskRow, 0, len(tasks))
	for _, task := range tasks {
		rows = append(rows, taskRow(task, true))
	}
	return rows
}

func taskRow(task localci.TaskStatusView, includeOutput bool) cliTaskRow {
	name := task.ShortName
	if name == "" {
		name = trimTaskLabel(task.Name)
	}
	row := cliTaskRow{
		Status:   task.Status,
		Task:     name,
		Attempt:  attemptLabel(task.Attempt, task.AttemptCount),
		Duration: durationLabel(task.DurationMilliseconds),
		Failure:  task.Failure,
	}
	if includeOutput {
		row.Output = task.OutputDir
	}
	return row
}

func taskViewsFromRecords(records []localci.TaskRecord) []localci.TaskStatusView {
	tasks := make([]localci.TaskStatusView, 0, len(records))
	for _, record := range records {
		tasks = append(tasks, localci.TaskStatusView{
			Name:                 record.Name,
			ShortName:            record.ShortName,
			Attempt:              record.Attempt,
			AttemptCount:         1,
			Status:               executionStatusFromTaskStatus(record.Status),
			OutputDir:            record.OutputDir,
			DurationMilliseconds: record.DurationMilliseconds,
			Failure:              record.Failure,
		})
	}
	return tasks
}

func executionStatusFromTaskStatus(status localci.TaskStatus) localci.ExecutionStatus {
	switch status {
	case localci.TaskStatusSucceeded:
		return localci.ExecutionStatusSucceeded
	case localci.TaskStatusFailed:
		return localci.ExecutionStatusFailed
	case localci.TaskStatusTimedOut:
		return localci.ExecutionStatusTimedOut
	case localci.TaskStatusRunning:
		return localci.ExecutionStatusRunning
	default:
		return localci.ExecutionStatusNotRun
	}
}

func styleStatus(status localci.ExecutionStatus, value string) string {
	switch status {
	case localci.ExecutionStatusSucceeded:
		return cliOKStyle.Render(value)
	case localci.ExecutionStatusFailed:
		return cliBadStyle.Render(value)
	case localci.ExecutionStatusTimedOut:
		return cliWarnStyle.Render(value)
	case localci.ExecutionStatusRunning, localci.ExecutionStatusQueued:
		return cliRunStyle.Render(value)
	default:
		return cliMutedStyle.Render(value)
	}
}

func padRight(value string, width int) string {
	padding := width - lipgloss.Width(value)
	if padding <= 0 {
		return value
	}
	return value + strings.Repeat(" ", padding)
}
