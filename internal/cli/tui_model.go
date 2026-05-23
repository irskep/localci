package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"localci/internal/localci"
)

type tuiView int

const (
	tuiViewHome tuiView = iota
	tuiViewRepos
	tuiViewQueue
	tuiViewRepo
	tuiViewCommit
	tuiViewTask
	tuiViewArtifacts
	tuiViewArtifact
)

type tuiRoute struct {
	view     tuiView
	apiPath  string
	title    string
	repoPath string
	repoDir  string
	commit   string
	task     string
	attempt  int
	artifact string
}

type tuiModel struct {
	client *tuiClient
	route  tuiRoute
	back   []tuiRoute

	width  int
	height int

	home      *tuiHomeResponse
	repos     []tuiRepoSummary
	queue     *tuiQueueResponse
	repo      *tuiRepoResponse
	commit    *tuiCommitResponse
	task      *tuiTaskResponse
	artifacts *tuiArtifactListResponse
	artifact  *tuiArtifactResponse

	cursor      int
	secondary   int
	scroll      int
	loading     bool
	err         string
	notice      string
	socketState string
	events      chan tuiEventMsg
	stream      *tuiStreamState
	keys        tuiKeyMap
	help        help.Model
	log         viewport.Model
	artifactLog viewport.Model
	progress    progress.Model
}

type tuiKeyMap struct {
	Open      key.Binding
	Back      key.Binding
	Quit      key.Binding
	Help      key.Binding
	Refresh   key.Binding
	Up        key.Binding
	Down      key.Binding
	PageUp    key.Binding
	PageDown  key.Binding
	HomeKey   key.Binding
	End       key.Binding
	HomeView  key.Binding
	Repos     key.Binding
	Queue     key.Binding
	Retry     key.Binding
	Cancel    key.Binding
	Artifacts key.Binding
}

func defaultTUIKeys() tuiKeyMap {
	return tuiKeyMap{
		Open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:      key.NewBinding(key.WithKeys("left", "backspace", "esc"), key.WithHelp("left/esc", "back")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Refresh:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
		PageDown:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "scroll down")),
		HomeKey:   key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		End:       key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		HomeView:  key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "home")),
		Repos:     key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "repos")),
		Queue:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "queue")),
		Retry:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry")),
		Cancel:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cancel")),
		Artifacts: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "artifacts")),
	}
}

func (k tuiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Back, k.Help, k.Quit}
}

func (k tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.Back, k.Quit, k.Help},
		{k.Up, k.Down, k.PageUp, k.PageDown, k.HomeKey, k.End},
		{k.HomeView, k.Repos, k.Queue, k.Refresh},
		{k.Retry, k.Cancel, k.Artifacts},
	}
}

type tuiStreamState struct {
	gen    int
	cancel context.CancelFunc
}

type tuiLoadedMsg struct {
	route tuiRoute
	data  any
}

type tuiErrMsg struct {
	route tuiRoute
	err   error
}

type tuiActionMsg struct {
	notice string
	err    error
}

type tuiEventMsg struct {
	gen   int
	event tuiAPIEvent
	err   error
}

type tuiStreamStartedMsg struct{}
type tuiTickMsg struct{}

func newTUIModel(client *tuiClient, route tuiRoute) tuiModel {
	keys := defaultTUIKeys()
	helpModel := help.New()
	helpModel.ShowAll = false
	logViewport := viewport.New(0, 0)
	logViewport.MouseWheelEnabled = true
	artifactViewport := viewport.New(0, 0)
	artifactViewport.MouseWheelEnabled = true
	return tuiModel{
		client:      client,
		route:       route,
		loading:     true,
		socketState: "connecting",
		events:      make(chan tuiEventMsg, 64),
		stream:      &tuiStreamState{},
		keys:        keys,
		help:        helpModel,
		log:         logViewport,
		artifactLog: artifactViewport,
		progress:    progress.New(progress.WithDefaultGradient()),
	}
}

