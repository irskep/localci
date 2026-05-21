package localci

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type apiRepoSummary struct {
	RepoDir  string `json:"repo_dir"`
	RepoPath string `json:"repo_path"`
	RepoName string `json:"repo_name"`
}

type apiCommitSummary struct {
	Repo       apiRepoSummary `json:"repo"`
	Commit     string         `json:"commit"`
	Summary    string         `json:"summary"`
	TaskCount  int            `json:"task_count"`
	ActivityAt time.Time      `json:"activity_at"`
}

type apiQueueEntry struct {
	Repo   apiRepoSummary `json:"repo"`
	Commit string         `json:"commit"`
	Task   string         `json:"task"`
}

type apiQueueResponse struct {
	Active  *apiQueueEntry  `json:"active,omitempty"`
	Pending []apiQueueEntry `json:"pending"`
}

type apiHomeResponse struct {
	Repos         []apiRepoSummary   `json:"repos"`
	RecentCommits []apiCommitSummary `json:"recent_commits"`
	Queue         apiQueueResponse   `json:"queue"`
}

type apiRepoResponse struct {
	Repo    apiRepoSummary     `json:"repo"`
	Commits []CommitStatusView `json:"commits"`
}

type apiCommitResponse struct {
	Repo   apiRepoSummary   `json:"repo"`
	Commit CommitStatusView `json:"commit"`
}

type apiTaskResponse struct {
	Repo            apiRepoSummary `json:"repo"`
	Commit          string         `json:"commit"`
	Task            TaskStatusView `json:"task"`
	SelectedAttempt int            `json:"selected_attempt"`
	IsLive          bool           `json:"is_live"`
	PrimaryArtifact string         `json:"primary_artifact"`
	PrimaryLog      string         `json:"primary_log"`
}

type apiArtifactListResponse struct {
	Repo      apiRepoSummary `json:"repo"`
	Commit    string         `json:"commit"`
	Task      string         `json:"task"`
	Attempt   int            `json:"attempt"`
	Artifacts []ArtifactView `json:"artifacts"`
}

type apiArtifactResponse struct {
	Repo     apiRepoSummary `json:"repo"`
	Commit   string         `json:"commit"`
	Task     string         `json:"task"`
	Attempt  int            `json:"attempt"`
	Artifact ArtifactView   `json:"artifact"`
	Content  string         `json:"content"`
}

func (s WebServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	segments, err := splitEscapedPath(r.URL.EscapedPath())
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if len(segments) == 0 || segments[0] != "api" {
		http.NotFound(w, r)
		return
	}
	if len(segments) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPIHome(w, r)
		return
	}

	switch segments[1] {
	case "queue":
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPIQueue(w, r)
		return
	case "repo":
		s.handleAPIRepoRoutes(w, r, segments[2:])
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s WebServer) handleAPIHome(w http.ResponseWriter, _ *http.Request) {
	repos, err := HistoryReader{Paths: s.Paths}.ListRepos()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}

	views, err := s.buildHomeTaskViews(repos)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}

	queueResponse, err := s.apiQueueResponse()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}

	resp := apiHomeResponse{
		Repos:         make([]apiRepoSummary, 0, len(repos)),
		RecentCommits: make([]apiCommitSummary, 0, len(views)),
		Queue:         queueResponse,
	}
	for _, repo := range repos {
		resp.Repos = append(resp.Repos, s.apiRepoSummary(repo.RepoDir))
	}
	for _, view := range views {
		resp.RecentCommits = append(resp.RecentCommits, apiCommitSummary{
			Repo:       s.apiRepoSummary(view.RepoDir),
			Commit:     view.Commit,
			Summary:    commitSummaryText(view),
			TaskCount:  len(view.Tasks),
			ActivityAt: s.commitActivityAt(view),
		})
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s WebServer) handleAPIQueue(w http.ResponseWriter, _ *http.Request) {
	resp, err := s.apiQueueResponse()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s WebServer) handleAPIRepoRoutes(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) == 0 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPIRepoIndex(w, r)
		return
	}

	commitIndex := indexOfSegment(segments, "commit")
	var repoSegments []string
	var tail []string
	if commitIndex < 0 {
		repoSegments = segments
	} else {
		repoSegments = segments[:commitIndex]
		tail = segments[commitIndex:]
	}

	repoDir, err := s.repoDirFromRoute(repoSegments)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}

	if len(tail) == 0 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPIRepo(w, repoDir)
		return
	}

	if len(tail) == 1 && tail[0] == "commit" {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPIRepoCommitIndex(w, repoDir)
		return
	}

	if len(tail) < 2 || tail[0] != "commit" {
		http.NotFound(w, r)
		return
	}
	commit := tail[1]

	if len(tail) == 2 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPICommit(w, repoDir, commit)
		return
	}

	if tail[2] != "task" {
		http.NotFound(w, r)
		return
	}
	if len(tail) == 3 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPITaskIndex(w, repoDir, commit)
		return
	}

	taskName := tail[3]
	if len(tail) == 4 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPITask(w, repoDir, commit, taskName, 0)
		return
	}

	switch tail[4] {
	case "retry":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleAPIRetry(w, repoDir, commit, taskName)
		return
	case "attempt":
		s.handleAPIAttemptRoutes(w, r, repoDir, commit, taskName, tail[5:])
		return
	default:
		http.NotFound(w, r)
		return
	}
}

