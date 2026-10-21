package localci

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maxTextArtifactBytes = 2 << 20

var ErrArtifactNotDisplayable = errors.New("artifact is not a displayable text file")

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
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	FinishedAt  *time.Time        `json:"finished_at,omitempty"`
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
	MarkedArtifacts      []MarkedArtifact  `json:"marked_artifacts,omitempty"`
}

type ArtifactView struct {
	Name        string         `json:"name"`
	DisplayName string         `json:"display_name"`
	Path        string         `json:"path"`
	MarkedName  string         `json:"marked_name,omitempty"`
	Action      ArtifactAction `json:"action,omitempty"`
	IsText      bool           `json:"is_text,omitempty"`
	RawURL      string         `json:"raw_url,omitempty"`
	DownloadURL string         `json:"download_url,omitempty"`
}

type TaskAttemptView struct {
	Attempt              int             `json:"attempt"`
	Status               ExecutionStatus `json:"status"`
	DurationMilliseconds int64           `json:"duration_ms"`
	Failure              string          `json:"failure"`
}

func BuildCommitStatusView(paths Paths, repoDir string, commit string, discovered []Task, queued []QueueEntry, active *ActiveTask) (CommitStatusView, error) {
	return buildCommitStatusView(paths, repoDir, commit, discovered, queued, active, "", true, RunRepository{Paths: paths})
}

func buildCommitStatusView(paths Paths, repoDir, commit string, discovered []Task, queued []QueueEntry, active *ActiveTask, taskName string, includeArtifacts bool, repository RunRepository) (CommitStatusView, error) {
	view := CommitStatusView{
		RepoDir: repoDir,
		Commit:  commit,
		Tasks:   make([]TaskStatusView, 0, len(discovered)),
	}

	run, err := repository.ReadRun(repoDir, commit)
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return CommitStatusView{}, err
	}
	if err == nil {
		view.Annotations = cloneAnnotations(run.Annotations)
		if len(discovered) == 0 {
			discovered = run.DiscoveredTasks
		}
	}

	queuedByTask := queuedEntriesByTask(queued, repoDir, commit, discovered)
	timingRecords := []TaskRecord{}

	for _, task := range discovered {
		if taskName != "" && task.Name != taskName {
			continue
		}
		status := ExecutionStatusNotRun
		outputDir := paths.TaskOutputDir(repoDir, commit, task.Name)
		attempt := 0
		records, err := listTaskAttemptRecords(paths, repoDir, commit, task.Name)
		if err != nil {
			return CommitStatusView{}, err
		}
		attempts := make([]TaskAttemptView, 0, len(records))
		for _, r := range records {
			attempts = append(attempts, TaskAttemptView{Attempt: r.Attempt, Status: executionStatusFromTaskRecord(r), DurationMilliseconds: r.DurationMilliseconds, Failure: r.Failure})
		}
		var record TaskRecord
		hasRecord := len(records) > 0
		if hasRecord {
			record = records[0]
		} else {
			for _, r := range run.TaskResults {
				if r.Name == task.Name {
					record = r
					hasRecord = true
					break
				}
			}
		}
		attempts = mergeLiveAttemptViews(attempts, queuedByTask[task.Name], active, repoDir, commit, task.Name)

		var artifacts []ArtifactView
		var durationMilliseconds int64
		var failure string
		var markedArtifacts []MarkedArtifact
		if hasRecord {
			outputDir = record.OutputDir
			attempt = record.Attempt
			status = executionStatusFromTaskRecord(record)
			markedArtifacts = append([]MarkedArtifact{}, record.MarkedArtifacts...)
			durationMilliseconds = record.DurationMilliseconds
			failure = record.Failure
		}
		if active != nil && active.RepoDir == repoDir && active.Commit == commit && active.TaskName == task.Name {
			status = ExecutionStatusRunning
			if attempt != active.Attempt {
				markedArtifacts = nil
			}
			attempt = active.Attempt
			outputDir = paths.TaskAttemptDir(repoDir, commit, task.Name, active.Attempt)
			durationMilliseconds = 0
			failure = ""
		} else if queuedEntries := queuedByTask[task.Name]; len(queuedEntries) > 0 {
			status = ExecutionStatusQueued
			if attempt != latestQueuedAttempt(queuedEntries) {
				markedArtifacts = nil
			}
			attempt = latestQueuedAttempt(queuedEntries)
			outputDir = paths.TaskAttemptDir(repoDir, commit, task.Name, attempt)
			durationMilliseconds = 0
			failure = ""
		}
		if hasRecord && record.Attempt == attempt {
			timingRecords = append(timingRecords, record)
		}

		outputFiles := []string{}
		artifacts = []ArtifactView{}
		if includeArtifacts {
			outputFiles, err = listOutputFiles(outputDir)
			if err != nil {
				return CommitStatusView{}, err
			}
			artifacts = buildArtifactViews(outputDir, outputFiles, markedArtifacts)
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
			MarkedArtifacts:      markedArtifacts,
		})
	}

	sort.Slice(view.Tasks, func(i int, j int) bool {
		return view.Tasks[i].Name < view.Tasks[j].Name
	})
	view.StartedAt, view.FinishedAt = commitTiming(timingRecords, view.Tasks, active, repoDir, commit)

	return view, nil
}

