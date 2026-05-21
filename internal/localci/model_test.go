package localci

import "testing"

func TestTrimTaskPrefix(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"localci:test":       "test",
		"//:localci:build":   "build",
		"//web:localci:lint": "//web:lint",
		"//web:deploy":       "//web:deploy",
		"not-a-localci-task": "not-a-localci-task",
	}

	for input, want := range tests {
		if got := trimTaskPrefix(input); got != want {
			t.Fatalf("trimTaskPrefix(%q) = %q, want %q", input, got, want)
		}
	}
}
