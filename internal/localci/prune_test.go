package localci

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func pruneFixture(t *testing.T, paths Paths, commit string, at time.Time, size int) string {
	t.Helper()
	req := InvokeRequest{RepoDir: "/repo", Commit: commit}
	task := newTaskRecord(paths, req, Task{Name: "localci:test"}, 1, at)
	task.Status = TaskStatusFailed
	task.FinishedAt = at
	task.Failure = "exit"
	task.Message = "failed deliberately"
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := writeTaskRecord(task); err != nil {
		t.Fatal(err)
	}
	log := filepath.Join(task.OutputDir, "output.log")
	if err := os.WriteFile(log, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
	run := newRunRecord(req, at)
	run.FinishedAt = at
	run.TaskResults = []TaskRecord{task}
	run.RefreshSummary()
	if err := writeRunRecord(paths, run); err != nil {
		t.Fatal(err)
	}
	return log
}

func TestPruneAgeSizeAndHistory(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	now := time.Now().UTC()
	old := pruneFixture(t, paths, "old", now.Add(-40*24*time.Hour), 5)
	middle := pruneFixture(t, paths, "middle", now.Add(-2*24*time.Hour), 8)
	recent := pruneFixture(t, paths, "recent", now, 8)
	plan, err := PlanPrune(paths, PruneOptions{MaxAge: DefaultPruneAge, MaxSize: 10}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Runs) != 2 || plan.RemoveBytes != 13 || plan.TotalBytes != 21 {
		t.Fatalf("plan: %+v", plan)
	}
	if _, err := os.Stat(old); err != nil {
		t.Fatal("preview deleted output", err)
	}
	result, err := ApplyPrune(paths, plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Files != 2 || result.Bytes != 13 {
		t.Fatalf("result: %+v", result)
	}
	for _, path := range []string{old, middle} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("output survived: %s", path)
		}
	}
	if _, err := os.Stat(recent); err != nil {
		t.Fatal(err)
	}
	record, err := readTaskRecord(paths.TaskRecordPath("/repo", "old", "localci:test", 1))
	if err != nil {
		t.Fatal(err)
	}
	if record.Failure != "exit" || record.Message != "failed deliberately" {
		t.Fatalf("lost failure: %+v", record)
	}
	count, err := (RunRepository{Paths: paths}).OldOutputRunCount(now.Add(-DefaultPruneAge))
	if err != nil || count != 0 {
		t.Fatalf("hint count %d: %v", count, err)
	}
	plan, err = PlanPrune(paths, PruneOptions{MaxAge: DefaultPruneAge, MaxSize: 10}, now)
	if err != nil || len(plan.Runs) != 0 {
		t.Fatalf("repeat: %+v %v", plan, err)
	}
}

func TestPruneLargeFilesAndSymlinks(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	now := time.Now().UTC()
	large := pruneFixture(t, paths, "recent", now, 20)
	small := filepath.Join(filepath.Dir(large), "small.txt")
	if err := os.WriteFile(small, make([]byte, 10), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(outside, make([]byte, 100), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(filepath.Dir(large), "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	plan, err := PlanPrune(paths, PruneOptions{LargerThan: 10}, now)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Runs) != 1 || len(plan.Runs[0].Files) != 1 || plan.RemoveBytes != 20 {
		t.Fatalf("plan %+v", plan)
	}
	if _, err := ApplyPrune(paths, plan); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{small, link, outside} {
		if _, err := os.Stat(path); err != nil {
			t.Fatal(err)
		}
	}
	// Age pruning removes the link itself, never its target.
	plan, err = PlanPrune(paths, PruneOptions{MaxAge: time.Hour}, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ApplyPrune(paths, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatal(err)
	}
}

func TestPruneRechecksQueuedLockedAndChangedRuns(t *testing.T) {
	for _, change := range []string{"queued", "active", "locked", "file", "run"} {
		t.Run(change, func(t *testing.T) {
			paths := Paths{Root: t.TempDir()}
			now := time.Now().UTC()
			path := pruneFixture(t, paths, "old", now.Add(-40*24*time.Hour), 20)
			plan, err := PlanPrune(paths, PruneOptions{MaxAge: DefaultPruneAge}, now)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "queued", "active":
				queue := QueueStore{Paths: paths}
				if _, err := queue.EnqueueRun("/repo", "old", nil); err != nil {
					t.Fatal(err)
				}
				if change == "active" {
					if _, _, err := queue.ClaimNext(); err != nil {
						t.Fatal(err)
					}
				}
				protected, err := PlanPrune(paths, PruneOptions{MaxAge: DefaultPruneAge}, now)
				if err != nil || len(protected.Runs) != 0 || protected.ProtectedRuns != 1 {
					t.Fatalf("protection: %+v %v", protected, err)
				}
			case "locked":
				unlock, err := lockOutputs(paths, "/repo", "old", false)
				if err != nil {
					t.Fatal(err)
				}
				defer unlock()
			case "file":
				if err := os.WriteFile(path, []byte("changed"), 0o644); err != nil {
					t.Fatal(err)
				}
			case "run":
				repo := RunRepository{Paths: paths}
				run, err := repo.ReadRun("/repo", "old")
				if err != nil {
					t.Fatal(err)
				}
				run.FinishedAt = now
				if err := repo.WriteRun(run); err != nil {
					t.Fatal(err)
				}
			}
			result, err := ApplyPrune(paths, plan)
			if err != nil {
				t.Fatal(err)
			}
			if result.SkippedRuns != 1 || result.Files != 0 {
				t.Fatalf("result %+v", result)
			}
			if _, err := os.Stat(path); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPruneRejectsSymlinkOutputRoot(t *testing.T) {
	paths := Paths{Root: t.TempDir()}
	now := time.Now().UTC()
	path := pruneFixture(t, paths, "old", now.Add(-40*24*time.Hour), 20)
	root := filepath.Join(paths.CommitRoot("/repo", "old"), "out")
	if err := os.Rename(root, root+"-original"); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(root+"-original", root); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanPrune(paths, PruneOptions{MaxAge: DefaultPruneAge}, now); err == nil {
		t.Fatal("accepted symlink root")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}
