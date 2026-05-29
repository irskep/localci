package localci

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func RouteRepoPath(repoDir string) (string, error) {
	canonical, err := CanonicalRepoPath(repoDir)
	if err != nil {
		return "", err
	}

	segments := repoRouteSegments(canonical)
	if len(segments) == 0 {
		return "", fmt.Errorf("repo dir must not be filesystem root")
	}
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" || segment == "." {
			return "", fmt.Errorf("repo dir contains an invalid path segment")
		}
		escaped = append(escaped, url.PathEscape(segment))
	}
	return strings.Join(escaped, "/"), nil
}

func CanonicalRepoPath(repoDir string) (string, error) {
	repoDir = filepath.Clean(repoDir)
	if !filepath.IsAbs(repoDir) {
		return "", fmt.Errorf("repo dir must be absolute")
	}
	return filepath.ToSlash(repoDir), nil
}

func RepoDirFromRoute(repoSegments []string) (string, error) {
	if len(repoSegments) == 0 {
		return "", fmt.Errorf("repo path is required")
	}

	parts := make([]string, 0, len(repoSegments))
	for _, segment := range repoSegments {
		if segment == "" {
			continue
		}
		parts = append(parts, segment)
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("repo path is required")
	}

	if filepath.Separator == '\\' && strings.HasSuffix(parts[0], ":") {
		if len(parts) == 1 {
			return filepath.Clean(parts[0] + string(filepath.Separator)), nil
		}
		return filepath.Clean(parts[0] + string(filepath.Separator) + filepath.Join(parts[1:]...)), nil
	}

	return filepath.Clean(string(os.PathSeparator) + filepath.Join(parts...)), nil
}

func CommitRoutePath(repoDir string, commit string) (string, error) {
	repoPath, err := RouteRepoPath(repoDir)
	if err != nil {
		return "", err
	}
	return path.Join("/repo", repoPath, "commit", url.PathEscape(commit)), nil
}

func TaskRoutePath(repoDir string, commit string, taskName string) (string, error) {
	commitPath, err := CommitRoutePath(repoDir, commit)
	if err != nil {
		return "", err
	}
	return path.Join(commitPath, "task", url.PathEscape(taskName)), nil
}

func AttemptRoutePath(repoDir string, commit string, taskName string, attempt int) (string, error) {
	taskPath, err := TaskRoutePath(repoDir, commit, taskName)
	if err != nil {
		return "", err
	}
	return path.Join(taskPath, "attempt", fmt.Sprintf("%d", attempt)), nil
}

func ArtifactRoutePath(repoDir string, commit string, taskName string, attempt int, artifact string) (string, error) {
	attemptPath, err := AttemptRoutePath(repoDir, commit, taskName, attempt)
	if err != nil {
		return "", err
	}
	artifactPath := escapedArtifactPath(artifact)
	return path.Join(append([]string{attemptPath, "artifact"}, artifactPath...)...), nil
}

func RawArtifactRoutePath(repoDir string, commit string, taskName string, attempt int, artifact string) (string, error) {
	repoPath, err := RouteRepoPath(repoDir)
	if err != nil {
		return "", err
	}
	artifactPath := escapedArtifactPath(artifact)
	prefix := append([]string{"/artifacts", "repo"}, strings.Split(repoPath, "/")...)
	prefix = append(prefix, "commit", url.PathEscape(commit), "task", url.PathEscape(taskName), "attempt", fmt.Sprintf("%d", attempt))
	return path.Join(append(prefix, artifactPath...)...), nil
}

func repoRouteSegments(repoDir string) []string {
	clean := filepath.Clean(repoDir)
	volume := filepath.VolumeName(clean)
	rest := strings.TrimPrefix(clean, volume)
	rest = strings.Trim(rest, string(filepath.Separator))

	segments := make([]string, 0)
	if volume != "" {
		segments = append(segments, volume)
	}
	for _, segment := range strings.Split(rest, string(filepath.Separator)) {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	return segments
}

func escapedArtifactPath(artifact string) []string {
	segments := strings.Split(filepath.ToSlash(artifact), "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" || segment == "." {
			return nil
		}
		escaped = append(escaped, url.PathEscape(segment))
	}
	return escaped
}
