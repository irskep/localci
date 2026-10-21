package localci

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
)

type StatusReader struct {
	Paths Paths
}

type CommitStatus struct {
	Run   RunRecord    `json:"run"`
	Tasks []TaskRecord `json:"tasks"`
}

func (r StatusReader) ReadCommit(repoDir string, commit string) (CommitStatus, error) {
	req := InvokeRequest{
		RepoDir: repoDir,
		Commit:  commit,
	}

	run, err := readRunRecord(r.Paths, req)
	if err != nil {
		return CommitStatus{}, err
	}

	tasks, err := r.readTaskRecords(repoDir, commit)
	if err != nil {
		return CommitStatus{}, err
	}
	if len(tasks) == 0 && len(run.TaskResults) > 0 {
		tasks = append([]TaskRecord{}, run.TaskResults...)
	}

	run.TaskResults = tasks
	run.RefreshSummary()

	return CommitStatus{
		Run:   run,
		Tasks: tasks,
	}, nil
}

func (r StatusReader) readTaskRecords(repoDir string, commit string) ([]TaskRecord, error) {
	root := filepath.Join(r.Paths.CommitRoot(repoDir, commit), "out")
	taskRecords := map[string]TaskRecord{}

	entries, err := os.ReadDir(root)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	for _, task := range entries {
		if !task.IsDir() {
			continue
		}
		attempts, err := os.ReadDir(filepath.Join(root, task.Name()))
		if err != nil {
			return nil, err
		}
		for _, attempt := range attempts {
			if !attempt.IsDir() {
				continue
			}
			record, err := readTaskRecord(filepath.Join(root, task.Name(), attempt.Name(), taskRecordFileName))
			if errors.Is(err, ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			existing, ok := taskRecords[record.Name]
			if !ok || record.Attempt > existing.Attempt {
				taskRecords[record.Name] = record
			}
		}
	}

	if len(taskRecords) == 0 {
		run, err := (RunRepository{Paths: r.Paths}).ReadRun(repoDir, commit)
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return []TaskRecord{}, nil
			}
			return nil, err
		}
		for _, record := range run.TaskResults {
			taskRecords[record.Name] = record
		}
	}

	records := make([]TaskRecord, 0, len(taskRecords))
	for _, record := range taskRecords {
		records = append(records, record)
	}

	sort.Slice(records, func(i int, j int) bool {
		return records[i].Name < records[j].Name
	})

	return records, nil
}
