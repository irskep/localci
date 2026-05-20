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

type TaskStatus string

const (
	TaskStatusSucceeded TaskStatus = "succeeded"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusTimedOut  TaskStatus = "timed-out"
	TaskStatusRunning   TaskStatus = "running"
)

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

type Task struct {
	Name string `json:"name"`
}

type InvokeRequest struct {
	RepoDir string
	Commit  string
}

type InvokeResult struct {
	RepoDir     string       `json:"repo_dir"`
	Commit      string       `json:"commit"`
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at"`
	TaskResults []TaskResult `json:"task_results"`
}

func (r InvokeResult) Success() bool {
	for _, result := range r.TaskResults {
		if result.Status != TaskStatusSucceeded {
			return false
		}
	}

	return true
}

type TaskResult struct {
	Name         string        `json:"name"`
	OutputDir    string        `json:"output_dir"`
	TaskCacheDir string        `json:"task_cache_dir"`
	CacheDir     string        `json:"cache_dir"`
	Status       TaskStatus    `json:"status"`
	StartedAt    time.Time     `json:"started_at"`
	FinishedAt   time.Time     `json:"finished_at"`
	Duration     time.Duration `json:"duration"`
	Error        string        `json:"error,omitempty"`
}

type miseTask struct {
	Name string `json:"name"`
}

func (r Runner) Invoke(ctx context.Context, req InvokeRequest) (InvokeResult, error) {
	if strings.TrimSpace(req.RepoDir) == "" {
		return InvokeResult{}, fmt.Errorf("repo dir must not be empty")
	}
	if strings.TrimSpace(req.Commit) == "" {
		return InvokeResult{}, fmt.Errorf("commit must not be empty")
	}

	if err := os.MkdirAll(r.Paths.CommitRoot(req.RepoDir, req.Commit), 0o755); err != nil {
		return InvokeResult{}, fmt.Errorf("create commit root: %w", err)
	}

	tasks, err := r.DiscoverTasks(ctx, req.RepoDir)
	if err != nil {
		return InvokeResult{}, err
	}

	now := r.now()
	result := InvokeResult{
		RepoDir:     req.RepoDir,
		Commit:      req.Commit,
		StartedAt:   now,
		TaskResults: make([]TaskResult, 0, len(tasks)),
	}

	if err := r.writeInvokeResult(req, result); err != nil {
		return InvokeResult{}, err
	}

	for _, task := range tasks {
		taskResult, runErr := r.runTask(ctx, req, task)
		result.TaskResults = append(result.TaskResults, taskResult)
		result.FinishedAt = r.now()

		if writeErr := r.writeInvokeResult(req, result); writeErr != nil {
			return InvokeResult{}, writeErr
		}

		if runErr != nil && !errors.Is(runErr, errTaskFailed) && !errors.Is(runErr, errTaskTimedOut) {
			return result, runErr
		}
	}

	if result.FinishedAt.IsZero() {
		result.FinishedAt = r.now()
		if err := r.writeInvokeResult(req, result); err != nil {
			return InvokeResult{}, err
		}
	}

	return result, nil
}

func (r Runner) DiscoverTasks(ctx context.Context, repoDir string) ([]Task, error) {
	miseBin := r.miseBin()
	cmd := exec.CommandContext(ctx, miseBin, "tasks", "--json", "--local")
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
		if strings.HasPrefix(task.Name, taskPrefix) {
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

func (r Runner) runTask(ctx context.Context, req InvokeRequest, task Task) (TaskResult, error) {
	startedAt := r.now()
	outputDir := r.Paths.TaskOutputDir(req.RepoDir, req.Commit, task.Name)
	taskCacheDir := r.Paths.TaskCacheDir(req.RepoDir, task.Name)
	cacheDir := r.Paths.SharedCacheDir()

	for _, dir := range []string{
		r.Paths.CommitRoot(req.RepoDir, req.Commit),
		r.Paths.CommitCacheDir(req.RepoDir, req.Commit),
		outputDir,
		taskCacheDir,
		cacheDir,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return TaskResult{}, fmt.Errorf("create directory %q: %w", dir, err)
		}
	}

	result := TaskResult{
		Name:         task.Name,
		OutputDir:    outputDir,
		TaskCacheDir: taskCacheDir,
		CacheDir:     cacheDir,
		Status:       TaskStatusRunning,
		StartedAt:    startedAt,
	}

	if err := r.writeTaskResult(req, result); err != nil {
		return TaskResult{}, err
	}

	cmd := exec.CommandContext(ctx, r.miseBin(), "run", task.Name)
	cmd.Dir = req.RepoDir
	cmd.Env = append(r.env(),
		"LOCALCI_TASK_OUTPUT_DIR="+outputDir,
		"LOCALCI_TASK_CACHE_DIR="+taskCacheDir,
		"LOCALCI_CACHE_DIR="+cacheDir,
	)
	cmd.Stdout = r.stdout()
	cmd.Stderr = r.stderr()

	if err := cmd.Start(); err != nil {
		return TaskResult{}, fmt.Errorf("start task %q: %w", task.Name, err)
	}

	waitResult := make(chan error, 1)
	go func() {
		waitResult <- cmd.Wait()
	}()

	runErr := r.watchTask(ctx, cmd, outputDir, waitResult)
	result.FinishedAt = r.now()
	result.Duration = result.FinishedAt.Sub(startedAt)

	switch {
	case runErr == nil:
		result.Status = TaskStatusSucceeded
	case errors.Is(runErr, errTaskTimedOut):
		result.Status = TaskStatusTimedOut
		result.Error = runErr.Error()
	case errors.Is(runErr, errTaskFailed):
		result.Status = TaskStatusFailed
		result.Error = runErr.Error()
	default:
		return TaskResult{}, runErr
	}

	if err := r.writeTaskResult(req, result); err != nil {
		return TaskResult{}, err
	}

	return result, runErr
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
				return fmt.Errorf("%w: exit status %d", errTaskFailed, exitErr.ExitCode())
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
		return fmt.Errorf("%w: no process for task after %s", errTaskTimedOut, timeout)
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
		return fmt.Errorf("%w: no output for %s", errTaskTimedOut, timeout)
	case <-waitResult:
		return fmt.Errorf("%w: no output for %s", errTaskTimedOut, timeout)
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

func (r Runner) writeInvokeResult(req InvokeRequest, result InvokeResult) error {
	path := filepath.Join(r.Paths.CommitRoot(req.RepoDir, req.Commit), "invoke.json")
	return writeJSONFile(path, result)
}

func (r Runner) writeTaskResult(req InvokeRequest, result TaskResult) error {
	path := filepath.Join(result.OutputDir, "result.json")
	return writeJSONFile(path, result)
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = append(data, '\n')

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}
	return nil
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
	if len(r.Env) > 0 {
		return append([]string{}, r.Env...)
	}

	return os.Environ()
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
