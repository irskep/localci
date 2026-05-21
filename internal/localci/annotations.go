package localci

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

const AnnotationGitBranch = "git.branch"

func GitAnnotations(ctx context.Context, repoDir string) map[string]string {
	branch := currentGitBranch(ctx, repoDir)
	if branch == "" {
		return nil
	}
	return map[string]string{
		AnnotationGitBranch: branch,
	}
}

func GitInvokeCommitName(ctx context.Context, repoDir string) (string, error) {
	head, err := gitOutput(ctx, repoDir, "rev-parse", "HEAD")
	if err != nil {
		return "", fmt.Errorf("resolve git HEAD: %w", err)
	}
	return head + "*", nil
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
