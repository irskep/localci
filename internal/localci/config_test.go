package localci

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	root := filepath.Join(dir, "repos")
	if err := os.WriteFile(path, []byte("root = "+quoteTOML(root)+"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig returned error: %v", err)
	}
	if cfg.Root != root {
		t.Fatalf("Root = %q, want %q", cfg.Root, root)
	}
}

func TestLoadConfigRejectsRelativeRoot(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte("root = \"repos\"\n"), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	if _, err := LoadConfig(path); err == nil {
		t.Fatalf("LoadConfig returned nil error, want root validation error")
	}
}

func TestResolveRepoDirAcceptsRepoUnderRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := filepath.Join(root, "team", "repo")

	got, err := ResolveRepoDir(root, repo)
	if err != nil {
		t.Fatalf("ResolveRepoDir returned error: %v", err)
	}
	if got != repo {
		t.Fatalf("ResolveRepoDir = %q, want %q", got, repo)
	}
}

func TestResolveRepoDirRejectsRepoOutsideRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	repo := filepath.Join(filepath.Dir(root), "outside")

	if _, err := ResolveRepoDir(root, repo); err == nil {
		t.Fatalf("ResolveRepoDir returned nil error, want root escape error")
	}
}

func quoteTOML(value string) string {
	return `"` + value + `"`
}
