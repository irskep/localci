package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"localci/internal/localci"
)

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

func TestDocsPlainPrintsBundledTextWithoutRequirementCheck(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := App{
		Stdout: &stdout,
		Stderr: io.Discard,
		CheckRequirements: func() error {
			t.Fatalf("docs should not check daemon or repo requirements")
			return nil
		},
	}

	if err := app.Run([]string{"docs", "--plain"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "LocalCI") || !strings.Contains(got, "Getting Started") {
		t.Fatalf("docs output did not include bundled narrative docs: %q", got)
	}
}

func TestDocsRoffPrintsBundledManPage(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := App{Stdout: &stdout, Stderr: io.Discard}

	if err := app.Run([]string{"docs", "--roff"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, ".TH") || !strings.Contains(got, "LOCALCI") {
		t.Fatalf("roff output did not look like a man page: %q", got)
	}
}

func TestDocsDefaultRendersTempManPage(t *testing.T) {
	t.Parallel()

	called := false
	app := App{
		Stdout: io.Discard,
		Stderr: io.Discard,
		StdoutIsTerminal: func() bool {
			return true
		},
		RenderManPage: func(path string) error {
			called = true
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("ReadFile returned error: %v", err)
			}
			if got := string(content); !strings.Contains(got, ".TH") || !strings.Contains(got, "LOCALCI") {
				t.Fatalf("temp man page did not include generated roff: %q", got)
			}
			return nil
		},
	}

	if err := app.Run([]string{"docs"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !called {
		t.Fatalf("RenderManPage was not called")
	}
}

func TestDocsDefaultFallsBackToPlainText(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := App{
		Stdout: &stdout,
		Stderr: io.Discard,
		StdoutIsTerminal: func() bool {
			return true
		},
		RenderManPage: func(path string) error {
			return errors.New("no man")
		},
	}

	if err := app.Run([]string{"docs"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "LocalCI") || !strings.Contains(got, "Getting Started") {
		t.Fatalf("fallback output did not include bundled docs: %q", got)
	}
}

func TestDocsDefaultPrintsPlainTextWhenStdoutIsNotATerminal(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := App{
		Stdout: &stdout,
		Stderr: io.Discard,
		StdoutIsTerminal: func() bool {
			return false
		},
		RenderManPage: func(path string) error {
			t.Fatalf("non-terminal docs output should not invoke man")
			return nil
		},
	}

	if err := app.Run([]string{"docs"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if got := stdout.String(); !strings.Contains(got, "LocalCI") || !strings.Contains(got, "Getting Started") {
		t.Fatalf("non-terminal output did not include bundled plain text: %q", got)
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

func TestPostcommitWaitInstruction(t *testing.T) {
	t.Parallel()

	got := postcommitWaitInstruction("/repo with spaces", "abc123")
	want := "Wait: localci wait --repo '/repo with spaces' --commit abc123"
	if got != want {
		t.Fatalf("postcommitWaitInstruction = %q, want %q", got, want)
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

func TestPrintStatusSummaryUsesStructuredRows(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := App{Stdout: &stdout, Stderr: io.Discard}
	view := localci.CommitStatusView{
		RepoDir: "/repo",
		Commit:  "abc123",
		Annotations: map[string]string{
			"branch": "main",
		},
		Tasks: []localci.TaskStatusView{
			{
				ShortName:            "build",
				Status:               localci.ExecutionStatusSucceeded,
				Attempt:              1,
				DurationMilliseconds: 1200,
			},
			{
				ShortName:            "test",
				Status:               localci.ExecutionStatusFailed,
				Attempt:              2,
				AttemptCount:         3,
				DurationMilliseconds: 42,
				Failure:              "exit",
			},
		},
	}

	app.printStatusSummary(view, view.Tasks)

	rendered := ansi.Strip(stdout.String())
	for _, want := range []string{
		"Status\n",
		"repo     /repo\n",
		"commit   abc123\n",
		"summary  1 passed, 1 failed, 0 timed out, 0 not run\n",
		"branch   main\n",
		"status  task   attempt  duration  failure\n",
		"ok      build  1        1.2s      -",
		"failed  test   2/3      42ms      exit",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("status summary missing %q:\n%s", want, rendered)
		}
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
