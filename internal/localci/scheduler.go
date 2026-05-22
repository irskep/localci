package localci

import (
	"context"
	"errors"
	"time"
)

type Scheduler struct {
	Queue  QueueStore
	Runner Runner
	Events *EventNotifier
}

type RunNextResult struct {
	DidWork bool       `json:"did_work"`
	Entry   QueueEntry `json:"entry"`
	Task    TaskRecord `json:"task"`
	Run     RunRecord  `json:"run"`
}

func (s Scheduler) RunNext(ctx context.Context) (RunNextResult, error) {
	entry, claimed, err := s.Queue.ClaimNext()
	if err != nil {
		return RunNextResult{}, err
	}
	if !claimed && entry.TaskName != "" {
		return RunNextResult{
			DidWork: false,
			Entry:   entry,
		}, nil
	}
	if !claimed {
		return RunNextResult{}, nil
	}

	defer func() {
		_ = s.Queue.ClearActive()
		if s.Events != nil {
			s.Events.EntryChanged(entry)
		}
	}()
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}

	if s.Events != nil {
		s.Events.QueueChanged()
	}

	taskRecord, runErr := s.Runner.runTask(ctx, InvokeRequest{
		RepoDir: entry.RepoDir,
		Commit:  entry.Commit,
	}, Task{Name: entry.TaskName}, entry.Attempt)

	runRecord, writeErr := upsertTaskRecord(s.Runner.Paths, InvokeRequest{
		RepoDir: entry.RepoDir,
		Commit:  entry.Commit,
	}, taskRecord, s.Runner.now())
	if writeErr != nil {
		return RunNextResult{}, writeErr
	}
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}

	result := RunNextResult{
		DidWork: true,
		Entry:   entry,
		Task:    taskRecord,
		Run:     runRecord,
	}

	if runErr != nil {
		return result, runErr
	}

	return result, nil
}

func upsertTaskRecord(paths Paths, req InvokeRequest, record TaskRecord, now time.Time) (RunRecord, error) {
	run, err := readRunRecord(paths, req)
	if err != nil {
		if !errors.Is(err, ErrRecordNotFound) {
			return RunRecord{}, err
		}
		run = newRunRecord(req, record.StartedAt)
	}
	if len(req.Annotations) > 0 {
		if run.Annotations == nil {
			run.Annotations = map[string]string{}
		}
		for key, value := range req.Annotations {
			run.Annotations[key] = value
		}
	}

	replaced := false
	for i, existing := range run.TaskResults {
		if existing.Name == record.Name {
			run.TaskResults[i] = record
			replaced = true
			break
		}
	}
	if !replaced {
		run.TaskResults = append(run.TaskResults, record)
	}

	run.FinishedAt = now
	run.RefreshSummary()
	if err := writeRunRecord(paths, req, run); err != nil {
		return RunRecord{}, err
	}
	return run, nil
}
