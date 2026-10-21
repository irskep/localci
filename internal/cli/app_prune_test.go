package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"localci/internal/localci"
)

func TestPruneConfirmation(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		args        []string
		removed     bool
	}{
		{"no", "n\n", nil, false}, {"empty", "\n", nil, false}, {"eof", "", nil, false},
		{"yes", "y\n", nil, true}, {"flag", "", []string{"--yes"}, true},
		{"dry", "y\n", []string{"--dry-run", "--yes"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			app := testApp(root, root)
			app.Stdin = strings.NewReader(tc.input)
			var output bytes.Buffer
			app.Stdout = &output
			paths, err := app.localCIPaths()
			if err != nil {
				t.Fatal(err)
			}
			dir := paths.TaskAttemptDir("/repo", "abc", "localci:test", 1)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "log.txt")
			if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
				t.Fatal(err)
			}
			at := time.Now().Add(-40 * 24 * time.Hour)
			run := localci.RunRecord{RepoDir: "/repo", Commit: "abc", Status: localci.RunStatusSucceeded, StartedAt: at, FinishedAt: at}
			if err := (localci.RunRepository{Paths: paths}).WriteRun(run); err != nil {
				t.Fatal(err)
			}
			if err := app.Run(append([]string{"prune"}, tc.args...)); err != nil {
				t.Fatal(err)
			}
			_, err = os.Stat(path)
			if os.IsNotExist(err) != tc.removed {
				t.Fatalf("removed=%v, stat=%v; output=%s", tc.removed, err, output.String())
			}
			if !strings.Contains(output.String(), "1 files") {
				t.Fatalf("missing preview: %s", output.String())
			}
		})
	}
}

func TestPruneLimitParsing(t *testing.T) {
	for value, want := range map[string]int64{
		"1KB": 1_000, "1MB": 1_000_000, "1GB": 1_000_000_000,
		"1.5GB": 1_500_000_000, "1KiB": 1 << 10, "1MiB": 1 << 20,
		"1GiB": 1 << 30, "100B": 100, "100": 100,
	} {
		if got, err := parsePruneSize(value); err != nil || got != want {
			t.Errorf("parsePruneSize(%q) = %d, %v; want %d", value, got, err, want)
		}
	}

	if size, err := parsePruneSize("10GiB"); err != nil || size != localci.DefaultPruneSize {
		t.Fatalf("size %d %v", size, err)
	}
	if age, err := parsePruneAge("30d"); err != nil || age != localci.DefaultPruneAge {
		t.Fatalf("age %v %v", age, err)
	}
	for _, value := range []string{"-1", "NaN", "Inf", "999999999999999999999GiB", "2XB"} {
		if _, err := parsePruneSize(value); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	for _, value := range []string{"-1d", "999999999999999999d", "never"} {
		if _, err := parsePruneAge(value); err == nil {
			t.Fatalf("accepted %s", value)
		}
	}
	for _, args := range [][]string{{"prune", "--larger-than", "0"}, {"prune", "--larger-than", "1MiB", "--max-age", "2d"}} {
		app := testApp(t.TempDir(), t.TempDir())
		if err := app.Run(args); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
