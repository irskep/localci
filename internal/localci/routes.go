package localci

import (
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

func RouteRepoPath(root string, repoDir string) (string, error) {
	rel, err := CanonicalRepoPath(root, repoDir)
	if err != nil {
		return "", err
	}
	if rel == "" {
		return "", nil
	}

	segments := strings.Split(rel, "/")
	escaped := make([]string, 0, len(segments))
	for _, segment := range segments {
		if segment == "" || segment == "." {
			continue
		}
		escaped = append(escaped, url.PathEscape(segment))
	}
	return strings.Join(escaped, "/"), nil
}

func CanonicalRepoPath(root string, repoDir string) (string, error) {
	root = filepath.Clean(root)
	repoDir = filepath.Clean(repoDir)

	rel, err := filepath.Rel(root, repoDir)
	if err != nil {
		return "", err
	}
	if err := validateRelativeRepoPath(rel); err != nil {
		return "", err
	}
	if rel == "." {
		return "", nil
	}
	return filepath.ToSlash(rel), nil
}

func CommitRoutePath(root string, repoDir string, commit string) (string, error) {
	repoPath, err := RouteRepoPath(root, repoDir)
	if err != nil {
		return "", err
	}
	return path.Join("/repo", repoPath, "commit", url.PathEscape(commit)), nil
}

func TaskRoutePath(root string, repoDir string, commit string, taskName string) (string, error) {
	commitPath, err := CommitRoutePath(root, repoDir, commit)
	if err != nil {
		return "", err
	}
	return path.Join(commitPath, "task", url.PathEscape(taskName)), nil
}

func AttemptRoutePath(root string, repoDir string, commit string, taskName string, attempt int) (string, error) {
	taskPath, err := TaskRoutePath(root, repoDir, commit, taskName)
	if err != nil {
		return "", err
	}
	return path.Join(taskPath, "attempt", fmt.Sprintf("%d", attempt)), nil
}
