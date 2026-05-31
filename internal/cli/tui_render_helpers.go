package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"localci/internal/localci"
)

func runListLine(run tuiCommitSummary, width int) string {
	mark := "·"
	if hasTaskStatus(run.Tasks, localci.ExecutionStatusFailed) || hasTaskStatus(run.Tasks, localci.ExecutionStatusTimedOut) {
		mark = "x"
	} else if hasTaskStatus(run.Tasks, localci.ExecutionStatusRunning) || hasTaskStatus(run.Tasks, localci.ExecutionStatusQueued) {
		mark = ">"
	} else if hasTaskStatus(run.Tasks, localci.ExecutionStatusSucceeded) {
		mark = "✓"
	}
	line := fmt.Sprintf("%s %s  %s  %s  %s", mark, run.Repo.DisplayLabel(), shortCommit(run.Commit), taskCountsShort(run.Tasks), timeAgo(run.ActivityAt))
	if subject := run.Annotations[localci.AnnotationCommitSubject]; subject != "" && width >= 64 {
		line += "  " + subject
	}
	if branch := run.Annotations["branch"]; branch != "" && width >= 72 {
		line += "  " + branch
	}
	return truncate(line, width)
}

func taskHistoryLine(run tuiRepoTaskHistoryItem, width int) string {
	subject := run.Annotations[localci.AnnotationCommitSubject]
	if subject == "" {
		subject = "No commit subject"
	}
	line := fmt.Sprintf(
		"%s  %s  %s  attempt %s  %s  %s",
		shortCommit(run.Commit),
		subject,
		statusLabel(run.Task.Status),
		attemptLabel(run.Task.Attempt, run.Task.AttemptCount),
		durationLabel(run.Task.DurationMilliseconds),
		timeAgo(run.ActivityAt),
	)
	if run.Task.Failure != "" {
		line += "  " + run.Task.Failure
	}
	return truncate(line, width)
}

func taskCountsShort(tasks []tuiTaskSummary) string {
	counts := map[localci.ExecutionStatus]int{}
	for _, task := range tasks {
		counts[task.Status]++
	}
	labels := map[localci.ExecutionStatus]string{
		localci.ExecutionStatusFailed:    "fail",
		localci.ExecutionStatusTimedOut:  "timeout",
		localci.ExecutionStatusRunning:   "run",
		localci.ExecutionStatusQueued:    "queue",
		localci.ExecutionStatusSucceeded: "ok",
		localci.ExecutionStatusNotRun:    "skip",
	}
	parts := []string{}
	for _, status := range []localci.ExecutionStatus{localci.ExecutionStatusFailed, localci.ExecutionStatusTimedOut, localci.ExecutionStatusRunning, localci.ExecutionStatusQueued, localci.ExecutionStatusSucceeded, localci.ExecutionStatusNotRun} {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], labels[status]))
		}
	}
	if len(parts) == 0 {
		return "no tasks"
	}
	return strings.Join(parts, ", ")
}

func hasTaskStatus(tasks []tuiTaskSummary, status localci.ExecutionStatus) bool {
	for _, task := range tasks {
		if task.Status == status {
			return true
		}
	}
	return false
}

func taskNamesByStatus(tasks []tuiTaskSummary, status localci.ExecutionStatus) []string {
	names := []string{}
	for _, task := range tasks {
		if task.Status != status {
			continue
		}
		name := task.ShortName
		if name == "" {
			name = trimTaskLabel(task.Name)
		}
		names = append(names, name)
	}
	return names
}

func renderQueueLine(entry tuiQueueEntry, compact bool) string {
	task := trimTaskLabel(entry.Task)
	if compact {
		return fmt.Sprintf("%s %s", entry.Repo.DisplayLabel(), task)
	}
	return fmt.Sprintf("%s  %s  %s  attempt %d", entry.Repo.DisplayLabel(), shortCommit(entry.Commit), task, entry.Attempt)
}

func taskSummaryLine(name string, shortName string, status localci.ExecutionStatus, attempt int, attemptCount int, duration int64, failure string) string {
	label := shortName
	if label == "" {
		label = trimTaskLabel(name)
	}
	parts := []string{statusMark(status), label}
	if attemptCount > 1 && attempt > 0 {
		parts = append(parts, "a"+attemptLabel(attempt, attemptCount))
	}
	if duration > 0 {
		parts = append(parts, durationLabel(duration))
	}
	if failure != "" {
		parts = append(parts, failure)
	}
	return strings.Join(parts, "  ")
}

func statusMark(status localci.ExecutionStatus) string {
	switch status {
	case localci.ExecutionStatusSucceeded:
		return "ok"
	case localci.ExecutionStatusFailed:
		return "x"
	case localci.ExecutionStatusTimedOut:
		return "!"
	case localci.ExecutionStatusRunning:
		return ">"
	case localci.ExecutionStatusQueued:
		return "+"
	default:
		return "-"
	}
}

func statusLabel(status localci.ExecutionStatus) string {
	switch status {
	case localci.ExecutionStatusSucceeded:
		return "ok"
	case localci.ExecutionStatusFailed:
		return "failed"
	case localci.ExecutionStatusTimedOut:
		return "timed-out"
	case localci.ExecutionStatusRunning:
		return "running"
	case localci.ExecutionStatusQueued:
		return "queued"
	default:
		return "not-run"
	}
}

