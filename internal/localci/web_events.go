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

	"github.com/coder/websocket"
)

func (s WebServer) handleAPIEvents(w http.ResponseWriter, r *http.Request, segments []string) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if len(segments) < 2 || segments[len(segments)-1] != "events" {
		http.NotFound(w, r)
		return
	}

	resource, err := canonicalAPIResource(strings.TrimSuffix(requestEscapedPath(r), "/events"))
	if err != nil {
		writeAPIError(w, http.StatusBadRequest, err)
		return
	}

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "")
	ctx := conn.CloseRead(r.Context())

	events, unsubscribe := s.eventHub().Subscribe(resource)
	defer unsubscribe()

	if err := s.writeResourceSnapshot(ctx, conn, resource, EventTypeSnapshot); err != nil {
		_ = conn.Close(websocket.StatusInternalError, err.Error())
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			if event.Type == EventTypeAppend {
				if err := writeWebSocketJSON(ctx, conn, event); err != nil {
					return
				}
				continue
			}
			if err := s.writeResourceSnapshot(ctx, conn, resource, EventTypeReplace); err != nil {
				_ = conn.Close(websocket.StatusInternalError, err.Error())
				return
			}
		}
	}
}

func (s WebServer) writeResourceSnapshot(ctx context.Context, conn *websocket.Conn, resource string, eventType string) error {
	data, err := s.apiSnapshot(resource)
	if err != nil {
		return err
	}
	return writeWebSocketJSON(ctx, conn, APIEvent{
		Type:     eventType,
		Resource: resource,
		Data:     data,
	})
}

func canonicalAPIResource(escapedPath string) (string, error) {
	segments, err := splitEscapedPath(escapedPath)
	if err != nil {
		return "", err
	}
	if len(segments) == 0 {
		return "/api", nil
	}

	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		escaped = append(escaped, url.PathEscape(segment))
	}
	return "/" + path.Join(escaped...), nil
}

func writeWebSocketJSON(ctx context.Context, conn *websocket.Conn, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

func parsePositiveInt(value string, name string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", name)
	}
	return parsed, nil
}

func (s WebServer) apiSnapshot(resource string) (any, error) {
	segments, err := splitEscapedPath(resource)
	if err != nil {
		return nil, err
	}
	if len(segments) == 0 || segments[0] != "api" {
		return nil, fmt.Errorf("unsupported resource: %s", resource)
	}
	if len(segments) == 1 {
		return s.apiHomeSnapshot()
	}

	switch segments[1] {
	case "queue":
		if len(segments) != 2 {
			return nil, fmt.Errorf("unsupported queue resource: %s", resource)
		}
		return s.apiQueueResponse()
	case "repo":
		return s.apiRepoSnapshot(segments[2:])
	default:
		return nil, fmt.Errorf("unsupported resource: %s", resource)
	}
}

func (s WebServer) apiHomeSnapshot() (apiHomeResponse, error) {
	repository := RunRepository{Paths: s.Paths}
	repos, err := repository.ListRepoSummaries()
	if err != nil {
		return apiHomeResponse{}, err
	}
	runPage, err := repository.ListRecentRunPage(time.Time{}, defaultRunListLimit)
	if err != nil {
		return apiHomeResponse{}, err
	}
	repos = mergeRunRepos(repos, runPage.Runs)
	repoSummaries, err := newAPIRepoSummaries(repos)
	if err != nil {
		return apiHomeResponse{}, err
	}
	views, err := s.buildRunCommitSummariesWithRepos(runPage.Runs, repoSummaries)
	if err != nil {
		return apiHomeResponse{}, err
	}
	queueResponse, err := s.apiQueueResponse()
	if err != nil {
		return apiHomeResponse{}, err
	}
	resp := apiHomeResponse{
		Repos:         make([]apiRepoSummary, 0, len(repos)),
		RecentCommits: make([]apiCommitSummary, 0, len(views)),
		Queue:         queueResponse,
		NextBefore:    runPage.NextBefore,
		NewerBefore:   runPage.NewerBefore,
	}
	for _, repo := range repos {
		summary, err := repoSummaries.repo(repo.RepoDir)
		if err != nil {
			return apiHomeResponse{}, err
		}
		resp.Repos = append(resp.Repos, summary)
	}
	resp.RecentCommits = append(resp.RecentCommits, views...)
	return resp, nil
}

