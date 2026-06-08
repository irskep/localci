package docsgen

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateChangelogRemovesCommentsAndKeepsFinalNewline(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	inPath := filepath.Join(dir, "CHANGELOG.md")
	outPath := filepath.Join(dir, "changelog.md")
	source := "# Changelog\n\n<!-- comment -->\n\n## 1.2.3 - 2026-06-08\n\n- Change.\n"
	if err := os.WriteFile(inPath, []byte(source), 0o644); err != nil {
		t.Fatalf("WriteFile input returned error: %v", err)
	}

	if err := GenerateChangelog(inPath, outPath); err != nil {
		t.Fatalf("GenerateChangelog returned error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("ReadFile output returned error: %v", err)
	}
	want := "# Changelog\n\n\n## 1.2.3 - 2026-06-08\n\n- Change.\n"
	if got := string(data); got != want {
		t.Fatalf("generated changelog = %q, want %q", got, want)
	}
}
