package localci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/coder/websocket"
)

type WebServer struct {
	Paths         Paths
	Queue         QueueStore
	DiscoverTasks func(context.Context, string) ([]Task, error)
}

type CommitPageView struct {
	CommitStatusView
	TasksHTML template.HTML
}

type TaskPageView struct {
	RepoDir string
	Commit  string
	TaskStatusView
	FilesHTML template.HTML
}

func (s WebServer) Serve(ctx context.Context, listener net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/commit", s.handleCommit)
	mux.HandleFunc("/task", s.handleTask)
	mux.HandleFunc("/retry", s.handleRetry)
	mux.HandleFunc("/artifact", s.handleArtifact)
	mux.HandleFunc("/ws/status", s.handleStatusWebSocket)

	server := &http.Server{
		Handler: mux,
	}

	go func() {
		<-ctx.Done()
		_ = server.Close()
	}()

	err := server.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s WebServer) handleHome(w http.ResponseWriter, _ *http.Request) {
	_ = homeTemplate.Execute(w, nil)
}

func (s WebServer) handleCommit(w http.ResponseWriter, r *http.Request) {
	repoDir := r.URL.Query().Get("repo")
	commit := r.URL.Query().Get("commit")
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = commitTemplate.Execute(w, CommitPageView{
		CommitStatusView: view,
		TasksHTML:        template.HTML(renderCommitTasksHTML(view)),
	})
}

func (s WebServer) handleTask(w http.ResponseWriter, r *http.Request) {
	repoDir := r.URL.Query().Get("repo")
	commit := r.URL.Query().Get("commit")
	taskName := r.URL.Query().Get("task")

	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task, ok := findTaskStatus(view.Tasks, taskName)
	if ok {
		_ = taskTemplate.Execute(w, TaskPageView{
			RepoDir:        repoDir,
			Commit:         commit,
			TaskStatusView: task,
			FilesHTML:      template.HTML(renderTaskFilesHTML(repoDir, commit, task)),
		})
		return
	}

	http.Error(w, "task not found", http.StatusNotFound)
}

func (s WebServer) handleArtifact(w http.ResponseWriter, r *http.Request) {
	repoDir := r.URL.Query().Get("repo")
	commit := r.URL.Query().Get("commit")
	taskName := r.URL.Query().Get("task")
	path := r.URL.Query().Get("path")

	if repoDir == "" || commit == "" || taskName == "" || path == "" {
		http.Error(w, "repo, commit, task, and path are required", http.StatusBadRequest)
		return
	}

	outputDir := s.Paths.TaskOutputDir(repoDir, commit, taskName)
	cleanOutputDir := filepath.Clean(outputDir)
	cleanPath := filepath.Clean(path)
	if !strings.HasPrefix(cleanPath, cleanOutputDir+string(os.PathSeparator)) && cleanPath != cleanOutputDir {
		http.Error(w, "artifact path is outside task output", http.StatusBadRequest)
		return
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}

func (s WebServer) handleRetry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	repoDir := r.URL.Query().Get("repo")
	commit := r.URL.Query().Get("commit")
	taskName := r.URL.Query().Get("task")
	if repoDir == "" || commit == "" || taskName == "" {
		http.Error(w, "repo, commit, and task are required", http.StatusBadRequest)
		return
	}

	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if _, ok := findTaskStatus(view.Tasks, taskName); !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}

	active, err := s.Queue.IsTaskActive(repoDir, commit, taskName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !active {
		if _, err := s.Queue.Enqueue(repoDir, commit, taskName); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	http.Redirect(w, r, taskPageURL(repoDir, commit, taskName), http.StatusSeeOther)
}

func (s WebServer) handleStatusWebSocket(w http.ResponseWriter, r *http.Request) {
	repoDir := r.URL.Query().Get("repo")
	commit := r.URL.Query().Get("commit")
	if repoDir == "" || commit == "" {
		http.Error(w, "repo and commit are required", http.StatusBadRequest)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		view, err := s.buildStatusView(repoDir, commit)
		if err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return
		}
		data, err := renderStatusPayload(view)
		if err != nil {
			_ = conn.Close(websocket.StatusInternalError, err.Error())
			return
		}
		if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
			return
		}

		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (s WebServer) buildStatusView(repoDir string, commit string) (CommitStatusView, error) {
	if repoDir == "" || commit == "" {
		return CommitStatusView{}, fmt.Errorf("repo and commit are required")
	}
	if s.DiscoverTasks == nil {
		return CommitStatusView{}, fmt.Errorf("task discovery is not configured")
	}

	tasks, err := s.DiscoverTasks(context.Background(), repoDir)
	if err != nil {
		return CommitStatusView{}, err
	}

	queue, err := s.Queue.List()
	if err != nil {
		return CommitStatusView{}, err
	}

	active, err := s.Queue.ReadActive()
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return CommitStatusView{}, err
	}

	var activePtr *ActiveTask
	if err == nil {
		activePtr = &active
	}

	return BuildCommitStatusView(s.Paths, repoDir, commit, tasks, queue, activePtr)
}

