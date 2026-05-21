package localci

import (
	"context"
	"errors"
	"net"
	"os"
	"syscall"
	"testing"
	"time"
)

func TestDaemonServerPingQueueActiveAndShutdown(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}

	entry, err := queue.Enqueue("/repo", "abc123", "localci:test")
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	active, err := queue.MarkActive(entry)
	if err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !canBindUnixSocket(t, paths.DaemonSocketPath()) {
		t.Skip("unix sockets are not permitted in this environment")
	}

	shutdownCalled := false
	server := &DaemonServer{
		Paths: paths,
		Queue: queue,
		ReadState: func() (DaemonState, error) {
			return DaemonState{
				PID:       123,
				StartedAt: time.Date(2026, 5, 21, 1, 0, 0, 0, time.UTC),
			}, nil
		},
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{
				{Name: "localci:build"},
			}, nil
		},
		Shutdown: func() {
			shutdownCalled = true
			cancel()
		},
	}

	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx)
	}()

	waitForSocket(t, paths.DaemonSocketPath())

	client := DaemonClient{Paths: paths}
	state, err := client.Ping(context.Background())
	if err != nil {
		t.Fatalf("Ping returned error: %v", err)
	}
	if state.PID != 123 {
		t.Fatalf("Ping state pid = %d, want 123", state.PID)
	}

	queued, err := client.Queue(context.Background())
	if err != nil {
		t.Fatalf("Queue returned error: %v", err)
	}
	if len(queued) != 1 || queued[0].TaskName != entry.TaskName {
		t.Fatalf("unexpected queue response: %#v", queued)
	}

	readActive, err := client.ActiveTask(context.Background())
	if err != nil {
		t.Fatalf("ActiveTask returned error: %v", err)
	}
	if readActive == nil || readActive.TaskName != active.TaskName {
		t.Fatalf("unexpected active task: %#v", readActive)
	}

	if err := client.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
	if !shutdownCalled {
		t.Fatalf("shutdown callback was not called")
	}
	if err := <-errs; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestDaemonServerPostcommitEnqueuesNonActiveTasks(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{
		Paths: paths,
		Now: func() time.Time {
			return time.Date(2026, 5, 21, 1, 30, 0, 0, time.UTC)
		},
	}

	activeEntry := QueueEntry{
		RepoDir:    "/repo",
		RepoID:     normalizeRepoDir("/repo"),
		Commit:     "abc123",
		TaskName:   "localci:test",
		TaskKey:    sanitizeTaskName("localci:test"),
		EnqueuedAt: time.Date(2026, 5, 21, 1, 0, 0, 0, time.UTC),
	}
	if _, err := queue.MarkActive(activeEntry); err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !canBindUnixSocket(t, paths.DaemonSocketPath()) {
		t.Skip("unix sockets are not permitted in this environment")
	}

	server := &DaemonServer{
		Paths: paths,
		Queue: queue,
		ReadState: func() (DaemonState, error) {
			return DaemonState{PID: 123, StartedAt: time.Now().UTC()}, nil
		},
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{
				{Name: "localci:test"},
				{Name: "localci:build"},
			}, nil
		},
		Shutdown: cancel,
	}

	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx)
	}()
	waitForSocket(t, paths.DaemonSocketPath())

	client := DaemonClient{Paths: paths}
	enqueued, err := client.Postcommit(context.Background(), "/repo", "abc123")
	if err != nil {
		t.Fatalf("Postcommit returned error: %v", err)
	}
	if len(enqueued) != 1 || enqueued[0].TaskName != "localci:build" {
		t.Fatalf("unexpected enqueued tasks: %#v", enqueued)
	}

	queueEntries, err := queue.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(queueEntries) != 1 || queueEntries[0].TaskName != "localci:build" {
		t.Fatalf("unexpected queue contents: %#v", queueEntries)
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestDaemonServerStatusView(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	req := InvokeRequest{RepoDir: "/repo", Commit: "abc123"}

	taskRecord := newTaskRecord(paths, req, Task{Name: "localci:build"}, 1, time.Now().UTC())
	taskRecord.Status = TaskStatusSucceeded
	taskRecord.FinishedAt = taskRecord.StartedAt.Add(time.Second)
	if err := os.MkdirAll(taskRecord.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := writeTaskRecord(taskRecord); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, taskRecord.StartedAt)
	run.FinishedAt = taskRecord.FinishedAt
	run.TaskResults = []TaskRecord{taskRecord}
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	if _, err := queue.Enqueue("/repo", "abc123", "localci:fmt"); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !canBindUnixSocket(t, paths.DaemonSocketPath()) {
		t.Skip("unix sockets are not permitted in this environment")
	}

	server := &DaemonServer{
		Paths: paths,
		Queue: queue,
		ReadState: func() (DaemonState, error) {
			return DaemonState{PID: 123, StartedAt: time.Now().UTC()}, nil
		},
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{
				{Name: "localci:build"},
				{Name: "localci:fmt"},
				{Name: "localci:test"},
			}, nil
		},
		Shutdown: cancel,
	}

	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx)
	}()
	waitForSocket(t, paths.DaemonSocketPath())

	client := DaemonClient{Paths: paths}
	view, err := client.Status(context.Background(), "/repo", "abc123")
	if err != nil {
		t.Fatalf("Status returned error: %v", err)
	}
	if len(view.Tasks) != 3 {
		t.Fatalf("len(view.Tasks) = %d, want 3", len(view.Tasks))
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func TestDaemonServerRetryEnqueuesSingleTask(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if !canBindUnixSocket(t, paths.DaemonSocketPath()) {
		t.Skip("unix sockets are not permitted in this environment")
	}

	server := &DaemonServer{
		Paths: paths,
		Queue: queue,
		ReadState: func() (DaemonState, error) {
			return DaemonState{PID: 123, StartedAt: time.Now().UTC()}, nil
		},
		DiscoverTasks: func(context.Context, string) ([]Task, error) {
			return []Task{
				{Name: "localci:build"},
				{Name: "localci:test"},
			}, nil
		},
		Shutdown: cancel,
	}

	errs := make(chan error, 1)
	go func() {
		errs <- server.Serve(ctx)
	}()
	waitForSocket(t, paths.DaemonSocketPath())

	client := DaemonClient{Paths: paths}
	enqueued, err := client.Retry(context.Background(), "/repo", "abc123", "localci:test")
	if err != nil {
		t.Fatalf("Retry returned error: %v", err)
	}
	if len(enqueued) != 1 || enqueued[0].TaskName != "localci:test" {
		t.Fatalf("unexpected retry queue response: %#v", enqueued)
	}

	queueEntries, err := queue.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(queueEntries) != 1 || queueEntries[0].TaskName != "localci:test" {
		t.Fatalf("unexpected queue contents: %#v", queueEntries)
	}

	cancel()
	if err := <-errs; err != nil {
		t.Fatalf("Serve returned error: %v", err)
	}
}

func waitForSocket(t *testing.T, path string) {
	t.Helper()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, err := os.Stat(path)
		if err == nil {
			return
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(%s) returned error: %v", path, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("socket %s was not created", path)
}

func TestDaemonClientCallMissingSocket(t *testing.T) {
	t.Parallel()

	client := DaemonClient{Paths: Paths{Root: t.TempDir()}}
	_, err := client.Ping(context.Background())
	if err == nil {
		t.Fatalf("Ping returned nil error, want dial failure")
	}

	var opErr *net.OpError
	if !errors.As(err, &opErr) {
		t.Fatalf("error = %T, want *net.OpError", err)
	}
}

func canBindUnixSocket(t *testing.T, path string) bool {
	t.Helper()

	_ = os.Remove(path)
	listener, err := net.Listen("unix", path)
	if err != nil {
		if isSocketPermissionError(err) {
			return false
		}
		t.Fatalf("Listen(%s) returned error: %v", path, err)
	}
	_ = listener.Close()
	_ = os.Remove(path)
	return true
}

func isSocketPermissionError(err error) bool {
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		var errno syscall.Errno
		if errors.As(opErr.Err, &errno) && errno == syscall.EPERM {
			return true
		}
	}
	return false
}
