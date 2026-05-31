package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/coder/websocket"

	"localci/internal/localci"
)

type tuiRepoSummary struct {
	RepoDir   string `json:"repo_dir"`
	RepoPath  string `json:"repo_path"`
	RepoLabel string `json:"repo_label"`
}

func (r tuiRepoSummary) DisplayLabel() string {
	return r.RepoLabel
}

type tuiQueueEntry struct {
	Repo    tuiRepoSummary `json:"repo"`
	Commit  string         `json:"commit"`
	Task    string         `json:"task"`
	Attempt int            `json:"attempt"`
}

type tuiQueueResponse struct {
	Active  *tuiQueueEntry  `json:"active,omitempty"`
	Pending []tuiQueueEntry `json:"pending"`
}

type tuiHomeResponse struct {
	Repos         []tuiRepoSummary   `json:"repos"`
	RecentCommits []tuiCommitSummary `json:"recent_commits"`
	Queue         tuiQueueResponse   `json:"queue"`
	NextBefore    string             `json:"next_before,omitempty"`
	NewerBefore   string             `json:"newer_before,omitempty"`
}

type tuiRepoResponse struct {
	Repo        tuiRepoSummary     `json:"repo"`
	Commits     []tuiCommitSummary `json:"commits"`
	NextBefore  string             `json:"next_before,omitempty"`
	NewerBefore string             `json:"newer_before,omitempty"`
}

type tuiRepoTaskHistoryResponse struct {
	Repo      tuiRepoSummary           `json:"repo"`
	Task      string                   `json:"task"`
	ShortName string                   `json:"short_name"`
	Runs      []tuiRepoTaskHistoryItem `json:"runs"`
}

type tuiRepoTaskHistoryItem struct {
	Commit      string            `json:"commit"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Task        tuiTaskSummary    `json:"task"`
	ActivityAt  time.Time         `json:"activity_at"`
}

type tuiCommitSummary struct {
	Repo        tuiRepoSummary    `json:"repo"`
	Commit      string            `json:"commit"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Tasks       []tuiTaskSummary  `json:"tasks"`
	ActivityAt  time.Time         `json:"activity_at"`
}

type tuiTaskSummary struct {
	Name                 string                  `json:"name"`
	ShortName            string                  `json:"short_name"`
	Attempt              int                     `json:"attempt"`
	AttemptCount         int                     `json:"attempt_count"`
	Status               localci.ExecutionStatus `json:"status"`
	DurationMilliseconds int64                   `json:"duration_ms"`
	Failure              string                  `json:"failure"`
}

type tuiCommitResponse struct {
	Repo   tuiRepoSummary           `json:"repo"`
	Commit localci.CommitStatusView `json:"commit"`
}

type tuiTaskResponse struct {
	Repo            tuiRepoSummary         `json:"repo"`
	Commit          string                 `json:"commit"`
	Task            localci.TaskStatusView `json:"task"`
	SelectedAttempt int                    `json:"selected_attempt"`
	IsLive          bool                   `json:"is_live"`
	PrimaryArtifact string                 `json:"primary_artifact"`
	PrimaryLog      string                 `json:"primary_log"`
}

type tuiArtifactListResponse struct {
	Repo      tuiRepoSummary         `json:"repo"`
	Commit    string                 `json:"commit"`
	Task      string                 `json:"task"`
	Attempt   int                    `json:"attempt"`
	Artifacts []localci.ArtifactView `json:"artifacts"`
}

type tuiArtifactResponse struct {
	Repo     tuiRepoSummary       `json:"repo"`
	Commit   string               `json:"commit"`
	Task     string               `json:"task"`
	Attempt  int                  `json:"attempt"`
	Artifact localci.ArtifactView `json:"artifact"`
	Content  string               `json:"content"`
}

type tuiRetryResponse struct {
	Repo     tuiRepoSummary `json:"repo"`
	Commit   string         `json:"commit"`
	Task     string         `json:"task"`
	Attempt  int            `json:"attempt"`
	URL      string         `json:"url"`
	Enqueued bool           `json:"enqueued"`
}

type tuiCancelResponse struct {
	Repo     tuiRepoSummary `json:"repo"`
	Commit   string         `json:"commit"`
	Task     string         `json:"task"`
	Active   bool           `json:"active"`
	Pending  int            `json:"pending"`
	Canceled bool           `json:"canceled"`
}

type tuiRevealArtifactResponse struct {
	Path string `json:"path"`
	OK   bool   `json:"ok"`
}

type tuiAPIEvent struct {
	Type     string          `json:"type"`
	Resource string          `json:"resource"`
	Data     json.RawMessage `json:"data,omitempty"`
	Offset   int64           `json:"offset,omitempty"`
	Text     string          `json:"text,omitempty"`
	Message  string          `json:"message,omitempty"`
}

type tuiClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func newTUIClient(base string) (*tuiClient, error) {
	parsed, err := url.Parse(strings.TrimSpace(base))
	if err != nil {
		return nil, err
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("daemon HTTP URL is invalid: %s", base)
	}
	return &tuiClient{
		baseURL:    parsed,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}, nil
}

func (c *tuiClient) get(ctx context.Context, apiPath string, out any) error {
	return c.doJSON(ctx, http.MethodGet, apiPath, out)
}

func (c *tuiClient) post(ctx context.Context, apiPath string, out any) error {
	return c.doJSON(ctx, http.MethodPost, apiPath, out)
}

