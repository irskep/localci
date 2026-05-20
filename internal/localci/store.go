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
	return writeJSONFile(path, record)
}

func writeTaskRecord(record TaskRecord) error {
	path := filepath.Join(record.OutputDir, "task.json")
	return writeJSONFile(path, record)
}

var ErrRecordNotFound = errors.New("record not found")

func readRunRecord(paths Paths, req InvokeRequest) (RunRecord, error) {
	var record RunRecord
	if err := readJSONFile(paths.RunRecordPath(req.RepoDir, req.Commit), &record); err != nil {
		return RunRecord{}, err
	}
	return record, nil
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

	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmpPath, err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
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
