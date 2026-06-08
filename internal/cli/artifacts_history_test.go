package cli

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"localci/internal/localci"
)

func TestCatHelpIncludesRawAndExamples(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	app := App{
		Stdout: &stdout,
		Stderr: io.Discard,
		Cwd:    "/repo",
		CheckRequirements: func() error {
			t.Fatal("CheckRequirements should not run for help")
			return nil
		},
	}

	if err := app.Run([]string{"cat", "--help"}); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	rendered := stdout.String()
	for _, want := range []string{"localci cat --task noisy-fail", "localci cat report.txt --task build", "--raw"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("help output missing %q:\n%s", want, rendered)
		}
	}
}

func TestSelectCatArtifactUsesPrimaryByDefault(t *testing.T) {
	t.Parallel()

	task := catTestTask("localci:noisy-fail", 1, "combined.log", "report.txt")
	selected, err := selectCatArtifact(localci.Paths{}, "/repo", "abc123", []localci.TaskStatusView{task}, cliFlags{})
	if err != nil {
		t.Fatalf("selectCatArtifact returned error: %v", err)
	}
	if selected.Artifact.DisplayName != "combined.log" || !selected.Primary {
		t.Fatalf("selected = %#v, want primary combined.log", selected)
	}
}

func TestSelectCatArtifactMatchesNamedArtifact(t *testing.T) {
	t.Parallel()

	task := catTestTask("localci:noisy-fail", 1, "combined.log", "report.txt")
	selected, err := selectCatArtifact(localci.Paths{}, "/repo", "abc123", []localci.TaskStatusView{task}, cliFlags{Artifact: "report.txt"})
	if err != nil {
		t.Fatalf("selectCatArtifact returned error: %v", err)
	}
	if selected.Artifact.DisplayName != "report.txt" || selected.Primary {
		t.Fatalf("selected = %#v, want non-primary report.txt", selected)
	}
}

func TestSelectCatArtifactRejectsAmbiguousName(t *testing.T) {
	t.Parallel()

	tasks := []localci.TaskStatusView{
		catTestTask("localci:first", 1, "combined.log"),
		catTestTask("localci:second", 1, "combined.log"),
	}
	_, err := selectCatArtifact(localci.Paths{}, "/repo", "abc123", tasks, cliFlags{Artifact: "combined.log"})
	if err == nil {
		t.Fatalf("selectCatArtifact returned nil error, want ambiguity")
	}
	if got := err.Error(); !strings.Contains(got, "ambiguous") || !strings.Contains(got, "first attempt 1 combined.log") || !strings.Contains(got, "second attempt 1 combined.log") {
		t.Fatalf("error = %q, want candidate list", got)
	}
}

func TestSelectCatArtifactReportsMissingName(t *testing.T) {
	t.Parallel()

	_, err := selectCatArtifact(localci.Paths{}, "/repo", "abc123", []localci.TaskStatusView{
		catTestTask("localci:noisy-fail", 1, "combined.log"),
	}, cliFlags{Artifact: "missing.txt"})
	if err == nil {
		t.Fatalf("selectCatArtifact returned nil error, want not found")
	}
	if got := err.Error(); !strings.Contains(got, `artifact "missing.txt" not found`) {
		t.Fatalf("error = %q, want missing artifact message", got)
	}
}

func TestPrintTextCatArtifactRefusesNonText(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"artifact":{"display_name":"bin/tool"},"content":""}`)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := printTextCatArtifact(&stdout, server.URL, "/repo", "abc123", catTestTask("localci:build", 1, "bin/tool"), localci.ArtifactView{DisplayName: "bin/tool"})
	if err == nil {
		t.Fatalf("printTextCatArtifact returned nil error, want non-text error")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if got := err.Error(); !strings.Contains(got, "pass --raw") {
		t.Fatalf("error = %q, want --raw guidance", got)
	}
}

func TestStreamRawCatArtifactPrintsRawBytes(t *testing.T) {
	t.Parallel()

	raw := string([]byte{0, 1, 2, 3})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, raw)
	}))
	defer server.Close()

	var stdout bytes.Buffer
	err := streamRawCatArtifact(&stdout, server.URL, "/repo", "abc123", catTestTask("localci:build", 1, "bin/tool"), localci.ArtifactView{DisplayName: "bin/tool"})
	if err != nil {
		t.Fatalf("streamRawCatArtifact returned error: %v", err)
	}
	if got := stdout.String(); got != raw {
		t.Fatalf("stdout = %q, want raw bytes %q", got, raw)
	}
}

func catTestTask(name string, attempt int, artifacts ...string) localci.TaskStatusView {
	views := make([]localci.ArtifactView, 0, len(artifacts))
	for _, artifact := range artifacts {
		views = append(views, localci.ArtifactView{
			Name:        artifact,
			DisplayName: artifact,
		})
	}
	return localci.TaskStatusView{
		Name:      name,
		ShortName: strings.TrimPrefix(name, "localci:"),
		Attempt:   attempt,
		Artifacts: views,
	}
}