func commitTiming(records []TaskRecord, tasks []TaskStatusView, active *ActiveTask, repoDir string, commit string) (*time.Time, *time.Time) {
	var startedAt time.Time
	var finishedAt time.Time
	hasActiveRecord := false
	for _, record := range records {
		if !record.StartedAt.IsZero() && (startedAt.IsZero() || record.StartedAt.Before(startedAt)) {
			startedAt = record.StartedAt
		}
		if record.FinishedAt.After(finishedAt) {
			finishedAt = record.FinishedAt
		}
		if active != nil && record.Name == active.TaskName && record.Attempt == active.Attempt {
			hasActiveRecord = true
		}
	}
	if active != nil && !hasActiveRecord && activeTaskIsRunning(tasks, active) && active.RepoDir == repoDir && active.Commit == commit &&
		!active.StartedAt.IsZero() && (startedAt.IsZero() || active.StartedAt.Before(startedAt)) {
		startedAt = active.StartedAt
	}
	if startedAt.IsZero() {
		return nil, nil
	}

	for _, task := range tasks {
		if task.Status == ExecutionStatusQueued || task.Status == ExecutionStatusRunning {
			return timePointer(startedAt), nil
		}
	}
	if finishedAt.IsZero() {
		return timePointer(startedAt), nil
	}
	return timePointer(startedAt), timePointer(finishedAt)
}

func activeTaskIsRunning(tasks []TaskStatusView, active *ActiveTask) bool {
	for _, task := range tasks {
		if task.Name == active.TaskName && task.Attempt == active.Attempt && task.Status == ExecutionStatusRunning {
			return true
		}
	}
	return false
}

func timePointer(value time.Time) *time.Time {
	return &value
}

func queuedEntriesByTask(queued []QueueEntry, repoDir string, commit string, discovered []Task) map[string][]QueueEntry {
	byTask := map[string][]QueueEntry{}
	for _, entry := range queued {
		if entry.RepoDir != repoDir || entry.Commit != commit {
			continue
		}
		for _, taskName := range queueEntryTaskNames(entry, discovered) {
			taskEntry := entry
			taskEntry.TaskName = taskName
			taskEntry.TaskKey = sanitizeTaskName(taskName)
			byTask[taskName] = append(byTask[taskName], taskEntry)
		}
	}
	return byTask
}

func queueEntryTaskNames(entry QueueEntry, discovered []Task) []string {
	switch entry.Kind {
	case QueueEntryKindTask:
		if entry.TaskName == "" {
			return nil
		}
		return []string{entry.TaskName}
	case QueueEntryKindRun:
		if len(entry.RequestedTasks) > 0 {
			return append([]string{}, entry.RequestedTasks...)
		}
		names := make([]string, 0, len(discovered))
		for _, task := range discovered {
			names = append(names, task.Name)
		}
		return names
	default:
		return nil
	}
}

