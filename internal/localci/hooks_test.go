package localci

import (
	"os/exec"
	"strings"
	"testing"
)

func TestHookInstallerInstall(t *testing.T) {
	t.Parallel()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")

	installer := HookInstaller{RepoDir: repoDir}
	if err := installer.Install(); err != nil {
		t.Fatalf("Install returned error: %v", err)
	}

	event := strings.TrimSpace(runGit(t, repoDir, "config", "--local", "--get", "hook."+localCIHookName+".event"))
	if event != "post-commit" {
		t.Fatalf("event = %q, want %q", event, "post-commit")
	}

	command := strings.TrimSpace(runGit(t, repoDir, "config", "--local", "--get", "hook."+localCIHookName+".command"))
	if !strings.Contains(command, `mise run run -- postcommit "$repo" "$commit"`) {
		t.Fatalf("unexpected hook command: %q", command)
	}
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return string(output)
}
