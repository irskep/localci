package localci

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
)

type ExecutionStatus string

const (
	ExecutionStatusNotRun    ExecutionStatus = "not-run"
	ExecutionStatusQueued    ExecutionStatus = "queued"
	ExecutionStatusRunning   ExecutionStatus = "running"
	ExecutionStatusSucceeded ExecutionStatus = "succeeded"
	ExecutionStatusFailed    ExecutionStatus = "failed"
	ExecutionStatusTimedOut  ExecutionStatus = "timed-out"
)

type CommitStatusView struct {
	RepoDir string           `json:"repo_dir"`
	Commit  string           `json:"commit"`
	Tasks   []TaskStatusView `json:"tasks"`
}

type TaskStatusView struct {
	Name        string          `json:"name"`
	ShortName   string          `json:"short_name"`
	Status      ExecutionStatus `json:"status"`
	OutputDir   string          `json:"output_dir"`
	OutputFiles []string        `json:"output_files"`
}

func BuildCommitStatusView(paths Paths, repoDir string, commit string, discovered []Task, queued []QueueEntry, active *ActiveTask) (CommitStatusView, error) {
	view := CommitStatusView{
		RepoDir: repoDir,
		Commit:  commit,
		Tasks:   make([]TaskStatusView, 0, len(discovered)),
	}

	reader := StatusReader{Paths: paths}
	commitStatus, err := reader.ReadCommit(repoDir, commit)
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return CommitStatusView{}, err
	}

	taskRecords := map[string]TaskRecord{}
	if err == nil {
		for _, record := range commitStatus.Tasks {
			taskRecords[record.Name] = record
		}
	}

	queuedSet := map[string]bool{}
	for _, entry := range queued {
		if entry.RepoDir == repoDir && entry.Commit == commit {
			queuedSet[entry.TaskName] = true
		}
	}

	for _, task := range discovered {
		outputDir := paths.TaskOutputDir(repoDir, commit, task.Name)
		outputFiles, err := listOutputFiles(outputDir)
		if err != nil {
			return CommitStatusView{}, err
		}

		status := ExecutionStatusNotRun
		if active != nil && active.RepoDir == repoDir && active.Commit == commit && active.TaskName == task.Name {
			status = ExecutionStatusRunning
		} else if queuedSet[task.Name] {
			status = ExecutionStatusQueued
		} else if record, ok := taskRecords[task.Name]; ok {
			status = executionStatusFromTaskRecord(record)
		}

		view.Tasks = append(view.Tasks, TaskStatusView{
			Name:        task.Name,
			ShortName:   trimTaskPrefix(task.Name),
			Status:      status,
			OutputDir:   outputDir,
			OutputFiles: outputFiles,
		})
	}

	sort.Slice(view.Tasks, func(i int, j int) bool {
		return view.Tasks[i].Name < view.Tasks[j].Name
	})

	return view, nil
}

func executionStatusFromTaskRecord(record TaskRecord) ExecutionStatus {
	switch record.Status {
	case TaskStatusSucceeded:
		return ExecutionStatusSucceeded
	case TaskStatusFailed:
		return ExecutionStatusFailed
	case TaskStatusTimedOut:
		return ExecutionStatusTimedOut
	case TaskStatusRunning:
		return ExecutionStatusRunning
	default:
		return ExecutionStatusNotRun
	}
}

func listOutputFiles(outputDir string) ([]string, error) {
	files := []string{}

	err := filepath.WalkDir(outputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() {
			return nil
		}
		files = append(files, path)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Strings(files)
	return files, nil
}
