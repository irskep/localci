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
