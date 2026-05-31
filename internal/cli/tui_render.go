package cli

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"localci/internal/localci"
)

func (m tuiModel) View() string {
	if m.width <= 0 {
		return "localci\n"
	}
	theme := tuiTheme{}
	header := m.renderHeader(theme)
	bodyHeight := max(1, m.height-lipgloss.Height(header)-2)
	body := m.renderBody(theme, bodyHeight)
	if m.help.ShowAll {
		body = m.renderHelpModal(theme, bodyHeight)
	}
	footer := m.renderFooter(theme)
	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

func (m tuiModel) renderHeader(theme tuiTheme) string {
	crumbs := m.breadcrumbs()
	left := theme.title().Render(strings.Join(crumbs, " / "))
	status := "socket " + m.socketState
	if m.loading {
		status += " / loading"
	}
	right := theme.muted().Render(status)
	gap := max(1, m.width-lipgloss.Width(left)-lipgloss.Width(right)-1)
	line := left + strings.Repeat(" ", gap) + right
	if m.err != "" && !m.bodyHandlesError() {
		line += "\n" + theme.danger().Render(m.err)
	} else if m.notice != "" {
		line += "\n" + theme.muted().Render(m.notice)
	}
	return line
}

func (m tuiModel) breadcrumbs() []string {
	switch m.route.view {
	case tuiViewHome:
		return []string{"Home"}
	case tuiViewRepos:
		return []string{"Home", "Repo"}
	case tuiViewQueue:
		return []string{"Home", "Queue"}
	case tuiViewRepo:
		return []string{"Home", m.route.repoPath}
	case tuiViewRepoTaskHistory:
		return []string{"Home", m.route.repoPath, trimTaskLabel(m.route.task), "history"}
	case tuiViewCommit:
		return []string{"Home", m.route.repoPath, shortCommit(m.route.commit)}
	case tuiViewTask:
		crumbs := []string{"Home", m.route.repoPath, shortCommit(m.route.commit), trimTaskLabel(m.route.task)}
		if m.route.attempt > 0 {
			crumbs = append(crumbs, fmt.Sprintf("attempt %d", m.route.attempt))
		}
		return crumbs
	case tuiViewArtifacts:
		return []string{"Home", m.route.repoPath, shortCommit(m.route.commit), trimTaskLabel(m.route.task), fmt.Sprintf("attempt %d", m.route.attempt)}
	case tuiViewArtifact:
		return []string{"Home", m.route.repoPath, shortCommit(m.route.commit), trimTaskLabel(m.route.task), fmt.Sprintf("attempt %d", m.route.attempt), m.route.artifact}
	default:
		return []string{"Home"}
	}
}

func (m tuiModel) bodyHandlesError() bool {
	return m.route.view == tuiViewArtifact
}

func (m tuiModel) renderFooter(theme tuiTheme) string {
	if m.help.ShowAll {
		return theme.muted().Width(m.width).Render("?/esc/q close help  ctrl+c quit")
	}
	m.help.Width = m.width
	return theme.muted().Width(m.width).Render(m.help.View(m.keys))
}

func (m tuiModel) renderHelpModal(theme tuiTheme, height int) string {
	lines := []string{theme.title().Render("Keys")}
	for _, group := range m.keys.FullHelp() {
		lines = append(lines, "")
		for _, binding := range group {
			help := binding.Help()
			if help.Key == "" || help.Desc == "" {
				continue
			}
			lines = append(lines, fmt.Sprintf("%-10s %s", help.Key, help.Desc))
		}
	}
	body := strings.Join(lines, "\n")
	modal := renderTUIPanel(theme, min(36, max(28, m.width-4)), min(height, lipgloss.Height(body)+4), body)
	return lipgloss.Place(m.width, height, lipgloss.Center, lipgloss.Center, modal)
}

func (m tuiModel) renderBody(theme tuiTheme, height int) string {
	if !m.usesSidebar() {
		return lipgloss.NewStyle().Width(m.width).Height(height).Render(m.renderContent(theme, height))
	}
	navWidth := 18
	if m.width < 72 {
		navWidth = 14
	}
	contentWidth := max(20, m.width-navWidth-1)
	nav := m.renderNav(theme, navWidth, height)
	contentModel := m
	contentModel.width = contentWidth
	content := contentModel.renderContent(theme, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, nav, " ", lipgloss.NewStyle().Width(contentWidth).Height(height).Render(content))
}

func (m tuiModel) usesSidebar() bool {
	return m.route.view == tuiViewHome || m.route.view == tuiViewRepos || m.route.view == tuiViewQueue
}

func (m tuiModel) renderContent(theme tuiTheme, height int) string {
	switch m.route.view {
	case tuiViewHome:
		return m.renderHome(theme, height)
	case tuiViewRepos:
		return m.renderRepos(theme, height)
	case tuiViewQueue:
		return m.renderQueue(theme, height)
	case tuiViewRepo:
		return m.renderRepo(theme, height)
	case tuiViewRepoTaskHistory:
		return m.renderRepoTaskHistory(theme, height)
	case tuiViewCommit:
		return m.renderCommit(theme, height)
	case tuiViewTask:
		return m.renderTask(theme, height)
	case tuiViewArtifacts:
		return m.renderArtifacts(theme, height)
	case tuiViewArtifact:
		return m.renderArtifact(theme, height)
	default:
		return ""
	}
}

func (m tuiModel) renderNav(theme tuiTheme, width int, height int) string {
	items := []struct {
		key  string
		name string
		view tuiView
	}{
		{"h", "Home", tuiViewHome},
		{"l", "Repos", tuiViewRepos},
		{"p", "Queue", tuiViewQueue},
	}
	lines := []string{theme.title().Render("LocalCI")}
	for _, item := range items {
		label := item.key + " " + item.name
		lines = append(lines, selectableLine(theme, m.route.view == item.view, truncate(label, width-4)))
	}
	lines = append(lines, "", theme.muted().Render("socket"))
	lines = append(lines, truncate(m.socketState, width-4))
	if m.home != nil {
		lines = append(lines, "", theme.muted().Render("active"))
		if m.home.Queue.Active == nil {
			lines = append(lines, "none")
		} else {
			lines = append(lines, truncate(trimTaskLabel(m.home.Queue.Active.Task), width-4))
		}
	}
	body := strings.Join(lines, "\n")
	return renderTUIPanel(theme, width, min(height, lipgloss.Height(body)+2), body)
}

func (m tuiModel) renderHome(theme tuiTheme, height int) string {
	if m.home == nil {
		return loadingText(m.loading)
	}
	if m.width >= 96 {
		listWidth := clamp(m.width/2, 44, 64)
		detailWidth := max(24, m.width-listWidth-1)
		list := m.renderRecentRunList(theme, height, listWidth)
		detail := m.renderSelectedRunDetail(theme, height, detailWidth)
		return lipgloss.JoinHorizontal(lipgloss.Top, list, " ", detail)
	}
	return m.renderRecentRunList(theme, height, m.width)
}

func (m tuiModel) renderRecentRunList(theme tuiTheme, height int, width int) string {
	rows := []string{theme.title().Render("Recent runs")}
	if m.home != nil {
		for i, run := range visibleRuns(m.home.RecentCommits, m.scroll, height-2) {
			rows = append(rows, selectableLine(theme, m.scroll+i == m.cursor, runListLine(run, width-8)))
		}
	}
	if len(m.home.RecentCommits) == 0 {
		rows = append(rows, theme.muted().Render("No runs yet."))
	}
	return renderTUIPanel(theme, width, height, strings.Join(rows, "\n"))
}

func (m tuiModel) renderSelectedRunDetail(theme tuiTheme, height int, width int) string {
	lines := []string{theme.title().Render("Selected run")}
	if m.home == nil || len(m.home.RecentCommits) == 0 || m.cursor < 0 || m.cursor >= len(m.home.RecentCommits) {
		lines = append(lines, theme.muted().Render("No run selected."))
		body := strings.Join(lines, "\n")
		return renderTUIPanel(theme, width, min(height, lipgloss.Height(body)+2), body)
	}
	run := m.home.RecentCommits[m.cursor]
	lines = append(lines,
		truncate(run.Repo.DisplayLabel()+"  "+shortCommit(run.Commit), width-4),
		truncate(taskCounts(run.Tasks), width-4),
	)
	if !runComplete(run.Tasks) {
		bar := m.progress
		bar.Width = max(8, width-8)
		lines = append(lines, bar.ViewAs(runProgress(run.Tasks)))
	}
	if subject := run.Annotations[localci.AnnotationCommitSubject]; subject != "" {
		lines = append(lines, truncate("message: "+subject, width-4))
	}
	if branch := run.Annotations["branch"]; branch != "" {
		lines = append(lines, truncate("branch: "+branch, width-4))
	}
	lines = append(lines, "")
	for _, status := range []localci.ExecutionStatus{localci.ExecutionStatusFailed, localci.ExecutionStatusTimedOut, localci.ExecutionStatusRunning, localci.ExecutionStatusQueued, localci.ExecutionStatusSucceeded, localci.ExecutionStatusNotRun} {
		names := taskNamesByStatus(run.Tasks, status)
		if len(names) == 0 {
			continue
		}
		if status == localci.ExecutionStatusSucceeded && len(names) > 3 {
			lines = append(lines, truncate(fmt.Sprintf("ok: %d tasks", len(names)), width-4))
			continue
		}
		lines = append(lines, truncate(statusLabel(status)+": "+strings.Join(names, ", "), width-4))
	}
	body := strings.Join(lines, "\n")
	return renderTUIPanel(theme, width, min(height, lipgloss.Height(body)+2), body)
}

func (m tuiModel) renderQueue(theme tuiTheme, height int) string {
	if m.queue == nil {
		return loadingText(m.loading)
	}
	rows := []string{theme.title().Render("Queue")}
	index := 0
	if m.queue.Active != nil {
		rows = append(rows, selectableLine(theme, index == m.cursor, truncate("running  "+renderQueueLine(*m.queue.Active, false), m.width-4)))
		index++
	}
	for _, entry := range m.queue.Pending {
		rows = append(rows, selectableLine(theme, index == m.cursor, truncate("pending  "+renderQueueLine(entry, false), m.width-4)))
		index++
	}
	if index == 0 {
		rows = append(rows, theme.muted().Render("Queue is empty."))
	}
	return fitLines(rows, m.scroll, height, m.width)
}

func (m tuiModel) renderRepos(theme tuiTheme, height int) string {
	rows := []string{theme.title().Render("Repos")}
	for i, repo := range m.repos {
		if i < m.scroll || i >= m.scroll+height-2 {
			continue
		}
		rows = append(rows, selectableLine(theme, i == m.cursor, truncate(repo.DisplayLabel(), m.width-4)))
	}
	if len(m.repos) == 0 {
		rows = append(rows, theme.muted().Render("No repos yet."))
	}
	return fitLines(rows, 0, height, m.width)
}

func (m tuiModel) renderRepo(theme tuiTheme, height int) string {
	if m.repo == nil {
		return loadingText(m.loading)
	}
	rows := []string{theme.title().Render(m.repo.Repo.DisplayLabel())}
	for i, run := range visibleRuns(m.repo.Commits, m.scroll, height-2) {
		rows = append(rows, selectableLine(theme, m.scroll+i == m.cursor, runListLine(run, m.width-8)))
	}
	if len(m.repo.Commits) == 0 {
		rows = append(rows, theme.muted().Render("No runs for this repo."))
	}
	return renderTUIPanel(theme, m.width, height, strings.Join(rows, "\n"))
}

func (m tuiModel) renderRepoTaskHistory(theme tuiTheme, height int) string {
	if m.taskHistory == nil {
		return loadingText(m.loading)
	}
	rows := []string{theme.title().Render(m.taskHistory.ShortName + " history")}
	for i, run := range visibleTaskHistoryRows(m.taskHistory.Runs, m.scroll, height-2) {
		rows = append(rows, selectableLine(theme, m.scroll+i == m.cursor, taskHistoryLine(run, m.width-8)))
	}
	if len(m.taskHistory.Runs) == 0 {
		rows = append(rows, theme.muted().Render("No history recorded for this task."))
	}
	return renderTUIPanel(theme, m.width, height, strings.Join(rows, "\n"))
}

func (m tuiModel) renderCommit(theme tuiTheme, height int) string {
	if m.commit == nil {
		return loadingText(m.loading)
	}
	leftWidth := clamp(m.width/3, 28, 46)
	rightWidth := max(20, m.width-leftWidth-2)
	left := []string{theme.title().Render("Tasks")}
	for i, task := range visibleTasks(m.commit.Commit.Tasks, m.scroll, height-2) {
		line := taskSummaryLine(task.Name, task.ShortName, task.Status, task.Attempt, task.AttemptCount, task.DurationMilliseconds, task.Failure)
		left = append(left, selectableLine(theme, m.scroll+i == m.cursor, truncate(line, leftWidth-8)))
	}
	selected, ok := m.selectedCommitTask()
	right := []string{}
	if ok {
		right = append(right,
			"task: "+selected.Name,
			"status: "+statusLabel(selected.Status),
			"attempt: "+attemptLabel(selected.Attempt, selected.AttemptCount),
			"duration: "+durationLabel(selected.DurationMilliseconds),
		)
		if selected.Failure != "" {
			right = append(right, "failure: "+selected.Failure)
		}
		if len(selected.Artifacts) > 0 {
			right = append(right, "", "artifacts:")
			for _, artifact := range selected.Artifacts {
				right = append(right, "  "+artifact.DisplayName)
			}
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Top,
		renderTUIPanel(theme, leftWidth, height, strings.Join(left, "\n")),
		" ",
		lipgloss.NewStyle().Width(rightWidth).Height(height).Render(strings.Join(right, "\n")),
	)
}

func (m tuiModel) renderTask(theme tuiTheme, height int) string {
	if m.task == nil {
		return loadingText(m.loading)
	}
	top := []string{m.renderTaskStatusLine(theme, m.width)}
	if m.task.Task.Failure != "" {
		top = append(top, "failure: "+m.task.Task.Failure)
	}
	logWidth := m.width
	logHeight := max(1, height-len(top)-2)
	logViewport := m.log
	logViewport.Width = max(1, logWidth-4)
	logViewport.Height = max(1, logHeight-5)
	logViewport.SetContent(m.selectedTaskArtifactContent())
	tabLine := m.renderArtifactTabs(theme, logWidth)
	logView := ""
	if path := m.selectedTaskArtifactPath(); path != "" {
		logView += theme.muted().Render(path) + "\n\n"
	}
	if err := m.selectedTaskArtifactError(); err != "" {
		logView += theme.title().Render("Artifact unavailable") + "\n" + err + "\n\nOnly text artifacts can be viewed here.\nOpen the artifact from the output directory."
	} else if m.selectedTaskArtifactName() != "" && m.selectedTaskArtifactContent() == "" && m.selectedTaskArtifactName() != m.task.PrimaryArtifact {
		logView += theme.muted().Render("Loading artifact...")
	} else {
		logView += logViewport.View()
	}
	return strings.Join(top, "\n") + "\n" + renderTabbedTUIPanel(theme, logWidth, logHeight, tabLine, logView)
}

func (m tuiModel) renderTaskStatusLine(theme tuiTheme, width int) string {
	status := theme.statusKey().Render(strings.ToUpper(statusLabel(m.task.Task.Status)))
	task := theme.statusValue().Render(trimTaskLabel(m.task.Task.Name))
	rightText := fmt.Sprintf("attempt %s  %s", attemptLabel(m.task.Task.Attempt, m.task.Task.AttemptCount), durationLabel(m.task.Task.DurationMilliseconds))
	right := theme.statusValue().Render(rightText)
	gap := max(0, width-lipgloss.Width(status)-lipgloss.Width(task)-lipgloss.Width(right))
	return truncate(status+task+strings.Repeat(" ", gap)+right, width)
}

func (m tuiModel) renderArtifactTabs(theme tuiTheme, width int) string {
	if m.task == nil || len(m.task.Task.Artifacts) == 0 {
		return theme.muted().Render("No artifacts.")
	}
	artifacts := m.task.Task.Artifacts
	start, end := artifactTabRange(len(artifacts), m.cursor, width)
	visibleCount := max(1, end-start)
	labelWidth := max(1, (width-(visibleCount*artifactTabChromeWidth))/visibleCount)
	parts := make([]string, 0, visibleCount)
	for i := start; i < end; i++ {
		label := truncate(artifacts[i].DisplayName, labelWidth)
		if len(artifacts) > visibleCount && i == m.cursor {
			label = truncate(fmt.Sprintf("%d/%d %s", i+1, len(artifacts), artifacts[i].DisplayName), labelWidth)
		}
		if i == m.cursor {
			parts = append(parts, theme.activeTab().Render(label))
		} else {
			parts = append(parts, theme.inactiveTab().Render(label))
		}
	}
	tabs := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	fill := strings.Repeat("─", max(0, width-lipgloss.Width(tabs)))
	return tabs + theme.tabRule().Render(fill)
}

const (
	artifactTabChromeWidth = 4
	maxVisibleArtifactTabs = 4
	minArtifactTabLabel    = 8
)

func artifactTabRange(count int, cursor int, width int) (int, int) {
	if count <= 0 {
		return 0, 0
	}
	maxTabs := min(count, maxVisibleArtifactTabs)
	if width > 0 {
		maxTabs = min(maxTabs, max(1, width/(artifactTabChromeWidth+minArtifactTabLabel)))
	}
	cursor = min(max(0, cursor), count-1)
	start := cursor - maxTabs/2
	start = min(max(0, start), max(0, count-maxTabs))
	return start, start + maxTabs
}

func (m tuiModel) renderArtifacts(theme tuiTheme, height int) string {
	if m.artifacts == nil {
		return loadingText(m.loading)
	}
	rows := []string{theme.title().Render("Artifacts")}
	for i, artifact := range visibleArtifacts(m.artifacts.Artifacts, m.scroll, height-2) {
		rows = append(rows, selectableLine(theme, m.scroll+i == m.cursor, truncate(artifact.DisplayName, m.width-4)))
	}
	if len(m.artifacts.Artifacts) == 0 {
		rows = append(rows, theme.muted().Render("No artifacts."))
	}
	return fitLines(rows, 0, height, m.width)
}

func (m tuiModel) renderArtifact(theme tuiTheme, height int) string {
	if m.artifact == nil {
		if m.err != "" {
			lines := []string{
				theme.title().Render("Artifact unavailable"),
				m.err,
				"",
				"Only text artifacts can be viewed here.",
				"Open the artifact from the output directory.",
			}
			return renderTUIPanel(theme, m.width, height, strings.Join(lines, "\n"))
		}
		return loadingText(m.loading)
	}
	if !m.artifact.Artifact.IsText {
		lines := []string{theme.title().Render(m.artifact.Artifact.DisplayName)}
		if m.artifact.Artifact.Path != "" {
			lines = append(lines, theme.muted().Render(m.artifact.Artifact.Path), "")
		}
		lines = append(lines, "This artifact is not a text file.", "Use o to open it or e to edit it.")
		return renderTUIPanel(theme, m.width, height, strings.Join(lines, "\n"))
	}
	logViewport := m.artifactLog
	logViewport.Width = max(1, m.width-4)
	logViewport.Height = max(1, height-4)
	logViewport.SetContent(m.artifact.Content)
	lines := []string{theme.title().Render(m.artifact.Artifact.DisplayName)}
	if m.artifact.Artifact.Path != "" {
		lines = append(lines, theme.muted().Render(m.artifact.Artifact.Path), "")
	}
	lines = append(lines, logViewport.View())
	return renderTUIPanel(theme, m.width, height, strings.Join(lines, "\n"))
}

func (m tuiModel) renderRunSummary(theme tuiTheme, run tuiCommitSummary, selected bool) string {
	counts := taskCounts(run.Tasks)
	line := fmt.Sprintf("%s  %s  %s  %s", run.Repo.DisplayLabel(), shortCommit(run.Commit), counts, timeAgo(run.ActivityAt))
	if branch := run.Annotations["branch"]; branch != "" {
		line += "  branch:" + branch
	}
	taskLine := taskStatusGroups(run.Tasks)
	if taskLine != "" {
		line += "\n  " + taskLine
	}
	return selectableBlock(theme, selected, line)
}

func renderTUIPanel(theme tuiTheme, width int, height int, body string) string {
	return theme.panel().
		Width(max(1, width-4)).
		Height(max(1, height-2)).
		Render(body)
}

func renderTabbedTUIPanel(theme tuiTheme, width int, height int, tabs string, body string) string {
	contentHeight := max(1, height-lipgloss.Height(tabs)-1)
	content := lipgloss.NewStyle().
		PaddingLeft(2).
		Width(max(1, width-2)).
		Height(contentHeight).
		Render(body)
	return lipgloss.NewStyle().Width(width).Height(height).Render(tabs + "\n" + content)
}
