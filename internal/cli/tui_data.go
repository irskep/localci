package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *tuiModel) applyData(data any) {
	switch typed := data.(type) {
	case tuiHomeResponse:
		m.home = &typed
		queue := typed.Queue
		m.queue = &queue
	case tuiQueueResponse:
		m.queue = &typed
	case []tuiRepoSummary:
		m.repos = append([]tuiRepoSummary{}, typed...)
	case tuiRepoResponse:
		m.repo = &typed
	case tuiRepoTaskHistoryResponse:
		m.taskHistory = &typed
	case tuiCommitResponse:
		m.commit = &typed
	case tuiTaskResponse:
		m.task = &typed
	case tuiArtifactListResponse:
		m.artifacts = &typed
	case tuiArtifactResponse:
		m.artifact = &typed
	}
}

func (m *tuiModel) syncViewports(gotoBottom bool) {
	m.resizeViewports()
	if m.task != nil {
		m.log.SetContent(m.selectedTaskArtifactContent())
		if gotoBottom {
			m.log.GotoBottom()
		}
	}
	if m.artifact != nil {
		m.artifactLog.SetContent(m.artifact.Content)
		if gotoBottom {
			m.artifactLog.GotoBottom()
		}
	}
}

func (m tuiModel) selectedTaskArtifactContent() string {
	if m.task == nil {
		return ""
	}
	name := m.selectedTaskArtifactName()
	if name == "" || name == m.task.PrimaryArtifact {
		return m.task.PrimaryLog
	}
	if m.taskArtifact != nil && m.taskArtifact.Artifact.DisplayName == name {
		return m.taskArtifact.Content
	}
	return ""
}

func (m tuiModel) selectedTaskArtifactError() string {
	if m.task == nil {
		return ""
	}
	name := m.selectedTaskArtifactName()
	if name == "" || name == m.task.PrimaryArtifact {
		return ""
	}
	return m.taskArtifactErr
}

func (m tuiModel) loadSelectedTaskArtifact() tea.Cmd {
	if m.route.view != tuiViewTask || m.task == nil {
		return nil
	}
	name := m.selectedTaskArtifactName()
	if name == "" || name == m.task.PrimaryArtifact {
		return nil
	}
	route := tuiArtifactRoute(tuiArtifactListRoute(m.route, selectedAttempt(m.task)), name)
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		resp, err := m.client.loadArtifact(ctx, route.apiPath)
		return tuiTaskArtifactMsg{route: m.route, artifact: name, data: resp, err: err}
	}
}

func (m *tuiModel) resizeViewports() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	bodyHeight := max(1, m.height-3)
	navWidth := 18
	if m.width < 72 {
		navWidth = 14
	}
	contentWidth := m.width
	if m.usesSidebar() {
		contentWidth = max(20, m.width-navWidth-1)
	}
	m.log.Width = max(1, contentWidth-4)
	m.log.Height = max(1, bodyHeight-7)
	m.artifactLog.Width = max(1, contentWidth-4)
	m.artifactLog.Height = max(1, bodyHeight-4)
}

func (m tuiModel) loadRoute(route tuiRoute) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var data any
		var err error
		switch route.view {
		case tuiViewHome:
			data, err = m.client.loadHome(ctx)
		case tuiViewRepos:
			data, err = m.client.loadRepoIndex(ctx)
		case tuiViewQueue:
			data, err = m.client.loadQueue(ctx)
		case tuiViewRepo:
			data, err = m.client.loadRepo(ctx, route.apiPath)
		case tuiViewRepoTaskHistory:
			data, err = m.client.loadRepoTaskHistory(ctx, route.apiPath)
		case tuiViewCommit:
			data, err = m.client.loadCommit(ctx, route.apiPath)
		case tuiViewTask:
			data, err = m.client.loadTask(ctx, route.apiPath)
		case tuiViewArtifacts:
			data, err = m.client.loadArtifactList(ctx, route.apiPath)
		case tuiViewArtifact:
			data, err = m.client.loadArtifact(ctx, route.apiPath)
		}
		if err != nil {
			return tuiErrMsg{route: route, err: err}
		}
		return tuiLoadedMsg{route: route, data: data}
	}
}

