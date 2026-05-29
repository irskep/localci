package localci

import (
	"path/filepath"
	"testing"
)

func TestRepoLabelMapUsesCommonPrefix(t *testing.T) {
	localci := filepath.Join(string(filepath.Separator), "Users", "steve", "dev", "cli", "localci")
	smalldraw := filepath.Join(string(filepath.Separator), "Users", "steve", "dev", "apps", "smalldraw")

	labels := RepoLabelMap([]string{localci, smalldraw})

	if labels[localci] != "cli/localci" {
		t.Fatalf("localci label = %q", labels[localci])
	}
	if labels[smalldraw] != "apps/smalldraw" {
		t.Fatalf("smalldraw label = %q", labels[smalldraw])
	}
}

func TestRepoLabelMapSingleRepoUsesBasename(t *testing.T) {
	repo := filepath.Join(string(filepath.Separator), "Users", "steve", "dev", "cli", "localci")

	labels := RepoLabelMap([]string{repo})

	if labels[repo] != "localci" {
		t.Fatalf("label = %q", labels[repo])
	}
}
