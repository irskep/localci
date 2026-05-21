package localci

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWebServerCommitAndArtifactPages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	repoDir := "/repo"
	commit := "abc123"
	req := InvokeRequest{RepoDir: repoDir, Commit: commit}

	record := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, time.Now().UTC())
	record.Status = TaskStatusSucceeded
	if err := os.MkdirAll(record.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	artifactPath := filepath.Join(record.OutputDir, "test.log")
	if err := os.WriteFile(artifactPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := writeTaskRecord(record); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, record.StartedAt)
	run.FinishedAt = record.StartedAt.Add(time.Second)
	run.TaskResults = []TaskRecord{record}
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	server := WebServer{
		Paths: paths,
		Queue: queue,
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{{Name: "localci:test"}}, nil
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if isTCPPermissionError(err) {
			t.Skip("tcp listeners are not permitted in this environment")
		}
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx, listener)
	}()
	defer func() {
		cancel()
		<-errs
	}()

	baseURL := "http://" + listener.Addr().String()
	resp, err := http.Get(baseURL + "/commit?repo=" + url.QueryEscape(repoDir) + "&commit=" + url.QueryEscape(commit))
	if err != nil {
		t.Fatalf("GET commit returned error: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), ">test<") {
		t.Fatalf("commit page missing task link: %s", string(body))
	}
	if !strings.Contains(string(body), `/assets/app.js`) {
		t.Fatalf("commit page missing app.js: %s", string(body))
	}

	resp, err = http.Get(baseURL + "/task?repo=" + url.QueryEscape(repoDir) + "&commit=" + url.QueryEscape(commit) + "&task=" + url.QueryEscape("localci:test"))
	if err != nil {
		t.Fatalf("GET task returned error: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if !strings.Contains(string(body), "Retry task") {
		t.Fatalf("task page missing retry form: %s", string(body))
	}
	if !strings.Contains(string(body), "Attempt: 1") {
		t.Fatalf("task page missing attempt info: %s", string(body))
	}

	resp, err = http.Get(baseURL + "/artifact?repo=" + url.QueryEscape(repoDir) + "&commit=" + url.QueryEscape(commit) + "&task=" + url.QueryEscape("localci:test") + "&path=" + url.QueryEscape(artifactPath))
	if err != nil {
		t.Fatalf("GET artifact returned error: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if got := string(body); got != "hello" {
		t.Fatalf("artifact body = %q, want %q", got, "hello")
	}
}

func TestWebServerServesEmbeddedAssetsAndOverride(t *testing.T) {
	root := t.TempDir()
	paths := Paths{Root: root}

	t.Run("embedded", func(t *testing.T) {
		server := WebServer{Paths: paths, Queue: QueueStore{Paths: paths}}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			if isTCPPermissionError(err) {
				t.Skip("tcp listeners are not permitted in this environment")
			}
			t.Fatalf("Listen returned error: %v", err)
		}
		defer listener.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errs := make(chan error, 1)
		go func() {
			errs <- server.Serve(ctx, listener)
		}()
		defer func() {
			cancel()
			<-errs
		}()

		resp, err := http.Get("http://" + listener.Addr().String() + "/assets/app.js")
		if err != nil {
			t.Fatalf("GET embedded asset returned error: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if !strings.Contains(string(body), "[localci ui] loaded") {
			t.Fatalf("embedded asset missing expected content: %s", string(body))
		}
	})

	t.Run("override", func(t *testing.T) {
		assetDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(assetDir, "app.js"), []byte(`console.info("override asset");`), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}

		server := WebServer{
			Paths:    paths,
			Queue:    QueueStore{Paths: paths},
			AssetDir: assetDir,
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			if isTCPPermissionError(err) {
				t.Skip("tcp listeners are not permitted in this environment")
			}
			t.Fatalf("Listen returned error: %v", err)
		}
		defer listener.Close()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		errs := make(chan error, 1)
		go func() {
			errs <- server.Serve(ctx, listener)
		}()
		defer func() {
			cancel()
			<-errs
		}()

		resp, err := http.Get("http://" + listener.Addr().String() + "/assets/app.js")
		if err != nil {
			t.Fatalf("GET override asset returned error: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if got := string(body); !strings.Contains(got, "override asset") {
			t.Fatalf("override asset missing expected content: %s", got)
		}
	})
}

func TestWebServerAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	repoDir := "/repo"
	commit := "abc123"
	req := InvokeRequest{RepoDir: repoDir, Commit: commit}

	record := newTaskRecord(paths, req, Task{Name: "localci:test"}, 2, time.Now().UTC())
	record.Status = TaskStatusSucceeded
	record.FinishedAt = record.StartedAt.Add(time.Second)
	record.DurationMilliseconds = durationMilliseconds(record.StartedAt, record.FinishedAt)
	if err := os.MkdirAll(record.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	artifactPath := filepath.Join(record.OutputDir, "combined.log")
	if err := os.WriteFile(artifactPath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := writeTaskRecord(record); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, record.StartedAt)
	run.FinishedAt = record.FinishedAt
	run.TaskResults = []TaskRecord{record}
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	server := WebServer{
		Paths:    paths,
		Queue:    queue,
		RepoRoot: "/",
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{{Name: "localci:test"}}, nil
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if isTCPPermissionError(err) {
			t.Skip("tcp listeners are not permitted in this environment")
		}
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx, listener)
	}()
	defer func() {
		cancel()
		<-errs
	}()

	baseURL := "http://" + listener.Addr().String()

	resp, err := http.Get(baseURL + "/api")
	if err != nil {
		t.Fatalf("GET /api returned error: %v", err)
	}
	var home apiHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&home); err != nil {
		t.Fatalf("Decode home returned error: %v", err)
	}
	_ = resp.Body.Close()
	if len(home.Repos) != 1 || home.Repos[0].RepoPath != "repo" {
		t.Fatalf("unexpected home repos: %#v", home.Repos)
	}
	if len(home.RecentCommits) != 1 || home.RecentCommits[0].Commit != commit {
		t.Fatalf("unexpected recent commits: %#v", home.RecentCommits)
	}

	resp, err = http.Get(baseURL + "/api/repo/repo/commit/" + commit)
	if err != nil {
		t.Fatalf("GET commit api returned error: %v", err)
	}
	var commitResp apiCommitResponse
	if err := json.NewDecoder(resp.Body).Decode(&commitResp); err != nil {
		t.Fatalf("Decode commit returned error: %v", err)
	}
	_ = resp.Body.Close()
	if got := len(commitResp.Commit.Tasks); got != 1 {
		t.Fatalf("commit task count = %d, want 1", got)
	}

	taskPath := url.PathEscape("localci:test")
	resp, err = http.Get(baseURL + "/api/repo/repo/commit/" + commit + "/task/" + taskPath)
	if err != nil {
		t.Fatalf("GET task api returned error: %v", err)
	}
	var taskResp apiTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		t.Fatalf("Decode task returned error: %v", err)
	}
	_ = resp.Body.Close()
	if taskResp.PrimaryArtifact != "combined.log" || taskResp.PrimaryLog != "hello" {
		t.Fatalf("unexpected primary log response: %#v", taskResp)
	}

	resp, err = http.Get(baseURL + "/api/repo/repo/commit/" + commit + "/task/" + taskPath + "/attempt/2/artifact")
	if err != nil {
		t.Fatalf("GET artifact index returned error: %v", err)
	}
	var artifactList apiArtifactListResponse
	if err := json.NewDecoder(resp.Body).Decode(&artifactList); err != nil {
		t.Fatalf("Decode artifact list returned error: %v", err)
	}
	_ = resp.Body.Close()
	if len(artifactList.Artifacts) != 1 || artifactList.Artifacts[0].DisplayName != "combined.log" {
		t.Fatalf("unexpected artifacts: %#v", artifactList.Artifacts)
	}

	resp, err = http.Get(baseURL + "/api/repo/repo/commit/" + commit + "/task/" + taskPath + "/attempt/2/artifact/combined.log")
	if err != nil {
		t.Fatalf("GET artifact returned error: %v", err)
	}
	var artifactResp apiArtifactResponse
	if err := json.NewDecoder(resp.Body).Decode(&artifactResp); err != nil {
		t.Fatalf("Decode artifact returned error: %v", err)
	}
	_ = resp.Body.Close()
	if artifactResp.Content != "hello" {
		t.Fatalf("artifact content = %q, want %q", artifactResp.Content, "hello")
	}

	reqRetry, err := http.NewRequest(http.MethodPost, baseURL+"/api/repo/repo/commit/"+commit+"/task/"+taskPath+"/retry", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	resp, err = http.DefaultClient.Do(reqRetry)
	if err != nil {
		t.Fatalf("POST retry returned error: %v", err)
	}
	var retryResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&retryResp); err != nil {
		t.Fatalf("Decode retry returned error: %v", err)
	}
	_ = resp.Body.Close()
	if retryResp["enqueued"] != true {
		t.Fatalf("retry response = %#v, want enqueued=true", retryResp)
	}
}

