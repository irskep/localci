package localci

import "time"

type RunStatus string

const (
	RunStatusRunning   RunStatus = "running"
	RunStatusSucceeded RunStatus = "succeeded"
	RunStatusFailed    RunStatus = "failed"
)

type TaskStatus string

const (
	TaskStatusSucceeded TaskStatus = "succeeded"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusTimedOut  TaskStatus = "timed-out"
	TaskStatusRunning   TaskStatus = "running"
)

type Task struct {
	Name string `json:"name"`
}

type InvokeRequest struct {
	RepoDir string
	Commit  string
}

type RunRecord struct {
	RepoDir     string       `json:"repo_dir"`
	RepoID      string       `json:"repo_id"`
	Commit      string       `json:"commit"`
	Status      RunStatus    `json:"status"`
	StartedAt   time.Time    `json:"started_at"`
	FinishedAt  time.Time    `json:"finished_at,omitempty"`
	Summary     RunSummary   `json:"summary"`
	TaskResults []TaskRecord `json:"task_results"`
}

func (r RunRecord) Success() bool {
	return r.Status == RunStatusSucceeded
}

func (r *RunRecord) RefreshSummary() {
	summary := RunSummary{
		Total: len(r.TaskResults),
	}
	status := RunStatusSucceeded

	if r.FinishedAt.IsZero() {
		status = RunStatusRunning
	}

	for _, task := range r.TaskResults {
		switch task.Status {
		case TaskStatusSucceeded:
			summary.Succeeded++
		case TaskStatusFailed:
			summary.Failed++
			status = RunStatusFailed
		case TaskStatusTimedOut:
			summary.TimedOut++
			status = RunStatusFailed
		case TaskStatusRunning:
			status = RunStatusRunning
		}
	}

	if len(r.TaskResults) == 0 && !r.FinishedAt.IsZero() {
		status = RunStatusSucceeded
	}

	r.Status = status
	r.Summary = summary
}

type RunSummary struct {
	Total     int `json:"total"`
	Succeeded int `json:"succeeded"`
	Failed    int `json:"failed"`
	TimedOut  int `json:"timed_out"`
}

type TaskRecord struct {
	Name                 string     `json:"name"`
	ShortName            string     `json:"short_name"`
	OutputDir            string     `json:"output_dir"`
	TaskCacheDir         string     `json:"task_cache_dir"`
	SharedCacheDir       string     `json:"shared_cache_dir"`
	Status               TaskStatus `json:"status"`
	StartedAt            time.Time  `json:"started_at"`
	FinishedAt           time.Time  `json:"finished_at,omitempty"`
	DurationMilliseconds int64      `json:"duration_ms,omitempty"`
	ExitCode             *int       `json:"exit_code,omitempty"`
	Failure              string     `json:"failure,omitempty"`
	Message              string     `json:"message,omitempty"`
}

func newRunRecord(req InvokeRequest, startedAt time.Time) RunRecord {
	record := RunRecord{
		RepoDir:     req.RepoDir,
		RepoID:      normalizeRepoDir(req.RepoDir),
		Commit:      req.Commit,
		Status:      RunStatusRunning,
		StartedAt:   startedAt,
		TaskResults: []TaskRecord{},
	}
	record.RefreshSummary()
	return record
}

func newTaskRecord(paths Paths, req InvokeRequest, task Task, startedAt time.Time) TaskRecord {
	return TaskRecord{
		Name:           task.Name,
		ShortName:      trimTaskPrefix(task.Name),
		OutputDir:      paths.TaskOutputDir(req.RepoDir, req.Commit, task.Name),
		TaskCacheDir:   paths.TaskCacheDir(req.RepoDir, task.Name),
		SharedCacheDir: paths.SharedCacheDir(),
		Status:         TaskStatusRunning,
		StartedAt:      startedAt,
	}
}

func trimTaskPrefix(name string) string {
	if len(name) > len(taskPrefix) && name[:len(taskPrefix)] == taskPrefix {
		return name[len(taskPrefix):]
	}

	return name
}
