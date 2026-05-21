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

type HomePageView struct {
	ReposHTML template.HTML
	RunsHTML  template.HTML
}

type RepoPageView struct {
	RepoDir  string
	RepoName string
	RunsHTML template.HTML
}

type CommitPageView struct {
	CommitStatusView
	RepoName  string
	TasksHTML template.HTML
}

type TaskPageView struct {
	RepoDir  string
	RepoName string
	Commit   string
	TaskStatusView
	SelectedAttempt    int
	IsLive             bool
	AttemptHistoryHTML template.HTML
	FilesHTML          template.HTML
}

func (s WebServer) Serve(ctx context.Context, listener net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/repo", s.handleRepo)
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
	repos, err := HistoryReader{Paths: s.Paths}.ListRepos()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	views, err := s.buildHomeTaskViews(repos)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = homeTemplate.Execute(w, HomePageView{
		ReposHTML: template.HTML(renderRepoLinksHTML(repos)),
		RunsHTML:  template.HTML(renderHomeRunsHTML(views)),
	})
}

func (s WebServer) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func (s WebServer) handleRepo(w http.ResponseWriter, r *http.Request) {
	repoDir := r.URL.Query().Get("repo")
	if repoDir == "" {
		http.Error(w, "repo is required", http.StatusBadRequest)
		return
	}

	commits, err := HistoryReader{Paths: s.Paths}.ListRepoCommits(repoDir)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			http.Error(w, "repo not found", http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	views, err := s.buildRepoCommitViews(repoDir, commits)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = repoTemplate.Execute(w, RepoPageView{
		RepoDir:  repoDir,
		RepoName: repoLabel(repoDir),
		RunsHTML: template.HTML(renderRepoRunsHTML(views)),
	})
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
		RepoName:         repoLabel(repoDir),
		TasksHTML:        template.HTML(renderCommitTasksHTML(view)),
	})
}