func (m tuiModel) restartStream(route tuiRoute) tea.Cmd {
	if m.stream != nil && m.stream.cancel != nil {
		m.stream.cancel()
	}
	return m.startStream(route)
}

func (m tuiModel) startStream(route tuiRoute) tea.Cmd {
	if m.stream == nil {
		m.stream = &tuiStreamState{}
	}
	if m.stream.cancel != nil {
		m.stream.cancel()
	}
	m.stream.gen++
	gen := m.stream.gen
	ctx, cancel := context.WithCancel(context.Background())
	m.stream.cancel = cancel
	ch := m.events
	client := m.client
	return func() tea.Msg {
		go func() {
			backoff := 250 * time.Millisecond
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				err := client.streamEvents(ctx, route.apiPath, func(event tuiAPIEvent) bool {
					select {
					case <-ctx.Done():
						return false
					case ch <- tuiEventMsg{gen: gen, event: event}:
						return true
					}
				})
				if err != nil {
					select {
					case <-ctx.Done():
						return
					case ch <- tuiEventMsg{gen: gen, err: err}:
					}
					time.Sleep(backoff)
					if backoff < 5*time.Second {
						backoff *= 2
					}
				}
			}
		}()
		return tuiStreamStartedMsg{}
	}
}

func (m tuiModel) listenEvents() tea.Cmd {
	return func() tea.Msg {
		return <-m.events
	}
}

func (m *tuiModel) applyEvent(event tuiAPIEvent) tea.Cmd {
	if event.Type == "append" {
		if m.route.view == tuiViewTask && m.task != nil && m.task.PrimaryArtifact == "combined.log" {
			if int64(len(m.task.PrimaryLog)) == event.Offset {
				wasBottom := m.log.AtBottom()
				m.task.PrimaryLog += event.Text
				m.syncViewports(wasBottom)
				return nil
			}
			return m.loadRoute(m.route)
		}
		if m.route.view == tuiViewArtifact && m.artifact != nil {
			if int64(len(m.artifact.Content)) == event.Offset {
				wasBottom := m.artifactLog.AtBottom()
				m.artifact.Content += event.Text
				m.syncViewports(wasBottom)
				return nil
			}
			return m.loadRoute(m.route)
		}
		return nil
	}
	if event.Type == "error" {
		m.err = event.Message
		return nil
	}
	if event.Type != "snapshot" && event.Type != "replace" {
		return nil
	}
	data, err := decodeRouteData(m.route, event.Data)
	if err != nil {
		m.err = err.Error()
		return nil
	}
	m.applyData(data)
	m.syncViewports(false)
	m.loading = false
	m.err = ""
	m.clampSelection()
	return nil
}

func decodeRouteData(route tuiRoute, raw json.RawMessage) (any, error) {
	switch route.view {
	case tuiViewHome:
		var data tuiHomeResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewRepos:
		var data []tuiRepoSummary
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewQueue:
		var data tuiQueueResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewRepo:
		var data tuiRepoResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewRepoTaskHistory:
		var data tuiRepoTaskHistoryResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewCommit:
		var data tuiCommitResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewTask:
		var data tuiTaskResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewArtifacts:
		var data tuiArtifactListResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	case tuiViewArtifact:
		var data tuiArtifactResponse
		err := json.Unmarshal(raw, &data)
		return data, err
	default:
		return nil, fmt.Errorf("unsupported view")
	}
}

func tickTUI() tea.Cmd {
	return tea.Tick(2*time.Second, func(time.Time) tea.Msg { return tuiTickMsg{} })
}

func selectedAttempt(task *tuiTaskResponse) int {
	if task == nil {
		return 1
	}
	if task.SelectedAttempt > 0 {
		return task.SelectedAttempt
	}
	if task.Task.Attempt > 0 {
		return task.Task.Attempt
	}
	return 1
}
