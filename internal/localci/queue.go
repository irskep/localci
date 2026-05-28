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

type QueueEntryKind string

const (
	QueueEntryKindRun  QueueEntryKind = "run"
	QueueEntryKindTask QueueEntryKind = "task"
)

type QueueEntry struct {
	Kind           QueueEntryKind `json:"kind"`
	RepoDir        string         `json:"repo_dir"`
	RepoID         string         `json:"repo_id"`
	Commit         string         `json:"commit"`
	NoClone        bool           `json:"no_clone,omitempty"`
	TaskName       string         `json:"task_name,omitempty"`
	TaskKey        string         `json:"task_key,omitempty"`
	RequestedTasks []string       `json:"requested_tasks,omitempty"`
	Attempt        int            `json:"attempt,omitempty"`
	EnqueuedAt     time.Time      `json:"enqueued_at"`
}

type ActiveTask struct {
	QueueEntry
	StartedAt time.Time `json:"started_at"`
}

type QueueCancelResult struct {
	Active  bool
	Pending int
}

type QueueClaimState string

const (
	QueueClaimNone    QueueClaimState = "none"
	QueueClaimActive  QueueClaimState = "active"
	QueueClaimClaimed QueueClaimState = "claimed"
)

var queueMutationMu sync.Mutex

func (s QueueStore) Enqueue(repoDir string, commit string, taskName string) (QueueEntry, error) {
	return s.EnqueueWithOptions(repoDir, commit, taskName, false)
}

func (s QueueStore) EnqueueWithOptions(repoDir string, commit string, taskName string, noClone bool) (QueueEntry, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.enqueueTaskLocked(repoDir, commit, taskName, noClone)
}

func (s QueueStore) EnqueueRun(repoDir string, commit string, requestedTasks []string) (QueueEntry, error) {
	return s.EnqueueRunWithOptions(repoDir, commit, requestedTasks, false)
}

func (s QueueStore) EnqueueRunWithOptions(repoDir string, commit string, requestedTasks []string, noClone bool) (QueueEntry, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	return s.enqueueRunLocked(repoDir, commit, requestedTasks, noClone)
}

func (s QueueStore) enqueueRunLocked(repoDir string, commit string, requestedTasks []string, noClone bool) (QueueEntry, error) {
	enqueuedAt := s.now()
	entry := QueueEntry{
		Kind:           QueueEntryKindRun,
		RepoDir:        repoDir,
		RepoID:         normalizeRepoDir(repoDir),
		Commit:         commit,
		NoClone:        noClone,
		TaskKey:        "run",
		RequestedTasks: append([]string{}, requestedTasks...),
		EnqueuedAt:     enqueuedAt,
	}

	if err := os.MkdirAll(s.Paths.QueueRoot(), 0o755); err != nil {
		return QueueEntry{}, err
	}
	if err := writeJSONFile(s.Paths.QueueEntryPath(entry, enqueuedAt), entry); err != nil {
		return QueueEntry{}, err
	}

	return entry, nil
}

func (s QueueStore) enqueueTaskLocked(repoDir string, commit string, taskName string, noClone bool) (QueueEntry, error) {
	enqueuedAt := s.now()
	attempt, err := s.nextAttemptLocked(repoDir, commit, taskName)
	if err != nil {
		return QueueEntry{}, err
	}
	entry := QueueEntry{
		Kind:       QueueEntryKindTask,
		RepoDir:    repoDir,
		RepoID:     normalizeRepoDir(repoDir),
		Commit:     commit,
		NoClone:    noClone,
		TaskName:   taskName,
		TaskKey:    sanitizeTaskName(taskName),
		Attempt:    attempt,
		EnqueuedAt: enqueuedAt,
	}

	if err := os.MkdirAll(s.Paths.QueueRoot(), 0o755); err != nil {
		return QueueEntry{}, err
	}
	if err := writeJSONFile(s.Paths.QueueEntryPath(entry, enqueuedAt), entry); err != nil {
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
		if entry.Kind != QueueEntryKindTask {
			continue
		}
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
	if entry.Kind == QueueEntryKindTask && entry.Attempt <= 0 {
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

func (s QueueStore) Cancel(repoDir string, commit string, taskName string) (QueueCancelResult, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	result := QueueCancelResult{}

	entries, err := s.listLocked()
	if err != nil {
		return QueueCancelResult{}, err
	}
	for _, entry := range entries {
		if !queueEntryTargetsTask(entry, repoDir, commit, taskName) {
			continue
		}
		if err := s.removeLocked(entry); err != nil {
			return QueueCancelResult{}, err
		}
		result.Pending++
	}

	active, err := s.readActiveLocked()
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return result, nil
		}
		return QueueCancelResult{}, err
	}
	if active.RepoDir != repoDir || active.Commit != commit || active.TaskName != taskName {
		return result, nil
	}

	outputDir := s.Paths.TaskAttemptDir(repoDir, commit, taskName, active.Attempt)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return QueueCancelResult{}, err
	}
	if err := os.WriteFile(filepath.Join(outputDir, cancelMarkerName), []byte("canceled\n"), 0o644); err != nil {
		return QueueCancelResult{}, err
	}
	result.Active = true

	return result, nil
}

func queueEntryTargetsTask(entry QueueEntry, repoDir string, commit string, taskName string) bool {
	if entry.RepoDir != repoDir || entry.Commit != commit {
		return false
	}
	switch entry.Kind {
	case QueueEntryKindTask:
		return entry.TaskName == taskName
	case QueueEntryKindRun:
		return len(entry.RequestedTasks) == 1 && entry.RequestedTasks[0] == taskName
	default:
		return false
	}
}

func (s QueueStore) removeLocked(entry QueueEntry) error {
	path := s.Paths.QueueEntryPath(entry, entry.EnqueuedAt)
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s QueueStore) LiveRefs() (map[string]bool, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	refs := map[string]bool{}
	entries, err := s.listLocked()
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		refs[queueRefKey(entry.RepoDir, entry.Commit)] = true
	}

	active, err := s.readActiveLocked()
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return refs, nil
		}
		return nil, err
	}
	refs[queueRefKey(active.RepoDir, active.Commit)] = true
	return refs, nil
}

func (s QueueStore) ClaimNext() (QueueEntry, QueueClaimState, error) {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()

	active, err := s.readActiveLocked()
	if err == nil {
		return active.QueueEntry, QueueClaimActive, nil
	}
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return QueueEntry{}, QueueClaimNone, err
	}

	entries, err := s.listLocked()
	if err != nil {
		return QueueEntry{}, QueueClaimNone, err
	}
	if len(entries) == 0 {
		return QueueEntry{}, QueueClaimNone, nil
	}

	entry := entries[0]
	if _, err := s.markActiveLocked(entry); err != nil {
		return QueueEntry{}, QueueClaimNone, err
	}
	if err := s.removeLocked(entry); err != nil {
		_ = s.clearActiveLocked()
		return QueueEntry{}, QueueClaimNone, err
	}
	return entry, QueueClaimClaimed, nil
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

func queueRefKey(repoDir string, commit string) string {
	return normalizeRepoDir(repoDir) + ":" + commit
}