func TestWebServerHomeAndRepoPages(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{
		Paths: paths,
		Now: func() time.Time {
			return time.Date(2026, 5, 21, 11, 0, 0, 0, time.UTC)
		},
	}

	writeRun := func(repoDir string, commit string, startedAt time.Time) {
		t.Helper()
		req := InvokeRequest{RepoDir: repoDir, Commit: commit}
		record := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, startedAt)
		record.Status = TaskStatusSucceeded
		record.FinishedAt = startedAt.Add(time.Second)
		record.DurationMilliseconds = durationMilliseconds(record.StartedAt, record.FinishedAt)
		if err := os.MkdirAll(record.OutputDir, 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := writeTaskRecord(record); err != nil {
			t.Fatalf("writeTaskRecord returned error: %v", err)
		}

		run := newRunRecord(req, startedAt)
		run.FinishedAt = startedAt.Add(time.Second)
		run.TaskResults = []TaskRecord{record}
		run.RefreshSummary()
		if err := writeRunRecord(paths, req, run); err != nil {
			t.Fatalf("writeRunRecord returned error: %v", err)
		}
	}

	writeRun("/repo-a", "aaa111", time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC))
	writeRun("/repo-b", "bbb222", time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC))
	entry, err := queue.Enqueue("/repo-a", "queued123", "localci:test")
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if _, err := queue.MarkActive(entry); err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}
	if err := queue.Remove(entry); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}
	if _, err := queue.Enqueue("/repo-b", "queued456", "localci:build"); err != nil {
		t.Fatalf("Enqueue pending returned error: %v", err)
	}

	server := WebServer{
		Paths: paths,
		Queue: queue,
		DiscoverTasks: func(_ context.Context, repoDir string) ([]Task, error) {
			switch repoDir {
			case "/repo-a", "/repo-b":
				return []Task{{Name: "localci:test"}}, nil
			default:
				return nil, nil
			}
		},
	}

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if isTCPPermissionError(err) {
			t.Skip("tcp listeners are not permitted in this environment")
		}
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx, listener)
	}()
	defer func() {
		cancel()
		<-errs
	}()

	baseURL := "http://" + listener.Addr().String()
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET home returned error: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	rendered := string(body)
	if !strings.Contains(rendered, "repo-a") || !strings.Contains(rendered, "repo-b") {
		t.Fatalf("home page missing repo links: %s", rendered)
	}
	if !strings.Contains(rendered, "bbb222") || !strings.Contains(rendered, "1 passed") {
		t.Fatalf("home page missing run summary: %s", rendered)
	}
	if !strings.Contains(rendered, "Recent Commit Activity") || !strings.Contains(rendered, "Queue") {
		t.Fatalf("home page missing section headings: %s", rendered)
	}
	if !strings.Contains(rendered, "active") || !strings.Contains(rendered, "queued123") {
		t.Fatalf("home page missing active queue item: %s", rendered)
	}
	if !strings.Contains(rendered, "pending") || !strings.Contains(rendered, "queued456") {
		t.Fatalf("home page missing pending queue item: %s", rendered)
	}
	if strings.Contains(rendered, ">test</a> — succeeded") {
		t.Fatalf("home page should not inline commit task rows: %s", rendered)
	}

	resp, err = http.Get(baseURL + "/repo?repo=" + url.QueryEscape("/repo-b"))
	if err != nil {
		t.Fatalf("GET repo returned error: %v", err)
	}
	body, _ = io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	rendered = string(body)
	if !strings.Contains(rendered, "bbb222") || !strings.Contains(rendered, ">test<") {
		t.Fatalf("repo page missing commit/task info: %s", rendered)
	}
}

func isTCPPermissionError(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var errno syscall.Errno
		if errors.As(opErr.Err, &errno) && errno == syscall.EPERM {
			return true
		}
	}
	return false
}
