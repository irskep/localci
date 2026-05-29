package cli

import (
	"bytes"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

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
