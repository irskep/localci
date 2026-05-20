package localci

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSchedulerRunNextExecutesQueuedTaskAndClearsActive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := t.TempDir()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
if [ "$1" = "run" ] && [ "$2" = "localci:test" ]; then
  printf 'scheduler output\n' >"$LOCALCI_TASK_OUTPUT_DIR/test.log"
  exit 0
fi
exit 1
`)

	now := time.Date(2026, 5, 20, 23, 0, 0, 0, time.UTC)
	queue := QueueStore{
		Paths: Paths{Root: root},
		Now: func() time.Time {
			return now
		},
	}
	entry, err := queue.Enqueue(repoDir, "abc123", "localci:test")
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	runner := Runner{
		Paths:   Paths{Root: root},
		MiseBin: filepath.Join(binDir, "mise"),
		Env:     append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH")),
		Now: func() time.Time {
			return now.Add(time.Second)
		},
		PollInterval: 10 * time.Millisecond,
	}

	scheduler := Scheduler{
		Queue:  queue,
		Runner: runner,
	}

	result, err := scheduler.RunNext(context.Background())
	if err != nil {
		t.Fatalf("RunNext returned error: %v", err)
	}
	if !result.DidWork {
		t.Fatalf("RunNext DidWork = false, want true")
	}
	if result.Entry.TaskName != entry.TaskName {
		t.Fatalf("RunNext entry = %#v, want %#v", result.Entry, entry)
	}
	if result.Task.Status != TaskStatusSucceeded {
		t.Fatalf("Task status = %q, want %q", result.Task.Status, TaskStatusSucceeded)
	}
	if result.Run.Status != RunStatusSucceeded {
		t.Fatalf("Run status = %q, want %q", result.Run.Status, RunStatusSucceeded)
	}

	entries, err := queue.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) after RunNext = %d, want 0", len(entries))
	}

	_, err = queue.ReadActive()
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("ReadActive error = %v, want ErrRecordNotFound", err)
	}

	run, err := readRunRecord(runner.Paths, InvokeRequest{RepoDir: repoDir, Commit: "abc123"})
	if err != nil {
		t.Fatalf("readRunRecord returned error: %v", err)
	}
	if run.Summary.Succeeded != 1 {
		t.Fatalf("unexpected run summary: %#v", run.Summary)
	}
}

func TestSchedulerRunNextRespectsExistingActiveTask(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	queue := QueueStore{Paths: Paths{Root: root}}

	active := QueueEntry{
		RepoDir:    "/repo",
		RepoID:     normalizeRepoDir("/repo"),
		Commit:     "abc123",
		TaskName:   "localci:test",
		TaskKey:    sanitizeTaskName("localci:test"),
		EnqueuedAt: time.Date(2026, 5, 20, 23, 30, 0, 0, time.UTC),
	}
	if _, err := queue.MarkActive(active); err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}

	scheduler := Scheduler{
		Queue:  queue,
		Runner: Runner{Paths: Paths{Root: root}},
	}

	result, err := scheduler.RunNext(context.Background())
	if err != nil {
		t.Fatalf("RunNext returned error: %v", err)
	}
	if result.DidWork {
		t.Fatalf("RunNext DidWork = true, want false")
	}
	if result.Entry.TaskName != active.TaskName {
		t.Fatalf("unexpected active entry: %#v", result.Entry)
	}
}
