package localci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const defaultRunListLimit = 20

type runListPageParams struct {
	Limit  int
	Before time.Time
}

func (s WebServer) handleAPI(w http.ResponseWriter, r *http.Request) {
	escapedPath := requestEscapedPath(r)
	segments, err := splitEscapedPath(escapedPath)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	if len(segments) == 0 || segments[0] != "api" {
		http.NotFound(w, r)
		return
	}
	if len(segments) >= 2 && segments[len(segments)-1] == "events" {
		s.handleAPIEvents(w, r, segments)
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
	case "daemon":
		s.handleAPIDaemon(w, r, segments[2:])
		return
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

func (s WebServer) handleAPIDaemon(w http.ResponseWriter, r *http.Request, segments []string) {
	if len(segments) != 1 || segments[0] != "shutdown" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if s.Shutdown == nil {
		writeAPIError(w, http.StatusServiceUnavailable, fmt.Errorf("daemon shutdown is not configured"))
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	go s.Shutdown()
}

func (s WebServer) handleAPIHome(w http.ResponseWriter, r *http.Request) {
	page, err := parseRunListPageParams(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	repos, err := HistoryReader{Paths: s.Paths}.ListRepos()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}

	views, err := s.buildHomeCommitSummaries(repos)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	views, cursors := pageCommitSummaries(views, page)

	queueResponse, err := s.apiQueueResponse()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}

	resp := apiHomeResponse{
		Repos:         make([]apiRepoSummary, 0, len(repos)),
		RecentCommits: make([]apiCommitSummary, 0, len(views)),
		Queue:         queueResponse,
		NextBefore:    cursors.NextBefore,
		NewerBefore:   cursors.NewerBefore,
	}
	for _, repo := range repos {
		resp.Repos = append(resp.Repos, s.apiRepoSummary(repo.RepoDir))
	}
	resp.RecentCommits = append(resp.RecentCommits, views...)

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
		s.handleAPIRepo(w, r, repoDir)
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
	case "cancel":
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		s.handleAPICancel(w, repoDir, commit, taskName)
		return
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

func (s WebServer) handleAPIRepo(w http.ResponseWriter, r *http.Request, repoDir string) {
	page, err := parseRunListPageParams(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	commits, err := HistoryReader{Paths: s.Paths}.ListRepoCommits(repoDir)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Errorf("repo not found"))
			return
		}
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	views, err := s.buildRepoCommitSummaries(repoDir, commits)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	views, cursors := pageCommitSummaries(views, page)
	writeJSON(w, http.StatusOK, apiRepoResponse{
		Repo:        s.apiRepoSummary(repoDir),
		Commits:     views,
		NextBefore:  cursors.NextBefore,
		NewerBefore: cursors.NewerBefore,
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
	views, err := s.buildRepoCommitSummaries(repoDir, commits)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, views)
}

func parseRunListPageParams(r *http.Request) (runListPageParams, error) {
	page := runListPageParams{Limit: defaultRunListLimit}
	query := r.URL.Query()

	if rawBefore := strings.TrimSpace(query.Get("before")); rawBefore != "" {
		before, err := time.Parse(time.RFC3339Nano, rawBefore)
		if err != nil {
			return runListPageParams{}, fmt.Errorf("before must be an RFC3339 timestamp")
		}
		page.Before = before
	}

	return page, nil
}

type runListCursors struct {
	NextBefore  string
	NewerBefore string
}

func pageCommitSummaries(commits []apiCommitSummary, page runListPageParams) ([]apiCommitSummary, runListCursors) {
	sortAPICommitSummaries(commits)

	start := 0
	if !page.Before.IsZero() {
		start = len(commits)
		for index, commit := range commits {
			if commit.ActivityAt.Before(page.Before) {
				start = index
				break
			}
		}
	}

	end := start + page.Limit
	if end > len(commits) {
		end = len(commits)
	}
	if start > len(commits) {
		start = len(commits)
	}
	pageItems := commits[start:end]

	cursors := runListCursors{}
	if end < len(commits) && len(pageItems) > 0 {
		cursors.NextBefore = pageItems[len(pageItems)-1].ActivityAt.Format(time.RFC3339Nano)
	}
	if start > 0 {
		previousStart := start - page.Limit
		if previousStart <= 0 {
			cursors.NewerBefore = ""
		} else {
			cursors.NewerBefore = commits[previousStart-1].ActivityAt.Format(time.RFC3339Nano)
		}
	}
	return pageItems, cursors
}

func sortAPICommitSummaries(commits []apiCommitSummary) {
	sort.Slice(commits, func(i int, j int) bool {
		left := commits[i]
		right := commits[j]
		if left.ActivityAt.Equal(right.ActivityAt) {
			if left.Repo.RepoPath == right.Repo.RepoPath {
				return left.Commit > right.Commit
			}
			return left.Repo.RepoPath < right.Repo.RepoPath
		}
		return left.ActivityAt.After(right.ActivityAt)
	})
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
	if segments[len(segments)-1] == "reveal" {
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if len(segments) == 3 {
			http.NotFound(w, r)
			return
		}
		s.handleAPIRevealArtifact(w, repoDir, commit, taskName, attempt, path.Join(segments[2:len(segments)-1]...))
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	s.handleAPIArtifact(w, repoDir, commit, taskName, attempt, path.Join(segments[2:]...))
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

	active, err := s.Queue.ReadActive()
	if err != nil && !errorsIsRecordNotFound(err) {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	var entry QueueEntry
	enqueued := true
	if err == nil && active.RepoDir == repoDir && active.Commit == commit && active.TaskName == taskName {
		entry = active.QueueEntry
		enqueued = false
	} else {
		var enqueueErr error
		entry, enqueueErr = s.Queue.EnqueueRun(repoDir, commit, []string{taskName})
		if enqueueErr != nil {
			writeAPIError(w, http.StatusInternalServerError, enqueueErr)
			return
		}
		if s.Events != nil {
			s.Events.EntryChanged(entry)
		}
	}

	attempt, err := s.Queue.NextAttempt(repoDir, commit, taskName)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	route, err := AttemptRoutePath(s.configuredRepoRoot(), repoDir, commit, taskName, attempt)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiRetryResponse{
		Repo:     s.apiRepoSummary(repoDir),
		Commit:   commit,
		Task:     taskName,
		Attempt:  attempt,
		URL:      route,
		Enqueued: enqueued,
	})
}

func (s WebServer) handleAPICancel(w http.ResponseWriter, repoDir string, commit string, taskName string) {
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	task, ok := findTaskStatus(view.Tasks, taskName)
	if !ok {
		writeAPIError(w, http.StatusNotFound, fmt.Errorf("task not found"))
		return
	}

	result, err := s.Queue.Cancel(repoDir, commit, taskName)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	if s.Events != nil {
		if result.Active || result.Pending > 0 {
			s.Events.EntryChanged(QueueEntry{
				Kind:     QueueEntryKindTask,
				RepoDir:  repoDir,
				RepoID:   normalizeRepoDir(repoDir),
				Commit:   commit,
				TaskName: taskName,
				TaskKey:  sanitizeTaskName(taskName),
				Attempt:  task.Attempt,
			})
		}
	}

	writeJSON(w, http.StatusOK, apiCancelResponse{
		Repo:     s.apiRepoSummary(repoDir),
		Commit:   commit,
		Task:     taskName,
		Active:   result.Active,
		Pending:  result.Pending,
		Canceled: result.Active || result.Pending > 0,
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
		activeEntry := s.apiQueueEntry(active.QueueEntry)
		resp.Active = &activeEntry
	}
	for _, entry := range queueEntries {
		resp.Pending = append(resp.Pending, s.apiQueueEntry(entry))
	}
	return resp, nil
}

func (s WebServer) apiQueueEntry(entry QueueEntry) apiQueueEntry {
	return apiQueueEntry{
		Repo:    s.apiRepoSummary(entry.RepoDir),
		Commit:  entry.Commit,
		Task:    entry.TaskName,
		Attempt: entry.Attempt,
	}
}

func (s WebServer) apiRepoSummary(repoDir string) apiRepoSummary {
	repoPath := s.canonicalRepoPath(repoDir)
	return apiRepoSummary{
		RepoDir:  repoDir,
		RepoPath: repoPath,
	}
}

func (s WebServer) buildHomeCommitSummaries(repos []RepoHistory) ([]apiCommitSummary, error) {
	views := []apiCommitSummary{}
	for _, repo := range repos {
		commitViews, err := s.buildRepoCommitSummaries(repo.RepoDir, repo.Commits)
		if err != nil {
			return nil, err
		}
		views = append(views, commitViews...)
	}
	return views, nil
}

func (s WebServer) buildRepoCommitSummaries(repoDir string, commits []RunRecord) ([]apiCommitSummary, error) {
	queue, err := s.Queue.List()
	if err != nil {
		return nil, err
	}

	active, err := s.Queue.ReadActive()
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return nil, err
	}
	var activePtr *ActiveTask
	if err == nil {
		activePtr = &active
	}

	views := make([]apiCommitSummary, 0, len(commits))
	for _, commit := range commits {
		views = append(views, s.buildCommitSummary(repoDir, commit, commit.DiscoveredTasks, queue, activePtr))
	}
	return views, nil
}

func (s WebServer) buildCommitSummary(repoDir string, run RunRecord, discovered []Task, queued []QueueEntry, active *ActiveTask) apiCommitSummary {
	records := map[string]TaskRecord{}
	for _, record := range run.TaskResults {
		records[record.Name] = record
	}

	queuedByTask := map[string]int{}
	queuedTaskSeen := map[string]bool{}
	for taskName, entries := range queuedEntriesByTask(queued, repoDir, run.Commit, discovered) {
		for _, entry := range entries {
			queuedTaskSeen[taskName] = true
			if entry.Attempt > queuedByTask[taskName] {
				queuedByTask[taskName] = entry.Attempt
			}
		}
	}

	tasks := make([]apiTaskSummary, 0, len(discovered))
	for _, task := range discovered {
		summary := apiTaskSummary{
			Name:      task.Name,
			ShortName: trimTaskPrefix(task.Name),
			Status:    ExecutionStatusNotRun,
		}
		if record, ok := records[task.Name]; ok {
			summary.Attempt = record.Attempt
			summary.Status = executionStatusFromTaskRecord(record)
			summary.DurationMilliseconds = record.DurationMilliseconds
			summary.Failure = record.Failure
			if record.Attempt > 0 {
				summary.AttemptCount = record.Attempt
			}
		}
		if active != nil && active.RepoDir == repoDir && active.Commit == run.Commit && active.TaskName == task.Name {
			summary.Status = ExecutionStatusRunning
			summary.Attempt = active.Attempt
			if active.Attempt > summary.AttemptCount {
				summary.AttemptCount = active.Attempt
			}
		} else if queuedTaskSeen[task.Name] {
			queuedAttempt := queuedByTask[task.Name]
			summary.Status = ExecutionStatusQueued
			summary.Attempt = queuedAttempt
			if queuedAttempt > summary.AttemptCount {
				summary.AttemptCount = queuedAttempt
			}
		}
		tasks = append(tasks, summary)
	}

	return apiCommitSummary{
		Repo:        s.apiRepoSummary(repoDir),
		Commit:      run.Commit,
		Annotations: cloneAnnotations(run.Annotations),
		Tasks:       tasks,
		ActivityAt:  RunActivityAt(run),
	}
}

func (s WebServer) discoverTasks(repoDir string) ([]Task, error) {
	if s.DiscoverTasks == nil {
		return nil, fmt.Errorf("task discovery is not configured")
	}
	return s.DiscoverTasks(context.Background(), repoDir)
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
	task = ApplySelectedAttempt(s.Paths, repoDir, commit, task, selectedAttempt)
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

func (s WebServer) canonicalRepoPath(repoDir string) string {
	repoPath, err := CanonicalRepoPath(s.configuredRepoRoot(), repoDir)
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

func requestEscapedPath(r *http.Request) string {
	if r.URL.RawPath != "" {
		return r.URL.RawPath
	}
	if r.RequestURI != "" {
		path, _, _ := strings.Cut(r.RequestURI, "?")
		if path != "" {
			return path
		}
	}
	return r.URL.EscapedPath()
}

func indexOfSegment(segments []string, target string) int {
	for i, segment := range segments {
		if segment == target {
			return i
		}
	}
	return -1
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
