package localci

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type CloneManager struct {
	Paths Paths
	Now   func() time.Time
}

type CloneInfo struct {
	RepoDir   string    `json:"repo_dir"`
	RepoID    string    `json:"repo_id"`
	Commit    string    `json:"commit"`
	CloneDir  string    `json:"clone_dir"`
	Worktree  string    `json:"worktree"`
	CreatedAt time.Time `json:"created_at"`
}

func (m CloneManager) Prepare(ctx context.Context, repoDir string, commit string) (CloneInfo, error) {
	info := CloneInfo{
		RepoDir:   repoDir,
		RepoID:    normalizeRepoDir(repoDir),
		Commit:    commit,
		CloneDir:  m.Paths.CloneDir(repoDir, commit),
		Worktree:  m.Paths.CloneWorktreeDir(repoDir, commit),
		CreatedAt: m.now(),
	}

	if _, err := os.Stat(filepath.Join(info.Worktree, ".git")); err == nil {
		if err := writeJSONFile(m.Paths.CloneInfoPath(repoDir, commit), info); err != nil {
			return CloneInfo{}, err
		}
		return info, nil
	}

	if err := os.RemoveAll(info.CloneDir); err != nil {
		return CloneInfo{}, err
	}
	if err := os.MkdirAll(filepath.Dir(info.Worktree), 0o755); err != nil {
		return CloneInfo{}, err
	}
	if err := runCloneGit(ctx, "", "clone", "--no-checkout", repoDir, info.Worktree); err != nil {
		_ = os.RemoveAll(info.CloneDir)
		return CloneInfo{}, fmt.Errorf("clone repo: %w", err)
	}
	if err := runCloneGit(ctx, info.Worktree, "checkout", "--detach", commit); err != nil {
		_ = os.RemoveAll(info.CloneDir)
		return CloneInfo{}, fmt.Errorf("checkout commit: %w", err)
	}
	if err := writeJSONFile(m.Paths.CloneInfoPath(repoDir, commit), info); err != nil {
		_ = os.RemoveAll(info.CloneDir)
		return CloneInfo{}, err
	}

	return info, nil
}

func (m CloneManager) Cleanup(repoDir string, commit string) error {
	if err := os.RemoveAll(m.Paths.CloneDir(repoDir, commit)); err != nil {
		return err
	}
	err := os.Remove(m.Paths.CloneInfoPath(repoDir, commit))
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (m CloneManager) CleanupUnreferenced(queue QueueStore) error {
	live, err := queue.LiveRefs()
	if err != nil {
		return err
	}

	infos, err := filepath.Glob(filepath.Join(m.Paths.cacheRoot(), "*", "clones", "*.info.json"))
	if err != nil {
		return err
	}
	for _, path := range infos {
		var info CloneInfo
		if err := readJSONFile(path, &info); err != nil {
			return err
		}
		if live[queueRefKey(info.RepoDir, info.Commit)] {
			continue
		}
		if err := m.Cleanup(info.RepoDir, info.Commit); err != nil {
			return err
		}
	}
	return nil
}

func (m CloneManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now().UTC()
}

func runCloneGit(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}
