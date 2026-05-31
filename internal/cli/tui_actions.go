package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"localci/internal/localci"
)

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

func (m tuiModel) taskHistoryRoute() (tuiRoute, bool) {
	if m.route.view != tuiViewTask || m.task == nil {
		return tuiRoute{}, false
	}
	return tuiRepoTaskHistoryRoute(m.task.Repo, m.task.Task.Name), true
}

func (m tuiModel) editSelectedArtifact() tea.Cmd {
	path := m.selectedArtifactPath()
	if path == "" {
		return staticExternalCommandMsg("edit", "", fmt.Errorf("no artifact path selected"))
	}
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor == "" {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
	}
	if editor == "" {
		editor = "vi"
	}
	cmd := exec.Command(editor, path)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return tuiExternalCommandMsg{action: "edit", path: path, err: err}
	})
}

func (m tuiModel) openSelectedArtifact() tea.Cmd {
	path := m.selectedArtifactPath()
	if path == "" {
		return staticExternalCommandMsg("open", "", fmt.Errorf("no artifact path selected"))
	}
	cmd, err := openPathCommand(path)
	if err != nil {
		return staticExternalCommandMsg("open", path, err)
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return tuiExternalCommandMsg{action: "open", path: path, err: err}
	})
}

func (m tuiModel) revealSelectedArtifact() tea.Cmd {
	route, ok := m.selectedArtifactRoute()
	if !ok {
		return func() tea.Msg {
			return tuiActionMsg{err: fmt.Errorf("no artifact selected")}
		}
	}
	return func() tea.Msg {
		resp, err := m.client.revealArtifact(context.Background(), route.apiPath)
		if err != nil {
			return tuiActionMsg{err: err}
		}
		if resp.OK {
			return tuiActionMsg{notice: "shown in Finder: " + resp.Path}
		}
		return tuiActionMsg{notice: "Finder did not report an artifact"}
	}
}

func staticExternalCommandMsg(action string, path string, err error) tea.Cmd {
	return func() tea.Msg {
		return tuiExternalCommandMsg{action: action, path: path, err: err}
	}
}

func openPathCommand(path string) (*exec.Cmd, error) {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", path), nil
	case "windows":
		return exec.Command("cmd", "/c", "start", "", path), nil
	case "linux", "freebsd", "openbsd", "netbsd":
		return exec.Command("xdg-open", path), nil
	default:
		return nil, fmt.Errorf("opening files is not supported on %s", runtime.GOOS)
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

func (m tuiModel) selectedTaskHistoryRow() (tuiRepoTaskHistoryItem, bool) {
	if m.taskHistory == nil || m.cursor < 0 || m.cursor >= len(m.taskHistory.Runs) {
		return tuiRepoTaskHistoryItem{}, false
	}
	return m.taskHistory.Runs[m.cursor], true
}

func (m tuiModel) selectedTaskArtifactName() string {
	if m.task == nil || m.cursor < 0 || m.cursor >= len(m.task.Task.Artifacts) {
		return ""
	}
	return m.task.Task.Artifacts[m.cursor].DisplayName
}

func (m tuiModel) selectedTaskArtifactPath() string {
	if m.task == nil || m.cursor < 0 || m.cursor >= len(m.task.Task.Artifacts) {
		return ""
	}
	return m.task.Task.Artifacts[m.cursor].Path
}

func (m tuiModel) selectedArtifactPath() string {
	switch m.route.view {
	case tuiViewTask:
		return m.selectedTaskArtifactPath()
	case tuiViewArtifact:
		if m.artifact != nil {
			return m.artifact.Artifact.Path
		}
	case tuiViewArtifacts:
		if m.artifacts != nil && m.cursor >= 0 && m.cursor < len(m.artifacts.Artifacts) {
			return m.artifacts.Artifacts[m.cursor].Path
		}
	}
	return ""
}
