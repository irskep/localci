package localci

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildCommitStatusView(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	req := InvokeRequest{RepoDir: "/repo", Commit: "abc123"}

	taskSucceeded := newTaskRecord(paths, req, Task{Name: "localci:build"}, 1, time.Now().UTC())
	taskSucceeded.Status = TaskStatusSucceeded
	taskSucceeded.FinishedAt = taskSucceeded.StartedAt.Add(time.Second)
	if err := os.MkdirAll(taskSucceeded.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(taskSucceeded.OutputDir, "build.log"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := writeTaskRecord(taskSucceeded); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}

	run := newRunRecord(req, taskSucceeded.StartedAt)
	run.FinishedAt = taskSucceeded.FinishedAt
	run.TaskResults = []TaskRecord{taskSucceeded}
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	queued := []QueueEntry{
		{RepoDir: "/repo", Commit: "abc123", TaskName: "localci:fmt"},
	}
	active := &ActiveTask{
		QueueEntry: QueueEntry{RepoDir: "/repo", Commit: "abc123", TaskName: "localci:test"},
	}

	view, err := BuildCommitStatusView(paths, "/repo", "abc123", []Task{
		{Name: "localci:test"},
		{Name: "localci:fmt"},
		{Name: "localci:build"},
		{Name: "localci:lint"},
	}, queued, active)
	if err != nil {
		t.Fatalf("BuildCommitStatusView returned error: %v", err)
	}

	if len(view.Tasks) != 4 {
		t.Fatalf("len(view.Tasks) = %d, want 4", len(view.Tasks))
	}

	statuses := map[string]ExecutionStatus{}
	for _, task := range view.Tasks {
		statuses[task.Name] = task.Status
	}

	if statuses["localci:build"] != ExecutionStatusSucceeded {
		t.Fatalf("build status = %q, want %q", statuses["localci:build"], ExecutionStatusSucceeded)
	}
	if statuses["localci:fmt"] != ExecutionStatusQueued {
		t.Fatalf("fmt status = %q, want %q", statuses["localci:fmt"], ExecutionStatusQueued)
	}
	if statuses["localci:test"] != ExecutionStatusRunning {
		t.Fatalf("test status = %q, want %q", statuses["localci:test"], ExecutionStatusRunning)
	}
	if statuses["localci:lint"] != ExecutionStatusNotRun {
		t.Fatalf("lint status = %q, want %q", statuses["localci:lint"], ExecutionStatusNotRun)
	}
	for _, task := range view.Tasks {
		if task.Name == "localci:build" {
			if task.Attempt != 1 {
				t.Fatalf("build attempt = %d, want 1", task.Attempt)
			}
			if got, want := task.OutputDir, taskSucceeded.OutputDir; got != want {
				t.Fatalf("build output dir = %q, want %q", got, want)
			}
		}
	}
}
