package localci

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestAppendWriterPreservesSplitUTF8AndByteOffsets(t *testing.T) {
	var disk bytes.Buffer
	var events []APIEvent
	w := appendPublishingWriter{writer: &disk, publishAppend: func(offset int64, text string) {
		events = append(events, APIEvent{Type: EventTypeAppend, Offset: offset, EndOffset: offset + int64(len(text)), Text: text})
	}}
	data := []byte("a🙂éz")
	for _, b := range data {
		if _, err := w.Write([]byte{b}); err != nil {
			t.Fatal(err)
		}
	}
	w.flush(true)
	var text string
	var offset int64
	for _, e := range events {
		if e.Offset != offset || !utf8.ValidString(e.Text) {
			t.Fatalf("invalid append: %#v", e)
		}
		text += e.Text
		offset = e.EndOffset
	}
	if text != string(data) || disk.String() != text || offset != int64(len(data)) {
		t.Fatalf("text %q, end %d", text, offset)
	}
	raw, err := json.Marshal(events[0])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(raw, []byte(`"offset":0`)) {
		t.Fatalf("zero offset omitted: %s", raw)
	}
}

func TestTaskAndArtifactRequestsIgnoreUnrelatedOutputMetadata(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	req := InvokeRequest{RepoDir: "/repo", Commit: "abc"}
	run := newRunRecord(req, time.Now())
	run.DiscoveredTasks = []Task{{Name: "localci:test"}, {Name: "localci:other"}}
	task := newTaskRecord(paths, req, run.DiscoveredTasks[0], 1, time.Now())
	task.Status = TaskStatusSucceeded
	if err := os.MkdirAll(task.OutputDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeTaskRecord(task); err != nil {
		t.Fatal(err)
	}
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatal(err)
	}
	// Output files called localci.task.json are not task metadata. Neither are
	// records belonging to other tasks relevant to this task detail request.
	for _, file := range []string{
		filepath.Join(task.OutputDir, "report", taskRecordFileName),
		paths.TaskRecordPath(req.RepoDir, req.Commit, "localci:other", 1),
	} {
		if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(file, []byte("not JSON"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(task.OutputDir, "combined.log"), []byte("ok"), 0644); err != nil {
		t.Fatal(err)
	}
	s := WebServer{Paths: paths, Queue: QueueStore{Paths: paths}}
	got, err := s.selectedTaskStatus(req.RepoDir, req.Commit, task.Name, 0)
	if err != nil || got.Status != ExecutionStatusSucceeded {
		t.Fatalf("task: %#v, %v", got, err)
	}
	_, artifact, err := s.artifactForRoute(req.RepoDir, req.Commit, task.Name, 1, "combined.log")
	if err != nil || !artifact.IsText {
		t.Fatalf("artifact: %#v, %v", artifact, err)
	}
}

func TestPrimaryLogPreviewIsBoundedAndKeepsUTF8Boundary(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "combined.log")
	data := strings.Repeat("a", maxTextArtifactBytes-1) + "🙂tail"
	if err := os.WriteFile(file, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
	task := TaskStatusView{OutputDir: dir, Artifacts: []ArtifactView{{DisplayName: "combined.log"}}}
	_, text, end, truncated := loadPrimaryLogPreview(task)
	if !truncated || end != maxTextArtifactBytes-1 || len(text) != int(end) || !utf8.ValidString(text) {
		t.Fatalf("end=%d truncated=%v", end, truncated)
	}
	task.Status = ExecutionStatusRunning
	if err := os.WriteFile(file, []byte{'a', 0xf0, 0x9f}, 0644); err != nil {
		t.Fatal(err)
	}
	_, text, end, truncated = loadPrimaryLogPreview(task)
	if text != "a" || end != 1 || truncated {
		t.Fatalf("live prefix=%q end=%d", text, end)
	}
}

func BenchmarkHistoryRepoSummaries(b *testing.B) {
	paths := Paths{Root: b.TempDir()}
	repository := RunRepository{Paths: paths}
	run := newRunRecord(InvokeRequest{RepoDir: "/repo", Commit: "abc"}, time.Now())
	if err := repository.WriteRun(run); err != nil {
		b.Fatal(err)
	}
	b.Run("per-operation", func(b *testing.B) {
		for b.Loop() {
			if _, err := repository.ListRepoSummaries(); err != nil {
				b.Fatal(err)
			}
		}
	})
	db, err := repository.open()
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	repository.DB = db
	b.Run("shared", func(b *testing.B) {
		for b.Loop() {
			if _, err := repository.ListRepoSummaries(); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestTaskHistoryPagesOnlyMatchingTasks(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	repository := RunRepository{Paths: paths}
	for i := 0; i < 54; i++ {
		run := newRunRecord(InvokeRequest{RepoDir: "/repo", Commit: fmt.Sprintf("commit-%03d", i)}, time.Now().Add(time.Duration(i)*time.Second))
		name := "localci:test"
		if i == 53 {
			name = "localci:other"
		}
		run.DiscoveredTasks = []Task{{Name: name}}
		if err := repository.WriteRun(run); err != nil {
			t.Fatal(err)
		}
	}
	s := WebServer{Paths: paths, Queue: QueueStore{Paths: paths}}
	first, err := s.apiRepoTaskHistory("/repo", "localci:test")
	if err != nil || len(first.Runs) != 50 || first.NextOffset != 50 {
		t.Fatalf("first count=%d next=%d err=%v", len(first.Runs), first.NextOffset, err)
	}
	second, err := s.apiRepoTaskHistoryPage("/repo", "localci:test", first.NextOffset)
	if err != nil || len(second.Runs) != 3 || second.NextOffset != 0 {
		t.Fatalf("second count=%d next=%d err=%v", len(second.Runs), second.NextOffset, err)
	}
	if first.Runs[0].Commit != "commit-052" || second.Runs[2].Commit != "commit-000" {
		t.Fatal("wrong task history order")
	}
	db, err := repository.open()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	repository.DB, repository.Context = db, ctx
	if _, err := repository.ListRepoSummaries(); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled query: %v", err)
	}
}
