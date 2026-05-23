package localci

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

type Scheduler struct {
	Queue  QueueStore
	Runner Runner
	Clones CloneManager
	Events *EventNotifier
}

type RunNextResult struct {
	DidWork bool       `json:"did_work"`
	Entry   QueueEntry `json:"entry"`
	Task    TaskRecord `json:"task"`
	Run     RunRecord  `json:"run"`
}

func (s Scheduler) RunNext(ctx context.Context) (RunNextResult, error) {
	entry, claimState, err := s.Queue.ClaimNext()
	if err != nil {
		return RunNextResult{}, err
	}
	if claimState == QueueClaimActive {
		return RunNextResult{
			DidWork: false,
			Entry:   entry,
		}, nil
	}
	if claimState == QueueClaimNone {
		return RunNextResult{}, nil
	}

	defer func() {
		_ = s.Queue.ClearActive()
		_ = s.cleanupCloneIfDrained(entry)
		if s.Events != nil {
			s.Events.EntryChanged(entry)
		}
	}()
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}

	if s.Events != nil {
		s.Events.QueueChanged()
	}

	if entry.Kind == QueueEntryKindRun {
		return s.expandRun(ctx, entry)
	}

	taskRecord, runErr := s.Runner.runTask(ctx, InvokeRequest{
		RepoDir: entry.RepoDir,
		Commit:  entry.Commit,
	}, s.Runner.Paths.CloneWorktreeDir(entry.RepoDir, entry.Commit), Task{Name: entry.TaskName}, entry.Attempt)

	runRecord, writeErr := upsertTaskRecord(s.Runner.Paths, InvokeRequest{
		RepoDir: entry.RepoDir,
		Commit:  entry.Commit,
	}, taskRecord, s.Runner.now())
	if writeErr != nil {
		return RunNextResult{}, writeErr
	}
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}

	result := RunNextResult{
		DidWork: true,
		Entry:   entry,
		Task:    taskRecord,
		Run:     runRecord,
	}

	if runErr != nil {
		return result, runErr
	}

	return result, nil
}

func (s Scheduler) expandRun(ctx context.Context, entry QueueEntry) (RunNextResult, error) {
	clones := s.cloneManager()
	info, err := clones.Prepare(ctx, entry.RepoDir, entry.Commit)
	if err != nil {
		task, run, recordErr := recordSetupFailure(s.Runner.Paths, InvokeRequest{RepoDir: entry.RepoDir, Commit: entry.Commit}, err, s.Runner.now())
		if recordErr != nil {
			return RunNextResult{}, recordErr
		}
		_ = clones.Cleanup(entry.RepoDir, entry.Commit)
		return RunNextResult{DidWork: true, Entry: entry, Task: task, Run: run}, errTaskFailed
	}

	req := InvokeRequest{
		RepoDir: entry.RepoDir,
		Commit:  entry.Commit,
	}
	if err := s.Runner.Trust(ctx, info.Worktree); err != nil {
		task, run, recordErr := recordSetupFailure(s.Runner.Paths, req, err, s.Runner.now())
		if recordErr != nil {
			return RunNextResult{}, recordErr
		}
		_ = clones.Cleanup(entry.RepoDir, entry.Commit)
		return RunNextResult{DidWork: true, Entry: entry, Task: task, Run: run}, errTaskFailed
	}

	tasks, err := s.Runner.DiscoverTasks(ctx, info.Worktree)
	if err != nil {
		task, run, recordErr := recordSetupFailure(s.Runner.Paths, req, err, s.Runner.now())
		if recordErr != nil {
			return RunNextResult{}, recordErr
		}
		_ = clones.Cleanup(entry.RepoDir, entry.Commit)
		return RunNextResult{DidWork: true, Entry: entry, Task: task, Run: run}, errTaskFailed
	}

	setup, userTasks, hasSetup := splitSetupTask(tasks)
	run, err := ensureRunRecordWithTasks(s.Runner.Paths, InvokeRequest{
		RepoDir:     entry.RepoDir,
		Commit:      entry.Commit,
		Annotations: nil,
	}, tasks, s.Runner.now())
	if err != nil {
		return RunNextResult{}, err
	}

	var setupRecord TaskRecord
	if hasSetup {
		var setupErr error
		setupRecord, setupErr = s.Runner.runTask(ctx, req, info.Worktree, setup, 0)
		run, err = upsertTaskRecord(s.Runner.Paths, InvokeRequest{
			RepoDir: entry.RepoDir,
			Commit:  entry.Commit,
		}, setupRecord, s.Runner.now())
		if err != nil {
			return RunNextResult{}, err
		}
		if setupErr != nil {
			_ = clones.Cleanup(entry.RepoDir, entry.Commit)
			return RunNextResult{DidWork: true, Entry: entry, Task: setupRecord, Run: run}, setupErr
		}
	}

	requested := map[string]bool{}
	for _, task := range entry.RequestedTasks {
		requested[task] = true
	}
	for _, task := range userTasks {
		if len(requested) > 0 && !requested[task.Name] {
			continue
		}
		active, err := s.Queue.IsTaskActive(entry.RepoDir, entry.Commit, task.Name)
		if err != nil {
			return RunNextResult{}, err
		}
		if active {
			continue
		}
		queued, err := s.Queue.Enqueue(entry.RepoDir, entry.Commit, task.Name)
		if err != nil {
			return RunNextResult{}, err
		}
		if s.Events != nil {
			s.Events.EntryChanged(queued)
		}
	}
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}
	return RunNextResult{DidWork: true, Entry: entry, Task: setupRecord, Run: run}, nil
}

