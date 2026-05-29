package localci

import (
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
	return (RunRepository{Paths: r.Paths}).ListRepos()
}

func (r HistoryReader) ListRepoCommits(repoDir string) ([]RunRecord, error) {
	return (RunRepository{Paths: r.Paths}).ListRepoCommits(repoDir)
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