func (c *tuiClient) doJSON(ctx context.Context, method string, apiPath string, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.urlFor(apiPath).String(), nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(data, &apiErr); err == nil && apiErr.Error != "" {
			return errors.New(apiErr.Error)
		}
		return fmt.Errorf("%s %s failed: %s", method, apiPath, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	return dec.Decode(out)
}

func (c *tuiClient) readEvent(ctx context.Context, apiPath string) (tuiAPIEvent, error) {
	wsURL := c.urlFor(strings.TrimRight(apiPath, "/") + "/events")
	switch wsURL.Scheme {
	case "https":
		wsURL.Scheme = "wss"
	default:
		wsURL.Scheme = "ws"
	}

	conn, _, err := websocket.Dial(ctx, wsURL.String(), nil)
	if err != nil {
		return tuiAPIEvent{}, err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, data, err := conn.Read(ctx)
	if err != nil {
		return tuiAPIEvent{}, err
	}
	var event tuiAPIEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return tuiAPIEvent{}, err
	}
	return event, nil
}

func (c *tuiClient) streamEvents(ctx context.Context, apiPath string, handle func(tuiAPIEvent) bool) error {
	wsURL := c.urlFor(strings.TrimRight(apiPath, "/") + "/events")
	switch wsURL.Scheme {
	case "https":
		wsURL.Scheme = "wss"
	default:
		wsURL.Scheme = "ws"
	}

	conn, _, err := websocket.Dial(ctx, wsURL.String(), nil)
	if err != nil {
		return err
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var event tuiAPIEvent
		if err := json.Unmarshal(data, &event); err != nil {
			return err
		}
		if !handle(event) {
			return nil
		}
	}
}

func (c *tuiClient) urlFor(apiPath string) *url.URL {
	next := *c.baseURL
	if strings.TrimSpace(apiPath) == "" {
		apiPath = "/api"
	}
	if !strings.HasPrefix(apiPath, "/") {
		apiPath = "/" + apiPath
	}
	escapedPath := path.Clean(apiPath)
	if strings.HasSuffix(apiPath, "/") && !strings.HasSuffix(escapedPath, "/") {
		escapedPath += "/"
	}
	if decodedPath, err := url.PathUnescape(escapedPath); err == nil {
		next.Path = decodedPath
		next.RawPath = escapedPath
	} else {
		next.Path = escapedPath
		next.RawPath = ""
	}
	next.RawQuery = ""
	return &next
}

func (c *tuiClient) loadHome(ctx context.Context) (tuiHomeResponse, error) {
	var resp tuiHomeResponse
	err := c.get(ctx, "/api", &resp)
	return resp, err
}

func (c *tuiClient) loadQueue(ctx context.Context) (tuiQueueResponse, error) {
	var resp tuiQueueResponse
	err := c.get(ctx, "/api/queue", &resp)
	return resp, err
}

func (c *tuiClient) loadRepoIndex(ctx context.Context) ([]tuiRepoSummary, error) {
	var resp []tuiRepoSummary
	err := c.get(ctx, "/api/repo", &resp)
	return resp, err
}

func (c *tuiClient) loadRepo(ctx context.Context, apiPath string) (tuiRepoResponse, error) {
	var resp tuiRepoResponse
	err := c.get(ctx, apiPath, &resp)
	return resp, err
}

func (c *tuiClient) loadRepoTaskHistory(ctx context.Context, apiPath string) (tuiRepoTaskHistoryResponse, error) {
	var resp tuiRepoTaskHistoryResponse
	err := c.get(ctx, apiPath, &resp)
	return resp, err
}

func (c *tuiClient) loadCommit(ctx context.Context, apiPath string) (tuiCommitResponse, error) {
	var resp tuiCommitResponse
	err := c.get(ctx, apiPath, &resp)
	return resp, err
}

func (c *tuiClient) loadTask(ctx context.Context, apiPath string) (tuiTaskResponse, error) {
	var resp tuiTaskResponse
	err := c.get(ctx, apiPath, &resp)
	return resp, err
}

func (c *tuiClient) loadArtifactList(ctx context.Context, apiPath string) (tuiArtifactListResponse, error) {
	var resp tuiArtifactListResponse
	err := c.get(ctx, apiPath, &resp)
	return resp, err
}

func (c *tuiClient) loadArtifact(ctx context.Context, apiPath string) (tuiArtifactResponse, error) {
	var resp tuiArtifactResponse
	err := c.get(ctx, apiPath, &resp)
	return resp, err
}

func (c *tuiClient) retryTask(ctx context.Context, apiPath string) (tuiRetryResponse, error) {
	var resp tuiRetryResponse
	err := c.post(ctx, strings.TrimRight(apiPath, "/")+"/retry", &resp)
	return resp, err
}

func (c *tuiClient) cancelTask(ctx context.Context, apiPath string) (tuiCancelResponse, error) {
	var resp tuiCancelResponse
	err := c.post(ctx, strings.TrimRight(apiPath, "/")+"/cancel", &resp)
	return resp, err
}

func (c *tuiClient) revealArtifact(ctx context.Context, apiPath string) (tuiRevealArtifactResponse, error) {
	var resp tuiRevealArtifactResponse
	err := c.post(ctx, strings.TrimRight(apiPath, "/")+"/reveal", &resp)
	return resp, err
}
