package localci

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCloneManagerPrepareCreatesShallowClone(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initGitRepo(t, repoDir, "first")

	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("second commit\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README returned error: %v", err)
	}
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir, "commit", "-m", "second")
	commit := strings.TrimSpace(runGit(t, repoDir, "rev-parse", "HEAD"))

	manager := CloneManager{Paths: Paths{Root: t.TempDir()}}
	info, err := manager.Prepare(context.Background(), repoDir, commit)
	if err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}

	head := strings.TrimSpace(runGit(t, info.Worktree, "rev-parse", "HEAD"))
	if head != commit {
		t.Fatalf("clone HEAD = %q, want %q", head, commit)
	}

	if _, err := os.Stat(filepath.Join(info.Worktree, ".git", "shallow")); err != nil {
		t.Fatalf("shallow marker missing: %v", err)
	}
}
