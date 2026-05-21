package localci

import (
	"context"
	"fmt"
	"time"
)

type Waiter struct {
	Client       DaemonClient
	PollInterval time.Duration
}

func (w Waiter) Wait(ctx context.Context, repoDir string, commit string) (CommitStatusView, error) {
	interval := w.PollInterval
	if interval <= 0 {
		interval = time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		view, err := w.Client.Status(ctx, repoDir, commit)
		if err != nil {
			return CommitStatusView{}, err
		}
		if CommitComplete(view) {
			return view, nil
		}

		select {
		case <-ctx.Done():
			return CommitStatusView{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func CommitComplete(view CommitStatusView) bool {
	for _, task := range view.Tasks {
		switch task.Status {
		case ExecutionStatusQueued, ExecutionStatusRunning:
			return false
		}
	}
	return true
}

func CommitFailed(view CommitStatusView) bool {
	for _, task := range view.Tasks {
		switch task.Status {
		case ExecutionStatusFailed, ExecutionStatusTimedOut:
			return true
		}
	}
	return false
}

func FailedTasks(view CommitStatusView) []TaskStatusView {
	failed := []TaskStatusView{}
	for _, task := range view.Tasks {
		switch task.Status {
		case ExecutionStatusFailed, ExecutionStatusTimedOut:
			failed = append(failed, task)
		}
	}
	return failed
}

func SummarizeCommit(view CommitStatusView) string {
	succeeded := 0
	failed := 0
	timedOut := 0
	notRun := 0
	for _, task := range view.Tasks {
		switch task.Status {
		case ExecutionStatusSucceeded:
			succeeded++
		case ExecutionStatusFailed:
			failed++
		case ExecutionStatusTimedOut:
			timedOut++
		case ExecutionStatusNotRun:
			notRun++
		}
	}
	if failed == 0 && timedOut == 0 && notRun == 0 {
		return fmt.Sprintf("%d tasks passed", succeeded)
	}
	return fmt.Sprintf("%d passed, %d failed, %d timed out, %d not run", succeeded, failed, timedOut, notRun)
}
