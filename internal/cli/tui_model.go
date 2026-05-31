package cli

import (
	"context"
	"fmt"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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

	home            *tuiHomeResponse
	repos           []tuiRepoSummary
	queue           *tuiQueueResponse
	repo            *tuiRepoResponse
	commit          *tuiCommitResponse
	task            *tuiTaskResponse
	artifacts       *tuiArtifactListResponse
	artifact        *tuiArtifactResponse
	taskArtifact    *tuiArtifactResponse
	taskArtifactErr string

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
	PrevTab   key.Binding
	NextTab   key.Binding
	HomeKey   key.Binding
	End       key.Binding
	HomeView  key.Binding
	Repos     key.Binding
	Queue     key.Binding
	Retry     key.Binding
	Cancel    key.Binding
	Artifacts key.Binding
	Edit      key.Binding
	OpenFile  key.Binding
	Finder    key.Binding
}

func defaultTUIKeys() tuiKeyMap {
	return tuiKeyMap{
		Open:      key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		Back:      key.NewBinding(key.WithKeys("left", "backspace", "esc"), key.WithHelp("esc", "back")),
		Quit:      key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q", "quit")),
		Help:      key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "help")),
		Refresh:   key.NewBinding(key.WithKeys("R"), key.WithHelp("R", "refresh")),
		Up:        key.NewBinding(key.WithKeys("up", "k"), key.WithHelp("↑/k", "up")),
		Down:      key.NewBinding(key.WithKeys("down", "j"), key.WithHelp("↓/j", "down")),
		PageUp:    key.NewBinding(key.WithKeys("pgup"), key.WithHelp("pgup", "scroll up")),
		PageDown:  key.NewBinding(key.WithKeys("pgdown"), key.WithHelp("pgdn", "scroll down")),
		PrevTab:   key.NewBinding(key.WithKeys("left"), key.WithHelp("←", "prev tab")),
		NextTab:   key.NewBinding(key.WithKeys("right"), key.WithHelp("→", "next tab")),
		HomeKey:   key.NewBinding(key.WithKeys("home"), key.WithHelp("home", "top")),
		End:       key.NewBinding(key.WithKeys("end"), key.WithHelp("end", "bottom")),
		HomeView:  key.NewBinding(key.WithKeys("h"), key.WithHelp("h", "home")),
		Repos:     key.NewBinding(key.WithKeys("l"), key.WithHelp("l", "repos")),
		Queue:     key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "queue")),
		Retry:     key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "retry")),
		Cancel:    key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "cancel")),
		Artifacts: key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "artifacts")),
		Edit:      key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit artifact")),
		OpenFile:  key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open artifact")),
		Finder:    key.NewBinding(key.WithKeys("f"), key.WithHelp("f", "show in finder")),
	}
}

func (k tuiKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Open, k.Back, k.PrevTab, k.NextTab, k.Artifacts, k.Edit, k.OpenFile, k.Finder, k.Help, k.Quit}
}

func (k tuiKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Open, k.Back, k.Quit, k.Help},
		{k.Up, k.Down, k.PrevTab, k.NextTab, k.PageUp, k.PageDown, k.HomeKey, k.End},
		{k.HomeView, k.Repos, k.Queue, k.Refresh},
		{k.Retry, k.Cancel, k.Artifacts, k.Edit, k.OpenFile, k.Finder},
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

type tuiExternalCommandMsg struct {
	action string
	path   string
	err    error
}

type tuiTaskArtifactMsg struct {
	route    tuiRoute
	artifact string
	data     tuiArtifactResponse
	err      error
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
		} else if m.route.view == tuiViewTask && m.isTaskScrollKey(msg) {
			var cmd tea.Cmd
			m.log, cmd = m.log.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
		return m.handleKey(msg)
	case tea.MouseMsg:
		switch m.route.view {
		case tuiViewTask:
			var cmd tea.Cmd
			m.log, cmd = m.log.Update(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		case tuiViewArtifact:
			var cmd tea.Cmd
			m.artifactLog, cmd = m.artifactLog.Update(msg)
			if cmd != nil {
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
			if m.route.view == tuiViewTask {
				cmds = append(cmds, m.loadSelectedTaskArtifact())
			}
		}
	case tuiErrMsg:
		if msg.route.apiPath == m.route.apiPath {
			m.loading = false
			m.err = msg.err.Error()
		}
	case tuiTaskArtifactMsg:
		if msg.route.apiPath == m.route.apiPath && m.selectedTaskArtifactName() == msg.artifact {
			m.taskArtifact = nil
			m.taskArtifactErr = ""
			if msg.err != nil {
				m.taskArtifactErr = msg.err.Error()
			} else {
				artifact := msg.data
				m.taskArtifact = &artifact
			}
			m.syncViewports(true)
		}
	case tuiActionMsg:
		if msg.err != nil {
			m.notice = msg.err.Error()
		} else {
			m.notice = msg.notice
		}
		m.loading = true
		cmds = append(cmds, m.loadRoute(m.route), m.restartStream(m.route))
	case tuiExternalCommandMsg:
		if msg.err != nil {
			m.notice = fmt.Sprintf("%s failed: %v", msg.action, msg.err)
		} else {
			m.notice = fmt.Sprintf("%s: %s", msg.action, msg.path)
		}
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

func (m tuiModel) isTaskScrollKey(msg tea.KeyMsg) bool {
	return key.Matches(msg, m.keys.Up) ||
		key.Matches(msg, m.keys.Down) ||
		key.Matches(msg, m.keys.PageUp) ||
		key.Matches(msg, m.keys.PageDown) ||
		key.Matches(msg, m.keys.HomeKey) ||
		key.Matches(msg, m.keys.End)
}

func (m tuiModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	switch {
	case m.route.view == tuiViewTask && key.Matches(msg, m.keys.PrevTab):
		if m.moveTaskArtifact(-1) {
			cmds = append(cmds, m.loadSelectedTaskArtifact())
		}
	case m.route.view == tuiViewTask && key.Matches(msg, m.keys.NextTab):
		if m.moveTaskArtifact(1) {
			cmds = append(cmds, m.loadSelectedTaskArtifact())
		}
	case key.Matches(msg, m.keys.Quit):
		if m.stream != nil && m.stream.cancel != nil {
			m.stream.cancel()
		}
		return m, tea.Quit
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
	case key.Matches(msg, m.keys.Edit):
		cmds = append(cmds, m.editSelectedArtifact())
	case key.Matches(msg, m.keys.OpenFile):
		cmds = append(cmds, m.openSelectedArtifact())
	case key.Matches(msg, m.keys.Finder):
		cmds = append(cmds, m.revealSelectedArtifact())
	case key.Matches(msg, m.keys.Retry):
		if cmd := m.retrySelected(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	case key.Matches(msg, m.keys.Cancel):
		if cmd := m.cancelSelected(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return m, tea.Batch(cmds...)
}