func taskCounts(tasks []tuiTaskSummary) string {
	counts := map[localci.ExecutionStatus]int{}
	for _, task := range tasks {
		counts[task.Status]++
	}
	parts := []string{}
	for _, status := range []localci.ExecutionStatus{localci.ExecutionStatusFailed, localci.ExecutionStatusTimedOut, localci.ExecutionStatusRunning, localci.ExecutionStatusQueued, localci.ExecutionStatusSucceeded, localci.ExecutionStatusNotRun} {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], statusLabel(status)))
		}
	}
	if len(parts) == 0 {
		return "no tasks"
	}
	return strings.Join(parts, ", ")
}

func runComplete(tasks []tuiTaskSummary) bool {
	if len(tasks) == 0 {
		return true
	}
	for _, task := range tasks {
		if task.Status == localci.ExecutionStatusRunning || task.Status == localci.ExecutionStatusQueued || task.Status == localci.ExecutionStatusNotRun {
			return false
		}
	}
	return true
}

func runProgress(tasks []tuiTaskSummary) float64 {
	if len(tasks) == 0 {
		return 1
	}
	complete := 0
	for _, task := range tasks {
		if task.Status != localci.ExecutionStatusRunning && task.Status != localci.ExecutionStatusQueued && task.Status != localci.ExecutionStatusNotRun {
			complete++
		}
	}
	return float64(complete) / float64(len(tasks))
}

func taskStatusGroups(tasks []tuiTaskSummary) string {
	groups := map[localci.ExecutionStatus][]string{}
	for _, task := range tasks {
		groups[task.Status] = append(groups[task.Status], task.ShortName)
	}
	parts := []string{}
	for _, status := range []localci.ExecutionStatus{localci.ExecutionStatusFailed, localci.ExecutionStatusTimedOut, localci.ExecutionStatusRunning, localci.ExecutionStatusQueued, localci.ExecutionStatusSucceeded, localci.ExecutionStatusNotRun} {
		if len(groups[status]) > 0 {
			parts = append(parts, statusLabel(status)+": "+strings.Join(groups[status], ", "))
		}
	}
	return strings.Join(parts, " / ")
}

func selectableLine(theme tuiTheme, selected bool, line string) string {
	if selected {
		return theme.selected().Render("> " + line)
	}
	return "  " + line
}

func selectableBlock(theme tuiTheme, selected bool, block string) string {
	if selected {
		lines := splitLines(block)
		for i := range lines {
			lines[i] = theme.selected().Render("> " + lines[i])
		}
		return strings.Join(lines, "\n")
	}
	return "  " + strings.ReplaceAll(block, "\n", "\n  ")
}

func visibleRuns(items []tuiCommitSummary, start int, height int) []tuiCommitSummary {
	if start < 0 {
		start = 0
	}
	end := min(len(items), start+max(1, height))
	if start > end {
		start = end
	}
	return items[start:end]
}

func visibleTaskHistoryRows(items []tuiRepoTaskHistoryItem, start int, height int) []tuiRepoTaskHistoryItem {
	if start < 0 {
		start = 0
	}
	end := min(len(items), start+max(1, height))
	if start > end {
		start = end
	}
	return items[start:end]
}

func visibleTasks(items []localci.TaskStatusView, start int, height int) []localci.TaskStatusView {
	if start < 0 {
		start = 0
	}
	end := min(len(items), start+max(1, height))
	if start > end {
		start = end
	}
	return items[start:end]
}

func visibleArtifacts(items []localci.ArtifactView, start int, height int) []localci.ArtifactView {
	if start < 0 {
		start = 0
	}
	end := min(len(items), start+max(1, height))
	if start > end {
		start = end
	}
	return items[start:end]
}

func fitLines(lines []string, scroll int, height int, width int) string {
	if scroll < 0 {
		scroll = 0
	}
	flat := []string{}
	for _, line := range lines {
		for _, part := range splitLines(line) {
			flat = append(flat, truncate(part, width))
		}
	}
	end := min(len(flat), scroll+max(1, height))
	if scroll > end {
		scroll = end
	}
	return strings.Join(flat[scroll:end], "\n")
}

func splitLines(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSuffix(text, "\n")
	if text == "" {
		return []string{}
	}
	return strings.Split(text, "\n")
}

func truncate(value string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(value) <= width {
		return value
	}
	if width <= 1 {
		return value[:min(len(value), width)]
	}
	runes := []rune(value)
	for lipgloss.Width(string(runes)) > width && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return string(runes)
}

func pageSize(height int) int {
	return max(5, height-6)
}

func loadingText(loading bool) string {
	if loading {
		return "Loading..."
	}
	return ""
}

func shortCommit(commit string) string {
	suffix := ""
	commitHash := commit
	if strings.HasSuffix(commit, "*") {
		suffix = "*"
		commitHash = strings.TrimSuffix(commit, "*")
	}
	if len(commitHash) > 12 {
		commitHash = commitHash[:12]
	}
	return commitHash + suffix
}

func trimTaskLabel(name string) string {
	if strings.HasPrefix(name, "//:localci:") {
		return strings.TrimPrefix(name, "//:localci:")
	}
	if strings.HasPrefix(name, "localci:") {
		return strings.TrimPrefix(name, "localci:")
	}
	return strings.Replace(name, ":localci:", ":", 1)
}

func attemptLabel(attempt int, count int) string {
	if attempt <= 0 {
		return "-"
	}
	if count > attempt {
		return fmt.Sprintf("%d/%d", attempt, count)
	}
	return fmt.Sprintf("%d", attempt)
}

func durationLabel(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	if ms < 1000 {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", float64(ms)/1000)
}

func timeAgo(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	d := time.Since(t)
	if d < time.Minute {
		return "now"
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	}
	return fmt.Sprintf("%dd ago", int(d.Hours()/24))
}

func clamp(value int, low int, high int) int {
	return min(max(value, low), high)
}
