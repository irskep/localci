package localci

import (
	"context"
	"testing"
)

func TestGitCommitSubject(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initGitRepo(t, repoDir, "abc123")
	commit := gitHeadCommitForTest(t, repoDir)

	got, err := GitCommitSubject(context.Background(), repoDir, commit)
	if err != nil {
		t.Fatalf("GitCommitSubject returned error: %v", err)
	}
	if want := "initial"; got != want {
		t.Fatalf("GitCommitSubject = %q, want %q", got, want)
	}
}

func gitHeadCommitForTest(t *testing.T, repoDir string) string {
	t.Helper()

	commit, err := GitHeadCommit(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("GitHeadCommit returned error: %v", err)
	}
	return commit
}
