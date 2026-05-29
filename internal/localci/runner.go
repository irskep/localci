package localci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"
)

const (
	combinedLogName  = "combined.log"
	cancelMarkerName = ".localci-cancel"
	taskPrefix       = "localci:"
)

type Runner struct {
	Paths             Paths
	MiseBin           string
	Env               []string
	Stdout            io.Writer
	Stderr            io.Writer
	Events            *EventNotifier
	InactivityTimeout time.Duration
	TerminateGrace    time.Duration
	PollInterval      time.Duration
	Now               func() time.Time
}

type miseTask struct {
	Name string `json:"name"`
}

const setupTaskName = "localci:setup"

func (r Runner) Invoke(ctx context.Context, req InvokeRequest) (RunRecord, error) {
	if strings.TrimSpace(req.RepoDir) == "" {
		return RunRecord{}, fmt.Errorf("repo dir must not be empty")
	}
	if strings.TrimSpace(req.Commit) == "" {
		return RunRecord{}, fmt.Errorf("commit must not be empty")
	}

	if err := os.MkdirAll(r.Paths.CommitRoot(req.RepoDir, req.Commit), 0o755); err != nil {
		return RunRecord{}, fmt.Errorf("create commit root: %w", err)
	}

	workDir := req.RepoDir
	if !req.NoClone {
		clones := CloneManager{Paths: r.Paths, Now: r.Now}
		info, err := clones.Prepare(ctx, req.RepoDir, req.Commit)
		if err != nil {
			_, run, recordErr := recordSetupFailure(r.Paths, req, err, r.now())
			if recordErr != nil {
				return RunRecord{}, recordErr
			}
			return run, nil
		}
		defer func() {
			_ = clones.Cleanup(req.RepoDir, req.Commit)
		}()
		workDir = info.Worktree
	}

	if err := r.Trust(ctx, workDir); err != nil {
		_, run, recordErr := recordSetupFailure(r.Paths, req, err, r.now())
		if recordErr != nil {
			return RunRecord{}, recordErr
		}
		return run, nil
	}

	tasks, err := r.DiscoverTasks(ctx, workDir)
	if err != nil {
		_, run, recordErr := recordSetupFailure(r.Paths, req, err, r.now())
		if recordErr != nil {
			return RunRecord{}, recordErr
		}
		return run, nil
	}
	tasks = filterRequestedTasks(tasks, req.RequestedTasks)

	now := r.now()
	record, err := ensureRunRecordWithTasks(r.Paths, req, tasks, now)
	if err != nil {
		return RunRecord{}, err
	}

	setup, userTasks, hasSetup := splitSetupTask(tasks)
	if hasSetup {
		taskRecord, runErr := r.runTask(ctx, req, workDir, setup, 0)
		record, err = upsertTaskRecord(r.Paths, req, taskRecord, r.now())
		if err != nil {
			return RunRecord{}, err
		}
		if runErr != nil {
			return record, nil
		}
	}

	for _, task := range userTasks {
		taskRecord, runErr := r.runTask(ctx, req, workDir, task, 0)
		record.TaskResults = append(record.TaskResults, taskRecord)
		record.FinishedAt = r.now()
		record.RefreshSummary()

		if writeErr := writeRunRecord(r.Paths, record); writeErr != nil {
			return RunRecord{}, writeErr
		}

		if runErr != nil && !errors.Is(runErr, errTaskFailed) && !errors.Is(runErr, errTaskTimedOut) {
			return record, runErr
		}
	}

	if record.FinishedAt.IsZero() {
		record.FinishedAt = r.now()
		record.RefreshSummary()
		if err := writeRunRecord(r.Paths, record); err != nil {
			return RunRecord{}, err
		}
	}

	return record, nil
}

func filterRequestedTasks(tasks []Task, requested []string) []Task {
	if len(requested) == 0 {
		return tasks
	}
	allowed := map[string]bool{}
	for _, name := range requested {
		allowed[name] = true
	}
	filtered := make([]Task, 0, len(tasks))
	for _, task := range tasks {
		if allowed[task.Name] || task.Name == setupTaskName {
			filtered = append(filtered, task)
		}
	}
	return filtered
}

func (r Runner) DiscoverTasks(ctx context.Context, repoDir string) ([]Task, error) {
	miseBin := r.miseBin()
	cmd := exec.CommandContext(ctx, miseBin, "tasks", "--json", "--all")
	cmd.Dir = repoDir
	cmd.Env = r.envForDir(repoDir)

	output, err := cmd.Output()
	if err != nil {
		return nil, commandError("discover tasks", err)
	}

	var allTasks []miseTask
	if err := json.Unmarshal(output, &allTasks); err != nil {
		return nil, fmt.Errorf("parse mise task output: %w", err)
	}

	tasks := make([]Task, 0, len(allTasks))
	for _, task := range allTasks {
		if isLocalCITask(task.Name) {
			tasks = append(tasks, Task{Name: task.Name})
		}
	}

	slices.SortFunc(tasks, func(a Task, b Task) int {
		return strings.Compare(a.Name, b.Name)
	})

	return tasks, nil
}

