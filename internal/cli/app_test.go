package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
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

func TestParseCommitTargetWithoutArgsUsesLatestCommitForRepo(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.LatestCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "latest123", nil
	}

	got, err := app.parseCommitTarget(nil, "usage")
	if err != nil {
		t.Fatalf("parseCommitTarget returned error: %v", err)
	}
	if got.RepoDir != repoDir {
		t.Fatalf("RepoDir = %q, want %q", got.RepoDir, repoDir)
	}
	if got.Commit != "latest123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "latest123")
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

func TestParseCommitTargetResolvesHeadAlias(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	got, err := app.parseCommitTarget([]string{"HEAD"}, "usage")
	if err != nil {
		t.Fatalf("parseCommitTarget returned error: %v", err)
	}

	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}
}

func TestParseCommitTargetWithCommitAndTaskUsesCWD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	got, err := app.parseCommitTarget([]string{"HEAD", "//:localci:noisy-fail"}, "usage")
	if err != nil {
		t.Fatalf("parseCommitTarget returned error: %v", err)
	}

	if got.RepoDir != repoDir {
		t.Fatalf("RepoDir = %q, want %q", got.RepoDir, repoDir)
	}
	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}
	if got.Task != "//:localci:noisy-fail" {
		t.Fatalf("Task = %q, want %q", got.Task, "//:localci:noisy-fail")
	}
}

func TestParseCommitTargetWithMissingPathStillReportsPathError(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	_, err := app.parseCommitTarget([]string{"./missing", "HEAD"}, "usage")
	if err == nil {
		t.Fatalf("parseCommitTarget returned nil error, want path error")
	}
}

func TestParseWebTargetResolvesHeadAlias(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	got, err := app.parseWebTarget([]string{"HEAD"})
	if err != nil {
		t.Fatalf("parseWebTarget returned error: %v", err)
	}

	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}
}

func TestParseWebTargetWithCommitAndTaskUsesCWD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	got, err := app.parseWebTarget([]string{"HEAD", "//:localci:noisy-fail"})
	if err != nil {
		t.Fatalf("parseWebTarget returned error: %v", err)
	}

	if got.RepoDir != repoDir {
		t.Fatalf("RepoDir = %q, want %q", got.RepoDir, repoDir)
	}
	if got.Commit != "abc123" {
		t.Fatalf("Commit = %q, want %q", got.Commit, "abc123")
	}
	if got.Task != "//:localci:noisy-fail" {
		t.Fatalf("Task = %q, want %q", got.Task, "//:localci:noisy-fail")
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

func TestAppRunRecognizesInvokeUsage(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			return nil
		},
	}

	err := app.Run([]string{"invoke", "a", "b", "c"})
	if err == nil {
		t.Fatalf("Run returned nil error, want usage error")
	}

	if got, want := err.Error(), "usage: localci invoke [--wait] [--no-clone] [--annotation key=value] [dir] [commit]"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestParseInvokeTargetDefaultsToCWDAndHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	repo, commit, err := app.parseInvokeTarget(nil)
	if err != nil {
		t.Fatalf("parseInvokeTarget returned error: %v", err)
	}
	if commit == "" {
		commit, err = app.headCommit(repo)
		if err != nil {
			t.Fatalf("headCommit returned error: %v", err)
		}
	}

	if repo != repoDir {
		t.Fatalf("repo = %q, want %q", repo, repoDir)
	}
	if commit != "abc123" {
		t.Fatalf("commit = %q, want %q", commit, "abc123")
	}
}

func TestInvokeTargetResolvesExplicitHeadBeforeNoCloneLabel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	repo, commit, err := app.parseInvokeTarget([]string{repoDir, "HEAD"})
	if err != nil {
		t.Fatalf("parseInvokeTarget returned error: %v", err)
	}
	commit, err = app.resolveCommitAlias(repo, commit)
	if err != nil {
		t.Fatalf("resolveCommitAlias returned error: %v", err)
	}
	if got, want := noCloneCommitLabel(commit), "abc123*"; got != want {
		t.Fatalf("no-clone invoke commit = %q, want %q", got, want)
	}
}

