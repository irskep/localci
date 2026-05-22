package localci

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
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
	Attempt    int       `json:"attempt"`
	EnqueuedAt time.Time `json:"enqueued_at"`
}

type ActiveTask struct {
	QueueEntry
	StartedAt time.Time `json:"started_at"`
}

var queueMutationMu sync.Mutex

func (s QueueStore) Enqueue(repoDir string, commit string, taskName string) (QueueEntry, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.enqueueLocked(repoDir, commit, taskName)
}

func (s QueueStore) enqueueLocked(repoDir string, commit string, taskName string) (QueueEntry, error) {
	enqueuedAt := s.now()
	attempt, err := s.nextAttemptLocked(repoDir, commit, taskName)
	if err != nil {
		return QueueEntry{}, err
	}
	entry := QueueEntry{
		RepoDir:    repoDir,
		RepoID:     normalizeRepoDir(repoDir),
		Commit:     commit,
		TaskName:   taskName,
		TaskKey:    sanitizeTaskName(taskName),
		Attempt:    attempt,
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

func (s QueueStore) NextAttempt(repoDir string, commit string, taskName string) (int, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.nextAttemptLocked(repoDir, commit, taskName)
}

func (s QueueStore) nextAttemptLocked(repoDir string, commit string, taskName string) (int, error) {
	attempt, err := nextTaskAttempt(s.Paths, repoDir, commit, taskName)
	if err != nil {
		return 0, err
	}

	entries, err := s.listLocked()
	if err != nil {
		return 0, err
	}
	for _, entry := range entries {
		if entry.RepoDir == repoDir && entry.Commit == commit && entry.TaskName == taskName && entry.Attempt >= attempt {
			attempt = entry.Attempt + 1
		}
	}

	active, err := s.readActiveLocked()
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return 0, err
	}
	if err == nil && active.RepoDir == repoDir && active.Commit == commit && active.TaskName == taskName && active.Attempt >= attempt {
		attempt = active.Attempt + 1
	}

	return attempt, nil
}

func (s QueueStore) List() ([]QueueEntry, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.listLocked()
}

func (s QueueStore) listLocked() ([]QueueEntry, error) {
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
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.markActiveLocked(entry)
}

func (s QueueStore) markActiveLocked(entry QueueEntry) (ActiveTask, error) {
	if entry.Attempt <= 0 {
		attempt, err := s.nextAttemptLocked(entry.RepoDir, entry.Commit, entry.TaskName)
		if err != nil {
			return ActiveTask{}, err
		}
		entry.Attempt = attempt
	}
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
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.readActiveLocked()
}

func (s QueueStore) readActiveLocked() (ActiveTask, error) {
	var active ActiveTask
	if err := readJSONFile(s.Paths.ActiveTaskPath(), &active); err != nil {
		return ActiveTask{}, err
	}
	return active, nil
}

func (s QueueStore) ClearActive() error {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.clearActiveLocked()
}

func (s QueueStore) clearActiveLocked() error {
	err := os.Remove(s.Paths.ActiveTaskPath())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s QueueStore) IsTaskActive(repoDir string, commit string, taskName string) (bool, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	active, err := s.readActiveLocked()
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return false, nil
		}
		return false, err
	}

	return active.RepoDir == repoDir && active.Commit == commit && active.TaskName == taskName, nil
}

func (s QueueStore) Remove(entry QueueEntry) error {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.removeLocked(entry)
}

func (s QueueStore) removeLocked(entry QueueEntry) error {
	path := s.Paths.QueueEntryPath(entry.RepoDir, entry.Commit, entry.TaskName, entry.EnqueuedAt)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s QueueStore) ClaimNext() (QueueEntry, bool, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	active, err := s.readActiveLocked()
	if err == nil {
		return active.QueueEntry, false, nil
	}
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return QueueEntry{}, false, err
	}

	entries, err := s.listLocked()
	if err != nil {
		return QueueEntry{}, false, err
	}
	if len(entries) == 0 {
		return QueueEntry{}, false, nil
	}

	entry := entries[0]
	if _, err := s.markActiveLocked(entry); err != nil {
		return QueueEntry{}, false, err
	}
	if err := s.removeLocked(entry); err != nil {
		_ = s.clearActiveLocked()
		return QueueEntry{}, false, err
	}
	return entry, true, nil
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