func mergeLiveAttemptViews(existing []TaskAttemptView, queued []QueueEntry, active *ActiveTask, repoDir string, commit string, taskName string) []TaskAttemptView {
	views := append([]TaskAttemptView{}, existing...)
	if active != nil && active.RepoDir == repoDir && active.Commit == commit && active.TaskName == taskName && active.Attempt > 0 {
		views = upsertAttemptView(views, TaskAttemptView{
			Attempt: active.Attempt,
			Status:  ExecutionStatusRunning,
		})
	}
	for _, entry := range queued {
		if entry.Attempt <= 0 {
			continue
		}
		views = upsertAttemptView(views, TaskAttemptView{
			Attempt: entry.Attempt,
			Status:  ExecutionStatusQueued,
		})
	}
	sort.Slice(views, func(i int, j int) bool {
		return views[i].Attempt > views[j].Attempt
	})
	return views
}

func upsertAttemptView(views []TaskAttemptView, next TaskAttemptView) []TaskAttemptView {
	for i, existing := range views {
		if existing.Attempt == next.Attempt {
			views[i] = next
			return views
		}
	}
	return append(views, next)
}

func latestQueuedAttempt(entries []QueueEntry) int {
	latest := 0
	for _, entry := range entries {
		if entry.Attempt > latest {
			latest = entry.Attempt
		}
	}
	return latest
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

	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return records, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		attempt, err := strconv.Atoi(strings.TrimPrefix(entry.Name(), "attempt-"))
		if err != nil || attempt <= 0 {
			continue
		}
		record, err := readTaskRecord(filepath.Join(root, entry.Name(), taskRecordFileName))
		if errors.Is(err, ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	sort.Slice(records, func(i int, j int) bool {
		return records[i].Attempt > records[j].Attempt
	})
	return records, nil
}

func buildArtifactViews(outputDir string, files []string, markedArtifacts []MarkedArtifact) []ArtifactView {
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
	artifacts = applyMarkedArtifactMetadata(artifacts, markedArtifacts)
	sort.Slice(artifacts, func(i int, j int) bool {
		leftMarked := markedArtifactIndex(artifacts[i], markedArtifacts)
		rightMarked := markedArtifactIndex(artifacts[j], markedArtifacts)
		if leftMarked >= 0 || rightMarked >= 0 {
			if leftMarked < 0 {
				return false
			}
			if rightMarked < 0 {
				return true
			}
			return leftMarked < rightMarked
		}
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
	return filepath.ToSlash(relative)
}

func outputFilesOrNil(outputDir string) []string {
	files, err := listOutputFiles(outputDir)
	if err != nil {
		return nil
	}
	return files
}

func applyMarkedArtifactMetadata(artifacts []ArtifactView, markedArtifacts []MarkedArtifact) []ArtifactView {
	markedByPath := map[string]MarkedArtifact{}
	for _, marked := range markedArtifacts {
		markedByPath[marked.Path] = marked
	}
	for index := range artifacts {
		if marked, ok := markedByPath[artifacts[index].DisplayName]; ok {
			artifacts[index].MarkedName = marked.Name
			artifacts[index].Action = marked.Action
		}
	}
	return artifacts
}

func markedArtifactIndex(artifact ArtifactView, markedArtifacts []MarkedArtifact) int {
	for index, marked := range markedArtifacts {
		if marked.Path == artifact.DisplayName {
			return index
		}
	}
	return -1
}

func artifactSortKey(name string) int {
	switch strings.ToLower(name) {
	case "combined.log":
		return 0
	case taskRecordFileName:
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
	return strings.EqualFold(displayName, taskRecordFileName) ||
		strings.EqualFold(displayName, cancelMarkerName) ||
		strings.EqualFold(displayName, taskArtifactsManifestName)
}

func LoadPrimaryLog(task TaskStatusView) (string, string) {
	artifact, ok := PrimaryArtifact(task)
	if !ok {
		return "", ""
	}
	data, err := readTaskArtifact(task, artifact.DisplayName)
	if err != nil {
		return artifact.DisplayName, ""
	}
	return artifact.DisplayName, string(data)
}

func readTaskArtifact(task TaskStatusView, displayName string) ([]byte, error) {
	path, err := resolveArtifactPath(task.OutputDir, displayName)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func openTextTaskArtifact(task TaskStatusView, displayName string) (*os.File, error) {
	path, err := resolveArtifactPath(task.OutputDir, displayName)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	if err := classifyTextFile(file); err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func classifyTextFile(file *os.File) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > maxTextArtifactBytes {
		return ErrArtifactNotDisplayable
	}
	sample := make([]byte, 512)
	n, err := file.ReadAt(sample, 0)
	if err != nil && err != io.EOF {
		return err
	}
	if !isDisplayableText(sample[:n]) {
		return ErrArtifactNotDisplayable
	}
	return nil
}

func classifyTextTaskArtifact(task TaskStatusView, displayName string) error {
	file, err := openTextTaskArtifact(task, displayName)
	if err != nil {
		return err
	}
	return file.Close()
}

func readTextTaskArtifact(task TaskStatusView, displayName string) ([]byte, error) {
	file, err := openTextTaskArtifact(task, displayName)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxTextArtifactBytes+1))
	if len(data) > maxTextArtifactBytes {
		return nil, ErrArtifactNotDisplayable
	}
	if task.Status == ExecutionStatusRunning {
		data = data[:completeUTF8Prefix(data)]
	}
	return data, err
}

func isDisplayableText(sample []byte) bool {
	if len(sample) == 0 {
		return true
	}
	if bytes.IndexByte(sample, 0) >= 0 {
		return false
	}
	contentType := http.DetectContentType(sample)
	return strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" ||
		contentType == "application/xml"
}

func resolveArtifactPath(outputDir string, displayName string) (string, error) {
	if outputDir == "" {
		return "", errors.New("task output dir is required")
	}
	if filepath.IsAbs(displayName) {
		return "", errors.New("artifact path must be relative")
	}

	cleanName := filepath.Clean(displayName)
	if cleanName == "." || cleanName == ".." || strings.HasPrefix(cleanName, ".."+string(filepath.Separator)) {
		return "", errors.New("artifact path is outside task output")
	}

	outputDir = filepath.Clean(outputDir)
	candidate := filepath.Join(outputDir, cleanName)
	if err := verifyPathUnderDir(outputDir, candidate); err != nil {
		return "", err
	}
	return candidate, nil
}

func verifyPathUnderDir(dir string, candidate string) error {
	dir = filepath.Clean(dir)
	candidate = filepath.Clean(candidate)
	realOutputDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return err
	}
	realCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(realOutputDir, realCandidate)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errors.New("artifact path is outside task output")
	}
	return nil
}

// Primary logs use a bounded prefix in API snapshots. Downloads and `cat` keep
// access to the complete file. Exclude unfinished UTF-8 at the live write edge.
func loadPrimaryLogPreview(task TaskStatusView) (name, text string, end int64, truncated bool) {
	artifact, ok := PrimaryArtifact(task)
	if !ok {
		return
	}
	name = artifact.DisplayName
	path, err := resolveArtifactPath(task.OutputDir, name)
	if err != nil {
		return
	}
	file, err := os.Open(path)
	if err != nil {
		return
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return
	}
	data, err := io.ReadAll(io.LimitReader(file, maxTextArtifactBytes+1))
	if err != nil {
		return
	}
	truncated = len(data) > maxTextArtifactBytes
	if truncated {
		data = data[:maxTextArtifactBytes]
	}
	if truncated || task.Status == ExecutionStatusRunning {
		data = data[:completeUTF8Prefix(data)]
	}
	return name, string(data), int64(len(data)), truncated
}