func (m tuiModel) Init() tea.Cmd {
	return tea.Batch(m.loadRoute(m.route), m.startStream(m.route), m.listenEvents(), tickTUI())
}

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width
		m.resizeViewports()
	case tea.KeyMsg:
		if m.help.ShowAll {
			switch {
			case key.Matches(msg, m.keys.Help), key.Matches(msg, m.keys.Back), key.Matches(msg, m.keys.Quit):
				if msg.String() == "ctrl+c" {
					if m.stream != nil && m.stream.cancel != nil {
						m.stream.cancel()
					}
					return m, tea.Quit
				}
				m.help.ShowAll = false
				return m, nil
			default:
				return m, nil
			}
		}
		if m.route.view == tuiViewArtifact {
			var cmd tea.Cmd
			m.artifactLog, cmd = m.artifactLog.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if m.route.view == tuiViewTask && (key.Matches(msg, m.keys.PageUp) || key.Matches(msg, m.keys.PageDown)) {
			var cmd tea.Cmd
			m.log, cmd = m.log.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			if msg.String() == "q" || msg.String() == "ctrl+c" {
				if m.stream != nil && m.stream.cancel != nil {
					m.stream.cancel()
				}
				return m, tea.Quit
			}
		case key.Matches(msg, m.keys.Help):
			m.help.ShowAll = true
		case key.Matches(msg, m.keys.Refresh):
			m.loading = true
			cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
		case key.Matches(msg, m.keys.HomeView):
			m = m.gotoRoute(tuiRoute{view: tuiViewHome, apiPath: "/api", title: "Home"}, true)
			cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
		case key.Matches(msg, m.keys.Queue):
			m = m.gotoRoute(tuiRoute{view: tuiViewQueue, apiPath: "/api/queue", title: "Queue"}, true)
			cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
		case key.Matches(msg, m.keys.Repos):
			m = m.gotoRoute(tuiRoute{view: tuiViewRepos, apiPath: "/api/repo", title: "Repos"}, true)
			cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
		case key.Matches(msg, m.keys.Back):
			if len(m.back) > 0 {
				route := m.back[len(m.back)-1]
				m.back = m.back[:len(m.back)-1]
				m = m.gotoRoute(route, false)
				cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
			}
		case key.Matches(msg, m.keys.Up):
			m.moveCursor(-1)
		case key.Matches(msg, m.keys.Down):
			m.moveCursor(1)
		case key.Matches(msg, m.keys.PageUp):
			m.scroll -= pageSize(m.height)
			if m.scroll < 0 {
				m.scroll = 0
			}
		case key.Matches(msg, m.keys.PageDown):
			m.scroll += pageSize(m.height)
		case key.Matches(msg, m.keys.HomeKey):
			m.cursor = 0
			m.scroll = 0
		case key.Matches(msg, m.keys.End):
			m.cursor = m.maxCursor()
			if m.route.view == tuiViewTask {
				m.log.GotoBottom()
			}
			if m.route.view == tuiViewArtifact {
				m.artifactLog.GotoBottom()
			}
		case key.Matches(msg, m.keys.Open):
			if next, ok := m.openSelected(); ok {
				m = m.gotoRoute(next, true)
				cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
			}
		case key.Matches(msg, m.keys.Artifacts):
			if next, ok := m.artifactListRoute(); ok {
				m = m.gotoRoute(next, true)
				cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
			}
		case key.Matches(msg, m.keys.Retry):
			if cmd := m.retrySelected(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		case key.Matches(msg, m.keys.Cancel):
			if cmd := m.cancelSelected(); cmd != nil {
				cmds = append(cmds, cmd)
			}
		}
	case tuiLoadedMsg:
		if msg.route.apiPath == m.route.apiPath {
			m.loading = false
			m.err = ""
			m.applyData(msg.data)
			m.syncViewports(true)
			m.clampSelection()
		}
	case tuiErrMsg:
		if msg.route.apiPath == m.route.apiPath {
			m.loading = false
			m.err = msg.err.Error()
		}
	case tuiActionMsg:
		if msg.err != nil {
			m.notice = msg.err.Error()
		} else {
			m.notice = msg.notice
		}
		m.loading = true
		cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
	case tuiStreamStartedMsg:
		m.socketState = "connected"
	case tuiEventMsg:
		cmds = append(cmds, m.listenEvents())
		if m.stream == nil || msg.gen != m.stream.gen {
			break
		}
		if msg.err != nil {
			m.socketState = "reconnecting"
			m.syncViewports(false)
			cmds = append(cmds, m.startStream(m.route))
			break
		}
		m.socketState = "connected"
		cmds = append(cmds, m.applyEvent(msg.event))
	case tuiTickMsg:
		cmds = append(cmds, tickTUI())
		if m.socketState != "connected" {
			cmds = append(cmds, m.loadRoute(m.route))
		}
	}
	return m, tea.Batch(cmds...)
}

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

func (m tuiModel) retrySelected() tea.Cmd {
	route, ok := m.actionTaskRoute()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		resp, err := m.client.retryTask(context.Background(), route.apiPath)
		if err != nil {
			return tuiActionMsg{err: err}
		}
		if resp.Enqueued {
			return tuiActionMsg{notice: fmt.Sprintf("retry queued: %s attempt %d", trimTaskLabel(resp.Task), resp.Attempt)}
		}
		return tuiActionMsg{notice: fmt.Sprintf("already running: %s attempt %d", trimTaskLabel(resp.Task), resp.Attempt)}
	}
}

