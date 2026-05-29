package localci

import (
	"os"
	"path/filepath"
)

func (p Paths) DaemonRoot() string {
	return filepath.Join(p.configRoot(), "daemon")
}

func (p Paths) DaemonStatePath() string {
	return filepath.Join(p.DaemonRoot(), "state.json")
}

func (p Paths) DaemonSocketPath() string {
	if p.DaemonSocketOverride != "" {
		return p.DaemonSocketOverride
	}

	return filepath.Join(os.TempDir(), "localci-"+normalizeRepoDir(p.configRoot())+".sock")
}

func (p Paths) DaemonLogPath() string {
	return filepath.Join(p.DaemonRoot(), "daemon.log")
}
