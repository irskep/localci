package localci

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestAPIEventWebSocketSendsSnapshot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	repoDir := filepath.Join(root, "repo")
	commit := "abc123"
	req := InvokeRequest{RepoDir: repoDir, Commit: commit}

	task := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, time.Now().UTC())
	task.Status = TaskStatusSucceeded
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(task.OutputDir, "test.log"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := writeTaskRecord(task); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, task.StartedAt)
	run.FinishedAt = task.StartedAt.Add(time.Second)
	run.TaskResults = []TaskRecord{task}
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
		RepoRoot: root,
		EventHub: NewEventHub(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", server.handleAPI)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if isTCPPermissionError(err) {
			t.Skip("tcp listeners are not permitted in this environment")
		}
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()

	httpServer := &http.Server{Handler: mux}
	errs := make(chan error, 1)
	go func() {
		errs <- httpServer.Serve(listener)
	}()
	defer func() {
		_ = httpServer.Close()
		<-errs
	}()

	wsURL := "ws://" + listener.Addr().String() + "/api/repo/repo/commit/" + commit + "/events"
	conn, _, err := websocket.Dial(context.Background(), wsURL, nil)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, data, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	var event APIEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if event.Type != EventTypeSnapshot {
		t.Fatalf("event type = %q, want snapshot", event.Type)
	}
	if event.Resource != "/api/repo/repo/commit/abc123" {
		t.Fatalf("resource = %q", event.Resource)
	}
	payload, ok := event.Data.(map[string]any)
	if !ok || payload["commit"] == nil {
		t.Fatalf("payload missing commit response: %#v", event.Data)
	}

	server.EventHub.PublishReplace("/api/repo/repo/commit/abc123")
	_, data, err = conn.Read(context.Background())
	if err != nil {
		t.Fatalf("Read replacement returned error: %v", err)
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal replacement returned error: %v", err)
	}
	if event.Type != EventTypeReplace {
		t.Fatalf("event type = %q, want replace", event.Type)
	}
}

func TestAPIEventWebSocketSendsArtifactAppend(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	repoDir := filepath.Join(root, "repo")
	commit := "abc123"
	req := InvokeRequest{RepoDir: repoDir, Commit: commit}

	task := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, time.Now().UTC())
	task.Status = TaskStatusRunning
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(task.OutputDir, "combined.log"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := writeTaskRecord(task); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, task.StartedAt)
	run.TaskResults = []TaskRecord{task}
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
		RepoRoot: root,
		EventHub: NewEventHub(),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/", server.handleAPI)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		if isTCPPermissionError(err) {
			t.Skip("tcp listeners are not permitted in this environment")
		}
		t.Fatalf("Listen returned error: %v", err)
	}
	defer listener.Close()

	httpServer := &http.Server{Handler: mux}
	errs := make(chan error, 1)
	go func() {
		errs <- httpServer.Serve(listener)
	}()
	defer func() {
		_ = httpServer.Close()
		<-errs
	}()

	resource := "/api/repo/repo/commit/abc123/task/localci:test/attempt/1/artifact/combined.log"
	conn, _, err := websocket.Dial(context.Background(), "ws://"+listener.Addr().String()+resource+"/events", nil)
	if err != nil {
		t.Fatalf("Dial returned error: %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, data, err := conn.Read(context.Background())
	if err != nil {
		t.Fatalf("Read snapshot returned error: %v", err)
	}
	var event APIEvent
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal snapshot returned error: %v", err)
	}
	if event.Type != EventTypeSnapshot {
		t.Fatalf("event type = %q, want snapshot", event.Type)
	}

	server.EventHub.PublishAppend(resource, 2, "++")
	_, data, err = conn.Read(context.Background())
	if err != nil {
		t.Fatalf("Read append returned error: %v", err)
	}
	if err := json.Unmarshal(data, &event); err != nil {
		t.Fatalf("Unmarshal append returned error: %v", err)
	}
	if event.Type != EventTypeAppend || event.Offset != 2 || event.Text != "++" {
		t.Fatalf("append event = %#v", event)
	}
}
