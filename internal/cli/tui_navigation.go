package cli

func (m *tuiModel) moveCursor(delta int) {
	maxCursor := m.maxCursor()
	m.cursor += delta
	if m.cursor < 0 {
		m.cursor = 0
	}
	if m.cursor > maxCursor {
		m.cursor = maxCursor
	}
	visible := pageSize(m.height)
	if m.cursor < m.scroll {
		m.scroll = m.cursor
	}
	if m.cursor >= m.scroll+visible {
		m.scroll = m.cursor - visible + 1
	}
}

func (m *tuiModel) moveTaskArtifact(delta int) bool {
	if m.task == nil || len(m.task.Task.Artifacts) == 0 {
		return false
	}
	next := m.cursor + delta
	if next < 0 {
		next = len(m.task.Task.Artifacts) - 1
	}
	if next >= len(m.task.Task.Artifacts) {
		next = 0
	}
	if next == m.cursor {
		return false
	}
	m.cursor = next
	m.scroll = 0
	m.taskArtifact = nil
	m.taskArtifactErr = ""
	m.log.GotoTop()
	return true
}

func (m tuiModel) maxCursor() int {
	switch m.route.view {
	case tuiViewHome:
		if m.home == nil {
			return 0
		}
		return max(0, len(m.home.RecentCommits)-1)
	case tuiViewRepos:
		return max(0, len(m.repos)-1)
	case tuiViewQueue:
		if m.queue == nil {
			return 0
		}
		count := len(m.queue.Pending)
		if m.queue.Active != nil {
			count++
		}
		return max(0, count-1)
	case tuiViewRepo:
		if m.repo == nil {
			return 0
		}
		return max(0, len(m.repo.Commits)-1)
	case tuiViewRepoTaskHistory:
		if m.taskHistory == nil {
			return 0
		}
		return max(0, len(m.taskHistory.Runs)-1)
	case tuiViewCommit:
		if m.commit == nil {
			return 0
		}
		return max(0, len(m.commit.Commit.Tasks)-1)
	case tuiViewTask:
		if m.task == nil {
			return 0
		}
		return max(0, len(m.task.Task.Artifacts)-1)
	case tuiViewArtifacts:
		if m.artifacts == nil {
			return 0
		}
		return max(0, len(m.artifacts.Artifacts)-1)
	}
	return 0
}

func (m *tuiModel) clampSelection() {
	if m.cursor > m.maxCursor() {
		m.cursor = m.maxCursor()
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m tuiModel) gotoRoute(route tuiRoute, push bool) tuiModel {
	if push && m.route.apiPath != "" && m.route.apiPath != route.apiPath {
		m.back = append(m.back, m.route)
	}
	m.route = route
	m.cursor = 0
	m.secondary = 0
	m.scroll = 0
	m.loading = true
	m.err = ""
	m.notice = ""
	m.resizeViewports()
	m.taskArtifact = nil
	m.taskArtifactErr = ""
	return m
}

func (m tuiModel) openSelected() (tuiRoute, bool) {
	switch m.route.view {
	case tuiViewHome:
		if m.home == nil || len(m.home.RecentCommits) == 0 {
			return tuiRoute{}, false
		}
		run := m.home.RecentCommits[m.cursor]
		return tuiCommitRoute(run.Repo, run.Commit), true
	case tuiViewQueue:
		entry, ok := m.selectedQueueEntry()
		if !ok {
			return tuiRoute{}, false
		}
		return tuiCommitRoute(entry.Repo, entry.Commit), true
	case tuiViewRepos:
		if m.cursor < 0 || m.cursor >= len(m.repos) {
			return tuiRoute{}, false
		}
		return tuiRepoRoute(m.repos[m.cursor]), true
	case tuiViewRepo:
		if m.repo == nil || len(m.repo.Commits) == 0 {
			return tuiRoute{}, false
		}
		run := m.repo.Commits[m.cursor]
		return tuiCommitRoute(run.Repo, run.Commit), true
	case tuiViewRepoTaskHistory:
		row, ok := m.selectedTaskHistoryRow()
		if !ok {
			return tuiRoute{}, false
		}
		return tuiTaskRoute(m.taskHistory.Repo, row.Commit, m.taskHistory.Task), true
	case tuiViewCommit:
		task, ok := m.selectedCommitTask()
		if !ok {
			return tuiRoute{}, false
		}
		return tuiTaskRoute(m.commit.Repo, m.commit.Commit.Commit, task.Name), true
	case tuiViewTask:
		if m.task == nil || len(m.task.Task.Artifacts) == 0 {
			return tuiRoute{}, false
		}
		route := tuiArtifactListRoute(m.route, selectedAttempt(m.task))
		return tuiArtifactRoute(route, m.task.Task.Artifacts[m.cursor].DisplayName), true
	case tuiViewArtifacts:
		if m.artifacts == nil || len(m.artifacts.Artifacts) == 0 {
			return tuiRoute{}, false
		}
		return tuiArtifactRoute(m.route, m.artifacts.Artifacts[m.cursor].DisplayName), true
	}
	return tuiRoute{}, false
}

func (m tuiModel) artifactListRoute() (tuiRoute, bool) {
	if m.route.view != tuiViewTask || m.task == nil {
		return tuiRoute{}, false
	}
	return tuiArtifactListRoute(m.route, selectedAttempt(m.task)), true
}

func (m tuiModel) selectedArtifactRoute() (tuiRoute, bool) {
	switch m.route.view {
	case tuiViewTask:
		if m.task == nil || len(m.task.Task.Artifacts) == 0 || m.cursor < 0 || m.cursor >= len(m.task.Task.Artifacts) {
			return tuiRoute{}, false
		}
		route := tuiArtifactListRoute(m.route, selectedAttempt(m.task))
		return tuiArtifactRoute(route, m.task.Task.Artifacts[m.cursor].DisplayName), true
	case tuiViewArtifacts:
		if m.artifacts == nil || len(m.artifacts.Artifacts) == 0 || m.cursor < 0 || m.cursor >= len(m.artifacts.Artifacts) {
			return tuiRoute{}, false
		}
		return tuiArtifactRoute(m.route, m.artifacts.Artifacts[m.cursor].DisplayName), true
	case tuiViewArtifact:
		if m.route.apiPath == "" {
			return tuiRoute{}, false
		}
		return m.route, true
	default:
		return tuiRoute{}, false
	}
}
