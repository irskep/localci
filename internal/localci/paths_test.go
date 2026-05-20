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
}