func (s WebServer) handleTask(w http.ResponseWriter, r *http.Request) {
	repoDir := r.URL.Query().Get("repo")
	commit := r.URL.Query().Get("commit")
	taskName := r.URL.Query().Get("task")
	selectedAttempt := parseAttemptQuery(r.URL.Query().Get("attempt"))

	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task, ok := findTaskStatus(view.Tasks, taskName)
	if ok {
		task = applySelectedAttempt(s.Paths, repoDir, commit, task, selectedAttempt)
		_ = taskTemplate.Execute(w, TaskPageView{
			RepoDir:            repoDir,
			RepoName:           repoLabel(repoDir),
			Commit:             commit,
			TaskStatusView:     task,
			SelectedAttempt:    task.Attempt,
			IsLive:             isLatestAttempt(task),
			AttemptHistoryHTML: template.HTML(renderAttemptHistoryHTML(repoDir, commit, task)),
			FilesHTML:          template.HTML(renderTaskFilesHTML(repoDir, commit, task)),
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
<h2>Repos</h2>
<ul>{{.ReposHTML}}</ul>
<h2>Recent Runs</h2>
<ul>{{.RunsHTML}}</ul>
</body>
</html>`))

var repoTemplate = template.Must(template.New("repo").Parse(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>{{.RepoName}} - localci</title></head>
<body>
<p><a href="/">All repos</a></p>
<h1>{{.RepoName}}</h1>
<ul>{{.RunsHTML}}</ul>
</body>
</html>`))

var commitTemplate = template.Must(template.New("commit").Parse(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>{{.Commit}} - localci</title></head>
<body>
<p><a href="/">All repos</a> / <a href="/repo?repo={{urlquery .RepoDir}}">{{.RepoName}}</a></p>
<h1>{{.Commit}}</h1>
<p>{{len .Tasks}} task{{if ne (len .Tasks) 1}}s{{end}}</p>
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
<p><a href="/">All repos</a> / <a href="/repo?repo={{urlquery .RepoDir}}">{{.RepoName}}</a> / <a href="/commit?repo={{urlquery .RepoDir}}&commit={{urlquery .Commit}}">{{.Commit}}</a></p>
<h1>{{.Name}}</h1>
<p id="task-status">Status: {{.Status}}</p>
<p id="task-attempt">Attempt: {{.Attempt}}</p>
{{if .Failure}}<p id="task-failure">Failure: {{.Failure}}</p>{{end}}
{{if .DurationMilliseconds}}<p id="task-duration">Duration: {{.DurationMilliseconds}}ms</p>{{end}}
<form method="post" action="/retry?repo={{urlquery .RepoDir}}&commit={{urlquery .Commit}}&task={{urlquery .Name}}">
  <button type="submit">Retry task</button>
</form>
<h2>Attempts</h2>
<ul>{{.AttemptHistoryHTML}}</ul>
<h2>Artifacts</h2>
<ul id="task-files">{{.FilesHTML}}</ul>
{{if .IsLive}}
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
    const failureEl = document.getElementById("task-failure");
    const durationEl = document.getElementById("task-duration");
    const filesEl = document.getElementById("task-files");
    if (statusEl) statusEl.textContent = "Status: " + task.status;
    if (attemptEl) attemptEl.textContent = "Attempt: " + task.attempt;
    if (failureEl) failureEl.textContent = task.failure ? "Failure: " + task.failure : "";
    if (durationEl) durationEl.textContent = task.duration_ms ? "Duration: " + task.duration_ms + "ms" : "";
    if (filesEl) filesEl.innerHTML = task.files_html;
  };
})();
</script>
{{end}}
</body>
</html>`))

type statusPayload struct {
	Tasks      []taskPayload `json:"tasks"`
	CommitHTML string        `json:"commit_html"`
}

type taskPayload struct {
	Name       string `json:"name"`
	Attempt    int    `json:"attempt"`
	Status     string `json:"status"`
	Failure    string `json:"failure"`
	DurationMS int64  `json:"duration_ms"`
	FilesHTML  string `json:"files_html"`
}

func renderStatusPayload(view CommitStatusView) ([]byte, error) {
	payload := statusPayload{
		Tasks:      make([]taskPayload, 0, len(view.Tasks)),
		CommitHTML: renderCommitTasksHTML(view),
	}

	for _, task := range view.Tasks {
		payload.Tasks = append(payload.Tasks, taskPayload{
			Name:       task.Name,
			Attempt:    task.Attempt,
			Status:     string(task.Status),
			Failure:    task.Failure,
			DurationMS: task.DurationMilliseconds,
			FilesHTML:  renderTaskFilesHTML(view.RepoDir, view.Commit, task),
		})
	}

	return json.Marshal(payload)
}

func renderCommitTasksHTML(view CommitStatusView) string {
	var b strings.Builder
	for _, task := range view.Tasks {
		fmt.Fprintf(&b, `<li><a href="%s">%s</a> — %s`,
			template.HTMLEscapeString(taskPageURL(view.RepoDir, view.Commit, task.Name)),
			template.HTMLEscapeString(task.ShortName),
			template.HTMLEscapeString(string(task.Status)),
		)
		if task.Attempt > 0 {
			fmt.Fprintf(&b, ` (attempt %d`, task.Attempt)
			if task.AttemptCount > 1 {
				fmt.Fprintf(&b, ` of %d`, task.AttemptCount)
			}
			b.WriteString(`)`)
		}
		if task.DurationMilliseconds > 0 {
			fmt.Fprintf(&b, ` · %dms`, task.DurationMilliseconds)
		}
		if task.Failure != "" {
			fmt.Fprintf(&b, ` · %s`, template.HTMLEscapeString(task.Failure))
		}
		b.WriteString(`</li>`)
	}
	return b.String()
}

func renderRepoLinksHTML(repos []RepoHistory) string {
	var b strings.Builder
	for _, repo := range repos {
		fmt.Fprintf(&b, `<li><a href="%s">%s</a> (%d commit`,
			template.HTMLEscapeString(repoPageURL(repo.RepoDir)),
			template.HTMLEscapeString(repoLabel(repo.RepoDir)),
			len(repo.Commits),
		)
		if len(repo.Commits) != 1 {
			b.WriteString("s")
		}
		b.WriteString(`)</li>`)
	}
	return b.String()
}

func renderRepoRunsHTML(views []CommitStatusView) string {
	var b strings.Builder
	for _, view := range views {
		fmt.Fprintf(&b, `<li><a href="%s">%s</a> — %s`,
			template.HTMLEscapeString(commitPageURL(view.RepoDir, view.Commit)),
			template.HTMLEscapeString(view.Commit),
			template.HTMLEscapeString(commitSummaryText(view)),
		)
		if len(view.Tasks) > 0 {
			b.WriteString(`<ul>`)
			b.WriteString(renderCommitTasksHTML(view))
			b.WriteString(`</ul>`)
		}
		b.WriteString(`</li>`)
	}
	return b.String()
}

func renderHomeRunsHTML(views []CommitStatusView) string {
	var b strings.Builder
	for _, view := range views {
		fmt.Fprintf(&b, `<li><a href="%s">%s</a> <a href="%s"><code>%s</code></a> — %s</li>`,
			template.HTMLEscapeString(repoPageURL(view.RepoDir)),
			template.HTMLEscapeString(repoLabel(view.RepoDir)),
			template.HTMLEscapeString(commitPageURL(view.RepoDir, view.Commit)),
			template.HTMLEscapeString(view.Commit),
			template.HTMLEscapeString(commitSummaryText(view)),
		)
	}
	return b.String()
}

func renderTaskFilesHTML(repoDir string, commit string, task TaskStatusView) string {
	var b strings.Builder
	for _, artifact := range task.Artifacts {
		fmt.Fprintf(&b, `<li><a href="/artifact?repo=%s&commit=%s&task=%s&path=%s">%s</a></li>`,
			template.URLQueryEscaper(repoDir),
			template.URLQueryEscaper(commit),
			template.URLQueryEscaper(task.Name),
			template.URLQueryEscaper(artifact.Path),
			template.HTMLEscapeString(artifact.DisplayName),
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

func taskAttemptPageURL(repoDir string, commit string, taskName string, attempt int) string {
	return taskPageURL(repoDir, commit, taskName) + "&attempt=" + template.URLQueryEscaper(fmt.Sprintf("%d", attempt))
}

func commitPageURL(repoDir string, commit string) string {
	return "/commit?repo=" + template.URLQueryEscaper(repoDir) +
		"&commit=" + template.URLQueryEscaper(commit)
}

func repoPageURL(repoDir string) string {
	return "/repo?repo=" + template.URLQueryEscaper(repoDir)
}

func (s WebServer) buildHomeTaskViews(repos []RepoHistory) ([]CommitStatusView, error) {
	views := []CommitStatusView{}
	for _, repo := range repos {
		commitViews, err := s.buildRepoCommitViews(repo.RepoDir, repo.Commits)
		if err != nil {
			return nil, err
		}
		views = append(views, commitViews...)
	}
	return views, nil
}

func (s WebServer) buildRepoCommitViews(repoDir string, commits []RunRecord) ([]CommitStatusView, error) {
	views := make([]CommitStatusView, 0, len(commits))
	for _, commit := range commits {
		view, err := s.buildStatusView(repoDir, commit.Commit)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func commitSummaryText(view CommitStatusView) string {
	var succeeded int
	var failed int
	var running int
	for _, task := range view.Tasks {
		switch task.Status {
		case ExecutionStatusSucceeded:
			succeeded++
		case ExecutionStatusFailed, ExecutionStatusTimedOut:
			failed++
		case ExecutionStatusQueued, ExecutionStatusRunning:
			running++
		}
	}

	parts := []string{fmt.Sprintf("%d task", len(view.Tasks))}
	if len(view.Tasks) != 1 {
		parts[0] += "s"
	}
	if succeeded > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", succeeded))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	if running > 0 {
		parts = append(parts, fmt.Sprintf("%d live", running))
	}
	return strings.Join(parts, " · ")
}

func parseAttemptQuery(raw string) int {
	var attempt int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &attempt); err != nil || attempt <= 0 {
		return 0
	}
	return attempt
}

func applySelectedAttempt(paths Paths, repoDir string, commit string, task TaskStatusView, selectedAttempt int) TaskStatusView {
	if selectedAttempt <= 0 || task.AttemptCount <= 1 {
		return task
	}

	records, err := listTaskAttemptRecords(paths, repoDir, commit, task.Name)
	if err != nil {
		return task
	}

	for _, record := range records {
		if record.Attempt != selectedAttempt {
			continue
		}
		task.Attempt = record.Attempt
		task.Status = executionStatusFromTaskRecord(record)
		task.DurationMilliseconds = record.DurationMilliseconds
		task.Failure = record.Failure
		task.OutputDir = record.OutputDir
		task.OutputFiles = outputFilesOrNil(record.OutputDir)
		task.Artifacts = buildArtifactViews(record.OutputDir, task.OutputFiles)
		return task
	}

	return task
}

func renderAttemptHistoryHTML(repoDir string, commit string, task TaskStatusView) string {
	var b strings.Builder
	for _, attempt := range task.Attempts {
		fmt.Fprintf(&b, `<li><a href="%s">attempt %d</a> — %s`,
			template.HTMLEscapeString(taskAttemptPageURL(repoDir, commit, task.Name, attempt.Attempt)),
			attempt.Attempt,
			template.HTMLEscapeString(string(attempt.Status)),
		)
		if attempt.DurationMilliseconds > 0 {
			fmt.Fprintf(&b, ` · %dms`, attempt.DurationMilliseconds)
		}
		if attempt.Failure != "" {
			fmt.Fprintf(&b, ` · %s`, template.HTMLEscapeString(attempt.Failure))
		}
		b.WriteString(`</li>`)
	}
	return b.String()
}

func isLatestAttempt(task TaskStatusView) bool {
	return task.Attempt == 0 || len(task.Attempts) == 0 || task.Attempt == task.Attempts[0].Attempt
}

func repoLabel(repoDir string) string {
	label := filepath.Base(filepath.Clean(repoDir))
	if label == "." || label == string(filepath.Separator) || label == "" {
		return repoDir
	}
	return label
}
