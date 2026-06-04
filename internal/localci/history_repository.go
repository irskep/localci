package localci

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
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

type RunPage struct {
	Runs        []RunRecord
	NextBefore  string
	NewerBefore string
}

func (r RunRepository) ListRepoSummaries() ([]RepoHistory, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(context.Background(), `
		select repo_dir, max(repo_id), max(activity_at) as latest_activity
		from runs
		group by repo_dir
		order by latest_activity desc, repo_dir asc
	`)
	if err != nil {
		return nil, fmt.Errorf("list repo summaries: %w", err)
	}
	defer rows.Close()

	repos := []RepoHistory{}
	for rows.Next() {
		var repo RepoHistory
		var latestActivity string
		if err := rows.Scan(&repo.RepoDir, &repo.RepoID, &latestActivity); err != nil {
			return nil, fmt.Errorf("scan repo summary: %w", err)
		}
		repos = append(repos, repo)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list repo summaries: %w", err)
	}
	return repos, nil
}

func (r RunRepository) ListRecentRunPage(before time.Time, limit int) (RunPage, error) {
	return r.listRunPage("", before, limit)
}

func (r RunRepository) ListRepoRunPage(repoDir string, before time.Time, limit int) (RunPage, error) {
	return r.listRunPage(repoDir, before, limit)
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

func (r RunRepository) listRunPage(repoDir string, before time.Time, limit int) (RunPage, error) {
	db, err := r.open()
	if err != nil {
		return RunPage{}, err
	}
	defer db.Close()

	if limit <= 0 {
		limit = 20
	}
	runs, err := queryRunPage(context.Background(), db, repoDir, before, limit+1)
	if err != nil {
		return RunPage{}, err
	}
	if repoDir != "" && len(runs) == 0 && before.IsZero() {
		return RunPage{}, ErrRecordNotFound
	}

	page := RunPage{Runs: runs}
	if len(page.Runs) > limit {
		page.NextBefore = formatDBTime(RunActivityAt(page.Runs[limit-1]))
		page.Runs = page.Runs[:limit]
	}
	if !before.IsZero() {
		newerBefore, err := queryNewerBefore(context.Background(), db, repoDir, before, limit)
		if err != nil {
			return RunPage{}, err
		}
		page.NewerBefore = newerBefore
	}

	return page, nil
}

func queryRunPage(ctx context.Context, db *sql.DB, repoDir string, before time.Time, limit int) ([]RunRecord, error) {
	args := []any{}
	where := ""
	if repoDir != "" {
		where = "where repo_dir = ?"
		args = append(args, repoDir)
	}
	if !before.IsZero() {
		if where == "" {
			where = "where activity_at < ?"
		} else {
			where += " and activity_at < ?"
		}
		args = append(args, formatDBTime(before))
	}
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, fmt.Sprintf(`
		select repo_dir, run_json
		from runs
		%s
		order by activity_at desc, repo_dir asc, commit_ref desc
		limit ?
	`, where), args...)
	if err != nil {
		return nil, fmt.Errorf("query run page: %w", err)
	}
	defer rows.Close()
	return scanRunRows(rows)
}

func queryNewerBefore(ctx context.Context, db *sql.DB, repoDir string, before time.Time, limit int) (string, error) {
	args := []any{}
	where := "where activity_at >= ?"
	args = append(args, formatDBTime(before))
	if repoDir != "" {
		where += " and repo_dir = ?"
		args = append(args, repoDir)
	}

	var newerCount int
	if err := db.QueryRowContext(ctx, fmt.Sprintf(`select count(*) from runs %s`, where), args...).Scan(&newerCount); err != nil {
		return "", fmt.Errorf("count newer runs: %w", err)
	}
	if newerCount <= limit {
		return "", nil
	}

	offset := newerCount - limit - 1
	args = []any{}
	where = ""
	if repoDir != "" {
		where = "where repo_dir = ?"
		args = append(args, repoDir)
	}
	args = append(args, 1, offset)

	var activityAt string
	err := db.QueryRowContext(ctx, fmt.Sprintf(`
		select activity_at
		from runs
		%s
		order by activity_at desc, repo_dir asc, commit_ref desc
		limit ? offset ?
	`, where), args...).Scan(&activityAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("query newer cursor: %w", err)
	}
	return activityAt, nil
}

func (r RunRepository) ListRuns() ([]RunRecord, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(context.Background(), `
		select repo_dir, run_json
		from runs
		order by activity_at desc, repo_dir asc, commit_ref desc
	`)
	if err != nil {
		return nil, fmt.Errorf("list runs: %w", err)
	}
	defer rows.Close()
	return scanRunRows(rows)
}

func (r RunRepository) ListRepoCommits(repoDir string) ([]RunRecord, error) {
	db, err := r.open()
	if err != nil {
		return nil, err
	}
	defer db.Close()

	rows, err := db.QueryContext(context.Background(), `
		select repo_dir, run_json
		from runs
		where repo_dir = ?
		order by activity_at desc, commit_ref desc
	`, repoDir)
	if err != nil {
		return nil, fmt.Errorf("list repo runs: %w", err)
	}
	defer rows.Close()
	runs, err := scanRunRows(rows)
	if err != nil {
		return nil, err
	}
	if len(runs) == 0 {
		return nil, ErrRecordNotFound
	}
	return runs, nil
}

func scanRunRows(rows *sql.Rows) ([]RunRecord, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("scan run columns: %w", err)
	}
	runs := []RunRecord{}
	for rows.Next() {
		var repoDir string
		var raw string
		switch len(columns) {
		case 1:
			if err := rows.Scan(&raw); err != nil {
				return nil, fmt.Errorf("scan run: %w", err)
			}
		default:
			if err := rows.Scan(&repoDir, &raw); err != nil {
				return nil, fmt.Errorf("scan run: %w", err)
			}
		}
		var run RunRecord
		if err := json.Unmarshal([]byte(raw), &run); err != nil {
			return nil, fmt.Errorf("decode run history: %w", err)
		}
		if strings.TrimSpace(run.RepoDir) == "" {
			run.RepoDir = repoDir
		}
		runs = append(runs, run)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan runs: %w", err)
	}
	return runs, nil
}

func (r RunRepository) ReadRun(repoDir string, commit string) (RunRecord, error) {
	db, err := r.open()
	if err != nil {
		return RunRecord{}, err
	}
	defer db.Close()

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
	if strings.TrimSpace(r.Paths.configRoot()) == "" {
		return nil, fmt.Errorf("history config root is required")
	}
	if err := os.MkdirAll(r.Paths.configRoot(), 0o755); err != nil {
		return nil, fmt.Errorf("create history config root: %w", err)
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
		create index if not exists runs_activity_order_idx on runs (activity_at desc, repo_dir asc, commit_ref desc);
		create index if not exists runs_repo_activity_order_idx on runs (repo_dir, activity_at desc, commit_ref desc);
	`); err != nil {
		return fmt.Errorf("initialize history database: %w", err)
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
