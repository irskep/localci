package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"localci/internal/localci"
)

func TestParseCommitTargetDefaultsDirToCWD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	got, err := app.parseCommitTarget([]string{"abc123"}, "usage")
	if err != nil {
		t.Fatalf("parseCommitTarget returned error: %v", err)
	}

	if got.RepoDir != repoDir {
		t.Fatalf("RepoDir = %q, want %q", got.RepoDir, repoDir)
	}
	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}
	if got.Task != "" {
		t.Fatalf("Task = %q, want empty", got.Task)
	}
}

func TestParseCommitTargetWithRepoAndTaskUsesCWD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	baseDir := filepath.Join(root, "workspace")
	repoDir := filepath.Join(baseDir, "worktree")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	app := testApp(root, baseDir)
	got, err := app.parseCommitTarget([]string{"./worktree", "abc123", "localci:test"}, "usage")
	if err != nil {
		t.Fatalf("parseCommitTarget returned error: %v", err)
	}

	if got.RepoDir != repoDir {
		t.Fatalf("RepoDir = %q, want %q", got.RepoDir, repoDir)
	}
	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}
	if got.Task != "localci:test" {
		t.Fatalf("Task = %q, want %q", got.Task, "localci:test")
	}
}

func TestParseCommitTargetRejectsPathsOutsideConfiguredRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	baseDir := filepath.Join(root, "workspace")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	app := testApp(root, baseDir)
	_, err := app.parseCommitTarget([]string{"../../outside", "abc123"}, "usage")
	if err == nil {
		t.Fatalf("parseCommitTarget returned nil error, want path error")
	}
}

func TestLoadConfigDefaultsRootToSlashWhenConfigIsMissing(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout:     io.Discard,
		Stderr:     io.Discard,
		Cwd:        "/repo",
		ConfigPath: filepath.Join(t.TempDir(), "missing-config.toml"),
	}

	cfg, err := app.loadConfig()
	if err != nil {
		t.Fatalf("loadConfig returned error: %v", err)
	}
	if cfg.Root != string(filepath.Separator) {
		t.Fatalf("Root = %q, want %q", cfg.Root, string(filepath.Separator))
	}
}

func TestParseWebTargetDefaultsToCWDRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	got, err := app.parseWebTarget(nil)
	if err != nil {
		t.Fatalf("parseWebTarget returned error: %v", err)
	}

	want := commitTarget{RepoDir: repoDir}
	if got != want {
		t.Fatalf("got = %#v, want %#v", got, want)
	}
}

func TestParseWebTargetSingleDirOpensRepoPage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := filepath.Join(root, "workspace")
	repoDir := filepath.Join(cwd, "worktree")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	app := testApp(root, cwd)
	got, err := app.parseWebTarget([]string{"./worktree"})
	if err != nil {
		t.Fatalf("parseWebTarget returned error: %v", err)
	}
	if got.RepoDir != repoDir || got.Commit != "" || got.Task != "" {
		t.Fatalf("unexpected target: %#v", got)
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

func TestAppRunRecognizesRestart(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			return nil
		},
	}

	err := app.Run([]string{"restart", "extra"})
	if err == nil {
		t.Fatalf("Run returned nil error, want usage error")
	}

	if got, want := err.Error(), "restart takes no arguments"; got != want {
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
	if !errors.Is(err, io.EOF) {
		t.Fatalf("error = %v, want %v", err, io.EOF)
	}
}

func TestAppRunRecognizesInstallHooks(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			return nil
		},
	}

	err := app.Run([]string{"install-hooks", "a", "b"})
	if err == nil {
		t.Fatalf("Run returned nil error, want usage error")
	}

	if got, want := err.Error(), "usage: localci install-hooks [dir]"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestParseCommitTargetReturnsCommandSpecificUsage(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	_, err := app.parseCommitTarget([]string{}, "usage: localci web [dir] <commit> [task]")
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

func TestBuildWebURLRepo(t *testing.T) {
	t.Parallel()

	got, err := buildWebURL("http://127.0.0.1:4312", commitTarget{
		RepoDir: "/repo",
	})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/repo?repo=%2Frepo"
	if got != want {
		t.Fatalf("buildWebURL = %q, want %q", got, want)
	}
}

func TestBuildWebURLHome(t *testing.T) {
	t.Parallel()

	got, err := buildWebURL("http://127.0.0.1:4312", commitTarget{})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/"
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

func TestPluralizeTask(t *testing.T) {
	t.Parallel()

	if got := pluralizeTask(1); got != "task" {
		t.Fatalf("pluralizeTask(1) = %q, want %q", got, "task")
	}
	if got := pluralizeTask(2); got != "tasks" {
		t.Fatalf("pluralizeTask(2) = %q, want %q", got, "tasks")
	}
}

func TestShellQuote(t *testing.T) {
	t.Parallel()

	if got := shellQuote("/repo/path"); got != "/repo/path" {
		t.Fatalf("shellQuote simple = %q, want %q", got, "/repo/path")
	}
	if got := shellQuote("/repo with spaces"); got != "'/repo with spaces'" {
		t.Fatalf("shellQuote spaces = %q, want %q", got, "'/repo with spaces'")
	}
	if got := shellQuote("O'Brien"); got != `'O'"'"'Brien'` {
		t.Fatalf("shellQuote apostrophe = %q, want %q", got, `'O'"'"'Brien'`)
	}
}

func testApp(root string, cwd string) App {
	return App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    cwd,
		LoadConfig: func(string) (localci.Config, error) {
			return localci.Config{Root: root}, nil
		},
	}
}
