package localci

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBuildCommitStatusView(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	req := InvokeRequest{
		RepoDir: "/repo",
		Commit:  "abc123",
		Annotations: map[string]string{
			"branch": "main",
		},
	}

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
	run.DiscoveredTasks = []Task{
		{Name: "localci:test"},
		{Name: "localci:fmt"},
		{Name: "localci:build"},
		{Name: "localci:lint"},
	}
	run.TaskResults = []TaskRecord{taskSucceeded}
	run.RefreshSummary()
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	queued := []QueueEntry{
		{Kind: QueueEntryKindTask, RepoDir: "/repo", Commit: "abc123", TaskName: "localci:fmt"},
	}
	active := &ActiveTask{
		QueueEntry: QueueEntry{Kind: QueueEntryKindTask, RepoDir: "/repo", Commit: "abc123", TaskName: "localci:test"},
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
	if got, want := view.Annotations["branch"], "main"; got != want {
		t.Fatalf("branch annotation = %q, want %q", got, want)
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

func TestBuildCommitStatusViewRollsUpLatestTaskTiming(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	repoDir := "/repo"
	commit := "abc123"
	req := InvokeRequest{RepoDir: repoDir, Commit: commit}
	base := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)

	oldBuild := newTaskRecord(paths, req, Task{Name: "localci:build"}, 1, base)
	oldBuild.Status = TaskStatusSucceeded
	oldBuild.FinishedAt = base.Add(time.Minute)
	oldBuild.DurationMilliseconds = durationMilliseconds(oldBuild.StartedAt, oldBuild.FinishedAt)
	latestBuild := newTaskRecord(paths, req, Task{Name: "localci:build"}, 2, base.Add(10*time.Minute))
	latestBuild.Status = TaskStatusSucceeded
	latestBuild.FinishedAt = base.Add(12 * time.Minute)
	latestBuild.DurationMilliseconds = durationMilliseconds(latestBuild.StartedAt, latestBuild.FinishedAt)
	testTask := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, base.Add(5*time.Minute))
	testTask.Status = TaskStatusSucceeded
	testTask.FinishedAt = base.Add(15 * time.Minute)
	testTask.DurationMilliseconds = durationMilliseconds(testTask.StartedAt, testTask.FinishedAt)

	for _, record := range []TaskRecord{oldBuild, latestBuild, testTask} {
		if err := os.MkdirAll(record.OutputDir, 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := writeTaskRecord(record); err != nil {
			t.Fatalf("writeTaskRecord returned error: %v", err)
		}
	}
	run := newRunRecord(req, oldBuild.StartedAt)
	run.FinishedAt = testTask.FinishedAt
	run.DiscoveredTasks = []Task{{Name: "localci:build"}, {Name: "localci:test"}}
	run.TaskResults = []TaskRecord{latestBuild, testTask}
	run.RefreshSummary()
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	view, err := BuildCommitStatusView(paths, repoDir, commit, nil, nil, nil)
	if err != nil {
		t.Fatalf("BuildCommitStatusView returned error: %v", err)
	}
	if view.StartedAt == nil || !view.StartedAt.Equal(testTask.StartedAt) {
		t.Fatalf("StartedAt = %v, want %v", view.StartedAt, testTask.StartedAt)
	}
	if view.FinishedAt == nil || !view.FinishedAt.Equal(testTask.FinishedAt) {
		t.Fatalf("FinishedAt = %v, want %v", view.FinishedAt, testTask.FinishedAt)
	}
}

func TestBuildCommitStatusViewOmitsTimingForQueuedRetryWithoutMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	repoDir := "/repo"
	commit := "abc123"
	req := InvokeRequest{RepoDir: repoDir, Commit: commit}
	previous := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, time.Now().UTC().Add(-time.Minute))
	previous.Status = TaskStatusSucceeded
	previous.FinishedAt = previous.StartedAt.Add(time.Second)
	if err := os.MkdirAll(previous.OutputDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := writeTaskRecord(previous); err != nil {
		t.Fatalf("writeTaskRecord returned error: %v", err)
	}
	run := newRunRecord(req, previous.StartedAt)
	run.FinishedAt = previous.FinishedAt
	run.DiscoveredTasks = []Task{{Name: previous.Name}}
	run.TaskResults = []TaskRecord{previous}
	run.RefreshSummary()
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	view, err := BuildCommitStatusView(paths, repoDir, commit, nil, []QueueEntry{{
		Kind: QueueEntryKindTask, RepoDir: repoDir, Commit: commit, TaskName: previous.Name, Attempt: 2,
	}}, nil)
	if err != nil {
		t.Fatalf("BuildCommitStatusView returned error: %v", err)
	}
	if view.StartedAt != nil || view.FinishedAt != nil {
		t.Fatalf("timing = (%v, %v), want no timing", view.StartedAt, view.FinishedAt)
	}
}

func TestCommitTimingUsesActiveStartAndOmitsEnd(t *testing.T) {
	t.Parallel()

	startedAt := time.Date(2026, 8, 29, 12, 0, 0, 0, time.UTC)
	active := &ActiveTask{
		QueueEntry: QueueEntry{RepoDir: "/repo", Commit: "abc123", TaskName: "localci:test", Attempt: 1},
		StartedAt:  startedAt,
	}
	gotStart, gotFinish := commitTiming(nil, []TaskStatusView{{
		Name: active.TaskName, Attempt: active.Attempt, Status: ExecutionStatusRunning,
	}}, active, "/repo", "abc123")
	if gotStart == nil || !gotStart.Equal(startedAt) {
		t.Fatalf("started at = %v, want %v", gotStart, startedAt)
	}
	if gotFinish != nil {
		t.Fatalf("finished at = %v, want nil", gotFinish)
	}
}

func TestCommitTimingIgnoresActiveRunWithoutTaskMetadata(t *testing.T) {
	t.Parallel()

	active := &ActiveTask{
		QueueEntry: QueueEntry{Kind: QueueEntryKindRun, RepoDir: "/repo", Commit: "abc123"},
		StartedAt:  time.Now().UTC(),
	}
	gotStart, gotFinish := commitTiming(nil, nil, active, "/repo", "abc123")
	if gotStart != nil || gotFinish != nil {
		t.Fatalf("timing = (%v, %v), want no timing", gotStart, gotFinish)
	}
}

func TestResolveArtifactPathRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	outsideDir := t.TempDir()
	outsideFile := filepath.Join(outsideDir, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("secret"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	if err := os.Symlink(outsideFile, filepath.Join(outputDir, "leak.txt")); err != nil {
		t.Fatalf("Symlink returned error: %v", err)
	}

	if _, err := resolveArtifactPath(outputDir, "leak.txt"); err == nil {
		t.Fatalf("resolveArtifactPath returned nil error, want symlink escape error")
	}
}

func TestReadTextTaskArtifactRejectsBinaryArtifact(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outputDir, "bin"), 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "bin", "localci"), []byte{0xcf, 0xfa, 0xed, 0xfe, 0x00, 0x00}, 0o755); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	task := TaskStatusView{OutputDir: outputDir}

	if _, err := readTextTaskArtifact(task, "bin/localci"); !errors.Is(err, ErrArtifactNotDisplayable) {
		t.Fatalf("readTextTaskArtifact error = %v, want ErrArtifactNotDisplayable", err)
	}
}

