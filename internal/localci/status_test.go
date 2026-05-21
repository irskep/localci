package localci

import (
	"errors"
	"os"
	"testing"
	"time"
)

func TestStatusReaderReadCommit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	req := InvokeRequest{
		RepoDir: "/tmp/repo",
		Commit:  "abc123",
	}

	run := newRunRecord(req, time.Date(2026, 5, 20, 21, 0, 0, 0, time.UTC))
	run.FinishedAt = run.StartedAt.Add(time.Second)
	run.Status = RunStatusSucceeded

	taskA := newTaskRecord(paths, req, Task{Name: "localci:build"}, 1, run.StartedAt)
	taskA.Status = TaskStatusSucceeded
	taskA.FinishedAt = taskA.StartedAt.Add(100 * time.Millisecond)
	taskA.DurationMilliseconds = durationMilliseconds(taskA.StartedAt, taskA.FinishedAt)

	taskARetry := newTaskRecord(paths, req, Task{Name: "localci:build"}, 2, run.StartedAt.Add(time.Second))
	taskARetry.Status = TaskStatusFailed
	taskARetry.FinishedAt = taskARetry.StartedAt.Add(50 * time.Millisecond)
	taskARetry.DurationMilliseconds = durationMilliseconds(taskARetry.StartedAt, taskARetry.FinishedAt)
	taskARetry.Failure = "exit"
	taskARetry.ExitCode = intPtr(2)

	taskB := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, run.StartedAt)
	taskB.Status = TaskStatusFailed
	taskB.FinishedAt = taskB.StartedAt.Add(200 * time.Millisecond)
	taskB.DurationMilliseconds = durationMilliseconds(taskB.StartedAt, taskB.FinishedAt)
	taskB.Failure = "exit"
	taskB.ExitCode = intPtr(1)

	run.TaskResults = []TaskRecord{taskARetry, taskB}
	run.RefreshSummary()

	for _, dir := range []string{
		paths.CommitRoot(req.RepoDir, req.Commit),
		taskA.OutputDir,
		taskARetry.OutputDir,
		taskB.OutputDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) returned error: %v", dir, err)
		}
	}

	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}
	if err := writeTaskRecord(taskB); err != nil {
		t.Fatalf("writeTaskRecord(taskB) returned error: %v", err)
	}
	if err := writeTaskRecord(taskA); err != nil {
		t.Fatalf("writeTaskRecord(taskA) returned error: %v", err)
	}
	if err := writeTaskRecord(taskARetry); err != nil {
		t.Fatalf("writeTaskRecord(taskARetry) returned error: %v", err)
	}

	reader := StatusReader{Paths: paths}
	status, err := reader.ReadCommit(req.RepoDir, req.Commit)
	if err != nil {
		t.Fatalf("ReadCommit returned error: %v", err)
	}

	if status.Run.Status != RunStatusFailed {
		t.Fatalf("Run.Status = %q, want %q", status.Run.Status, RunStatusFailed)
	}
	if status.Run.Summary.Total != 2 || status.Run.Summary.Failed != 2 {
		t.Fatalf("unexpected run summary: %#v", status.Run.Summary)
	}
	if len(status.Tasks) != 2 {
		t.Fatalf("len(status.Tasks) = %d, want 2", len(status.Tasks))
	}
	if status.Tasks[0].Name != "localci:build" || status.Tasks[1].Name != "localci:test" {
		t.Fatalf("unexpected task order: %#v", status.Tasks)
	}
	if status.Tasks[0].Attempt != 2 {
		t.Fatalf("build attempt = %d, want 2", status.Tasks[0].Attempt)
	}
}

func TestStatusReaderReadCommitMissingRun(t *testing.T) {
	t.Parallel()

	reader := StatusReader{Paths: Paths{Root: t.TempDir()}}
	_, err := reader.ReadCommit("/tmp/repo", "missing")
	if err == nil {
		t.Fatalf("ReadCommit returned nil error, want not found")
	}
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("error = %v, want ErrRecordNotFound", err)
	}
}
