package localci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path"
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
	repository := RunRepository{Paths: s.Paths}
	repos, err := repository.ListRepoSummaries()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}

	runPage, err := repository.ListRecentRunPage(page.Before, page.Limit)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	views, err := s.buildRunCommitSummaries(runPage.Runs)
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
		NextBefore:    runPage.NextBefore,
		NewerBefore:   runPage.NewerBefore,
	}
	for _, repo := range repos {
		summary, err := s.apiRepoSummary(repo.RepoDir)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		resp.Repos = append(resp.Repos, summary)
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

	repoSegments, tail := splitRepoRouteTail(segments)

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

	if tail[0] == "task" {
		if len(tail) != 2 {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		s.handleAPIRepoTaskHistory(w, repoDir, tail[1])
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
	repos, err := (RunRepository{Paths: s.Paths}).ListRepoSummaries()
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	resp := make([]apiRepoSummary, 0, len(repos))
	for _, repo := range repos {
		summary, err := s.apiRepoSummary(repo.RepoDir)
		if err != nil {
			writeAPIError(w, http.StatusInternalServerError, err)
			return
		}
		resp = append(resp, summary)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s WebServer) handleAPIRepo(w http.ResponseWriter, r *http.Request, repoDir string) {
	page, err := parseRunListPageParams(r)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	runPage, err := (RunRepository{Paths: s.Paths}).ListRepoRunPage(repoDir, page.Before, page.Limit)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Errorf("repo not found"))
			return
		}
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	views, err := s.buildRunCommitSummaries(runPage.Runs)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiRepoResponse{
		Repo:        repo,
		Commits:     views,
		NextBefore:  runPage.NextBefore,
		NewerBefore: runPage.NewerBefore,
	})
}