func TestNoCloneCommitLabel(t *testing.T) {
	t.Parallel()

	if got, want := noCloneCommitLabel("abc123"), "abc123*"; got != want {
		t.Fatalf("noCloneCommitLabel = %q, want %q", got, want)
	}
	if got, want := noCloneCommitLabel("abc123*"), "abc123*"; got != want {
		t.Fatalf("noCloneCommitLabel should not double suffix: %q", got)
	}
}

func TestResolveCommitAliasSupportsNoCloneHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	got, err := app.resolveCommitAlias(repoDir, "HEAD*")
	if err != nil {
		t.Fatalf("resolveCommitAlias returned error: %v", err)
	}
	if got != "abc123*" {
		t.Fatalf("commit = %q, want abc123*", got)
	}
}

func TestParseStatusTargetNoCloneDefaultsToCurrentHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	got, err := app.parseStatusTarget(nil, true, "usage")
	if err != nil {
		t.Fatalf("parseStatusTarget returned error: %v", err)
	}
	if got.RepoDir != repoDir || got.Commit != "abc123*" {
		t.Fatalf("target = %#v, want repo %q commit abc123*", got, repoDir)
	}
}

func TestParseStatusTargetNoClonePathDefaultsToPathHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := filepath.Join(root, "cwd")
	repoDir := filepath.Join(root, "repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("MkdirAll cwd returned error: %v", err)
	}
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir repo returned error: %v", err)
	}

	app := testApp(root, cwd)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "def456", nil
	}

	got, err := app.parseStatusTarget([]string{"../repo"}, true, "usage")
	if err != nil {
		t.Fatalf("parseStatusTarget returned error: %v", err)
	}
	if got.RepoDir != repoDir || got.Commit != "def456*" {
		t.Fatalf("target = %#v, want repo %q commit def456*", got, repoDir)
	}
}

func TestParseAnnotationArgs(t *testing.T) {
	t.Parallel()

	annotations, remaining, err := parseAnnotationArgs([]string{
		"--annotation", "branch=main",
		"--annotation=ci.source=manual",
		"/repo",
		"abc123",
	})
	if err != nil {
		t.Fatalf("parseAnnotationArgs returned error: %v", err)
	}

	if got, want := annotations["branch"], "main"; got != want {
		t.Fatalf("branch = %q, want %q", got, want)
	}
	if got, want := annotations["ci.source"], "manual"; got != want {
		t.Fatalf("ci.source = %q, want %q", got, want)
	}
	if got, want := strings.Join(remaining, " "), "/repo abc123"; got != want {
		t.Fatalf("remaining = %q, want %q", got, want)
	}
}

