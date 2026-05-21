package localci

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	RepoDir     string            `json:"repo_dir"`
	Commit      string            `json:"commit"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Tasks       []TaskStatusView  `json:"tasks"`
}

type TaskStatusView struct {
	Name                 string            `json:"name"`
	ShortName            string            `json:"short_name"`
	Attempt              int               `json:"attempt"`
	AttemptCount         int               `json:"attempt_count"`
	Status               ExecutionStatus   `json:"status"`
	OutputDir            string            `json:"output_dir"`
	OutputFiles          []string          `json:"output_files"`
	Artifacts            []ArtifactView    `json:"artifacts"`
	DurationMilliseconds int64             `json:"duration_ms"`
	Failure              string            `json:"failure"`
	Attempts             []TaskAttemptView `json:"attempts"`
}

type ArtifactView struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Path        string `json:"path"`
}

type TaskAttemptView struct {
	Attempt              int             `json:"attempt"`
	Status               ExecutionStatus `json:"status"`
	DurationMilliseconds int64           `json:"duration_ms"`
	Failure              string          `json:"failure"`
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
	if err == nil {
		view.Annotations = cloneAnnotations(commitStatus.Run.Annotations)
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
		status := ExecutionStatusNotRun
		outputDir := paths.TaskOutputDir(repoDir, commit, task.Name)
		attempt := 0
		attempts, err := listTaskAttemptViews(paths, repoDir, commit, task.Name)
		if err != nil {
			return CommitStatusView{}, err
		}

		var artifacts []ArtifactView
		var durationMilliseconds int64
		var failure string
		if record, ok := taskRecords[task.Name]; ok {
			outputDir = record.OutputDir
			attempt = record.Attempt
			status = executionStatusFromTaskRecord(record)
			artifacts = buildArtifactViews(record.OutputDir, outputFilesOrNil(record.OutputDir))
			durationMilliseconds = record.DurationMilliseconds
			failure = record.Failure
		}
		if active != nil && active.RepoDir == repoDir && active.Commit == commit && active.TaskName == task.Name {
			status = ExecutionStatusRunning
		} else if queuedSet[task.Name] {
			status = ExecutionStatusQueued
		}

		outputFiles, err := listOutputFiles(outputDir)
		if err != nil {
			return CommitStatusView{}, err
		}
		if len(artifacts) == 0 {
			artifacts = buildArtifactViews(outputDir, outputFiles)
		}

		view.Tasks = append(view.Tasks, TaskStatusView{
			Name:                 task.Name,
			ShortName:            trimTaskPrefix(task.Name),
			Attempt:              attempt,
			AttemptCount:         len(attempts),
			Status:               status,
			OutputDir:            outputDir,
			OutputFiles:          outputFiles,
			Artifacts:            artifacts,
			DurationMilliseconds: durationMilliseconds,
			Failure:              failure,
			Attempts:             attempts,
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

func listTaskAttemptViews(paths Paths, repoDir string, commit string, taskName string) ([]TaskAttemptView, error) {
	records, err := listTaskAttemptRecords(paths, repoDir, commit, taskName)
	if err != nil {
		return nil, err
	}

	views := make([]TaskAttemptView, 0, len(records))
	for _, record := range records {
		views = append(views, TaskAttemptView{
			Attempt:              record.Attempt,
			Status:               executionStatusFromTaskRecord(record),
			DurationMilliseconds: record.DurationMilliseconds,
			Failure:              record.Failure,
		})
	}

	return views, nil
}

func listTaskAttemptRecords(paths Paths, repoDir string, commit string, taskName string) ([]TaskRecord, error) {
	root := paths.TaskOutputDir(repoDir, commit, taskName)
	records := []TaskRecord{}

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != "task.json" {
			return nil
		}

		record, err := readTaskRecord(path)
		if err != nil {
			return err
		}
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(records, func(i int, j int) bool {
		return records[i].Attempt > records[j].Attempt
	})
	return records, nil
}

func buildArtifactViews(outputDir string, files []string) []ArtifactView {
	artifacts := make([]ArtifactView, 0, len(files))
	for _, file := range files {
		displayName := relativeArtifactName(outputDir, file)
		if shouldHideArtifact(displayName) {
			continue
		}
		artifacts = append(artifacts, ArtifactView{
			Name:        filepath.Base(file),
			DisplayName: displayName,
			Path:        file,
		})
	}
	sort.Slice(artifacts, func(i int, j int) bool {
		left := artifactSortKey(artifacts[i].DisplayName)
		right := artifactSortKey(artifacts[j].DisplayName)
		if left == right {
			return artifacts[i].DisplayName < artifacts[j].DisplayName
		}
		return left < right
	})
	return artifacts
}

func relativeArtifactName(outputDir string, file string) string {
	relative, err := filepath.Rel(outputDir, file)
	if err != nil {
		return filepath.Base(file)
	}
	return relative
}

func outputFilesOrNil(outputDir string) []string {
	files, err := listOutputFiles(outputDir)
	if err != nil {
		return nil
	}
	return files
}

func artifactSortKey(name string) int {
	switch strings.ToLower(name) {
	case "combined.log":
		return 0
	case "task.json":
		return 100
	default:
		if strings.HasPrefix(name, "bin/") {
			return 50
		}
		return 10
	}
}

func PrimaryArtifact(task TaskStatusView) (ArtifactView, bool) {
	for _, preferred := range []string{"combined.log"} {
		for _, artifact := range task.Artifacts {
			if artifact.DisplayName == preferred {
				return artifact, true
			}
		}
	}
	if len(task.Artifacts) > 0 {
		return task.Artifacts[0], true
	}
	return ArtifactView{}, false
}

func shouldHideArtifact(displayName string) bool {
	return strings.EqualFold(displayName, "task.json")
}

func LoadPrimaryLog(task TaskStatusView) (string, string) {
	artifact, ok := PrimaryArtifact(task)
	if !ok {
		return "", ""
	}
	data, err := os.ReadFile(artifact.Path)
	if err != nil {
		return artifact.DisplayName, ""
	}
	return artifact.DisplayName, string(data)
}