func (s WebServer) handleAPIRepoTaskHistory(w http.ResponseWriter, repoDir string, taskName string) {
	resp, err := s.apiRepoTaskHistory(repoDir, taskName)
	if err != nil {
		if errorsIsRecordNotFound(err) {
			writeAPIError(w, http.StatusNotFound, fmt.Errorf("repo not found"))
			return
		}
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
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

func (s WebServer) handleAPICommit(w http.ResponseWriter, repoDir string, commit string) {
	view, err := s.buildStatusView(repoDir, commit)
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiCommitResponse{
		Repo:   repo,
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
	task = s.enrichTaskArtifacts(repoDir, commit, task)
	primaryArtifact, primaryLog := LoadPrimaryLog(task)
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiTaskResponse{
		Repo:            repo,
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
		entry, enqueueErr = s.Queue.Enqueue(repoDir, commit, taskName)
		if enqueueErr != nil {
			writeAPIError(w, http.StatusInternalServerError, enqueueErr)
			return
		}
		if s.Events != nil {
			s.Events.EntryChanged(entry)
		}
	}

	route, err := AttemptRoutePath(repoDir, commit, taskName, entry.Attempt)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiRetryResponse{
		Repo:     repo,
		Commit:   commit,
		Task:     taskName,
		Attempt:  entry.Attempt,
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

	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		writeAPIError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, apiCancelResponse{
		Repo:     repo,
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
		activeEntry, err := s.apiQueueEntry(active.QueueEntry)
		if err != nil {
			return apiQueueResponse{}, err
		}
		resp.Active = &activeEntry
	}
	for _, entry := range queueEntries {
		queueEntry, err := s.apiQueueEntry(entry)
		if err != nil {
			return apiQueueResponse{}, err
		}
		resp.Pending = append(resp.Pending, queueEntry)
	}
	return resp, nil
}

func (s WebServer) apiQueueEntry(entry QueueEntry) (apiQueueEntry, error) {
	repo, err := s.apiRepoSummary(entry.RepoDir)
	if err != nil {
		return apiQueueEntry{}, err
	}
	var artifacts []ArtifactView
	if entry.TaskName != "" && entry.Attempt > 0 {
		if task, err := s.selectedTaskStatus(entry.RepoDir, entry.Commit, entry.TaskName, entry.Attempt); err == nil {
			task = s.enrichTaskArtifacts(entry.RepoDir, entry.Commit, task)
			artifacts = markedArtifactViews(task)
		}
	}
	return apiQueueEntry{
		Repo:      repo,
		Commit:    entry.Commit,
		Task:      entry.TaskName,
		Attempt:   entry.Attempt,
		Artifacts: artifacts,
	}, nil
}

func (s WebServer) apiRepoSummary(repoDir string) (apiRepoSummary, error) {
	repoPath, err := RouteRepoPath(repoDir)
	if err != nil {
		return apiRepoSummary{}, err
	}
	repoLabel, err := s.repoLabel(repoDir)
	if err != nil {
		return apiRepoSummary{}, err
	}
	return apiRepoSummary{
		RepoDir:   repoDir,
		RepoPath:  repoPath,
		RepoLabel: repoLabel,
	}, nil
}

func (s WebServer) buildRepoCommitSummaries(repoDir string, commits []RunRecord) ([]apiCommitSummary, error) {
	for index := range commits {
		if commits[index].RepoDir == "" {
			commits[index].RepoDir = repoDir
		}
	}
	return s.buildRunCommitSummaries(commits)
}

func (s WebServer) buildRunCommitSummaries(runs []RunRecord) ([]apiCommitSummary, error) {
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

	views := make([]apiCommitSummary, 0, len(runs))
	for _, run := range runs {
		view, err := s.buildCommitSummary(run.RepoDir, run, run.DiscoveredTasks, queue, activePtr)
		if err != nil {
			return nil, err
		}
		views = append(views, view)
	}
	return views, nil
}

func (s WebServer) apiRepoTaskHistory(repoDir string, taskName string) (apiRepoTaskHistoryResponse, error) {
	commits, err := HistoryReader{Paths: s.Paths}.ListRepoCommits(repoDir)
	if err != nil {
		return apiRepoTaskHistoryResponse{}, err
	}
	summaries, err := s.buildRepoCommitSummaries(repoDir, commits)
	if err != nil {
		return apiRepoTaskHistoryResponse{}, err
	}
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		return apiRepoTaskHistoryResponse{}, err
	}

	resp := apiRepoTaskHistoryResponse{
		Repo:      repo,
		Task:      taskName,
		ShortName: trimTaskPrefix(taskName),
		Runs:      make([]apiRepoTaskHistoryItem, 0, len(summaries)),
	}
	for _, summary := range summaries {
		task, ok := findAPITaskSummary(summary.Tasks, taskName)
		if !ok {
			continue
		}
		resp.ShortName = task.ShortName
		resp.Runs = append(resp.Runs, apiRepoTaskHistoryItem{
			Commit:      summary.Commit,
			Annotations: cloneAnnotations(summary.Annotations),
			Task:        task,
			ActivityAt:  summary.ActivityAt,
		})
	}
	return resp, nil
}

func findAPITaskSummary(tasks []apiTaskSummary, name string) (apiTaskSummary, bool) {
	for _, task := range tasks {
		if task.Name == name {
			return task, true
		}
	}
	return apiTaskSummary{}, false
}

func (s WebServer) buildCommitSummary(repoDir string, run RunRecord, discovered []Task, queued []QueueEntry, active *ActiveTask) (apiCommitSummary, error) {
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
			summary.Artifacts = s.enrichMarkedArtifacts(repoDir, run.Commit, TaskStatusView{
				Name:            task.Name,
				Attempt:         record.Attempt,
				OutputDir:       record.OutputDir,
				MarkedArtifacts: record.MarkedArtifacts,
				Artifacts:       buildArtifactViews(record.OutputDir, outputFilesOrNil(record.OutputDir), record.MarkedArtifacts),
			})
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

	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		return apiCommitSummary{}, err
	}
	return apiCommitSummary{
		Repo:        repo,
		Commit:      run.Commit,
		Annotations: cloneAnnotations(run.Annotations),
		Tasks:       tasks,
		ActivityAt:  RunActivityAt(run),
	}, nil
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

func (s WebServer) repoDirFromRoute(repoSegments []string) (string, error) {
	return RepoDirFromRoute(repoSegments)
}

func (s WebServer) repoLabel(repoDir string) (string, error) {
	repos, err := (RunRepository{Paths: s.Paths}).ListRepoSummaries()
	if err != nil {
		return "", err
	}
	repoDirs := make([]string, 0, len(repos)+1)
	for _, repo := range repos {
		repoDirs = append(repoDirs, repo.RepoDir)
	}
	return RepoDisplayLabel(repoDir, repoDirs), nil
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

func splitRepoRouteTail(segments []string) ([]string, []string) {
	commitIndex := indexOfSegment(segments, "commit")
	taskIndex := indexOfSegment(segments, "task")
	tailIndex := -1
	switch {
	case commitIndex >= 0 && taskIndex >= 0:
		tailIndex = min(commitIndex, taskIndex)
	case commitIndex >= 0:
		tailIndex = commitIndex
	case taskIndex >= 0:
		tailIndex = taskIndex
	}
	if tailIndex < 0 {
		return segments, nil
	}
	return segments[:tailIndex], segments[tailIndex:]
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
