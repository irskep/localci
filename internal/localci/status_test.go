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
	run.DiscoveredTasks = []Task{
		{Name: "localci:build"},
		{Name: "localci:test"},
	}

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

	if err := writeRunRecord(paths, run); err != nil {
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

func TestStatusReaderUsesPersistedRunTasksWhenOutputFilesAreDeleted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	req := InvokeRequest{
		RepoDir: "/tmp/repo",
		Commit:  "abc123",
	}

	task := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, time.Date(2026, 5, 20, 21, 0, 0, 0, time.UTC))
	task.Status = TaskStatusSucceeded
	task.FinishedAt = task.StartedAt.Add(time.Second)
	task.DurationMilliseconds = durationMilliseconds(task.StartedAt, task.FinishedAt)

	run := newRunRecord(req, task.StartedAt)
	run.FinishedAt = task.FinishedAt
	run.DiscoveredTasks = []Task{{Name: task.Name}}
	run.TaskResults = []TaskRecord{task}
	run.RefreshSummary()
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}
	if err := os.RemoveAll(paths.CommitRoot(req.RepoDir, req.Commit)); err != nil {
		t.Fatalf("RemoveAll commit output returned error: %v", err)
	}

	status, err := (StatusReader{Paths: paths}).ReadCommit(req.RepoDir, req.Commit)
	if err != nil {
		t.Fatalf("ReadCommit returned error: %v", err)
	}
	if len(status.Tasks) != 1 || status.Tasks[0].Name != task.Name {
		t.Fatalf("status tasks = %#v, want persisted task", status.Tasks)
	}
}

func TestBuildCommitStatusViewIncludesQueuedAttempt(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	repoDir := "/tmp/repo"
	commit := "abc123"
	req := InvokeRequest{
		RepoDir: repoDir,
		Commit:  commit,
	}

	record := newTaskRecord(paths, req, Task{Name: "localci:test"}, 2, time.Date(2026, 5, 20, 21, 0, 0, 0, time.UTC))
	record.Status = TaskStatusFailed
	record.FinishedAt = record.StartedAt.Add(time.Second)
	record.DurationMilliseconds = durationMilliseconds(record.StartedAt, record.FinishedAt)
	record.Failure = "exit"
	if err := os.MkdirAll(record.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
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

	view, err := BuildCommitStatusView(paths, repoDir, commit, []Task{{Name: "localci:test"}}, []QueueEntry{{
		Kind:     QueueEntryKindTask,
		RepoDir:  repoDir,
		RepoID:   normalizeRepoDir(repoDir),
		Commit:   commit,
		TaskName: "localci:test",
		TaskKey:  sanitizeTaskName("localci:test"),
		Attempt:  3,
	}}, nil)
	if err != nil {
		t.Fatalf("BuildCommitStatusView returned error: %v", err)
	}

	task := view.Tasks[0]
	if task.Status != ExecutionStatusQueued || task.Attempt != 3 || task.AttemptCount != 2 {
		t.Fatalf("unexpected queued task view: %#v", task)
	}
	if task.Attempts[0].Attempt != 3 || task.Attempts[0].Status != ExecutionStatusQueued {
		t.Fatalf("latest attempt = %#v, want queued attempt 3", task.Attempts[0])
	}
}
