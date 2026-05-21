package localci

import (
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var minimumGitVersion = version{Major: 2, Minor: 54, Patch: 0}

type RequirementsChecker struct {
	LookPath      func(string) (string, error)
	RunVersion    func(name string, args ...string) (string, error)
	MinGitVersion version
}

func (c RequirementsChecker) Check() error {
	if err := c.checkGit(); err != nil {
		return err
	}
	if err := c.checkMise(); err != nil {
		return err
	}
	return nil
}

func (c RequirementsChecker) checkGit() error {
	if _, err := c.lookPath()("git"); err != nil {
		return fmt.Errorf("git is required: %w", err)
	}

	output, err := c.runVersion()("git", "--version")
	if err != nil {
		return fmt.Errorf("git is required: %w", err)
	}

	current, err := parseGitVersion(output)
	if err != nil {
		return err
	}
	if current.lessThan(c.minGitVersion()) {
		return fmt.Errorf("git %s or newer is required, found %s", c.minGitVersion(), current)
	}
	return nil
}

func (c RequirementsChecker) checkMise() error {
	if _, err := c.lookPath()("mise"); err != nil {
		return fmt.Errorf("mise is required: %w", err)
	}

	if _, err := c.runVersion()("mise", "--version"); err != nil {
		return fmt.Errorf("mise is required: %w", err)
	}

	return nil
}

func (c RequirementsChecker) lookPath() func(string) (string, error) {
	if c.LookPath != nil {
		return c.LookPath
	}
	return exec.LookPath
}

func (c RequirementsChecker) runVersion() func(string, ...string) (string, error) {
	if c.RunVersion != nil {
		return c.RunVersion
	}
	return func(name string, args ...string) (string, error) {
		output, err := exec.Command(name, args...).CombinedOutput()
		return string(output), err
	}
}

func (c RequirementsChecker) minGitVersion() version {
	if c.MinGitVersion != (version{}) {
		return c.MinGitVersion
	}
	return minimumGitVersion
}

type version struct {
	Major int
	Minor int
	Patch int
}

func (v version) lessThan(other version) bool {
	if v.Major != other.Major {
		return v.Major < other.Major
	}
	if v.Minor != other.Minor {
		return v.Minor < other.Minor
	}
	return v.Patch < other.Patch
}

func (v version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

var gitVersionPattern = regexp.MustCompile(`git version (\d+)\.(\d+)(?:\.(\d+))?`)

func parseGitVersion(output string) (version, error) {
	matches := gitVersionPattern.FindStringSubmatch(strings.TrimSpace(output))
	if len(matches) == 0 {
		return version{}, fmt.Errorf("parse git version from %q", strings.TrimSpace(output))
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return version{}, err
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return version{}, err
	}

	patch := 0
	if matches[3] != "" {
		patch, err = strconv.Atoi(matches[3])
		if err != nil {
			return version{}, err
		}
	}

	return version{Major: major, Minor: minor, Patch: patch}, nil
}
