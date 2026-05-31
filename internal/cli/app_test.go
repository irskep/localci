package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"localci/internal/localci"
)

func TestDiscoverRepoFromNestedCWD(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	nestedDir := filepath.Join(repoDir, "sub", "dir")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	writeGitMetadata(t, repoDir)

	app := testApp(root, nestedDir)
	got, err := app.repoFromFlagOrCwd("")
	if err != nil {
		t.Fatalf("repoFromFlagOrCwd returned error: %v", err)
	}
	if got != repoDir {
		t.Fatalf("repo = %q, want %q", got, repoDir)
	}
}

func TestDiscoverRepoFromWorktreeGitFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.MkdirAll(repoDir, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}
	writeGitFile(t, repoDir)

	app := testApp(root, repoDir)
	got, err := app.repoFromFlagOrCwd("")
	if err != nil {
		t.Fatalf("repoFromFlagOrCwd returned error: %v", err)
	}
	if got != repoDir {
		t.Fatalf("repo = %q, want %q", got, repoDir)
	}
}

func TestDiscoverRepoErrorMentionsRepoFlag(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	cwd := filepath.Join(root, "not-repo")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("MkdirAll returned error: %v", err)
	}

	app := testApp(root, cwd)
	_, err := app.repoFromFlagOrCwd("")
	if err == nil {
		t.Fatalf("repoFromFlagOrCwd returned nil error, want discovery error")
	}
	if got := err.Error(); !strings.Contains(got, "pass --repo <path>") {
		t.Fatalf("error = %q, want --repo guidance", got)
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

	if got := err.Error(); !strings.Contains(got, "unknown command") {
		t.Fatalf("error = %q, want cobra positional argument error", got)
	}
}

func TestAppRunRecognizesRunUsage(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			return nil
		},
	}

	err := app.Run([]string{"run", "abc123"})
	if err == nil {
		t.Fatalf("Run returned nil error, want usage error")
	}

	if got := err.Error(); !strings.Contains(got, "unknown command") {
		t.Fatalf("error = %q, want cobra positional argument error", got)
	}
}

func TestRunNoCloneCommitLabel(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	writeGitMetadata(t, repoDir)

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	commit, err := app.resolveRunCommit(repoDir, cliFlags{NoClone: true})
	if err != nil {
		t.Fatalf("resolveRunCommit returned error: %v", err)
	}
	if commit != "abc123*" {
		t.Fatalf("commit = %q, want abc123*", commit)
	}
}

func TestResolveRunCommitDefaultsToHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	writeGitMetadata(t, repoDir)

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	repo, err := app.resolveSelectorRepo("")
	if err != nil {
		t.Fatalf("resolveSelectorRepo returned error: %v", err)
	}
	commit, err := app.resolveRunCommit(repo, cliFlags{})
	if err != nil {
		t.Fatalf("resolveRunCommit returned error: %v", err)
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
	writeGitMetadata(t, repoDir)

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	commit, err := app.resolveRunCommit(repoDir, cliFlags{Commit: "HEAD", NoClone: true})
	if err != nil {
		t.Fatalf("resolveRunCommit returned error: %v", err)
	}
	if got, want := commit, "abc123*"; got != want {
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
	writeGitMetadata(t, repoDir)

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

func TestResolveQueryCommitNoCloneDefaultsToCurrentHead(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repoDir := filepath.Join(root, "repo")
	if err := os.Mkdir(repoDir, 0o755); err != nil {
		t.Fatalf("Mkdir returned error: %v", err)
	}
	writeGitMetadata(t, repoDir)

	app := testApp(root, repoDir)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "abc123", nil
	}

	got, err := app.resolveQueryCommit(repoDir, cliFlags{NoClone: true})
	if err != nil {
		t.Fatalf("resolveQueryCommit returned error: %v", err)
	}
	if got != "abc123*" {
		t.Fatalf("commit = %q, want abc123*", got)
	}
}

func TestResolveQueryCommitNoCloneRepoFlagDefaultsToPathHead(t *testing.T) {
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
	writeGitMetadata(t, repoDir)

	app := testApp(root, cwd)
	app.HeadCommit = func(repo string) (string, error) {
		if repo != repoDir {
			t.Fatalf("repo = %q, want %q", repo, repoDir)
		}
		return "def456", nil
	}

	repo, err := app.resolveSelectorRepo("../repo")
	if err != nil {
		t.Fatalf("resolveSelectorRepo returned error: %v", err)
	}
	got, err := app.resolveQueryCommit(repo, cliFlags{NoClone: true})
	if err != nil {
		t.Fatalf("resolveQueryCommit returned error: %v", err)
	}
	if repo != repoDir || got != "def456*" {
		t.Fatalf("repo %q commit %q, want repo %q commit def456*", repo, got, repoDir)
	}
}

func TestAppRunRejectsInvalidAnnotations(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			t.Fatalf("requirements check should not run before flag validation")
			return nil
		},
	}

	err := app.Run([]string{"postcommit", "--annotation", "missing-value"})
	if err == nil {
		t.Fatalf("Run returned nil error, want invalid annotation error")
	}
	if got := err.Error(); !strings.Contains(got, "--annotation requires key=value") {
		t.Fatalf("error = %q, want invalid annotation error", got)
	}
}