func recordSetupFailure(paths Paths, req InvokeRequest, cause error, now time.Time) (TaskRecord, RunRecord, error) {
	task := newTaskRecord(paths, req, Task{Name: setupTaskName}, 1, now)
	if err := os.MkdirAll(task.OutputDir, 0o755); err != nil {
		return TaskRecord{}, RunRecord{}, err
	}
	if err := os.WriteFile(filepath.Join(task.OutputDir, combinedLogName), []byte(cause.Error()+"\n"), 0o644); err != nil {
		return TaskRecord{}, RunRecord{}, err
	}
	task.Status = TaskStatusFailed
	task.Failure = "setup"
	task.Message = cause.Error()
	task.FinishedAt = now
	task.DurationMilliseconds = durationMilliseconds(task.StartedAt, task.FinishedAt)
	if err := writeTaskRecord(task); err != nil {
		return TaskRecord{}, RunRecord{}, err
	}
	if _, err := ensureRunRecordWithTasks(paths, req, []Task{{Name: setupTaskName}}, now); err != nil {
		return TaskRecord{}, RunRecord{}, err
	}
	run, err := upsertTaskRecord(paths, req, task, now)
	if err != nil {
		return TaskRecord{}, RunRecord{}, err
	}
	return task, run, nil
}

func (s Scheduler) cloneManager() CloneManager {
	if s.Clones.Paths.Root != "" {
		return s.Clones
	}
	return CloneManager{Paths: s.Runner.Paths, Now: s.Runner.Now}
}

func (s Scheduler) cleanupCloneIfDrained(entry QueueEntry) error {
	live, err := s.Queue.LiveRefs()
	if err != nil {
		return err
	}
	if live[queueRefKey(entry.RepoDir, entry.Commit)] {
		return nil
	}
	return s.cloneManager().Cleanup(entry.RepoDir, entry.Commit)
}

func upsertTaskRecord(paths Paths, req InvokeRequest, record TaskRecord, now time.Time) (RunRecord, error) {
	run, err := readRunRecord(paths, req)
	if err != nil {
		if !errors.Is(err, ErrRecordNotFound) {
			return RunRecord{}, err
		}
		run = newRunRecord(req, record.StartedAt)
	}
	if len(req.Annotations) > 0 {
		if run.Annotations == nil {
			run.Annotations = map[string]string{}
		}
		for key, value := range req.Annotations {
			run.Annotations[key] = value
		}
	}

	replaced := false
	for i, existing := range run.TaskResults {
		if existing.Name == record.Name {
			run.TaskResults[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		run.TaskResults = append(run.TaskResults, record)
	}

	run.FinishedAt = now
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		return RunRecord{}, err
	}
	return run, nil
}

func ensureRunRecordWithTasks(paths Paths, req InvokeRequest, tasks []Task, now time.Time) (RunRecord, error) {
	run, err := readRunRecord(paths, req)
	if err != nil {
		if !errors.Is(err, ErrRecordNotFound) {
			return RunRecord{}, err
		}
		run = newRunRecord(req, now)
	}
	run.DiscoveredTasks = append([]Task{}, tasks...)
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		return RunRecord{}, err
	}
	return run, nil
}

func splitSetupTask(tasks []Task) (Task, []Task, bool) {
	var setup Task
	hasSetup := false
	rest := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if !isSetupTask(task.Name) {
			rest = append(rest, task)
			continue
		}
		if isRootSetupTask(task.Name) && !hasSetup {
			setup = task
			hasSetup = true
		}
	}
	return setup, rest, hasSetup
}

func isSetupTask(name string) bool {
	_, taskName := splitTaskName(name)
	return taskName == setupTaskName
}

func isRootSetupTask(name string) bool {
	prefix, taskName := splitTaskName(name)
	return taskName == setupTaskName && (prefix == "" || prefix == "//:")
}
