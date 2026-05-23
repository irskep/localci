package localci

import (
	"fmt"
	"os/exec"
	"strings"
)

const localCIHookName = "localci-postcommit"

type HookInstaller struct {
	RepoDir string
}

func (i HookInstaller) Install() error {
	if strings.TrimSpace(i.RepoDir) == "" {
		return fmt.Errorf("repo dir must not be empty")
	}

	command := `sh -c 'repo=$(git rev-parse --show-toplevel) && commit=$(git rev-parse HEAD) && exec localci postcommit --repo "$repo" "$commit"'`
	for _, args := range [][]string{
		{"config", "--local", "--replace-all", "hook." + localCIHookName + ".event", "post-commit"},
		{"config", "--local", "--replace-all", "hook." + localCIHookName + ".command", command},
		{"config", "--local", "--replace-all", "hook." + localCIHookName + ".enabled", "true"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = i.RepoDir
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
		}
	}

	return nil
}