func (m tuiModel) cancelSelected() tea.Cmd {
	route, ok := m.actionTaskRoute()
	if !ok {
		return nil
	}
	return func() tea.Msg {
		resp, err := m.client.cancelTask(context.Background(), route.apiPath)
		if err != nil {
			return tuiActionMsg{err: err}
		}
		if resp.Canceled {
			return tuiActionMsg{notice: fmt.Sprintf("canceled: %s", trimTaskLabel(resp.Task))}
		}
		return tuiActionMsg{notice: fmt.Sprintf("nothing to cancel: %s", trimTaskLabel(resp.Task))}
	}
}

func (m tuiModel) actionTaskRoute() (tuiRoute, bool) {
	switch m.route.view {
	case tuiViewCommit:
		task, ok := m.selectedCommitTask()
		if !ok {
			return tuiRoute{}, false
		}
		return tuiTaskRoute(m.commit.Repo, m.commit.Commit.Commit, task.Name), true
	case tuiViewTask:
		if m.task == nil {
			return tuiRoute{}, false
		}
		return tuiTaskRoute(m.task.Repo, m.task.Commit, m.task.Task.Name), true
	default:
		return tuiRoute{}, false
	}
}

func (m tuiModel) selectedCommitTask() (localci.TaskStatusView, bool) {
	if m.commit == nil || m.cursor < 0 || m.cursor >= len(m.commit.Commit.Tasks) {
		return localci.TaskStatusView{}, false
	}
	return m.commit.Commit.Tasks[m.cursor], true
}

func (m tuiModel) selectedQueueEntry() (tuiQueueEntry, bool) {
	if m.queue == nil {
		return tuiQueueEntry{}, false
	}
	if m.queue.Active != nil {
		if m.cursor == 0 {
			return *m.queue.Active, true
		}
		index := m.cursor - 1
		if index >= 0 && index < len(m.queue.Pending) {
			return m.queue.Pending[index], true
		}
		return tuiQueueEntry{}, false
	}
	if m.cursor >= 0 && m.cursor < len(m.queue.Pending) {
		return m.queue.Pending[m.cursor], true
	}
	return tuiQueueEntry{}, false
}

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
		m.log.SetContent(m.task.PrimaryLog)
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

func (m *tuiModel) resizeViewports() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	bodyHeight := max(1, m.height-3)
	navWidth := 18
	if m.width < 72 {
		navWidth = 14
	}
	contentWidth := max(20, m.width-navWidth-1)
	artifactWidth := clamp(contentWidth/4, 22, 36)
	logWidth := max(20, contentWidth-artifactWidth-2)
	m.log.Width = max(1, logWidth-4)
	m.log.Height = max(1, bodyHeight-6)
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

type tuiTheme struct{}

func (t tuiTheme) title() lipgloss.Style {
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#c4b5fd"))
}

func (t tuiTheme) muted() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#71717a"))
}

func (t tuiTheme) selected() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#f5f3ff")).Background(lipgloss.Color("#3b2f63")).Bold(true)
}

func (t tuiTheme) panel() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("#3f3f46")).Padding(0, 1)
}

func (t tuiTheme) danger() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#f87171")).Bold(true)
}

func (t tuiTheme) ok() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#86efac")).Bold(true)
}

func (t tuiTheme) warning() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#fbbf24")).Bold(true)
}

func (t tuiTheme) running() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#67e8f9")).Bold(true)
}

func (t tuiTheme) chrome() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color("#d4d4d8")).Background(lipgloss.Color("#18181b"))
}

