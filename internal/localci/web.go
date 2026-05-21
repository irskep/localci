package localci

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type WebServer struct {
	Paths         Paths
	Queue         QueueStore
	DiscoverTasks func(context.Context, string) ([]Task, error)
}

type TaskPageView struct {
	RepoDir string
	Commit  string
	TaskStatusView
}

func (s WebServer) Serve(ctx context.Context, listener net.Listener) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleHome)
	mux.HandleFunc("/commit", s.handleCommit)
	mux.HandleFunc("/task", s.handleTask)
	mux.HandleFunc("/artifact", s.handleArtifact)

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

	_ = commitTemplate.Execute(w, view)
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

	for _, task := range view.Tasks {
		if task.Name == taskName {
			_ = taskTemplate.Execute(w, TaskPageView{
				RepoDir:        repoDir,
				Commit:         commit,
				TaskStatusView: task,
			})
			return
		}
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
<ul>
{{range .Tasks}}
  <li><a href="/task?repo={{urlquery $.RepoDir}}&commit={{urlquery $.Commit}}&task={{urlquery .Name}}">{{.Name}}</a> — {{.Status}}</li>
{{end}}
</ul>
</body>
</html>`))

var taskTemplate = template.Must(template.New("task").Parse(`<!doctype html>
<html>
<head><meta charset="utf-8"><title>{{.Name}} - localci</title></head>
<body>
<h1>{{.Name}}</h1>
<p>Status: {{.Status}}</p>
<p>Output: {{.OutputDir}}</p>
<ul>
{{range .OutputFiles}}
  <li><a href="/artifact?repo={{urlquery $.RepoDir}}&commit={{urlquery $.Commit}}&task={{urlquery $.Name}}&path={{urlquery .}}">{{.}}</a></li>
{{end}}
</ul>
</body>
</html>`))
