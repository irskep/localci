package localci

import (
	"errors"
	"os/exec"
	"testing"
)

func TestParseGitVersion(t *testing.T) {
	t.Parallel()

	got, err := parseGitVersion("git version 2.51.0\n")
	if err != nil {
		t.Fatalf("parseGitVersion returned error: %v", err)
	}
	want := version{Major: 2, Minor: 51, Patch: 0}
	if got != want {
		t.Fatalf("parseGitVersion = %#v, want %#v", got, want)
	}
}

func TestRequirementsCheckerCheck(t *testing.T) {
	t.Parallel()

	checker := RequirementsChecker{
		LookPath: func(string) (string, error) {
			return "/bin/tool", nil
		},
		RunVersion: func(name string, args ...string) (string, error) {
			if name == "git" {
				return "git version 2.51.0\n", nil
			}
			return "2025.11.3\n", nil
		},
	}

	if err := checker.Check(); err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
}

func TestRequirementsCheckerRejectsOldGit(t *testing.T) {
	t.Parallel()

	checker := RequirementsChecker{
		LookPath: func(string) (string, error) {
			return "/bin/tool", nil
		},
		RunVersion: func(name string, args ...string) (string, error) {
			if name == "git" {
				return "git version 2.39.0\n", nil
			}
			return "2025.11.3\n", nil
		},
	}

	err := checker.Check()
	if err == nil {
		t.Fatalf("Check returned nil error, want old git failure")
	}
}

func TestRequirementsCheckerFailsWhenMiseMissing(t *testing.T) {
	t.Parallel()

	checker := RequirementsChecker{
		LookPath: func(name string) (string, error) {
			if name == "mise" {
				return "", exec.ErrNotFound
			}
			return "/bin/tool", nil
		},
		RunVersion: func(name string, args ...string) (string, error) {
			return "git version 2.51.0\n", nil
		},
	}

	err := checker.Check()
	if err == nil {
		t.Fatalf("Check returned nil error, want missing mise failure")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("error = %v, want exec.ErrNotFound", err)
	}
}
