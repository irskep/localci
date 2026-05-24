package localci

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

func (r Runner) watchTask(ctx context.Context, cmd *exec.Cmd, outputDir string, waitResult <-chan error) error {
	pollTicker := time.NewTicker(r.pollInterval())
	defer pollTicker.Stop()

	lastActivity := r.now()
	timeout := r.inactivityTimeout()

	for {
		select {
		case err := <-waitResult:
			if err == nil {
				return nil
			}

			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return taskExitError{ExitCode: exitErr.ExitCode()}
			}

			return err
		case <-ctx.Done():
			if cmd.Process != nil {
				_ = cmd.Process.Kill()
			}
			<-waitResult
			return ctx.Err()
		case <-pollTicker.C:
			canceled, err := taskCancelRequested(outputDir)
			if err != nil {
				return fmt.Errorf("check cancellation marker: %w", err)
			}
			if canceled {
				return r.stopCanceledTask(cmd, waitResult)
			}

			latest, err := latestOutputActivity(outputDir)
			if err != nil {
				return fmt.Errorf("scan output activity: %w", err)
			}
			if latest.After(lastActivity) {
				lastActivity = latest
				continue
			}
			if r.now().Sub(lastActivity) < timeout {
				continue
			}
			return r.stopTimedOutTask(cmd, waitResult, timeout)
		}
	}
}

func (r Runner) stopTimedOutTask(cmd *exec.Cmd, waitResult <-chan error, timeout time.Duration) error {
	if cmd.Process == nil {
		return taskTimedOutError{After: timeout}
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate timed out task: %w", err)
	}

	timer := time.NewTimer(r.terminateGrace())
	defer timer.Stop()

	select {
	case <-timer.C:
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill timed out task: %w", err)
		}
		<-waitResult
		return taskTimedOutError{After: timeout}
	case <-waitResult:
		return taskTimedOutError{After: timeout}
	}
}

func (r Runner) stopCanceledTask(cmd *exec.Cmd, waitResult <-chan error) error {
	if cmd.Process == nil {
		return taskCanceledError{}
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate canceled task: %w", err)
	}

	timer := time.NewTimer(r.terminateGrace())
	defer timer.Stop()

	select {
	case <-timer.C:
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill canceled task: %w", err)
		}
		<-waitResult
		return taskCanceledError{}
	case <-waitResult:
		return taskCanceledError{}
	}
}

func taskCancelRequested(outputDir string) (bool, error) {
	_, err := os.Stat(filepath.Join(outputDir, cancelMarkerName))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, err
}

func latestOutputActivity(dir string) (time.Time, error) {
	var latest time.Time

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(latest) {
			latest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return time.Time{}, err
	}

	return latest, nil
}
