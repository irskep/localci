package docs

import "testing"

func TestBundledDocsAreEmbedded(t *testing.T) {
	t.Parallel()

	if got := PlainText(); got == "" {
		t.Fatalf("PlainText returned empty string")
	}
	if got := ManPage(); got == "" {
		t.Fatalf("ManPage returned empty string")
	}
}
