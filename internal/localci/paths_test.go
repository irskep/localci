package localci

import "testing"

func TestPaths(t *testing.T) {
	t.Parallel()

	paths := Paths{Root: "/tmp/.localci"}

	repoRoot := paths.RepoRoot("/Users/steve/src/project")
	if repoRoot == "/tmp/.localci" || repoRoot == "" {
		t.Fatalf("RepoRoot returned an invalid path: %q", repoRoot)
	}

	commitCacheDir := paths.CommitCacheDir("/Users/steve/src/project", "abc123")
	if want := repoRoot + "/abc123/cache"; commitCacheDir != want {
		t.Fatalf("CommitCacheDir = %q, want %q", commitCacheDir, want)
	}

	taskCacheDir := paths.TaskCacheDir("/Users/steve/src/project", "localci:test/unit")
	if want := repoRoot + "/cache/localci_test_unit"; taskCacheDir != want {
		t.Fatalf("TaskCacheDir = %q, want %q", taskCacheDir, want)
	}

	taskDir := paths.TaskOutputDir("/Users/steve/src/project", "abc123", "localci:test/unit")
	if want := repoRoot + "/abc123/out/localci_test_unit"; taskDir != want {
		t.Fatalf("TaskOutputDir = %q, want %q", taskDir, want)
	}

	attemptDir := paths.TaskAttemptDir("/Users/steve/src/project", "abc123", "localci:test/unit", 2)
	if want := repoRoot + "/abc123/out/localci_test_unit/attempt-002"; attemptDir != want {
		t.Fatalf("TaskAttemptDir = %q, want %q", attemptDir, want)
	}
}

func TestPathsSplitDurableAndCacheRoots(t *testing.T) {
	t.Parallel()

	paths := Paths{
		ConfigRoot: "/tmp/localci-config",
		CacheRoot:  "/tmp/localci-cache",
	}

	if got, want := paths.HistoryDBPath(), "/tmp/localci-config/history.db"; got != want {
		t.Fatalf("HistoryDBPath = %q, want %q", got, want)
	}
	if got, want := paths.DaemonStatePath(), "/tmp/localci-config/daemon/state.json"; got != want {
		t.Fatalf("DaemonStatePath = %q, want %q", got, want)
	}
	if got, want := paths.QueueRoot(), "/tmp/localci-config/queue"; got != want {
		t.Fatalf("QueueRoot = %q, want %q", got, want)
	}

	repoRoot := paths.RepoRoot("/Users/steve/src/project")
	if got, want := paths.SharedCacheDir(), "/tmp/localci-cache/cache"; got != want {
		t.Fatalf("SharedCacheDir = %q, want %q", got, want)
	}
	if repoRoot == "/tmp/localci-cache" || repoRoot == "" {
		t.Fatalf("RepoRoot returned an invalid path: %q", repoRoot)
	}
	if got, want := paths.CloneRoot("/Users/steve/src/project"), repoRoot+"/clones"; got != want {
		t.Fatalf("CloneRoot = %q, want %q", got, want)
	}
	if got, want := paths.TaskOutputDir("/Users/steve/src/project", "abc123", "localci:test"), repoRoot+"/abc123/out/localci_test"; got != want {
		t.Fatalf("TaskOutputDir = %q, want %q", got, want)
	}
}
