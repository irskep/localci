package cli

import "testing"

func TestShortCommitPreservesNoCloneMarker(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		commit string
		want   string
	}{
		{
			name:   "full commit",
			commit: "3a974feb300e734293a95d7bea0809c11293f2c9",
			want:   "3a974feb300e",
		},
		{
			name:   "full no-clone commit",
			commit: "3a974feb300e734293a95d7bea0809c11293f2c9*",
			want:   "3a974feb300e*",
		},
		{
			name:   "short no-clone commit",
			commit: "abc123*",
			want:   "abc123*",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := shortCommit(test.commit); got != test.want {
				t.Fatalf("shortCommit(%q) = %q, want %q", test.commit, got, test.want)
			}
		})
	}
}
