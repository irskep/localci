package localci

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDiscoverTasksFiltersAndSortsLocalCITasks(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
if [ "$1" = "tasks" ] && [ "$2" = "--json" ] && [ "$3" = "--all" ]; then
  if [ "${MISE_EXPERIMENTAL:-}" != "1" ]; then
    exit 2
  fi
  printf '%s\n' '[{"name":"zeta"},{"name":"localci:test"},{"name":"//web:localci:lint"},{"name":"localci:build"}]'
  exit 0
fi
exit 1
`)

	runner := Runner{
		Paths:   Paths{Root: t.TempDir()},
		MiseBin: filepath.Join(binDir, "mise"),
		Env:     append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH")),
	}

	tasks, err := runner.DiscoverTasks(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("DiscoverTasks returned error: %v", err)
	}

	if len(tasks) != 3 {
		t.Fatalf("len(tasks) = %d, want 3", len(tasks))
	}
	if tasks[0].Name != "//web:localci:lint" || tasks[1].Name != "localci:build" || tasks[2].Name != "localci:test" {
		t.Fatalf("unexpected task order: %#v", tasks)
	}
}

func TestInvokeRunsTasksSeriallyAndWritesResults(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	rootDir := t.TempDir()
	binDir := t.TempDir()
	markerFile := filepath.Join(rootDir, "serial-order.txt")

	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
if [ "$1" = "tasks" ] && [ "$2" = "--json" ] && [ "$3" = "--all" ]; then
  printf '%s\n' '[{"name":"localci:first"},{"name":"localci:second"}]'
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "localci:first" ]; then
  printf 'first\n' >>"`+markerFile+`"
  printf 'first stdout\n'
  printf 'first stderr\n' >&2
  printf 'first output\n' >"$LOCALCI_TASK_OUTPUT_DIR/first.log"
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "localci:second" ]; then
  printf 'second\n' >>"`+markerFile+`"
  printf 'second stdout\n'
  printf 'second stderr\n' >&2
  printf 'second output\n' >"$LOCALCI_TASK_OUTPUT_DIR/second.log"
  exit 0
fi
exit 1
`)

	runner := Runner{
		Paths:        Paths{Root: rootDir},
		MiseBin:      filepath.Join(binDir, "mise"),
		Env:          append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH")),
		PollInterval: 10 * time.Millisecond,
	}

	result, err := runner.Invoke(context.Background(), InvokeRequest{
		RepoDir: repoDir,
		Commit:  "abc123",
	})
	if err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}
	if !result.Success() {
		t.Fatalf("Invoke result should be successful: %#v", result.TaskResults)
	}

	orderBytes, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("ReadFile(markerFile) returned error: %v", err)
	}
	if got, want := strings.TrimSpace(string(orderBytes)), "first\nsecond"; got != want {
		t.Fatalf("serial execution order = %q, want %q", got, want)
	}

	runPath := filepath.Join(rootDir, normalizeRepoDir(repoDir), "abc123", "run.json")
	if _, err := os.Stat(runPath); err != nil {
		t.Fatalf("run metadata missing at %s: %v", runPath, err)
	}

	if result.Status != RunStatusSucceeded {
		t.Fatalf("Run status = %q, want %q", result.Status, RunStatusSucceeded)
	}
	if result.Summary.Total != 2 || result.Summary.Succeeded != 2 {
		t.Fatalf("unexpected run summary: %#v", result.Summary)
	}

	for _, task := range []string{"localci:first", "localci:second"} {
		taskDir := filepath.Join(rootDir, normalizeRepoDir(repoDir), "abc123", "out", sanitizeTaskName(task), "attempt-001")
		taskPath := filepath.Join(taskDir, "task.json")
		if _, err := os.Stat(taskPath); err != nil {
			t.Fatalf("task record missing for %s at %s: %v", task, taskPath, err)
		}
		for _, logName := range []string{"combined.log"} {
			logPath := filepath.Join(taskDir, logName)
			if _, err := os.Stat(logPath); err != nil {
				t.Fatalf("log %s missing for %s: %v", logName, task, err)
			}
		}
	}
}

func TestInvokeIncrementsTaskAttemptDirectories(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	rootDir := t.TempDir()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
if [ "$1" = "tasks" ] && [ "$2" = "--json" ] && [ "$3" = "--all" ]; then
  printf '%s\n' '[{"name":"localci:test"}]'
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "localci:test" ]; then
  printf 'ok\n' >"$LOCALCI_TASK_OUTPUT_DIR/test.log"
  exit 0
fi
exit 1
`)

	runner := Runner{
		Paths:        Paths{Root: rootDir},
		MiseBin:      filepath.Join(binDir, "mise"),
		Env:          append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH")),
		PollInterval: 10 * time.Millisecond,
	}

	for range 2 {
		if _, err := runner.Invoke(context.Background(), InvokeRequest{
			RepoDir: repoDir,
			Commit:  "abc123",
		}); err != nil {
			t.Fatalf("Invoke returned error: %v", err)
		}
	}

	for _, attempt := range []string{"attempt-001", "attempt-002"} {
		taskPath := filepath.Join(rootDir, normalizeRepoDir(repoDir), "abc123", "out", sanitizeTaskName("localci:test"), attempt, "task.json")
		if _, err := os.Stat(taskPath); err != nil {
			t.Fatalf("task record missing for %s at %s: %v", attempt, taskPath, err)
		}
	}
}

func writeExecutable(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
}

func TestInvokeCapturesCombinedLog(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	rootDir := t.TempDir()
	binDir := t.TempDir()

	writeExecutable(t, filepath.Join(binDir, "mise"), `#!/bin/sh
set -eu
if [ "$1" = "tasks" ] && [ "$2" = "--json" ] && [ "$3" = "--all" ]; then
  printf '%s\n' '[{"name":"localci:test"}]'
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "localci:test" ]; then
  printf 'hello stdout\n'
  printf 'hello stderr\n' >&2
  exit 0
fi
exit 1
`)

	runner := Runner{
		Paths:        Paths{Root: rootDir},
		MiseBin:      filepath.Join(binDir, "mise"),
		Env:          append(os.Environ(), "PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH")),
		PollInterval: 10 * time.Millisecond,
	}

	if _, err := runner.Invoke(context.Background(), InvokeRequest{
		RepoDir: repoDir,
		Commit:  "abc123",
	}); err != nil {
		t.Fatalf("Invoke returned error: %v", err)
	}

	taskDir := filepath.Join(rootDir, normalizeRepoDir(repoDir), "abc123", "out", sanitizeTaskName("localci:test"), "attempt-001")
	combinedBytes, err := os.ReadFile(filepath.Join(taskDir, "combined.log"))
	if err != nil {
		t.Fatalf("ReadFile(combined.log) returned error: %v", err)
	}
	combined := string(combinedBytes)
	if !strings.Contains(combined, "hello stdout\n") || !strings.Contains(combined, "hello stderr\n") {
		t.Fatalf("combined.log missing expected output: %q", combined)
	}
}