func (r Runner) Trust(ctx context.Context, workDir string) error {
	cmd := exec.CommandContext(ctx, r.miseBin(), "trust")
	cmd.Dir = workDir
	cmd.Env = r.envForDir(workDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("mise trust: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

var (
	errTaskFailed   = errors.New("task failed")
	errTaskTimedOut = errors.New("task timed out")
)

func (r Runner) runTask(ctx context.Context, req InvokeRequest, workDir string, task Task, reservedAttempt int) (TaskRecord, error) {
	startedAt := r.now()
	attempt := reservedAttempt
	if attempt <= 0 {
		nextAttempt, err := nextTaskAttempt(r.Paths, req.RepoDir, req.Commit, task.Name)
		if err != nil {
			return TaskRecord{}, err
		}
		attempt = nextAttempt
	}
	record := newTaskRecord(r.Paths, req, task, attempt, startedAt)

	for _, dir := range []string{
		r.Paths.CommitRoot(req.RepoDir, req.Commit),
		r.Paths.CommitCacheDir(req.RepoDir, req.Commit),
		record.OutputDir,
		record.TaskCacheDir,
		record.SharedCacheDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return TaskRecord{}, fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	if err := writeTaskRecord(record); err != nil {
		return TaskRecord{}, err
	}
	entry := QueueEntry{
		Kind:     QueueEntryKindTask,
		RepoDir:  req.RepoDir,
		RepoID:   normalizeRepoDir(req.RepoDir),
		Commit:   req.Commit,
		TaskName: task.Name,
		TaskKey:  sanitizeTaskName(task.Name),
		Attempt:  attempt,
	}
	if r.Events != nil {
		r.Events.EntryChanged(entry)
	}

	cmd := exec.CommandContext(ctx, r.miseBin(), "run", task.Name)
	cmd.Dir = workDir
	cmd.Env = append(r.envForDir(workDir),
		"LOCALCI_TASK_OUTPUT_DIR="+record.OutputDir,
		"LOCALCI_TASK_CACHE_DIR="+record.TaskCacheDir,
		"LOCALCI_CACHE_DIR="+record.SharedCacheDir,
	)

	logWriters, err := newTaskLogWriters(record.OutputDir, r.stdout(), func(offset int64, text string) {
		if r.Events != nil {
			r.Events.ArtifactAppended(entry, combinedLogName, offset, text)
		}
	})
	if err != nil {
		return TaskRecord{}, err
	}
	defer logWriters.Close()
	cmd.Stdout = logWriters.writer
	cmd.Stderr = logWriters.writer

	if err := cmd.Start(); err != nil {
		record.FinishedAt = r.now()
		record.DurationMilliseconds = durationMilliseconds(record.StartedAt, record.FinishedAt)
		record.Status = TaskStatusFailed
		record.Failure = "start"
		record.Message = err.Error()
		if writeErr := writeTaskRecord(record); writeErr != nil {
			return TaskRecord{}, writeErr
		}
		return record, errTaskFailed
	}

	waitResult := make(chan error, 1)
	go func() {
		waitResult <- cmd.Wait()
	}()

	runErr := r.watchTask(ctx, cmd, record.OutputDir, waitResult)
	record.FinishedAt = r.now()
	record.DurationMilliseconds = durationMilliseconds(record.StartedAt, record.FinishedAt)
	var timeoutErr taskTimedOutError
	var canceledErr taskCanceledError
	var exitErr taskExitError

	switch {
	case runErr == nil:
		record.Status = TaskStatusSucceeded
	case errors.As(runErr, &timeoutErr):
		record.Status = TaskStatusTimedOut
		record.Failure = "timed-out"
		record.Message = timeoutErr.Error()
	case errors.As(runErr, &canceledErr):
		record.Status = TaskStatusFailed
		record.Failure = "canceled"
		record.Message = canceledErr.Error()
	case errors.As(runErr, &exitErr):
		record.Status = TaskStatusFailed
		record.Failure = "exit"
		record.ExitCode = intPtr(exitErr.ExitCode)
		record.Message = exitErr.Error()
	default:
		return TaskRecord{}, runErr
	}

	if err := writeTaskRecord(record); err != nil {
		return TaskRecord{}, err
	}
	if r.Events != nil {
		r.Events.EntryChanged(entry)
	}

	return record, runErr
}

func nextTaskAttempt(paths Paths, repoDir string, commit string, task string) (int, error) {
	dirEntries, err := os.ReadDir(paths.TaskOutputDir(repoDir, commit, task))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 1, nil
		}
		return 0, err
	}

	maxAttempt := 0
	for _, entry := range dirEntries {
		if !entry.IsDir() {
			continue
		}
		var attempt int
		if _, scanErr := fmt.Sscanf(entry.Name(), "attempt-%03d", &attempt); scanErr != nil {
			continue
		}
		if attempt > maxAttempt {
			maxAttempt = attempt
		}
	}

	return maxAttempt + 1, nil
}

func commandError(action string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%s: %s", action, strings.TrimSpace(string(exitErr.Stderr)))
	}

	return fmt.Errorf("%s: %w", action, err)
}

type taskExitError struct {
	ExitCode int
}

func (e taskExitError) Error() string {
	return fmt.Sprintf("task failed with exit status %d", e.ExitCode)
}

func (e taskExitError) Is(target error) bool {
	return target == errTaskFailed
}

type taskTimedOutError struct {
	After time.Duration
}

func (e taskTimedOutError) Error() string {
	return fmt.Sprintf("task timed out after %s with no output activity", e.After)
}

func (e taskTimedOutError) Is(target error) bool {
	return target == errTaskTimedOut
}

type taskCanceledError struct{}

func (e taskCanceledError) Error() string {
	return "task canceled"
}

func (e taskCanceledError) Is(target error) bool {
	return target == errTaskFailed
}

func durationMilliseconds(startedAt time.Time, finishedAt time.Time) int64 {
	return finishedAt.Sub(startedAt).Milliseconds()
}

func intPtr(value int) *int {
	return &value
}

func isLocalCITask(name string) bool {
	_, taskName := splitTaskName(name)
	return strings.HasPrefix(taskName, taskPrefix)
}

func splitTaskName(name string) (string, string) {
	if strings.HasPrefix(name, "//") {
		index := strings.Index(name[2:], ":")
		if index >= 0 {
			index += 2
			return name[:index+1], name[index+1:]
		}
	}
	return "", name
}
