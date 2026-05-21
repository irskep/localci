package localci

import (
	"errors"
	"html/template"
	"net/http"
	"os"
	"path"
	"strings"
)

func (s WebServer) handleQueuePage(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s WebServer) handleRoutePage(w http.ResponseWriter, r *http.Request) {
	segments, err := splitEscapedPath(r.URL.EscapedPath())
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if len(segments) == 0 || segments[0] != "repo" {
		http.NotFound(w, r)
		return
	}

	commitIndex := indexOfSegment(segments[1:], "commit")
	var repoSegments []string
	var tail []string
	if commitIndex < 0 {
		repoSegments = segments[1:]
	} else {
		repoSegments = segments[1 : commitIndex+1]
		tail = segments[commitIndex+1:]
	}

	repoDir, err := s.repoDirFromRoute(repoSegments)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(tail) == 0 {
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
		return
	}

	if len(tail) < 2 || tail[0] != "commit" {
		http.NotFound(w, r)
		return
	}
	commit := tail[1]

	if len(tail) == 2 {
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
		return
	}

	if len(tail) < 4 || tail[2] != "task" {
		http.NotFound(w, r)
		return
	}
	taskName := tail[3]

	if len(tail) == 4 {
		s.renderTaskPage(w, repoDir, commit, taskName, 0)
		return
	}

	if tail[4] != "attempt" || len(tail) < 6 {
		http.NotFound(w, r)
		return
	}
	attempt := parseAttemptQuery(tail[5])
	if attempt <= 0 {
		http.Error(w, "attempt must be a positive integer", http.StatusBadRequest)
		return
	}

	if len(tail) == 6 {
		s.renderTaskPage(w, repoDir, commit, taskName, attempt)
		return
	}
	if tail[6] != "artifact" || len(tail) < 8 {
		http.NotFound(w, r)
		return
	}
	s.renderArtifactPath(w, repoDir, commit, taskName, attempt, path.Join(tail[7:]...))
}

func (s WebServer) renderTaskPage(w http.ResponseWriter, repoDir string, commit string, taskName string, selectedAttempt int) {
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task, ok := findTaskStatus(view.Tasks, taskName)
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	task = applySelectedAttempt(s.Paths, repoDir, commit, task, selectedAttempt)
	primaryArtifact, primaryLog := LoadPrimaryLog(task)
	_ = taskTemplate.Execute(w, TaskPageView{
		RepoDir:            repoDir,
		RepoName:           repoLabel(repoDir),
		Commit:             commit,
		TaskStatusView:     task,
		SelectedAttempt:    task.Attempt,
		IsLive:             isLatestAttempt(task),
		PrimaryArtifact:    primaryArtifact,
		PrimaryLog:         primaryLog,
		AttemptHistoryHTML: template.HTML(renderAttemptHistoryHTML(repoDir, commit, task)),
		FilesHTML:          template.HTML(renderTaskFilesHTML(repoDir, commit, task)),
	})
}

func (s WebServer) renderArtifactPath(w http.ResponseWriter, repoDir string, commit string, taskName string, attempt int, artifactName string) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	artifact, ok := findArtifactByDisplayName(task.Artifacts, artifactName)
	if !ok {
		http.Error(w, "artifact not found", http.StatusNotFound)
		return
	}
	if !strings.HasPrefix(artifact.Path, task.OutputDir+string(os.PathSeparator)) && artifact.Path != task.OutputDir {
		http.Error(w, "artifact path is outside task output", http.StatusBadRequest)
		return
	}
	data, err := os.ReadFile(artifact.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write(data)
}
