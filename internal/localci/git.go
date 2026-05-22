package localci

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const (
	AnnotationBranch   = "branch"
	AnnotationWorktree = "worktree"
)

func GitAnnotations(ctx context.Context, repoDir string) map[string]string {
	annotations := map[string]string{}
	if branch := currentGitBranch(ctx, repoDir); branch != "" {
		annotations[AnnotationBranch] = branch
	}
	if GitWorktreeDirty(ctx, repoDir) {
		annotations[AnnotationWorktree] = "dirty"
	}
	if len(annotations) == 0 {
		return nil
	}
	return annotations
}

func GitHeadCommit(ctx context.Context, repoDir string) (string, error) {
	return GitResolveCommit(ctx, repoDir, "HEAD")
}

func GitResolveCommit(ctx context.Context, repoDir string, commitish string) (string, error) {
	commitish = strings.TrimSpace(commitish)
	if commitish == "" {
		return "", fmt.Errorf("commit must not be empty")
	}
	commit, err := gitOutput(ctx, repoDir, "rev-parse", "--verify", commitish+"^{commit}")
	if err != nil {
		return "", fmt.Errorf("resolve git commit %q: %w", commitish, err)
	}
	return commit, nil
}

func GitWorktreeDirty(ctx context.Context, repoDir string) bool {
	status, err := gitOutput(ctx, repoDir, "status", "--porcelain")
	return err == nil && strings.TrimSpace(status) != ""
}

func currentGitBranch(ctx context.Context, repoDir string) string {
	branch, err := gitOutput(ctx, repoDir, "branch", "--show-current")
	if err != nil {
		return ""
	}
	return branch
}

func gitOutput(ctx context.Context, repoDir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = repoDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}
