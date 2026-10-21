package localci

import (
	"fmt"
	"time"
)

func (s QueueStore) SetPhase(entry QueueEntry, phase, taskName string, attempt int) error {
	queueMutationMu.Lock()
	defer queueMutationMu.Unlock()
	active, err := s.readActiveLocked()
	if err != nil {
		return err
	}
	if active.RepoDir != entry.RepoDir || active.Commit != entry.Commit || !active.EnqueuedAt.Equal(entry.EnqueuedAt) {
		return fmt.Errorf("active entry changed")
	}
	active.Phase = phase
	active.PhaseStartedAt = s.now()
	active.CurrentTask = taskName
	active.CurrentAttempt = attempt
	return writeJSONFile(s.Paths.ActiveTaskPath(), active)
}

func PhaseLabel(phase string) string {
	switch phase {
	case "queued":
		return "Queued"
	case "preparing":
		return "Preparing checkout"
	case "trusting":
		return "Trusting Mise configuration"
	case "discovering":
		return "Discovering tasks"
	case "setup":
		return "Running setup"
	case "running":
		return "Running task"
	default:
		return phase
	}
}

func (s Scheduler) setPhase(entry QueueEntry, phase string) error {
	if err := s.Queue.SetPhase(entry, phase, "", 0); err != nil {
		return err
	}
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}
	return nil
}

func taskPhase(task TaskStatusView, active *ActiveTask, queued []QueueEntry, repoDir, commit string) (string, *time.Time) {
	if active != nil && active.RepoDir == repoDir && active.Commit == commit &&
		(active.CurrentTask == task.Name || active.TaskName == task.Name) && active.Phase != "" {
		return active.Phase, timePointer(active.PhaseStartedAt)
	}
	if task.Status == ExecutionStatusQueued && len(queued) > 0 {
		return "queued", timePointer(queued[len(queued)-1].EnqueuedAt)
	}
	return "", nil
}
