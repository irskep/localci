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

		resp, err := http.Get("http://" + listener.Addr().String() + "/")
		if err != nil {
			t.Fatalf("GET embedded asset returned error: %v", err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET embedded app returned status %d: %s", resp.StatusCode, string(body))
		}
		if !strings.Contains(string(body), "localci") {
			t.Fatalf("embedded app missing expected content: %s", string(body))
		}
	})

	t.Run("override", func(t *testing.T) {
		assetDir := t.TempDir()
		if err := os.WriteFile(filepath.Join(assetDir, "index.html"), []byte(`<!doctype html><html><body>override app</body></html>`), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
		if err := os.Mkdir(filepath.Join(assetDir, "assets"), 0o755); err != nil {
			t.Fatalf("Mkdir returned error: %v", err)
		}
		if err := os.WriteFile(filepath.Join(assetDir, "assets", "app.js"), []byte(`console.info("override asset");`), 0o644); err != nil {
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

		resp, err = http.Get("http://" + listener.Addr().String() + "/repo/cli/localci/commit/abc123")
		if err != nil {
			t.Fatalf("GET override app route returned error: %v", err)
		}
		body, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if got := string(body); !strings.Contains(got, "override app") {
			t.Fatalf("override app route missing index content: %s", got)
		}
	})
}

func TestWebServerAPI(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoRoot := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	repoDir := filepath.Join(repoRoot, "team", "repo")
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
	run.DiscoveredTasks = []Task{{Name: "localci:test"}}
	run.TaskResults = []TaskRecord{record}
	run.RefreshSummary()
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	var revealedPath string
	server := WebServer{
		Paths: paths,
		Queue: queue,
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{{Name: "localci:test"}}, nil
		},
		RevealArtifactInFinder: func(path string) error {
			revealedPath = path
			return nil
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
	repoRoutePath, err := RouteRepoPath(repoDir)
	if err != nil {
		t.Fatalf("RouteRepoPath returned error: %v", err)
	}
	if len(home.Repos) != 1 || home.Repos[0].RepoPath != repoRoutePath || home.Repos[0].RepoLabel != "repo" {
		t.Fatalf("unexpected home repos: %#v", home.Repos)
	}
	if len(home.RecentCommits) != 1 || home.RecentCommits[0].Commit != commit {
		t.Fatalf("unexpected recent commits: %#v", home.RecentCommits)
	}
	if home.RecentCommits[0].Repo.RepoPath != repoRoutePath {
		t.Fatalf("unexpected recent commit repo: %#v", home.RecentCommits[0].Repo)
	}

	resp, err = http.Get(baseURL + "/api/repo/" + repoRoutePath + "/commit/" + commit)
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
	taskAPIBase := baseURL + "/api/repo/" + repoRoutePath + "/commit/" + commit + "/task/" + taskPath
	resp, err = http.Get(taskAPIBase)
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

	resp, err = http.Get(taskAPIBase + "/attempt/2/artifact")
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

	resp, err = http.Get(taskAPIBase + "/attempt/2/artifact")
	if err != nil {
		t.Fatalf("GET artifact index for raw JSON returned error: %v", err)
	}
	var rawArtifactList struct {
		Artifacts []map[string]any `json:"artifacts"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rawArtifactList); err != nil {
		t.Fatalf("Decode raw artifact list returned error: %v", err)
	}
	_ = resp.Body.Close()
	if path, ok := rawArtifactList.Artifacts[0]["path"].(string); !ok || path == "" {
		t.Fatalf("artifact API omitted path: %#v", rawArtifactList.Artifacts[0])
	}

	resp, err = http.Get(taskAPIBase + "/attempt/2/artifact/combined.log")
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

	reqReveal, err := http.NewRequest(http.MethodPost, taskAPIBase+"/attempt/2/artifact/combined.log/reveal", nil)
	if err != nil {
		t.Fatalf("NewRequest reveal returned error: %v", err)
	}
	resp, err = http.DefaultClient.Do(reqReveal)
	if err != nil {
		t.Fatalf("POST reveal returned error: %v", err)
	}
	var revealResp apiRevealArtifactResponse
	if err := json.NewDecoder(resp.Body).Decode(&revealResp); err != nil {
		t.Fatalf("Decode reveal returned error: %v", err)
	}
	_ = resp.Body.Close()
	if !revealResp.OK || revealResp.Path != artifactPath || revealedPath != artifactPath {
		t.Fatalf("reveal response = %#v, revealed %q, want %q", revealResp, revealedPath, artifactPath)
	}

	reqRetry, err := http.NewRequest(http.MethodPost, taskAPIBase+"/retry", nil)
	if err != nil {
		t.Fatalf("NewRequest returned error: %v", err)
	}
	resp, err = http.DefaultClient.Do(reqRetry)
	if err != nil {
		t.Fatalf("POST retry returned error: %v", err)
	}
	var retryResp apiRetryResponse
	if err := json.NewDecoder(resp.Body).Decode(&retryResp); err != nil {
		t.Fatalf("Decode retry returned error: %v", err)
	}
	_ = resp.Body.Close()
	if !retryResp.Enqueued || retryResp.Attempt != 3 {
		t.Fatalf("retry response = %#v, want enqueued=true", retryResp)
	}
	expectedRetryURL := "/repo/" + repoRoutePath + "/commit/abc123/task/localci:test/attempt/3"
	if retryResp.URL != expectedRetryURL {
		t.Fatalf("retry URL = %q, want attempt route", retryResp.URL)
	}

	reqCancel, err := http.NewRequest(http.MethodPost, taskAPIBase+"/cancel", nil)
	if err != nil {
		t.Fatalf("NewRequest cancel returned error: %v", err)
	}
	resp, err = http.DefaultClient.Do(reqCancel)
	if err != nil {
		t.Fatalf("POST cancel returned error: %v", err)
	}
	var cancelResp apiCancelResponse
	if err := json.NewDecoder(resp.Body).Decode(&cancelResp); err != nil {
		t.Fatalf("Decode cancel returned error: %v", err)
	}
	_ = resp.Body.Close()
	if cancelResp.Active || cancelResp.Pending != 1 || !cancelResp.Canceled {
		t.Fatalf("cancel response = %#v, want pending cancellation", cancelResp)
	}
	entries, err := queue.List()
	if err != nil {
		t.Fatalf("List after cancel returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("queue after cancel = %#v, want empty", entries)
	}
}

func TestWebServerServesRawArtifacts(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoRoot := t.TempDir()
	paths := Paths{Root: root}
	repoDir := filepath.Join(repoRoot, "team", "repo")
	commit := "abc123"
	req := InvokeRequest{RepoDir: repoDir, Commit: commit}

	record := newTaskRecord(paths, req, Task{Name: "//web:localci:static-artifacts"}, 1, time.Now().UTC())
	record.Status = TaskStatusSucceeded
	if err := os.MkdirAll(filepath.Join(record.OutputDir, "static-site"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	files := map[string][]byte{
		"static-site/index.html": []byte(`<!doctype html><link rel="stylesheet" href="style.css"><img src="mark.svg">`),
		"static-site/style.css":  []byte(`body { color: rebeccapurple; }`),
		"static-site/mark.svg":   []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1 1"></svg>`),
		"static-site/blob.bin":   {0, 1, 2, 3},
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(record.OutputDir, filepath.FromSlash(name)), data, 0o644); err != nil {
			t.Fatalf("WriteFile %s returned error: %v", name, err)
		}
	}
	if err := writeTaskRecord(record); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, record.StartedAt)
	run.FinishedAt = record.StartedAt.Add(time.Second)
	run.DiscoveredTasks = []Task{{Name: record.Name}}
	run.TaskResults = []TaskRecord{record}
	run.RefreshSummary()
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	server := WebServer{
		Paths: paths,
		Queue: QueueStore{Paths: paths},
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{{Name: record.Name}}, nil
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

	rawPath, err := RawArtifactRoutePath(repoDir, commit, record.Name, record.Attempt, "static-site/index.html")
	if err != nil {
		t.Fatalf("RawArtifactRoutePath returned error: %v", err)
	}
	baseURL := "http://" + listener.Addr().String()
	noRedirectClient := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noRedirectClient.Get(baseURL + rawPath)
	if err != nil {
		t.Fatalf("GET raw index without redirect returned error: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("raw index status = %d, want 200 without redirect", resp.StatusCode)
	}
	assertRawArtifact(t, baseURL+rawPath, "text/html", files["static-site/index.html"])

	assertRawArtifact(t, baseURL+strings.Replace(rawPath, "index.html", "style.css", 1), "text/css", files["static-site/style.css"])
	assertRawArtifact(t, baseURL+strings.Replace(rawPath, "index.html", "mark.svg", 1), "image/svg+xml", files["static-site/mark.svg"])
	assertRawArtifact(t, baseURL+strings.Replace(rawPath, "index.html", "blob.bin", 1), "application/octet-stream", files["static-site/blob.bin"])

	resp, err = http.Get(baseURL + rawPath + "?download=1")
	if err != nil {
		t.Fatalf("GET download returned error: %v", err)
	}
	_ = resp.Body.Close()
	if got := resp.Header.Get("Content-Disposition"); !strings.Contains(got, "attachment") || !strings.Contains(got, "index.html") {
		t.Fatalf("Content-Disposition = %q, want attachment index.html", got)
	}

	resp, err = http.Get(baseURL + strings.Replace(rawPath, "static-site/index.html", "task.json", 1))
	if err != nil {
		t.Fatalf("GET internal artifact returned error: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("internal artifact status = %d, want 404", resp.StatusCode)
	}
}

func assertRawArtifact(t *testing.T, rawURL string, contentTypePrefix string, want []byte) {
	t.Helper()

	resp, err := http.Get(rawURL)
	if err != nil {
		t.Fatalf("GET %s returned error: %v", rawURL, err)
	}
	got, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s returned status %d: %s", rawURL, resp.StatusCode, string(got))
	}
	if string(got) != string(want) {
		t.Fatalf("GET %s body = %q, want %q", rawURL, got, want)
	}
	if contentType := resp.Header.Get("Content-Type"); !strings.HasPrefix(contentType, contentTypePrefix) {
		t.Fatalf("GET %s Content-Type = %q, want prefix %q", rawURL, contentType, contentTypePrefix)
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
		run.DiscoveredTasks = []Task{{Name: "localci:test"}}
		run.TaskResults = []TaskRecord{record}
		run.RefreshSummary()
		if err := writeRunRecord(paths, run); err != nil {
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
	resp, err := http.Get(baseURL + "/api")
	if err != nil {
		t.Fatalf("GET home api returned error: %v", err)
	}
	var home apiHomeResponse
	if err := json.NewDecoder(resp.Body).Decode(&home); err != nil {
		t.Fatalf("Decode home returned error: %v", err)
	}
	_ = resp.Body.Close()

	if len(home.Repos) != 2 || !hasAPIRepo(home.Repos, "repo-a") || !hasAPIRepo(home.Repos, "repo-b") {
		t.Fatalf("unexpected home repos: %#v", home.Repos)
	}
	if len(home.RecentCommits) != 2 || home.RecentCommits[0].Commit != "bbb222" {
		t.Fatalf("unexpected recent commits: %#v", home.RecentCommits)
	}
	if len(home.RecentCommits[0].Tasks) != 1 || home.RecentCommits[0].Tasks[0].Status != ExecutionStatusSucceeded {
		t.Fatalf("unexpected recent commit tasks: %#v", home.RecentCommits[0].Tasks)
	}
	if home.Queue.Active == nil || home.Queue.Active.Commit != "queued123" {
		t.Fatalf("unexpected active queue item: %#v", home.Queue.Active)
	}
	if len(home.Queue.Pending) != 1 || home.Queue.Pending[0].Commit != "queued456" {
		t.Fatalf("unexpected pending queue items: %#v", home.Queue.Pending)
	}

	resp, err = http.Get(baseURL + "/api/repo/repo-b")
	if err != nil {
		t.Fatalf("GET repo api returned error: %v", err)
	}
	var repo apiRepoResponse
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		t.Fatalf("Decode repo returned error: %v", err)
	}
	_ = resp.Body.Close()
	if repo.Repo.RepoPath != "repo-b" {
		t.Fatalf("unexpected repo summary: %#v", repo.Repo)
	}
	if len(repo.Commits) != 1 || repo.Commits[0].Commit != "bbb222" {
		t.Fatalf("unexpected repo commits: %#v", repo.Commits)
	}
	if len(repo.Commits[0].Tasks) != 1 || repo.Commits[0].Tasks[0].ShortName != "test" {
		t.Fatalf("unexpected repo tasks: %#v", repo.Commits[0].Tasks)
	}
}

func hasAPIRepo(repos []apiRepoSummary, repoPath string) bool {
	for _, repo := range repos {
		if repo.RepoPath == repoPath {
			return true
		}
	}
	return false
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
