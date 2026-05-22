package localci

import (
	"fmt"
	"path/filepath"
	"time"
)

func (p Paths) QueueRoot() string {
	return filepath.Join(p.Root, "queue")
}

func (p Paths) ActiveRoot() string {
	return filepath.Join(p.Root, "active")
}

func (p Paths) QueueEntryPath(entry QueueEntry, enqueuedAt time.Time) string {
	filename := fmt.Sprintf("%020d-%s-%s-%s.json", enqueuedAt.UTC().UnixNano(), normalizeRepoDir(entry.RepoDir), entry.Kind, sanitizeTaskName(entry.TaskKey))
	return filepath.Join(p.QueueRoot(), filename)
}

func (p Paths) ActiveTaskPath() string {
	return filepath.Join(p.ActiveRoot(), "task.json")
}
