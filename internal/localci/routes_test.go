package localci

import (
	"net/url"
	"path/filepath"
	"testing"
)

func TestRouteRepoPathRoundTripsAbsolutePath(t *testing.T) {
	repoDir := filepath.Join(string(filepath.Separator), "Users", "steve", "dev", "repo with spaces")

	routePath, err := RouteRepoPath(repoDir)
	if err != nil {
		t.Fatalf("RouteRepoPath returned error: %v", err)
	}
	if routePath != "Users/steve/dev/repo%20with%20spaces" {
		t.Fatalf("RouteRepoPath = %q", routePath)
	}

	segments := []string{}
	for _, segment := range []string{"Users", "steve", "dev", "repo%20with%20spaces"} {
		decoded, err := url.PathUnescape(segment)
		if err != nil {
			t.Fatalf("PathUnescape returned error: %v", err)
		}
		segments = append(segments, decoded)
	}
	got, err := RepoDirFromRoute(segments)
	if err != nil {
		t.Fatalf("RepoDirFromRoute returned error: %v", err)
	}
	if got != repoDir {
		t.Fatalf("RepoDirFromRoute = %q, want %q", got, repoDir)
	}
}

func TestRouteRepoPathRejectsRelativePath(t *testing.T) {
	if _, err := RouteRepoPath("repo"); err == nil {
		t.Fatalf("RouteRepoPath returned nil error, want relative path error")
	}
}
