package localci

import (
	"errors"
	"io/fs"
	"path/filepath"
	"sort"
	"time"
)

type HistoryReader struct {
	Paths Paths
}

type RepoHistory struct {
	RepoDir string
	RepoID  string
	Commits []RunRecord
}

func (r HistoryReader) ListRepos() ([]RepoHistory, error) {
	runs, err := r.listRuns()
	if err != nil {
		return nil, err
	}

	byRepo := map[string]*RepoHistory{}
	for _, run := range runs {
		repo := byRepo[run.RepoDir]
		if repo == nil {
			repo = &RepoHistory{
				RepoDir: run.RepoDir,
				RepoID:  run.RepoID,
			}
			byRepo[run.RepoDir] = repo
		}
		repo.Commits = append(repo.Commits, run)
	}

	repos := make([]RepoHistory, 0, len(byRepo))
	for _, repo := range byRepo {
		sortRunRecords(repo.Commits)
		repos = append(repos, *repo)
	}

	sort.Slice(repos, func(i int, j int) bool {
		left := mostRecentRun(repos[i].Commits)
		right := mostRecentRun(repos[j].Commits)
		leftAt := RunActivityAt(left)
		rightAt := RunActivityAt(right)
		if leftAt.Equal(rightAt) {
			return repos[i].RepoDir < repos[j].RepoDir
		}
		return leftAt.After(rightAt)
	})

	return repos, nil
}

func (r HistoryReader) ListRepoCommits(repoDir string) ([]RunRecord, error) {
	repos, err := r.ListRepos()
	if err != nil {
		return nil, err
	}

	for _, repo := range repos {
		if repo.RepoDir == repoDir {
			return repo.Commits, nil
		}
	}

	return nil, ErrRecordNotFound
}

func (r HistoryReader) listRuns() ([]RunRecord, error) {
	runs := []RunRecord{}

	err := filepath.WalkDir(r.Paths.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != "run.json" {
			return nil
		}

		var run RunRecord
		if err := readJSONFile(path, &run); err != nil {
			return err
		}
		runs = append(runs, run)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sortRunRecords(runs)
	return runs, nil
}

func sortRunRecords(runs []RunRecord) {
	sort.Slice(runs, func(i int, j int) bool {
		leftAt := RunActivityAt(runs[i])
		rightAt := RunActivityAt(runs[j])
		if leftAt.Equal(rightAt) {
			if runs[i].RepoDir == runs[j].RepoDir {
				return runs[i].Commit > runs[j].Commit
			}
			return runs[i].RepoDir < runs[j].RepoDir
		}
		return leftAt.After(rightAt)
	})
}

func mostRecentRun(runs []RunRecord) RunRecord {
	if len(runs) == 0 {
		return RunRecord{}
	}
	return runs[0]
}

func RunActivityAt(run RunRecord) time.Time {
	if !run.FinishedAt.IsZero() {
		return run.FinishedAt
	}
	return run.StartedAt
}
