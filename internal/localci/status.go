package localci

import (
	"errors"
	"io/fs"
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

	run.TaskResults = tasks
	run.RefreshSummary()

	return CommitStatus{
		Run:   run,
		Tasks: tasks,
	}, nil
}

func (r StatusReader) readTaskRecords(repoDir string, commit string) ([]TaskRecord, error) {
	root := filepath.Join(r.Paths.CommitRoot(repoDir, commit), "out")
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
		return records[i].Name < records[j].Name
	})

	return records, nil
}