var homeTemplate = template.Must(template.New("home").Parse(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>localci</title></head>
<body>
<h1>localci</h1>
<p>Open a commit view with <code>/commit?repo=...&commit=...</code>.</p>
</body>
</html>`))

var commitTemplate = template.Must(template.New("commit").Parse(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>{{.Commit}} - localci</title></head>
<body>
<h1>{{.Commit}}</h1>
<p>{{.RepoDir}}</p>
<ul id="task-list">{{.TasksHTML}}</ul>
<script>
(() => {
  const url = new URL("/ws/status", window.location.origin);
  url.searchParams.set("repo", {{.RepoDir}});
  url.searchParams.set("commit", {{.Commit}});
  const ws = new WebSocket(url);
  ws.onmessage = (event) => {
    const payload = JSON.parse(event.data);
    const taskList = document.getElementById("task-list");
    if (!taskList) return;
    taskList.innerHTML = payload.commit_html;
  };
})();
</script>
</body>
</html>`))

var taskTemplate = template.Must(template.New("task").Parse(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>{{.Name}} - localci</title></head>
<body>
<h1>{{.Name}}</h1>
<p id="task-status">Status: {{.Status}}</p>
<p id="task-attempt">Attempt: {{.Attempt}}</p>
<p id="task-output">Output: {{.OutputDir}}</p>
<form method="post" action="/retry?repo={{urlquery .RepoDir}}&commit={{urlquery .Commit}}&task={{urlquery .Name}}">
  <button type="submit">Retry task</button>
</form>
<ul id="task-files">{{.FilesHTML}}</ul>
<script>
(() => {
  const url = new URL("/ws/status", window.location.origin);
  url.searchParams.set("repo", {{.RepoDir}});
  url.searchParams.set("commit", {{.Commit}});
  const taskName = {{.Name}};
  url.searchParams.set("task", taskName);
  const ws = new WebSocket(url);
  ws.onmessage = (event) => {
    const payload = JSON.parse(event.data);
    const task = payload.tasks.find((item) => item.name === taskName);
    if (!task) return;
    const statusEl = document.getElementById("task-status");
    const attemptEl = document.getElementById("task-attempt");
    const outputEl = document.getElementById("task-output");
    const filesEl = document.getElementById("task-files");
    if (statusEl) statusEl.textContent = "Status: " + task.status;
    if (attemptEl) attemptEl.textContent = "Attempt: " + task.attempt;
    if (outputEl) outputEl.textContent = "Output: " + task.output_dir;
    if (filesEl) filesEl.innerHTML = task.files_html;
  };
})();
</script>
</body>
</html>`))

type statusPayload struct {
	Tasks      []taskPayload `json:"tasks"`
	CommitHTML string        `json:"commit_html"`
}

type taskPayload struct {
	Name      string `json:"name"`
	Attempt   int    `json:"attempt"`
	Status    string `json:"status"`
	OutputDir string `json:"output_dir"`
	FilesHTML string `json:"files_html"`
}

func renderStatusPayload(view CommitStatusView) ([]byte, error) {
	payload := statusPayload{
		Tasks:      make([]taskPayload, 0, len(view.Tasks)),
		CommitHTML: renderCommitTasksHTML(view),
	}

	for _, task := range view.Tasks {
		payload.Tasks = append(payload.Tasks, taskPayload{
			Name:      task.Name,
			Attempt:   task.Attempt,
			Status:    string(task.Status),
			OutputDir: task.OutputDir,
			FilesHTML: renderTaskFilesHTML(view.RepoDir, view.Commit, task),
		})
	}

	return json.Marshal(payload)
}

func renderCommitTasksHTML(view CommitStatusView) string {
	var b strings.Builder
	for _, task := range view.Tasks {
		fmt.Fprintf(&b, `<li><a href="%s">%s</a> — %s`,
			template.HTMLEscapeString(taskPageURL(view.RepoDir, view.Commit, task.Name)),
			template.HTMLEscapeString(task.Name),
			template.HTMLEscapeString(string(task.Status)),
		)
		if task.Attempt > 0 {
			fmt.Fprintf(&b, ` (attempt %d)`, task.Attempt)
		}
		b.WriteString(`</li>`)
	}
	return b.String()
}

func renderTaskFilesHTML(repoDir string, commit string, task TaskStatusView) string {
	var b strings.Builder
	for _, file := range task.OutputFiles {
		fmt.Fprintf(&b, `<li><a href="/artifact?repo=%s&commit=%s&task=%s&path=%s">%s</a></li>`,
			template.URLQueryEscaper(repoDir),
			template.URLQueryEscaper(commit),
			template.URLQueryEscaper(task.Name),
			template.URLQueryEscaper(file),
			template.HTMLEscapeString(file),
		)
	}
	return b.String()
}

func findTaskStatus(tasks []TaskStatusView, name string) (TaskStatusView, bool) {
	for _, task := range tasks {
		if task.Name == name {
			return task, true
		}
	}
	return TaskStatusView{}, false
}

func taskPageURL(repoDir string, commit string, taskName string) string {
	return "/task?repo=" + template.URLQueryEscaper(repoDir) +
		"&commit=" + template.URLQueryEscaper(commit) +
		"&task=" + template.URLQueryEscaper(taskName)
}
