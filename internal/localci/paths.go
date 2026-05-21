package localci

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
)

// Paths centralizes the on-disk layout so later implementation work can
// tighten the details without spreading path logic across the codebase.
type Paths struct {
	Root                 string
	DaemonSocketOverride string
}

func (p Paths) GlobalCacheRoot() string {
	return filepath.Join(p.Root, "cache")
}

func (p Paths) RepoRoot(repoDir string) string {
	return filepath.Join(p.Root, normalizeRepoDir(repoDir))
}

func (p Paths) RepoCacheRoot(repoDir string) string {
	return filepath.Join(p.RepoRoot(repoDir), "cache")
}

func (p Paths) CommitRoot(repoDir string, commit string) string {
	return filepath.Join(p.RepoRoot(repoDir), commit)
}

func (p Paths) RunRecordPath(repoDir string, commit string) string {
	return filepath.Join(p.CommitRoot(repoDir, commit), "run.json")
}

func (p Paths) CommitCacheDir(repoDir string, commit string) string {
	return filepath.Join(p.CommitRoot(repoDir, commit), "cache")
}

func (p Paths) SharedCacheDir() string {
	return p.GlobalCacheRoot()
}

func (p Paths) TaskCacheDir(repoDir string, task string) string {
	return filepath.Join(p.RepoCacheRoot(repoDir), sanitizeTaskName(task))
}

func (p Paths) TaskOutputDir(repoDir string, commit string, task string) string {
	return filepath.Join(p.CommitRoot(repoDir, commit), "out", sanitizeTaskName(task))
}

func (p Paths) TaskAttemptDir(repoDir string, commit string, task string, attempt int) string {
	return filepath.Join(p.TaskOutputDir(repoDir, commit, task), formatTaskAttempt(attempt))
}

func (p Paths) TaskRecordPath(repoDir string, commit string, task string, attempt int) string {
	return filepath.Join(p.TaskAttemptDir(repoDir, commit, task, attempt), "task.json")
}

func normalizeRepoDir(repoDir string) string {
	sum := sha256.Sum256([]byte(filepath.Clean(repoDir)))
	return hex.EncodeToString(sum[:16])
}

func sanitizeTaskName(task string) string {
	replacer := strings.NewReplacer("/", "_", ":", "_", " ", "_")
	return replacer.Replace(task)
}

func formatTaskAttempt(attempt int) string {
	return fmt.Sprintf("attempt-%03d", attempt)
}
