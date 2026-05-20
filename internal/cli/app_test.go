package cli

import (
	"io"
	"testing"
)

func TestParseCommitTargetDefaultsDirToCWD(t *testing.T) {
	t.Parallel()

	got, err := parseCommitTarget([]string{"abc123"}, "/repo")
	if err != nil {
		t.Fatalf("parseCommitTarget returned error: %v", err)
	}

	if got.RepoDir != "/repo" {
		t.Fatalf("RepoDir = %q, want %q", got.RepoDir, "/repo")
	}

	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}

	if got.Task != "" {
		t.Fatalf("Task = %q, want empty", got.Task)
	}
}

func TestParseCommitTargetWithRepoAndTask(t *testing.T) {
	t.Parallel()

	got, err := parseCommitTarget([]string{"./worktree", "abc123", "localci:test"}, "/repo")
	if err != nil {
		t.Fatalf("parseCommitTarget returned error: %v", err)
	}

	if got.RepoDir != "/repo/worktree" {
		t.Fatalf("RepoDir = %q, want %q", got.RepoDir, "/repo/worktree")
	}

	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}

	if got.Task != "localci:test" {
		t.Fatalf("Task = %q, want %q", got.Task, "localci:test")
	}
}

func TestAppRunRecognizesInvoke(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
	}

	err := app.Run([]string{"invoke"})
	if err == nil {
		t.Fatalf("Run returned nil error, want usage error")
	}

	if got, want := err.Error(), "usage: localci invoke <repo> <commit>"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
