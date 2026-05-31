package cli

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"localci/internal/localci"
)

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
			localci.AnnotationBranch:        "main",
			localci.AnnotationCommitSubject: "Fix artifact tab overflow",
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
		"message  Fix artifact tab overflow\n",
		"branch   main\n",
		"status  task   attempt  duration  failure\n",
		"ok      build  1        1.2s      -",
		"failed  test   2/3      42ms      exit",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("status summary missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, localci.AnnotationCommitSubject) {
		t.Fatalf("status summary rendered raw commit subject annotation:\n%s", rendered)
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