func TestReadTextTaskArtifactAllowsLogs(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(outputDir, "combined.log"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}
	task := TaskStatusView{OutputDir: outputDir}

	data, err := readTextTaskArtifact(task, "combined.log")
	if err != nil {
		t.Fatalf("readTextTaskArtifact returned error: %v", err)
	}
	if string(data) != "hello\n" {
		t.Fatalf("content = %q, want hello newline", data)
	}
}

func TestBuildArtifactViewsSortsMarkedArtifactsFirst(t *testing.T) {
	t.Parallel()

	outputDir := t.TempDir()
	for _, name := range []string{"combined.log", "site/index.html", "report.txt"} {
		path := filepath.Join(outputDir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		if err := os.WriteFile(path, []byte("ok"), 0o644); err != nil {
			t.Fatalf("WriteFile returned error: %v", err)
		}
	}
	files, err := listOutputFiles(outputDir)
	if err != nil {
		t.Fatalf("listOutputFiles returned error: %v", err)
	}

	artifacts := buildArtifactViews(outputDir, files, []MarkedArtifact{{
		Name:   "docs html",
		Path:   "site/index.html",
		Action: ArtifactActionOpen,
	}})

	if len(artifacts) < 2 {
		t.Fatalf("artifacts = %#v, want multiple artifacts", artifacts)
	}
	if artifacts[0].DisplayName != "site/index.html" {
		t.Fatalf("first artifact = %q, want marked artifact", artifacts[0].DisplayName)
	}
	if artifacts[0].MarkedName != "docs html" || artifacts[0].Action != ArtifactActionOpen {
		t.Fatalf("first artifact metadata = %#v, want marked open docs html", artifacts[0])
	}
}
