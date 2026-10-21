package localci

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"
)

const DefaultPruneAge = 30 * 24 * time.Hour
const DefaultPruneSize int64 = 10 << 30

// Output locks coordinate pruning with both daemon work and foreground invokes.
// Lock files must stay in place: unlinking one can create two independent locks.
func lockOutputs(paths Paths, repo, commit string, exclusive bool) (func(), error) {
	dir := filepath.Join(paths.configRoot(), "output-locks")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(filepath.Join(dir, normalizeRepoDir(repo+"\x00"+commit)+".lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	mode := syscall.LOCK_SH
	if exclusive {
		mode = syscall.LOCK_EX | syscall.LOCK_NB
	}
	if err := syscall.Flock(int(file.Fd()), mode); err != nil {
		file.Close()
		return nil, err
	}
	return func() { _ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN); _ = file.Close() }, nil
}

type PruneOptions struct {
	MaxAge     time.Duration
	MaxSize    int64
	LargerThan int64
}
type PruneFile struct {
	Path     string
	Size     int64
	Modified time.Time
	Mode     fs.FileMode
}
type PruneRun struct {
	RepoDir  string
	Commit   string
	Activity time.Time
	Files    []PruneFile
	Bytes    int64
}
type PrunePlan struct {
	Runs          []PruneRun
	TotalBytes    int64
	RemoveBytes   int64
	ProtectedRuns int
}
type PruneResult struct {
	Files, SkippedRuns int
	Bytes              int64
}

// outputFiles never follows symlinks, and never treats task metadata as an artifact.
func outputFiles(paths Paths, run RunRecord) ([]PruneFile, error) {
	if run.Commit == "." || run.Commit == ".." || filepath.Base(run.Commit) != run.Commit || run.Commit == "" {
		return nil, fmt.Errorf("invalid commit path %q", run.Commit)
	}
	root := filepath.Join(paths.CommitRoot(run.RepoDir, run.Commit), "out")
	for _, dir := range []string{paths.RepoRoot(run.RepoDir), paths.CommitRoot(run.RepoDir, run.Commit), root} {
		info, err := os.Lstat(dir)
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("output directory is not a directory: %s", dir)
		}
	}
	files := []PruneFile{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() == taskRecordFileName {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		files = append(files, PruneFile{Path: path, Size: info.Size(), Modified: info.ModTime(), Mode: info.Mode()})
		return nil
	})
	return files, err
}

func PlanPrune(paths Paths, options PruneOptions, now time.Time) (PrunePlan, error) {
	plan := PrunePlan{}
	if options.MaxAge < 0 || options.MaxSize < 0 || options.LargerThan < 0 {
		return plan, fmt.Errorf("prune limits cannot be negative")
	}
	runs, err := (RunRepository{Paths: paths}).ListRuns()
	if err != nil {
		return plan, err
	}
	live, err := (QueueStore{Paths: paths}).LiveRefs()
	if err != nil {
		return plan, err
	}
	sort.Slice(runs, func(i, j int) bool { return RunActivityAt(runs[i]).Before(RunActivityAt(runs[j])) })
	candidates := []PruneRun{}
	for _, run := range runs {
		files, err := outputFiles(paths, run)
		if err != nil {
			return plan, err
		}
		candidate := PruneRun{RepoDir: run.RepoDir, Commit: run.Commit, Activity: RunActivityAt(run)}
		for _, file := range files {
			plan.TotalBytes += file.Size
			if options.LargerThan > 0 && (!file.Mode.IsRegular() || file.Size <= options.LargerThan) {
				continue
			}
			candidate.Files = append(candidate.Files, file)
			candidate.Bytes += file.Size
		}
		if live[queueRefKey(run.RepoDir, run.Commit)] || run.Status == RunStatusRunning {
			plan.ProtectedRuns++
			continue
		}
		if len(candidate.Files) > 0 {
			candidates = append(candidates, candidate)
		}
	}
	remaining := plan.TotalBytes
	for _, candidate := range candidates {
		if options.LargerThan > 0 || (options.MaxAge > 0 && candidate.Activity.Before(now.Add(-options.MaxAge))) || (options.MaxSize > 0 && remaining > options.MaxSize) {
			plan.Runs = append(plan.Runs, candidate)
			plan.RemoveBytes += candidate.Bytes
			remaining -= candidate.Bytes
		}
	}
	return plan, nil
}

// ApplyPrune only removes the previewed files, after rechecking the run and files.
func ApplyPrune(paths Paths, plan PrunePlan) (PruneResult, error) {
	result := PruneResult{}
	for _, candidate := range plan.Runs {
		skipped, err := pruneRun(paths, candidate, &result)
		if err != nil {
			return result, err
		}
		if skipped {
			result.SkippedRuns++
		}
	}
	return result, nil
}

func pruneRun(paths Paths, candidate PruneRun, result *PruneResult) (bool, error) {
	unlock, err := lockOutputs(paths, candidate.RepoDir, candidate.Commit, true)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer unlock()
	live, err := (QueueStore{Paths: paths}).LiveRefs()
	if err != nil {
		return false, err
	}
	repo := RunRepository{Paths: paths}
	run, err := repo.ReadRun(candidate.RepoDir, candidate.Commit)
	if err != nil {
		return false, err
	}
	if live[queueRefKey(run.RepoDir, run.Commit)] || run.Status == RunStatusRunning || !RunActivityAt(run).Equal(candidate.Activity) {
		return true, nil
	}
	files, err := outputFiles(paths, run)
	if err != nil {
		return false, err
	}
	current := make(map[string]PruneFile, len(files))
	for _, file := range files {
		current[file.Path] = file
	}
	for _, file := range candidate.Files {
		actual, ok := current[file.Path]
		if !ok || actual.Size != file.Size || !actual.Modified.Equal(file.Modified) || actual.Mode != file.Mode {
			return true, nil
		}
	}
	for _, file := range candidate.Files {
		if err := os.Remove(file.Path); err != nil {
			return false, err
		}
		result.Bytes += file.Size
		result.Files++
	}
	// Only fully pruned runs disappear from the cheap age-based status hint.
	if len(files) == len(candidate.Files) {
		db, release, err := repo.acquire()
		if err != nil {
			return false, err
		}
		defer release()
		_, err = db.ExecContext(repo.context(), `insert into artifact_prunes(repo_dir,commit_ref,activity_at) values(?,?,?) on conflict(repo_dir,commit_ref) do update set activity_at=excluded.activity_at`, run.RepoDir, run.Commit, formatDBTime(candidate.Activity))
		if err != nil {
			return false, err
		}
	}
	return false, nil
}

// OldOutputRunCount uses history only; it does not inspect artifact directories.
func (r RunRepository) OldOutputRunCount(before time.Time) (int, error) {
	db, release, err := r.acquire()
	if err != nil {
		return 0, err
	}
	defer release()
	var count int
	err = db.QueryRowContext(r.context(), `select count(*) from runs r where r.activity_at < ? and r.status != 'running' and not exists (select 1 from artifact_prunes p where p.repo_dir=r.repo_dir and p.commit_ref=r.commit_ref and p.activity_at=r.activity_at)`, formatDBTime(before)).Scan(&count)
	return count, err
}
