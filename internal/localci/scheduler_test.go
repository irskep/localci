package localci

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchedulerRunNextExecutesQueuedTaskAndClearsActive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := t.TempDir()
	binDir := t.TempDir()
	paths := Paths{Root: root}
	if err := os.MkdirAll(paths.CloneWorktreeDir(repoDir, "abc123"), 0o755); err != nil {
		t.Fatalf("MkdirAll clone worktree returned error: %v", err)
	}

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
		Paths: paths,
		Now: func() time.Time {
			return now
		},
	}
	entry, err := queue.Enqueue(repoDir, "abc123", "localci:test")
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	runner := Runner{
		Paths:   paths,
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

func TestSchedulerRunNextExecutesNoCloneRunInRepoDir(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := t.TempDir()
	binDir := t.TempDir()
	paths := Paths{Root: root}
	pwdFile := filepath.Join(root, "pwd.txt")

	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
if [ "$1" = "trust" ]; then
  exit 0
fi
if [ "$1" = "tasks" ] && [ "$2" = "--json" ] && [ "$3" = "--all" ]; then
  printf '%s\n' '[{"name":"localci:pwd"}]'
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "localci:pwd" ]; then
  pwd >"`+pwdFile+`"
  exit 0
fi
exit 1
`)

	queue := QueueStore{Paths: paths}
	if _, err := queue.EnqueueRunWithOptions(repoDir, "abc123*", nil, true); err != nil {
		t.Fatalf("EnqueueRunWithOptions returned error: %v", err)
	}

	runner := Runner{
		Paths:        paths,
		MiseBin:      filepath.Join(binDir, "mise"),
		Env:          append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH")),
		PollInterval: 10 * time.Millisecond,
	}
	scheduler := Scheduler{
		Queue:  queue,
		Runner: runner,
	}

	if _, err := scheduler.RunNext(context.Background()); err != nil {
		t.Fatalf("RunNext expand returned error: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := scheduler.RunNext(context.Background()); err != nil {
			t.Fatalf("RunNext task %d returned error: %v", i, err)
		}
	}

	pwdBytes, err := os.ReadFile(pwdFile)
	if err != nil {
		t.Fatalf("ReadFile pwd marker returned error: %v", err)
	}
	if got := strings.TrimSpace(string(pwdBytes)); got != repoDir {
		t.Fatalf("task pwd = %q, want %q", got, repoDir)
	}
	run, err := readRunRecord(paths, InvokeRequest{RepoDir: repoDir, Commit: "abc123*", NoClone: true})
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
		Kind:       QueueEntryKindTask,
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

func TestSchedulerRunNextReusesTimeoutWatcher(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := t.TempDir()
	binDir := t.TempDir()
	paths := Paths{Root: root}
	if err := os.MkdirAll(paths.CloneWorktreeDir(repoDir, "abc123"), 0o755); err != nil {
		t.Fatalf("MkdirAll clone worktree returned error: %v", err)
	}

	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
if [ "$1" = "run" ] && [ "$2" = "localci:test" ]; then
  sleep 5
  exit 0
fi
exit 1
`)

	queue := QueueStore{Paths: paths}
	if _, err := queue.Enqueue(repoDir, "abc123", "localci:test"); err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	runner := Runner{
		Paths:             paths,
		MiseBin:           filepath.Join(binDir, "mise"),
		Env:               append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH")),
		InactivityTimeout: 50 * time.Millisecond,
		TerminateGrace:    50 * time.Millisecond,
		PollInterval:      10 * time.Millisecond,
	}

	scheduler := Scheduler{
		Queue:  queue,
		Runner: runner,
	}

	result, err := scheduler.RunNext(context.Background())
	if !errors.Is(err, errTaskTimedOut) {
		t.Fatalf("RunNext error = %v, want errTaskTimedOut", err)
	}
	if !result.DidWork {
		t.Fatalf("RunNext DidWork = false, want true")
	}
	if result.Task.Status != TaskStatusTimedOut {
		t.Fatalf("task status = %q, want %q", result.Task.Status, TaskStatusTimedOut)
	}
	if result.Run.Status != RunStatusFailed {
		t.Fatalf("run status = %q, want %q", result.Run.Status, RunStatusFailed)
	}
	if result.Run.Summary.TimedOut != 1 {
		t.Fatalf("timed out summary = %#v, want TimedOut=1", result.Run.Summary)
	}
}