func (s WebServer) handleAPIRepoIndex(w http.ResponseWriter, _ *http.Request) {
	repos, err := HistoryReader{Paths: s.Paths}.ListRepos()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	resp := make([]apiRepoSummary, 0, len(repos))
	for _, repo := range repos {
		resp = append(resp, s.apiRepoSummary(repo.RepoDir))
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s WebServer) handleAPIRepo(w http.ResponseWriter, repoDir string) {
	commits, err := HistoryReader{Paths: s.Paths}.ListRepoCommits(repoDir)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Errorf("repo not found"))
			return
		}
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	views, err := s.buildRepoCommitViews(repoDir, commits)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiRepoResponse{
		Repo:    s.apiRepoSummary(repoDir),
		Commits: views,
	})
}

func (s WebServer) handleAPIRepoCommitIndex(w http.ResponseWriter, repoDir string) {
	commits, err := HistoryReader{Paths: s.Paths}.ListRepoCommits(repoDir)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Errorf("repo not found"))
			return
		}
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	views, err := s.buildRepoCommitViews(repoDir, commits)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func (s WebServer) handleAPICommit(w http.ResponseWriter, repoDir string, commit string) {
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, apiCommitResponse{
		Repo:   s.apiRepoSummary(repoDir),
		Commit: view,
	})
}

func (s WebServer) handleAPITaskIndex(w http.ResponseWriter, repoDir string, commit string) {
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, view.Tasks)
}

func (s WebServer) handleAPITask(w http.ResponseWriter, repoDir string, commit string, taskName string, selectedAttempt int) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, selectedAttempt)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	primaryArtifact, primaryLog := LoadPrimaryLog(task)
	writeJSON(w, http.StatusOK, apiTaskResponse{
		Repo:            s.apiRepoSummary(repoDir),
		Commit:          commit,
		Task:            task,
		SelectedAttempt: task.Attempt,
		IsLive:          isLatestAttempt(task),
		PrimaryArtifact: primaryArtifact,
		PrimaryLog:      primaryLog,
	})
}

func (s WebServer) handleAPIAttemptRoutes(w http.ResponseWriter, r *http.Request, repoDir string, commit string, taskName string, segments []string) {
	if len(segments) == 0 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		task, err := s.selectedTaskStatus(repoDir, commit, taskName, 0)
		if err != nil {
			if errorsIsRecordNotFound(err) {
				writeAPIError(w, http.StatusNotFound, err)
				return
			}
			writeAPIError(w, http.StatusBadRequest, err)
			return
		}
		writeJSON(w, http.StatusOK, task.Attempts)
		return
	}

	attempt, err := strconv.Atoi(segments[0])
	if err != nil || attempt <= 0 {
		writeAPIError(w, http.StatusBadRequest, fmt.Errorf("attempt must be a positive integer"))
		return
	}
	if len(segments) == 1 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPITask(w, repoDir, commit, taskName, attempt)
		return
	}
	if segments[1] != "artifact" {
		http.NotFound(w, r)
		return
	}
	if len(segments) == 2 {
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPIArtifactIndex(w, repoDir, commit, taskName, attempt)
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	s.handleAPIArtifact(w, repoDir, commit, taskName, attempt, path.Join(segments[2:]...))
}

func (s WebServer) handleAPIArtifactIndex(w http.ResponseWriter, repoDir string, commit string, taskName string, attempt int) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, apiArtifactListResponse{
		Repo:      s.apiRepoSummary(repoDir),
		Commit:    commit,
		Task:      task.Name,
		Attempt:   task.Attempt,
		Artifacts: task.Artifacts,
	})
}

func (s WebServer) handleAPIArtifact(w http.ResponseWriter, repoDir string, commit string, taskName string, attempt int, artifactPath string) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, err)
			return
		}
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}

	artifact, ok := findArtifactByDisplayName(task.Artifacts, artifactPath)
	if !ok {
		writeAPIError(w, http.StatusNotFound, fmt.Errorf("artifact not found"))
		return
	}
	data, err := os.ReadFile(artifact.Path)
	if err != nil {
		writeAPIError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, apiArtifactResponse{
		Repo:     s.apiRepoSummary(repoDir),
		Commit:   commit,
		Task:     task.Name,
		Attempt:  task.Attempt,
		Artifact: artifact,
		Content:  string(data),
	})
}