func TestParseAnnotationArgsRejectsInvalidAnnotations(t *testing.T) {
	t.Parallel()

	if _, _, err := parseAnnotationArgs([]string{"--annotation", "missing-value"}); err == nil {
		t.Fatalf("parseAnnotationArgs returned nil error, want invalid annotation error")
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

func TestAppRunInstallHooksHelpSkipsRequirementsCheck(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	called := false
	app := App{
		Stdout: &stdout,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			called = true
			return nil
		},
	}

	if err := app.Run([]string{"install-hooks", "--help"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if called {
		t.Fatalf("requirements checker should not run for command help")
	}
	rendered := stdout.String()
	if !strings.Contains(rendered, "Usage:\n  localci install-hooks [dir]") {
		t.Fatalf("help missing usage: %s", rendered)
	}
	if !strings.Contains(rendered, "modern Git hook.* config") {
		t.Fatalf("help missing behavior summary: %s", rendered)
	}
	if !strings.Contains(rendered, `localci postcommit "$repo" "$commit"`) {
		t.Fatalf("help missing installed command: %s", rendered)
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
	app.LatestCommit = func(string) (string, error) {
		return "", errors.New("no runs")
	}
	_, err := app.parseCommitTarget([]string{"a", "b", "c", "d"}, "usage: localci web [dir] <commit> [task]")
	if err == nil {
		t.Fatalf("parseCommitTarget returned nil error, want usage error")
	}
	if got, want := err.Error(), "usage: localci web [dir] <commit> [task]"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestLatestCommitForRepoReturnsHelpfulErrorWhenMissing(t *testing.T) {
	t.Parallel()

	app := App{}
	_, err := app.latestCommitForRepo("/definitely/missing/repo")
	if err == nil {
		t.Fatalf("latestCommitForRepo returned nil error, want missing-run error")
	}
	if got := err.Error(); !strings.Contains(got, "no localci runs found") {
		t.Fatalf("error = %q, want no localci runs found", got)
	}
}

func TestBuildWebURLCommit(t *testing.T) {
	t.Parallel()

	app := testApp("/", "/repo")
	got, err := app.buildWebURL("http://127.0.0.1:4312", commitTarget{
		RepoDir: "/repo",
		Commit:  "abc123",
	})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/repo/repo/commit/abc123"
	if got != want {
		t.Fatalf("buildWebURL = %q, want %q", got, want)
	}
}

func TestBuildWebURLRepo(t *testing.T) {
	t.Parallel()

	app := testApp("/", "/repo")
	got, err := app.buildWebURL("http://127.0.0.1:4312", commitTarget{
		RepoDir: "/repo",
	})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/repo/repo"
	if got != want {
		t.Fatalf("buildWebURL = %q, want %q", got, want)
	}
}

func TestBuildWebURLHome(t *testing.T) {
	t.Parallel()

	app := testApp("/", "/repo")
	got, err := app.buildWebURL("http://127.0.0.1:4312", commitTarget{})
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

	app := testApp("/", "/repo")
	got, err := app.buildWebURL("http://127.0.0.1:4312", commitTarget{
		RepoDir: "/repo",
		Commit:  "abc123",
		Task:    "localci:test",
	})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/repo/repo/commit/abc123/task/localci:test"
	if got != want {
		t.Fatalf("buildWebURL = %q, want %q", got, want)
	}
}

func TestBuildWebURLTaskWithSlashes(t *testing.T) {
	t.Parallel()

	app := testApp("/", "/repo")
	got, err := app.buildWebURL("http://127.0.0.1:4312", commitTarget{
		RepoDir: "/repo",
		Commit:  "abc123",
		Task:    "//:localci:noisy-fail",
	})
	if err != nil {
		t.Fatalf("buildWebURL returned error: %v", err)
	}

	want := "http://127.0.0.1:4312/repo/repo/commit/abc123/task/%2F%2F:localci:noisy-fail"
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
		LoadConfig: func(string) (localci.Config, error) {
			return localci.Config{Root: "/"}, nil
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

	want := "http://127.0.0.1:4312/repo/repo/commit/abc123/task/localci:test"
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

func TestFormatTaskSummary(t *testing.T) {
	t.Parallel()

	got := formatTaskSummary(localci.TaskStatusView{
		ShortName:            "build",
		Status:               localci.ExecutionStatusSucceeded,
		Attempt:              2,
		AttemptCount:         3,
		DurationMilliseconds: 483,
	})
	want := "succeeded  build  attempt 2 of 3  483ms"
	if got != want {
		t.Fatalf("formatTaskSummary = %q, want %q", got, want)
	}
}

func TestPrintTaskDetailUsesArtifactDisplayNames(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := App{Stdout: &stdout, Stderr: io.Discard}
	app.printTaskDetail(localci.TaskStatusView{
		ShortName: "test",
		Status:    localci.ExecutionStatusFailed,
		Attempt:   1,
		OutputDir: "/tmp/out",
		Failure:   "exit-code",
		Artifacts: []localci.ArtifactView{
			{DisplayName: "combined.log", Path: "/tmp/out/combined.log"},
			{DisplayName: "dist/index.html", Path: "/tmp/out/dist/index.html"},
		},
	})

	rendered := stdout.String()
	if !strings.Contains(rendered, "failed  test  attempt 1  failure=exit-code") {
		t.Fatalf("detail missing summary: %s", rendered)
	}
	if !strings.Contains(rendered, "Artifacts:\n  combined.log\t/tmp/out/combined.log\n  dist/index.html\t/tmp/out/dist/index.html\n") {
		t.Fatalf("detail missing artifact paths: %s", rendered)
	}
	if !strings.Contains(rendered, "Primary log path: /tmp/out/combined.log\n") {
		t.Fatalf("detail missing primary log path: %s", rendered)
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
