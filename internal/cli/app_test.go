package cli

import (
	"bytes"
	"io"
	"testing"

	"localci/internal/localci"
)

func TestParseCommitTargetDefaultsDirToCWD(t *testing.T) {
	t.Parallel()

	got, err := parseCommitTarget([]string{"abc123"}, "/repo", "usage")
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

	got, err := parseCommitTarget([]string{"./worktree", "abc123", "localci:test"}, "/repo", "usage")
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
		CheckRequirements: func() error {
			return nil
		},
	}

	err := app.Run([]string{"invoke"})
	if err == nil {
		t.Fatalf("Run returned nil error, want usage error")
	}

	if got, want := err.Error(), "usage: localci invoke <repo> <commit>"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestAppRunSkipsRequirementsForHelp(t *testing.T) {
	t.Parallel()

	called := false
	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			called = true
			return nil
		},
	}

	if err := app.Run([]string{"help"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if called {
		t.Fatalf("requirements checker should not run for help")
	}
}

func TestAppRunChecksRequirementsForCommands(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			return io.EOF
		},
	}

	err := app.Run([]string{"status", "abc123"})
	if err == nil {
		t.Fatalf("Run returned nil error, want requirements error")
	}
	if err != io.EOF {
		t.Fatalf("error = %v, want %v", err, io.EOF)
	}
}

func TestParseCommitTargetReturnsCommandSpecificUsage(t *testing.T) {
	t.Parallel()

	_, err := parseCommitTarget([]string{}, "/repo", "usage: localci web [dir] <commit> [task]")
	if err == nil {
		t.Fatalf("parseCommitTarget returned nil error, want usage error")
	}
	if got, want := err.Error(), "usage: localci web [dir] <commit> [task]"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestBuildWebURLCommit(t *testing.T) {
	t.Parallel()

	got, err := buildWebURL("http://127.0.0.1:4312", commitTarget{
		RepoDir: "/repo",
		Commit:  "abc123",
	})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/commit?commit=abc123&repo=%2Frepo"
	if got != want {
		t.Fatalf("buildWebURL = %q, want %q", got, want)
	}
}

func TestBuildWebURLTask(t *testing.T) {
	t.Parallel()

	got, err := buildWebURL("http://127.0.0.1:4312", commitTarget{
		RepoDir: "/repo",
		Commit:  "abc123",
		Task:    "localci:test",
	})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/task?commit=abc123&repo=%2Frepo&task=localci%3Atest"
	if got != want {
		t.Fatalf("buildWebURL = %q, want %q", got, want)
	}
}

func TestOpenWebOpensURLAndPrintsIt(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var opened string
	app := App{
		Stdout: &stdout,
		Stderr: io.Discard,
		Cwd:    "/repo",
		OpenURL: func(target string) error {
			opened = target
			return nil
		},
	}

	err := app.openWeb(commitTarget{
		RepoDir: "/repo",
		Commit:  "abc123",
		Task:    "localci:test",
	}, localci.DaemonState{
		HTTPBaseURL: "http://127.0.0.1:4312",
	})
	if err != nil {
		t.Fatalf("openWeb returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/task?commit=abc123&repo=%2Frepo&task=localci%3Atest"
	if opened != want {
		t.Fatalf("opened url = %q, want %q", opened, want)
	}
	if got := stdout.String(); got != want+"\n" {
		t.Fatalf("stdout = %q, want %q", got, want+"\n")
	}
}