func (s WebServer) handleAPIRetry(w http.ResponseWriter, repoDir string, commit string, taskName string) {
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if _, ok := findTaskStatus(view.Tasks, taskName); !ok {
		writeAPIError(w, http.StatusNotFound, fmt.Errorf("task not found"))
		return
	}

	active, err := s.Queue.IsTaskActive(repoDir, commit, taskName)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	enqueued := false
	if !active {
		if _, err := s.Queue.Enqueue(repoDir, commit, taskName); err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		enqueued = true
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"repo":     s.apiRepoSummary(repoDir),
		"commit":   commit,
		"task":     taskName,
		"enqueued": enqueued,
	})
}

func (s WebServer) apiQueueResponse() (apiQueueResponse, error) {
	queueEntries, err := s.Queue.List()
	if err != nil {
		return apiQueueResponse{}, err
	}

	active, err := s.Queue.ReadActive()
	if err != nil && !errorsIsRecordNotFound(err) {
		return apiQueueResponse{}, err
	}

	resp := apiQueueResponse{
		Pending: make([]apiQueueEntry, 0, len(queueEntries)),
	}
	if err == nil {
		activeEntry := s.apiQueueEntry(active.RepoDir, active.Commit, active.TaskName)
		resp.Active = &activeEntry
	}
	for _, entry := range queueEntries {
		resp.Pending = append(resp.Pending, s.apiQueueEntry(entry.RepoDir, entry.Commit, entry.TaskName))
	}
	return resp, nil
}

func (s WebServer) apiQueueEntry(repoDir string, commit string, taskName string) apiQueueEntry {
	return apiQueueEntry{
		Repo:   s.apiRepoSummary(repoDir),
		Commit: commit,
		Task:   taskName,
	}
}

func (s WebServer) apiRepoSummary(repoDir string) apiRepoSummary {
	return apiRepoSummary{
		RepoDir:  repoDir,
		RepoPath: s.routeRepoPath(repoDir),
		RepoName: repoLabel(repoDir),
	}
}

func (s WebServer) commitActivityAt(view CommitStatusView) time.Time {
	var latest time.Time
	for _, task := range view.Tasks {
		for _, attempt := range task.Attempts {
			if attempt.Attempt > 0 && task.Attempt == attempt.Attempt {
				if !latest.IsZero() {
					break
				}
			}
		}
	}

	reader := StatusReader{Paths: s.Paths}
	status, err := reader.ReadCommit(view.RepoDir, view.Commit)
	if err == nil {
		return runActivityAt(status.Run)
	}
	return latest
}

func (s WebServer) selectedTaskStatus(repoDir string, commit string, taskName string, selectedAttempt int) (TaskStatusView, error) {
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		return TaskStatusView{}, err
	}
	task, ok := findTaskStatus(view.Tasks, taskName)
	if !ok {
		return TaskStatusView{}, fmt.Errorf("task not found")
	}
	task = applySelectedAttempt(s.Paths, repoDir, commit, task, selectedAttempt)
	if selectedAttempt > 0 && task.Attempt != selectedAttempt {
		return TaskStatusView{}, fmt.Errorf("attempt not found")
	}
	return task, nil
}

func (s WebServer) routeRepoPath(repoDir string) string {
	repoPath, err := RouteRepoPath(s.configuredRepoRoot(), repoDir)
	if err != nil {
		return ""
	}
	return repoPath
}

func (s WebServer) repoDirFromRoute(repoSegments []string) (string, error) {
	root := s.configuredRepoRoot()
	if len(repoSegments) == 0 {
		return "", fmt.Errorf("repo path is required")
	}
	decoded := make([]string, 0, len(repoSegments))
	for _, segment := range repoSegments {
		if segment == "" {
			continue
		}
		decoded = append(decoded, segment)
	}
	return ResolveRepoDir(root, filepath.Join(decoded...))
}

func (s WebServer) configuredRepoRoot() string {
	if strings.TrimSpace(s.RepoRoot) == "" {
		return string(filepath.Separator)
	}
	return filepath.Clean(s.RepoRoot)
}

func splitEscapedPath(escapedPath string) ([]string, error) {
	trimmed := strings.Trim(escapedPath, "/")
	if trimmed == "" {
		return nil, nil
	}
	rawSegments := strings.Split(trimmed, "/")
	segments := make([]string, 0, len(rawSegments))
	for _, raw := range rawSegments {
		decoded, err := url.PathUnescape(raw)
		if err != nil {
			return nil, err
		}
		segments = append(segments, decoded)
	}
	return segments, nil
}

func indexOfSegment(segments []string, target string) int {
	for i, segment := range segments {
		if segment == target {
			return i
		}
	}
	return -1
}

func findArtifactByDisplayName(artifacts []ArtifactView, displayName string) (ArtifactView, bool) {
	for _, artifact := range artifacts {
		if artifact.DisplayName == displayName {
			return artifact, true
		}
	}
	return ArtifactView{}, false
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeAPIError(w, http.StatusMethodNotAllowed, fmt.Errorf("method not allowed"))
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, ErrRecordNotFound)
}
