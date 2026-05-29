package localci

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type RunRepository struct {
	Paths Paths
}

func (r RunRepository) ListRepos() ([]RepoHistory, error) {
	runs, err := r.ListRuns()
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

func (r RunRepository) ListRuns() ([]RunRecord, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := r.importFilesystemRuns(context.Background(), db); err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(context.Background(), `
		select run_json
		from runs
		order by activity_at desc, repo_dir asc, commit_ref desc
	`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()

	runs := []RunRecord{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan run: %w", err)
		}
		var run RunRecord
		if err := json.Unmarshal([]byte(raw), &run); err != nil {
			return nil, fmt.Errorf("decode run history: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	return runs, nil
}

func (r RunRepository) ListRepoCommits(repoDir string) ([]RunRecord, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	if err := r.importFilesystemRuns(context.Background(), db); err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(context.Background(), `
		select run_json
		from runs
		where repo_dir = ?
		order by activity_at desc, commit_ref desc
	`, repoDir)
	if err != nil {
		return nil, fmt.Errorf("list repo runs: %w", err)
	}
	defer rows.Close()

	runs := []RunRecord{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, fmt.Errorf("scan repo run: %w", err)
		}
		var run RunRecord
		if err := json.Unmarshal([]byte(raw), &run); err != nil {
			return nil, fmt.Errorf("decode repo run history: %w", err)
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list repo runs: %w", err)
	}
	if len(runs) == 0 {
		return nil, ErrRecordNotFound
	}
	return runs, nil
}

func (r RunRepository) ReadRun(repoDir string, commit string) (RunRecord, error) {
	db, err := r.open()
	if err != nil {
		return RunRecord{}, err
	}
	defer db.Close()

	if err := r.importFilesystemRuns(context.Background(), db); err != nil {
		return RunRecord{}, err
	}

	var raw string
	err = db.QueryRowContext(context.Background(), `
		select run_json
		from runs
		where repo_dir = ? and commit_ref = ?
	`, repoDir, commit).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return RunRecord{}, fmt.Errorf("%w: %s/%s", ErrRecordNotFound, repoDir, commit)
	}
	if err != nil {
		return RunRecord{}, fmt.Errorf("read run history: %w", err)
	}

	var run RunRecord
	if err := json.Unmarshal([]byte(raw), &run); err != nil {
		return RunRecord{}, fmt.Errorf("decode run history: %w", err)
	}
	return run, nil
}

func (r RunRepository) WriteRun(run RunRecord) error {
	db, err := r.open()
	if err != nil {
		return err
	}
	defer db.Close()
	return upsertRun(context.Background(), db, run)
}

func (r RunRepository) open() (*sql.DB, error) {
	if strings.TrimSpace(r.Paths.Root) == "" {
		return nil, fmt.Errorf("history root is required")
	}
	if err := os.MkdirAll(r.Paths.Root, 0o755); err != nil {
		return nil, fmt.Errorf("create history root: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(r.Paths.HistoryDBPath()), 0o755); err != nil {
		return nil, fmt.Errorf("create history database directory: %w", err)
	}
	db, err := sql.Open("sqlite", r.Paths.HistoryDBPath())
	if err != nil {
		return nil, fmt.Errorf("open history database: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := ensureHistorySchema(context.Background(), db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func ensureHistorySchema(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `
		pragma busy_timeout = 5000;
		pragma journal_mode = wal;
		create table if not exists runs (
			repo_dir text not null,
			repo_id text not null,
			commit_ref text not null,
			status text not null,
			started_at text not null,
			finished_at text,
			activity_at text not null,
			run_json text not null,
			primary key (repo_dir, commit_ref)
		);
		create index if not exists runs_activity_at_idx on runs (activity_at desc);
		create index if not exists runs_repo_activity_at_idx on runs (repo_dir, activity_at desc);
		create table if not exists metadata (
			key text primary key,
			value text not null
		);
	`); err != nil {
		return fmt.Errorf("initialize history database: %w", err)
	}
	return nil
}

func (r RunRepository) importFilesystemRuns(ctx context.Context, db *sql.DB) error {
	var importedAt string
	err := db.QueryRowContext(ctx, `select value from metadata where key = 'filesystem_imported_at'`).Scan(&importedAt)
	if err == nil {
		return nil
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("read history import marker: %w", err)
	}

	err = filepath.WalkDir(r.Paths.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || d.Name() != "run.json" {
			return nil
		}
		rel, err := filepath.Rel(r.Paths.Root, path)
		if err != nil {
			return err
		}
		if len(strings.Split(filepath.ToSlash(rel), "/")) != 3 {
			return nil
		}

		var run RunRecord
		if err := readJSONFile(path, &run); err != nil {
			return err
		}
		return upsertRun(ctx, db, run)
	})
	if err != nil {
		return fmt.Errorf("import filesystem history: %w", err)
	}
	if _, err := db.ExecContext(ctx, `
		insert into metadata (key, value)
		values ('filesystem_imported_at', ?)
		on conflict(key) do update set value = excluded.value
	`, formatDBTime(time.Now())); err != nil {
		return fmt.Errorf("write history import marker: %w", err)
	}
	return nil
}

func upsertRun(ctx context.Context, db *sql.DB, run RunRecord) error {
	raw, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode run history: %w", err)
	}
	finishedAt := nullableTime(run.FinishedAt)
	if _, err := db.ExecContext(ctx, `
		insert into runs (
			repo_dir,
			repo_id,
			commit_ref,
			status,
			started_at,
			finished_at,
			activity_at,
			run_json
		) values (?, ?, ?, ?, ?, ?, ?, ?)
		on conflict(repo_dir, commit_ref) do update set
			repo_id = excluded.repo_id,
			status = excluded.status,
			started_at = excluded.started_at,
			finished_at = excluded.finished_at,
			activity_at = excluded.activity_at,
			run_json = excluded.run_json
	`, run.RepoDir, run.RepoID, run.Commit, string(run.Status), formatDBTime(run.StartedAt), finishedAt, formatDBTime(RunActivityAt(run)), string(raw)); err != nil {
		return fmt.Errorf("write run history: %w", err)
	}
	return nil
}

func nullableTime(value time.Time) any {
	if value.IsZero() {
		return nil
	}
	return formatDBTime(value)
}

func formatDBTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}
