package localci

import (
	"context"
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