func (m tuiModel) renderHeader(theme tuiTheme) string {
	crumbs := []string{"localci", viewName(m.route.view)}
	if m.route.repoPath != "" {
		crumbs = append(crumbs, m.route.repoPath)
	}
	if m.route.commit != "" {
		crumbs = append(crumbs, shortCommit(m.route.commit))
	}
	if m.route.task != "" {
		crumbs = append(crumbs, trimTaskLabel(m.route.task))
	}
	if m.route.artifact != "" {
		crumbs = append(crumbs, m.route.artifact)
	}
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
	return renderTUIPanel(theme, width, height, strings.Join(lines, "\n"))
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
		return renderTUIPanel(theme, width, height, strings.Join(lines, "\n"))
	}
	run := m.home.RecentCommits[m.cursor]
	lines = append(lines,
		truncate(run.Repo.RepoPath+"  "+shortCommit(run.Commit), width-4),
		truncate(taskCounts(run.Tasks), width-4),
	)
	if !runComplete(run.Tasks) {
		bar := m.progress
		bar.Width = max(8, width-8)
		lines = append(lines, bar.ViewAs(runProgress(run.Tasks)))
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
	return renderTUIPanel(theme, width, height, strings.Join(lines, "\n"))
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
		rows = append(rows, selectableLine(theme, i == m.cursor, truncate(repo.RepoPath, m.width-4)))
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
	rows := []string{theme.title().Render(m.repo.Repo.RepoPath)}
	for i, run := range visibleRuns(m.repo.Commits, m.scroll, height-2) {
		rows = append(rows, selectableLine(theme, m.scroll+i == m.cursor, runListLine(run, m.width-8)))
	}
	if len(m.repo.Commits) == 0 {
		rows = append(rows, theme.muted().Render("No runs for this repo."))
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
	right := []string{theme.title().Render(shortCommit(m.commit.Commit.Commit))}
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
	top := []string{
		theme.title().Render(trimTaskLabel(m.task.Task.Name)),
		fmt.Sprintf("%s  attempt %s  duration %s", statusLabel(m.task.Task.Status), attemptLabel(m.task.Task.Attempt, m.task.Task.AttemptCount), durationLabel(m.task.Task.DurationMilliseconds)),
	}
	if m.task.Task.Failure != "" {
		top = append(top, "failure: "+m.task.Task.Failure)
	}
	artifactWidth := clamp(m.width/4, 22, 36)
	logWidth := max(20, m.width-artifactWidth-2)
	left := []string{theme.title().Render("Artifacts")}
	for i, artifact := range m.task.Task.Artifacts {
		left = append(left, selectableLine(theme, i == m.cursor, truncate(artifact.DisplayName, artifactWidth-5)))
	}
	if len(m.task.Task.Artifacts) == 0 {
		left = append(left, theme.muted().Render("No artifacts."))
	}
	logTitle := "Log"
	if m.task.PrimaryArtifact != "" {
		logTitle = m.task.PrimaryArtifact
	}
	logHeight := max(1, height-len(top)-2)
	logViewport := m.log
	logViewport.Width = max(1, logWidth-4)
	logViewport.Height = max(1, logHeight-4)
	logViewport.SetContent(m.task.PrimaryLog)
	logView := theme.title().Render(logTitle) + "\n" + logViewport.View()
	lower := lipgloss.JoinHorizontal(lipgloss.Top,
		renderTUIPanel(theme, artifactWidth, logHeight, strings.Join(left, "\n")),
		" ",
		renderTUIPanel(theme, logWidth, logHeight, logView),
	)
	return strings.Join(top, "\n") + "\n" + lower
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
				"Text artifacts can be viewed here.",
				"Binary or oversized artifacts stay on disk.",
			}
			return renderTUIPanel(theme, m.width, height, strings.Join(lines, "\n"))
		}
		return loadingText(m.loading)
	}
	logViewport := m.artifactLog
	logViewport.Width = max(1, m.width-4)
	logViewport.Height = max(1, height-4)
	logViewport.SetContent(m.artifact.Content)
	lines := []string{
		theme.title().Render(m.artifact.Artifact.DisplayName),
		logViewport.View(),
	}
	return renderTUIPanel(theme, m.width, height, strings.Join(lines, "\n"))
}

func (m tuiModel) renderRunSummary(theme tuiTheme, run tuiCommitSummary, selected bool) string {
	counts := taskCounts(run.Tasks)
	line := fmt.Sprintf("%s  %s  %s  %s", run.Repo.RepoPath, shortCommit(run.Commit), counts, timeAgo(run.ActivityAt))
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

func runListLine(run tuiCommitSummary, width int) string {
	mark := "·"
	if hasTaskStatus(run.Tasks, localci.ExecutionStatusFailed) || hasTaskStatus(run.Tasks, localci.ExecutionStatusTimedOut) {
		mark = "x"
	} else if hasTaskStatus(run.Tasks, localci.ExecutionStatusRunning) || hasTaskStatus(run.Tasks, localci.ExecutionStatusQueued) {
		mark = ">"
	} else if hasTaskStatus(run.Tasks, localci.ExecutionStatusSucceeded) {
		mark = "✓"
	}
	line := fmt.Sprintf("%s %s  %s  %s  %s", mark, run.Repo.RepoPath, shortCommit(run.Commit), taskCountsShort(run.Tasks), timeAgo(run.ActivityAt))
	if branch := run.Annotations["branch"]; branch != "" && width >= 72 {
		line += "  " + branch
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
		return fmt.Sprintf("%s %s", entry.Repo.RepoPath, task)
	}
	return fmt.Sprintf("%s  %s  %s  attempt %d", entry.Repo.RepoPath, shortCommit(entry.Commit), task, entry.Attempt)
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

func viewName(view tuiView) string {
	switch view {
	case tuiViewRepos:
		return "Repos"
	case tuiViewQueue:
		return "Queue"
	case tuiViewRepo:
		return "Repo"
	case tuiViewCommit:
		return "Run"
	case tuiViewTask:
		return "Task"
	case tuiViewArtifacts:
		return "Artifacts"
	case tuiViewArtifact:
		return "Artifact"
	default:
		return "Home"
	}
}

func shortCommit(commit string) string {
	if len(commit) <= 12 {
		return commit
	}
	return commit[:12]
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
