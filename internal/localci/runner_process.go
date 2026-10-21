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

// Each task gets its own process group, including Mise and the commands it runs.
// WaitDelay bounds pipe draining if a child outlives the main process.
func configureTaskProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = 100 * time.Millisecond
	if cmd.Cancel != nil {
		cmd.Cancel = func() error { return signalTaskGroup(cmd, syscall.SIGKILL) }
	}
}

func signalTaskGroup(cmd *exec.Cmd, signal syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}
	err := syscall.Kill(-cmd.Process.Pid, signal)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

func (r Runner) watchTask(ctx context.Context, cmd *exec.Cmd, outputDir string, waitResult <-chan error) error {
	defer signalTaskGroup(cmd, syscall.SIGKILL) // Clean up background children even on success.
	pollTicker := time.NewTicker(r.pollInterval())
	defer pollTicker.Stop()
	lastActivity := r.now()
	timeout := r.inactivityTimeout()
	for {
		select {
		case err := <-waitResult:
			if err == nil || (errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.Success()) {
				return nil
			}
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				return taskExitError{ExitCode: exitErr.ExitCode()}
			}
			return err
		case <-ctx.Done():
			return r.stopTaskGroup(cmd, waitResult, ctx.Err())
		case <-pollTicker.C:
			canceled, err := taskCancelRequested(outputDir)
			if err != nil {
				return r.stopTaskGroup(cmd, waitResult, fmt.Errorf("check cancellation marker: %w", err))
			}
			if canceled {
				return r.stopCanceledTask(cmd, waitResult)
			}
			latest, err := latestOutputActivity(outputDir)
			if err != nil {
				return r.stopTaskGroup(cmd, waitResult, fmt.Errorf("scan output activity: %w", err))
			}
			if latest.After(lastActivity) {
				lastActivity = latest
				continue
			}
			if r.now().Sub(lastActivity) >= timeout {
				return r.stopTimedOutTask(cmd, waitResult, timeout)
			}
		}
	}
}

func (r Runner) stopTaskGroup(cmd *exec.Cmd, waitResult <-chan error, cause error) error {
	if err := signalTaskGroup(cmd, syscall.SIGTERM); err != nil {
		_ = signalTaskGroup(cmd, syscall.SIGKILL)
		<-waitResult
		return fmt.Errorf("terminate task process group: %w", err)
	}
	timer := time.NewTimer(r.terminateGrace())
	defer timer.Stop()
	select {
	case <-waitResult:
		// The leader may have exited while children ignore SIGTERM.
		_ = signalTaskGroup(cmd, syscall.SIGKILL)
	case <-timer.C:
		_ = signalTaskGroup(cmd, syscall.SIGKILL)
		<-waitResult
	}
	return cause
}

func (r Runner) stopTimedOutTask(cmd *exec.Cmd, waitResult <-chan error, timeout time.Duration) error {
	return r.stopTaskGroup(cmd, waitResult, taskTimedOutError{After: timeout})
}

func (r Runner) stopCanceledTask(cmd *exec.Cmd, waitResult <-chan error) error {
	return r.stopTaskGroup(cmd, waitResult, taskCanceledError{})
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
