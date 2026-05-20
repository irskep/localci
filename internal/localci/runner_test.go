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
if [ "$1" = "tasks" ] && [ "$2" = "--json" ] && [ "$3" = "--local" ]; then
  printf '%s\n' '[{"name":"zeta"},{"name":"localci:test"},{"name":"localci:build"}]'
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

	if len(tasks) != 2 {
		t.Fatalf("len(tasks) = %d, want 2", len(tasks))
	}
	if tasks[0].Name != "localci:build" || tasks[1].Name != "localci:test" {
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
if [ "$1" = "tasks" ] && [ "$2" = "--json" ] && [ "$3" = "--local" ]; then
  printf '%s\n' '[{"name":"localci:first"},{"name":"localci:second"}]'
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "localci:first" ]; then
  printf 'first\n' >>"`+markerFile+`"
  printf 'first output\n' >"$LOCALCI_TASK_OUTPUT_DIR/first.log"
  exit 0
fi
if [ "$1" = "run" ] && [ "$2" = "localci:second" ]; then
  printf 'second\n' >>"`+markerFile+`"
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

	invokePath := filepath.Join(rootDir, normalizeRepoDir(repoDir), "abc123", "invoke.json")
	if _, err := os.Stat(invokePath); err != nil {
		t.Fatalf("invoke metadata missing at %s: %v", invokePath, err)
	}

	for _, task := range []string{"localci:first", "localci:second"} {
		resultPath := filepath.Join(rootDir, normalizeRepoDir(repoDir), "abc123", "out", sanitizeTaskName(task), "result.json")
		if _, err := os.Stat(resultPath); err != nil {
			t.Fatalf("task result missing for %s at %s: %v", task, resultPath, err)
		}
	}
}

func writeExecutable(t *testing.T, path string, contents string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(contents), 0o755); err != nil {
		t.Fatalf("WriteFile(%s) returned error: %v", path, err)
	}
}