func (s WebServer) apiRepoSnapshot(segments []string) (any, error) {
	if len(segments) == 0 {
		repos, err := (RunRepository{Paths: s.Paths}).ListRepoSummaries()
		if err != nil {
			return nil, err
		}
		summaries, err := newAPIRepoSummaries(repos)
		if err != nil {
			return nil, err
		}
		resp := make([]apiRepoSummary, 0, len(repos))
		for _, repo := range repos {
			summary, err := summaries.repo(repo.RepoDir)
			if err != nil {
				return nil, err
			}
			resp = append(resp, summary)
		}
		return resp, nil
	}

	repoSegments, tail := splitRepoRouteTail(segments)

	repoDir, err := s.repoDirFromRoute(repoSegments)
	if err != nil {
		return nil, err
	}
	if len(tail) == 0 {
		runPage, err := (RunRepository{Paths: s.Paths}).ListRepoRunPage(repoDir, time.Time{}, defaultRunListLimit)
		if err != nil {
			return nil, err
		}
		views, err := s.buildRunCommitSummaries(runPage.Runs)
		if err != nil {
			return nil, err
		}
		repo, err := s.apiRepoSummary(repoDir)
		if err != nil {
			return nil, err
		}
		return apiRepoResponse{
			Repo:        repo,
			Commits:     views,
			NextBefore:  runPage.NextBefore,
			NewerBefore: runPage.NewerBefore,
		}, nil
	}
	if tail[0] == "task" {
		if len(tail) != 2 {
			return nil, fmt.Errorf("unsupported repo task resource: %s", strings.Join(tail, "/"))
		}
		return s.apiRepoTaskHistory(repoDir, tail[1])
	}
	if len(tail) < 2 || tail[0] != "commit" {
		return nil, fmt.Errorf("unsupported repo resource: %s", strings.Join(segments, "/"))
	}

	commit := tail[1]
	if len(tail) == 2 {
		view, err := s.buildStatusView(repoDir, commit)
		if err != nil {
			return nil, err
		}
		repo, err := s.apiRepoSummary(repoDir)
		if err != nil {
			return nil, err
		}
		return apiCommitResponse{Repo: repo, Commit: view}, nil
	}
	if tail[2] != "task" {
		return nil, fmt.Errorf("unsupported commit resource: %s", strings.Join(tail, "/"))
	}
	if len(tail) == 3 {
		view, err := s.buildStatusView(repoDir, commit)
		if err != nil {
			return nil, err
		}
		return view.Tasks, nil
	}

	taskName := tail[3]
	if len(tail) == 4 {
		return s.apiTaskSnapshot(repoDir, commit, taskName, 0)
	}
	if tail[4] != "attempt" || len(tail) < 6 {
		return nil, fmt.Errorf("unsupported task resource: %s", strings.Join(tail, "/"))
	}
	attempt, err := parsePositiveInt(tail[5], "attempt")
	if err != nil {
		return nil, err
	}
	if len(tail) == 6 {
		return s.apiTaskSnapshot(repoDir, commit, taskName, attempt)
	}
	if tail[6] != "artifact" {
		return nil, fmt.Errorf("unsupported attempt resource: %s", strings.Join(tail, "/"))
	}
	if len(tail) == 7 {
		task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
		if err != nil {
			return nil, err
		}
		task = s.enrichTaskArtifacts(repoDir, commit, task)
		repo, err := s.apiRepoSummary(repoDir)
		if err != nil {
			return nil, err
		}
		return apiArtifactListResponse{
			Repo:      repo,
			Commit:    commit,
			Task:      task.Name,
			Attempt:   task.Attempt,
			Artifacts: task.Artifacts,
		}, nil
	}
	return s.apiArtifactSnapshot(repoDir, commit, taskName, attempt, path.Join(tail[7:]...))
}

func (s WebServer) apiTaskSnapshot(repoDir string, commit string, taskName string, attempt int) (apiTaskResponse, error) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
	if err != nil {
		return apiTaskResponse{}, err
	}
	task = s.enrichTaskArtifacts(repoDir, commit, task)
	primaryArtifact, primaryLog := LoadPrimaryLog(task)
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		return apiTaskResponse{}, err
	}
	return apiTaskResponse{
		Repo:            repo,
		Commit:          commit,
		Task:            task,
		SelectedAttempt: task.Attempt,
		IsLive:          isLatestAttempt(task),
		PrimaryArtifact: primaryArtifact,
		PrimaryLog:      primaryLog,
	}, nil
}

func (s WebServer) apiArtifactSnapshot(repoDir string, commit string, taskName string, attempt int, artifactPath string) (apiArtifactResponse, error) {
	task, err := s.selectedTaskStatus(repoDir, commit, taskName, attempt)
	if err != nil {
		return apiArtifactResponse{}, err
	}
	task = s.enrichTaskArtifacts(repoDir, commit, task)
	artifact, ok := findArtifactByDisplayName(task.Artifacts, artifactPath)
	if !ok {
		return apiArtifactResponse{}, fmt.Errorf("artifact not found")
	}
	data, err := readTextTaskArtifact(task, artifact.DisplayName)
	if err != nil {
		if errors.Is(err, ErrArtifactNotDisplayable) {
			data = nil
		} else {
			return apiArtifactResponse{}, err
		}
	}
	repo, err := s.apiRepoSummary(repoDir)
	if err != nil {
		return apiArtifactResponse{}, err
	}
	return apiArtifactResponse{
		Repo:     repo,
		Commit:   commit,
		Task:     task.Name,
		Attempt:  task.Attempt,
		Artifact: artifact,
		Content:  string(data),
	}, nil
}

func (s WebServer) eventHub() *EventHub {
	if s.EventHub != nil {
		return s.EventHub
	}
	return NewEventHub()
}
