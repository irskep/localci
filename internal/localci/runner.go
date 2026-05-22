package localci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"
)

const taskPrefix = "localci:"

type Runner struct {
	Paths             Paths
	MiseBin           string
	Env               []string
	Stdout            io.Writer
	Stderr            io.Writer
	InactivityTimeout time.Duration
	TerminateGrace    time.Duration
	PollInterval      time.Duration
	Now               func() time.Time
}

type miseTask struct {
	Name string `json:"name"`
}

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

	tasks, err := r.DiscoverTasks(ctx, req.RepoDir)
	if err != nil {
		return RunRecord{}, err
	}

	now := r.now()
	record := newRunRecord(req, now)
	record.TaskResults = make([]TaskRecord, 0, len(tasks))

	if err := writeRunRecord(r.Paths, req, record); err != nil {
		return RunRecord{}, err
	}

	for _, task := range tasks {
		taskRecord, runErr := r.runTask(ctx, req, task, 0)
		record.TaskResults = append(record.TaskResults, taskRecord)
		record.FinishedAt = r.now()
		record.RefreshSummary()

		if writeErr := writeRunRecord(r.Paths, req, record); writeErr != nil {
			return RunRecord{}, writeErr
		}

		if runErr != nil && !errors.Is(runErr, errTaskFailed) && !errors.Is(runErr, errTaskTimedOut) {
			return record, runErr
		}
	}

	if record.FinishedAt.IsZero() {
		record.FinishedAt = r.now()
		record.RefreshSummary()
		if err := writeRunRecord(r.Paths, req, record); err != nil {
			return RunRecord{}, err
		}
	}

	return record, nil
}

func (r Runner) DiscoverTasks(ctx context.Context, repoDir string) ([]Task, error) {
	miseBin := r.miseBin()
	cmd := exec.CommandContext(ctx, miseBin, "tasks", "--json", "--all")
	cmd.Dir = repoDir
	cmd.Env = r.env()

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

var (
	errTaskFailed   = errors.New("task failed")
	errTaskTimedOut = errors.New("task timed out")
)

func (r Runner) runTask(ctx context.Context, req InvokeRequest, task Task, reservedAttempt int) (TaskRecord, error) {
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

	cmd := exec.CommandContext(ctx, r.miseBin(), "run", task.Name)
	cmd.Dir = req.RepoDir
	cmd.Env = append(r.env(),
		"LOCALCI_TASK_OUTPUT_DIR="+record.OutputDir,
		"LOCALCI_TASK_CACHE_DIR="+record.TaskCacheDir,
		"LOCALCI_CACHE_DIR="+record.SharedCacheDir,
	)

	logWriters, err := newTaskLogWriters(record.OutputDir, r.stdout())
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
	var exitErr taskExitError

	switch {
	case runErr == nil:
		record.Status = TaskStatusSucceeded
	case errors.As(runErr, &timeoutErr):
		record.Status = TaskStatusTimedOut
		record.Failure = "timed-out"
		record.Message = timeoutErr.Error()
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

func (r Runner) watchTask(ctx context.Context, cmd *exec.Cmd, outputDir string, waitResult <-chan error) error {
	pollTicker := time.NewTicker(r.pollInterval())
	defer pollTicker.Stop()

	lastActivity := r.now()
	timeout := r.inactivityTimeout()

	for {
		select {
		case err := <-waitResult:
			if err == nil {
				return nil
			}

			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return taskExitError{ExitCode: exitErr.ExitCode()}
			}

			return err
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-waitResult
			return ctx.Err()
		case <-pollTicker.C:
			latest, err := latestOutputActivity(outputDir)
			if err != nil {
				return fmt.Errorf("scan output activity: %w", err)
			}
			if latest.After(lastActivity) {
				lastActivity = latest
				continue
			}
			if r.now().Sub(lastActivity) < timeout {
				continue
			}
			return r.stopTimedOutTask(cmd, waitResult, timeout)
		}
	}
}

func (r Runner) stopTimedOutTask(cmd *exec.Cmd, waitResult <-chan error, timeout time.Duration) error {
	if cmd.Process == nil {
		return taskTimedOutError{After: timeout}
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate timed out task: %w", err)
	}

	timer := time.NewTimer(r.terminateGrace())
	defer timer.Stop()

	select {
	case <-timer.C:
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill timed out task: %w", err)
		}
		<-waitResult
		return taskTimedOutError{After: timeout}
	case <-waitResult:
		return taskTimedOutError{After: timeout}
	}
}

func latestOutputActivity(dir string) (time.Time, error) {
	var latest time.Time

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}

	return latest, nil
}

func commandError(action string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Errorf("%s: %s", action, strings.TrimSpace(string(exitErr.Stderr)))
	}

	return fmt.Errorf("%s: %w", action, err)
}

func (r Runner) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}

	return time.Now().UTC()
}

func (r Runner) env() []string {
	var env []string
	if len(r.Env) > 0 {
		env = append([]string{}, r.Env...)
	} else {
		env = os.Environ()
	}
	return withEnvVar(env, "MISE_EXPERIMENTAL", "1")
}

func (r Runner) miseBin() string {
	if r.MiseBin != "" {
		return r.MiseBin
	}

	return "mise"
}

func (r Runner) stdout() io.Writer {
	if r.Stdout != nil {
		return r.Stdout
	}

	return io.Discard
}

func (r Runner) stderr() io.Writer {
	if r.Stderr != nil {
		return r.Stderr
	}

	return io.Discard
}

func (r Runner) inactivityTimeout() time.Duration {
	if r.InactivityTimeout > 0 {
		return r.InactivityTimeout
	}

	return 5 * time.Minute
}

func (r Runner) terminateGrace() time.Duration {
	if r.TerminateGrace > 0 {
		return r.TerminateGrace
	}

	return 5 * time.Second
}

func (r Runner) pollInterval() time.Duration {
	if r.PollInterval > 0 {
		return r.PollInterval
	}

	return time.Second
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

func withEnvVar(env []string, key string, value string) []string {
	prefix := key + "="
	result := append([]string{}, env...)
	for index, entry := range result {
		if strings.HasPrefix(entry, prefix) {
			result[index] = prefix + value
			return result
		}
	}
	return append(result, prefix+value)
}

type taskLogWriters struct {
	combinedFile *os.File
	writer       io.Writer
}

func newTaskLogWriters(outputDir string, stdout io.Writer) (*taskLogWriters, error) {
	combinedFile, err := os.Create(filepath.Join(outputDir, "combined.log"))
	if err != nil {
		return nil, fmt.Errorf("create combined log: %w", err)
	}

	return &taskLogWriters{
		combinedFile: combinedFile,
		writer:       io.MultiWriter(combinedFile, stdout),
	}, nil
}

func (w *taskLogWriters) Close() {
	_ = w.combinedFile.Close()
}
