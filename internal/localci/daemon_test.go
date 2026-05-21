package localci

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestDaemonManagerRunWritesAndClearsState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	if !canBindUnixSocket(t, paths.DaemonSocketPath()) {
		t.Skip("unix sockets are not permitted in this environment")
	}

	manager := DaemonManager{
		Paths: paths,
		Now: func() time.Time {
			return time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
		},
		PollInterval: 10 * time.Millisecond,
		Scheduler: Scheduler{
			Queue:  QueueStore{Paths: paths},
			Runner: Runner{Paths: paths},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- manager.Run(ctx)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, err := manager.ReadState()
		if err == nil {
			cancel()
			break
		}
		if !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("ReadState returned error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	_, err := manager.ReadState()
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("ReadState after shutdown error = %v, want ErrRecordNotFound", err)
	}
}

func TestProcessAliveInvalidPID(t *testing.T) {
	t.Parallel()

	alive, err := processAlive(-1)
	if err != nil {
		t.Fatalf("processAlive returned error: %v", err)
	}
	if alive {
		t.Fatalf("processAlive returned true for invalid pid")
	}
}

func TestDaemonManagerRecoverInterruptedWorkRequeuesRunningTask(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	req := InvokeRequest{RepoDir: "/repo", Commit: "abc123"}
	startedAt := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
	recoveredAt := startedAt.Add(2 * time.Minute)

	task := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, startedAt)
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := writeTaskRecord(task); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, startedAt)
	run.TaskResults = []TaskRecord{task}
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	entry, err := queue.Enqueue(req.RepoDir, req.Commit, task.Name)
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if _, err := queue.MarkActive(entry); err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}
	if err := queue.Remove(entry); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	manager := DaemonManager{
		Paths: paths,
		Now: func() time.Time {
			return recoveredAt
		},
		Scheduler: Scheduler{
			Queue:  queue,
			Runner: Runner{Paths: paths},
		},
	}

	if err := manager.recoverInterruptedWork(); err != nil {
		t.Fatalf("recoverInterruptedWork returned error: %v", err)
	}

	recoveredRun, err := readRunRecord(paths, req)
	if err != nil {
		t.Fatalf("readRunRecord returned error: %v", err)
	}
	recoveredTask, ok := findTaskRecord(recoveredRun.TaskResults, task.Name)
	if !ok {
		t.Fatalf("task %q not found after recovery", task.Name)
	}
	if recoveredTask.Status != TaskStatusFailed {
		t.Fatalf("task status = %q, want %q", recoveredTask.Status, TaskStatusFailed)
	}
	if recoveredTask.Failure != "daemon-restart" {
		t.Fatalf("task failure = %q, want %q", recoveredTask.Failure, "daemon-restart")
	}

	entries, err := queue.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].TaskName != task.Name {
		t.Fatalf("unexpected queue after recovery: %#v", entries)
	}

	_, err = queue.ReadActive()
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("ReadActive error = %v, want ErrRecordNotFound", err)
	}
}

func TestDaemonManagerRecoverInterruptedWorkSkipsFinishedTask(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	queue := QueueStore{Paths: paths}
	req := InvokeRequest{RepoDir: "/repo", Commit: "abc123"}
	startedAt := time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)

	task := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, startedAt)
	task.Status = TaskStatusSucceeded
	task.FinishedAt = startedAt.Add(time.Minute)
	task.DurationMilliseconds = durationMilliseconds(task.StartedAt, task.FinishedAt)
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := writeTaskRecord(task); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, startedAt)
	run.FinishedAt = task.FinishedAt
	run.TaskResults = []TaskRecord{task}
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	entry, err := queue.Enqueue(req.RepoDir, req.Commit, task.Name)
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}
	if _, err := queue.MarkActive(entry); err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}
	if err := queue.Remove(entry); err != nil {
		t.Fatalf("Remove returned error: %v", err)
	}

	manager := DaemonManager{
		Paths: paths,
		Scheduler: Scheduler{
			Queue:  queue,
			Runner: Runner{Paths: paths},
		},
	}

	if err := manager.recoverInterruptedWork(); err != nil {
		t.Fatalf("recoverInterruptedWork returned error: %v", err)
	}

	entries, err := queue.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("unexpected queued tasks after recovery: %#v", entries)
	}
}
