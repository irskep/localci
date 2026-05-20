package localci

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type QueueStore struct {
	Paths Paths
	Now   func() time.Time
}

type QueueEntry struct {
	RepoDir    string    `json:"repo_dir"`
	RepoID     string    `json:"repo_id"`
	Commit     string    `json:"commit"`
	TaskName   string    `json:"task_name"`
	TaskKey    string    `json:"task_key"`
	EnqueuedAt time.Time `json:"enqueued_at"`
}

type ActiveTask struct {
	QueueEntry
	StartedAt time.Time `json:"started_at"`
}

func (s QueueStore) Enqueue(repoDir string, commit string, taskName string) (QueueEntry, error) {
	enqueuedAt := s.now()
	entry := QueueEntry{
		RepoDir:    repoDir,
		RepoID:     normalizeRepoDir(repoDir),
		Commit:     commit,
		TaskName:   taskName,
		TaskKey:    sanitizeTaskName(taskName),
		EnqueuedAt: enqueuedAt,
	}

	if err := os.MkdirAll(s.Paths.QueueRoot(), 0o755); err != nil {
		return QueueEntry{}, err
	}
	if err := writeJSONFile(s.Paths.QueueEntryPath(repoDir, commit, taskName, enqueuedAt), entry); err != nil {
		return QueueEntry{}, err
	}

	return entry, nil
}

func (s QueueStore) List() ([]QueueEntry, error) {
	entries := []QueueEntry{}

	dirEntries, err := os.ReadDir(s.Paths.QueueRoot())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return entries, nil
		}
		return nil, err
	}

	for _, dirEntry := range dirEntries {
		if dirEntry.IsDir() || filepath.Ext(dirEntry.Name()) != ".json" {
			continue
		}

		entry, err := s.readQueueEntry(filepath.Join(s.Paths.QueueRoot(), dirEntry.Name()))
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	sort.Slice(entries, func(i int, j int) bool {
		if entries[i].EnqueuedAt.Equal(entries[j].EnqueuedAt) {
			if entries[i].RepoID == entries[j].RepoID {
				return entries[i].TaskKey < entries[j].TaskKey
			}
			return entries[i].RepoID < entries[j].RepoID
		}
		return entries[i].EnqueuedAt.Before(entries[j].EnqueuedAt)
	})

	return entries, nil
}

func (s QueueStore) MarkActive(entry QueueEntry) (ActiveTask, error) {
	active := ActiveTask{
		QueueEntry: entry,
		StartedAt:  s.now(),
	}

	if err := os.MkdirAll(s.Paths.ActiveRoot(), 0o755); err != nil {
		return ActiveTask{}, err
	}
	if err := writeJSONFile(s.Paths.ActiveTaskPath(), active); err != nil {
		return ActiveTask{}, err
	}

	return active, nil
}

func (s QueueStore) ReadActive() (ActiveTask, error) {
	var active ActiveTask
	if err := readJSONFile(s.Paths.ActiveTaskPath(), &active); err != nil {
		return ActiveTask{}, err
	}
	return active, nil
}

func (s QueueStore) ClearActive() error {
	err := os.Remove(s.Paths.ActiveTaskPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s QueueStore) IsTaskActive(repoDir string, commit string, taskName string) (bool, error) {
	active, err := s.ReadActive()
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	return active.RepoDir == repoDir && active.Commit == commit && active.TaskName == taskName, nil
}

func (s QueueStore) Remove(entry QueueEntry) error {
	path := s.Paths.QueueEntryPath(entry.RepoDir, entry.Commit, entry.TaskName, entry.EnqueuedAt)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s QueueStore) readQueueEntry(path string) (QueueEntry, error) {
	var entry QueueEntry
	if err := readJSONFile(path, &entry); err != nil {
		return QueueEntry{}, err
	}
	return entry, nil
}

func (s QueueStore) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now().UTC()
}
