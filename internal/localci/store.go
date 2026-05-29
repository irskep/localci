package localci

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func writeRunRecord(paths Paths, req InvokeRequest, record RunRecord) error {
	path := paths.RunRecordPath(req.RepoDir, req.Commit)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create run record directory: %w", err)
	}
	if err := writeJSONFile(path, record); err != nil {
		return err
	}
	return (RunRepository{Paths: paths}).WriteRun(record)
}

func writeTaskRecord(record TaskRecord) error {
	path := filepath.Join(record.OutputDir, "task.json")
	return writeJSONFile(path, record)
}

var ErrRecordNotFound = errors.New("record not found")

func readRunRecord(paths Paths, req InvokeRequest) (RunRecord, error) {
	record, err := (RunRepository{Paths: paths}).ReadRun(req.RepoDir, req.Commit)
	if err == nil {
		return record, nil
	}
	if !errors.Is(err, ErrRecordNotFound) {
		return RunRecord{}, err
	}
	var fallback RunRecord
	if err := readJSONFile(paths.RunRecordPath(req.RepoDir, req.Commit), &fallback); err != nil {
		return RunRecord{}, err
	}
	return fallback, nil
}

func readTaskRecord(path string) (TaskRecord, error) {
	var record TaskRecord
	if err := readJSONFile(path, &record); err != nil {
		return TaskRecord{}, err
	}
	return record, nil
}

func writeJSONFile(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}
	data = append(data, '\n')

	tmpFile, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file for %s: %w", path, err)
	}
	tmpPath := tmpFile.Name()
	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("close %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename %s to %s: %w", tmpPath, path, err)
	}

	return nil
}

func readJSONFile(path string, value any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("%w: %s", ErrRecordNotFound, path)
		}
		return fmt.Errorf("read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, value); err != nil {
		return fmt.Errorf("unmarshal %s: %w", path, err)
	}
	return nil
}
