package localci

import (
	"os"
	"testing"
	"time"
)

func TestHistoryReaderListsReposAndCommitsNewestFirst(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}

	writeRun := func(repoDir string, commit string, startedAt time.Time) {
		t.Helper()
		if err := os.MkdirAll(paths.CommitRoot(repoDir, commit), 0o755); err != nil {
			t.Fatalf("MkdirAll returned error: %v", err)
		}
		run := newRunRecord(InvokeRequest{RepoDir: repoDir, Commit: commit}, startedAt)
		run.FinishedAt = startedAt.Add(time.Minute)
		run.RefreshSummary()
		if err := writeRunRecord(paths, InvokeRequest{RepoDir: repoDir, Commit: commit}, run); err != nil {
			t.Fatalf("writeRunRecord returned error: %v", err)
		}
	}

	writeRun("/repo-a", "aaa111", time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC))
	writeRun("/repo-b", "bbb222", time.Date(2026, 5, 21, 10, 0, 0, 0, time.UTC))
	writeRun("/repo-a", "aaa333", time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC))

	reader := HistoryReader{Paths: paths}
	repos, err := reader.ListRepos()
	if err != nil {
		t.Fatalf("ListRepos returned error: %v", err)
	}
	if len(repos) != 2 {
		t.Fatalf("len(repos) = %d, want 2", len(repos))
	}
	if repos[0].RepoDir != "/repo-a" {
		t.Fatalf("repos[0].RepoDir = %q, want /repo-a", repos[0].RepoDir)
	}
	if repos[1].RepoDir != "/repo-b" {
		t.Fatalf("repos[1].RepoDir = %q, want /repo-b", repos[1].RepoDir)
	}
	if len(repos[0].Commits) != 2 {
		t.Fatalf("len(repos[0].Commits) = %d, want 2", len(repos[0].Commits))
	}
	if repos[0].Commits[0].Commit != "aaa333" {
		t.Fatalf("repos[0].Commits[0].Commit = %q, want aaa333", repos[0].Commits[0].Commit)
	}

	commits, err := reader.ListRepoCommits("/repo-a")
	if err != nil {
		t.Fatalf("ListRepoCommits returned error: %v", err)
	}
	if len(commits) != 2 {
		t.Fatalf("len(commits) = %d, want 2", len(commits))
	}
	if commits[0].Commit != "aaa333" || commits[1].Commit != "aaa111" {
		t.Fatalf("unexpected commit order: %#v", commits)
	}
}

func TestHistoryReaderKeepsImportedRunsAfterRunFilesAreDeleted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	req := InvokeRequest{RepoDir: "/repo-a", Commit: "aaa111"}
	run := newRunRecord(req, time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC))
	run.FinishedAt = run.StartedAt.Add(time.Minute)
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		t.Fatalf("writeRunRecord returned error: %v", err)
	}

	reader := HistoryReader{Paths: paths}
	if _, err := reader.ListRepos(); err != nil {
		t.Fatalf("initial ListRepos returned error: %v", err)
	}
	if err := os.Remove(paths.RunRecordPath(req.RepoDir, req.Commit)); err != nil {
		t.Fatalf("Remove run record returned error: %v", err)
	}

	commits, err := reader.ListRepoCommits(req.RepoDir)
	if err != nil {
		t.Fatalf("ListRepoCommits after deleting run file returned error: %v", err)
	}
	if len(commits) != 1 || commits[0].Commit != req.Commit {
		t.Fatalf("commits after deleting run file = %#v, want persisted run", commits)
	}
}
