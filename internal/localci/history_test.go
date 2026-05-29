package localci

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestHistoryReaderListsReposAndCommitsNewestFirst(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	repository := RunRepository{Paths: paths}

	writeRun := func(repoDir string, commit string, startedAt time.Time) {
		t.Helper()
		run := newRunRecord(InvokeRequest{RepoDir: repoDir, Commit: commit}, startedAt)
		run.FinishedAt = startedAt.Add(time.Minute)
		run.RefreshSummary()
		if err := repository.WriteRun(run); err != nil {
			t.Fatalf("WriteRun returned error: %v", err)
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

func TestHistoryReaderKeepsRunsAfterOutputFilesAreDeleted(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	repository := RunRepository{Paths: paths}
	req := InvokeRequest{RepoDir: "/repo-a", Commit: "aaa111"}
	run := newRunRecord(req, time.Date(2026, 5, 20, 10, 0, 0, 0, time.UTC))
	run.FinishedAt = run.StartedAt.Add(time.Minute)
	run.RefreshSummary()
	if err := repository.WriteRun(run); err != nil {
		t.Fatalf("WriteRun returned error: %v", err)
	}

	reader := HistoryReader{Paths: paths}
	if err := os.RemoveAll(paths.CommitRoot(req.RepoDir, req.Commit)); err != nil {
		t.Fatalf("RemoveAll commit output returned error: %v", err)
	}

	commits, err := reader.ListRepoCommits(req.RepoDir)
	if err != nil {
		t.Fatalf("ListRepoCommits after deleting output files returned error: %v", err)
	}
	if len(commits) != 1 || commits[0].Commit != req.Commit {
		t.Fatalf("commits after deleting output files = %#v, want persisted run", commits)
	}
}

func TestRunRepositoryPagesRunsWithDateCursors(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	repo := RunRepository{Paths: paths}
	base := time.Date(2026, 5, 28, 12, 0, 0, 0, time.UTC)

	for index := range 45 {
		req := InvokeRequest{RepoDir: "/repo", Commit: fmt.Sprintf("c%02d", index)}
		run := newRunRecord(req, base.Add(-time.Duration(index)*time.Minute))
		run.FinishedAt = run.StartedAt
		run.RefreshSummary()
		if err := repo.WriteRun(run); err != nil {
			t.Fatalf("WriteRun returned error: %v", err)
		}
	}

	firstPage, err := repo.ListRecentRunPage(time.Time{}, defaultRunListLimit)
	if err != nil {
		t.Fatalf("ListRecentRunPage first returned error: %v", err)
	}
	if len(firstPage.Runs) != defaultRunListLimit {
		t.Fatalf("first page length = %d, want %d", len(firstPage.Runs), defaultRunListLimit)
	}
	if firstPage.Runs[0].Commit != "c00" || firstPage.Runs[len(firstPage.Runs)-1].Commit != "c19" {
		t.Fatalf("first page commits = %q..%q, want c00..c19", firstPage.Runs[0].Commit, firstPage.Runs[len(firstPage.Runs)-1].Commit)
	}
	if firstPage.NextBefore == "" {
		t.Fatalf("first page omitted next cursor")
	}
	if firstPage.NewerBefore != "" {
		t.Fatalf("first page newer cursor = %q, want empty", firstPage.NewerBefore)
	}

	before, err := time.Parse(time.RFC3339Nano, firstPage.NextBefore)
	if err != nil {
		t.Fatalf("Parse next cursor returned error: %v", err)
	}
	secondPage, err := repo.ListRecentRunPage(before, defaultRunListLimit)
	if err != nil {
		t.Fatalf("ListRecentRunPage second returned error: %v", err)
	}
	if len(secondPage.Runs) != defaultRunListLimit {
		t.Fatalf("second page length = %d, want %d", len(secondPage.Runs), defaultRunListLimit)
	}
	if secondPage.Runs[0].Commit != "c20" || secondPage.Runs[len(secondPage.Runs)-1].Commit != "c39" {
		t.Fatalf("second page commits = %q..%q, want c20..c39", secondPage.Runs[0].Commit, secondPage.Runs[len(secondPage.Runs)-1].Commit)
	}
	if secondPage.NextBefore == "" {
		t.Fatalf("second page omitted next cursor")
	}
	if secondPage.NewerBefore != "" {
		t.Fatalf("second page newer cursor = %q, want empty root cursor", secondPage.NewerBefore)
	}

	before, err = time.Parse(time.RFC3339Nano, secondPage.NextBefore)
	if err != nil {
		t.Fatalf("Parse second next cursor returned error: %v", err)
	}
	thirdPage, err := repo.ListRecentRunPage(before, defaultRunListLimit)
	if err != nil {
		t.Fatalf("ListRecentRunPage third returned error: %v", err)
	}
	if len(thirdPage.Runs) != 5 {
		t.Fatalf("third page length = %d, want 5", len(thirdPage.Runs))
	}
	if thirdPage.Runs[0].Commit != "c40" || thirdPage.Runs[len(thirdPage.Runs)-1].Commit != "c44" {
		t.Fatalf("third page commits = %q..%q, want c40..c44", thirdPage.Runs[0].Commit, thirdPage.Runs[len(thirdPage.Runs)-1].Commit)
	}
	if thirdPage.NewerBefore != firstPage.NextBefore {
		t.Fatalf("third page newer cursor = %q, want %q", thirdPage.NewerBefore, firstPage.NextBefore)
	}
}