func TestRunAnnotationsIncludeCommitSubjectOnlyForClonedRuns(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	initRealGitRepo(t, repoDir)
	commit, err := localci.GitHeadCommit(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("GitHeadCommit returned error: %v", err)
	}

	app := App{}
	cloned, err := app.runAnnotations(context.Background(), repoDir, commit, false, nil)
	if err != nil {
		t.Fatalf("runAnnotations returned error: %v", err)
	}
	if got, want := cloned[localci.AnnotationCommitSubject], "initial"; got != want {
		t.Fatalf("cloned commit subject = %q, want %q", got, want)
	}

	noClone, err := app.runAnnotations(context.Background(), repoDir, commit+"*", true, nil)
	if err != nil {
		t.Fatalf("no-clone runAnnotations returned error: %v", err)
	}
	if got := noClone[localci.AnnotationCommitSubject]; got != "" {
		t.Fatalf("no-clone commit subject = %q, want empty", got)
	}
}

func TestSelectedHistoryStatusesRejectsUnknownStatus(t *testing.T) {
	t.Parallel()

	_, err := selectedHistoryStatuses(cliFlags{Statuses: []string{"wat"}})
	if err == nil {
		t.Fatalf("selectedHistoryStatuses returned nil error, want invalid status error")
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

	if got := err.Error(); !strings.Contains(got, "unknown command") {
		t.Fatalf("error = %q, want cobra positional argument error", got)
	}
}

func TestAppRunSkipsRequirementsForHelp(t *testing.T) {
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

	if err := app.Run([]string{"help"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if called {
		t.Fatalf("requirements checker should not run for help")
	}
	rendered := stdout.String()
	for _, want := range []string{"localci is a local post-commit validation runner.", "Starting Tasks", "Viewing Status/History", "completion"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("help missing %q:\n%s", want, rendered)
		}
	}
}

func TestAppRunExposesCompletionHelp(t *testing.T) {
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

	if err := app.Run([]string{"completion", "--help"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if called {
		t.Fatalf("requirements checker should not run for completion help")
	}
	if rendered := stdout.String(); !strings.Contains(rendered, "Generate the autocompletion script") {
		t.Fatalf("completion help missing generated description: %s", rendered)
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
	if !strings.Contains(rendered, "Usage:\n  localci install-hooks [flags]") {
		t.Fatalf("help missing usage: %s", rendered)
	}
	if !strings.Contains(rendered, "Install LocalCI's Git post-commit hook") {
		t.Fatalf("help missing behavior summary: %s", rendered)
	}
	if !strings.Contains(rendered, "--repo string") {
		t.Fatalf("help missing repo flag: %s", rendered)
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

	err := app.Run([]string{"status"})
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

	if got := err.Error(); !strings.Contains(got, "unknown command") {
		t.Fatalf("error = %q, want cobra positional argument error", got)
	}
}

func TestAppRunRejectsPositionals(t *testing.T) {
	t.Parallel()

	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			return nil
		},
	}

	err := app.Run([]string{"status", "abc123"})
	if err == nil {
		t.Fatalf("Run returned nil error, want positional error")
	}
	if got := err.Error(); !strings.Contains(got, "unknown command") {
		t.Fatalf("error = %q, want cobra positional argument error", got)
	}
}

func TestLatestCommitForRepoReturnsHelpfulErrorWhenMissing(t *testing.T) {
	t.Parallel()

	app := App{LocalCIRoot: t.TempDir()}
	_, err := app.latestCommitForRepo("/definitely/missing/repo")
	if err == nil {
		t.Fatalf("latestCommitForRepo returned nil error, want missing-run error")
	}
	if got := err.Error(); !strings.Contains(got, "no localci runs found") {
		t.Fatalf("error = %q, want no localci runs found", got)
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

func TestPostcommitWaitInstruction(t *testing.T) {
	t.Parallel()

	got := postcommitWaitInstruction("/repo with spaces", "abc123")
	want := "Wait: localci wait --repo '/repo with spaces' --commit abc123"
	if got != want {
		t.Fatalf("postcommitWaitInstruction = %q, want %q", got, want)
	}
}

func TestPrintHistoryTaskRowsIncludesCommitSubject(t *testing.T) {
	t.Parallel()

	output := cliHistoryOutput{
		Repo: "/repo",
		Task: "localci:test",
		History: []cliHistoryRow{{
			Commit: "abc123456789",
			Annotations: map[string]string{
				localci.AnnotationCommitSubject: "Fix task history",
			},
			Task:         "localci:test",
			ShortTask:    "test",
			Status:       localci.ExecutionStatusFailed,
			Attempt:      2,
			AttemptCount: 3,
			Duration:     "54ms",
			Failure:      "exit",
			Updated:      "now",
		}},
	}

	var buf bytes.Buffer
	printHistoryOutput(&buf, output)
	rendered := buf.String()
	for _, want := range []string{"message", "Fix task history", "failed", "2/3", "exit"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("history output missing %q:\n%s", want, rendered)
		}
	}
}

func testApp(_ string, cwd string) App {
	return App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		Cwd:    cwd,
	}
}

func writeGitMetadata(t *testing.T, repoDir string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Join(repoDir, ".git"), 0o755); err != nil {
		t.Fatalf("MkdirAll .git returned error: %v", err)
	}
}

func writeGitFile(t *testing.T, repoDir string) {
	t.Helper()

	if err := os.WriteFile(filepath.Join(repoDir, ".git"), []byte("gitdir: /tmp/worktree\n"), 0o644); err != nil {
		t.Fatalf("WriteFile .git returned error: %v", err)
	}
}

func initRealGitRepo(t *testing.T, repoDir string) {
	t.Helper()

	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.email", "localci@example.test")
	runGit(t, repoDir, "config", "user.name", "localci")
	if err := os.WriteFile(filepath.Join(repoDir, "README.md"), []byte("test repo\n"), 0o644); err != nil {
		t.Fatalf("WriteFile README returned error: %v", err)
	}
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir, "commit", "-m", "initial")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
}
