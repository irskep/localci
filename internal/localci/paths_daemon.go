package localci

import "path/filepath"

func (p Paths) DaemonRoot() string {
	return filepath.Join(p.Root, "daemon")
}

func (p Paths) DaemonStatePath() string {
	return filepath.Join(p.DaemonRoot(), "state.json")
}

func (p Paths) DaemonLogPath() string {
	return filepath.Join(p.DaemonRoot(), "daemon.log")
}
