package localci

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

type DaemonState struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
}

type DaemonManager struct {
	Paths          Paths
	ExecutablePath string
	Env            []string
	Now            func() time.Time
	Scheduler      Scheduler
	PollInterval   time.Duration
}

type StartResult struct {
	Started bool        `json:"started"`
	State   DaemonState `json:"state"`
}

func (m DaemonManager) Start(ctx context.Context) (StartResult, error) {
	state, alive, err := m.readAliveState()
	if err != nil {
		return StartResult{}, err
	}
	if alive {
		return StartResult{Started: false, State: state}, nil
	}

	if err := os.MkdirAll(m.Paths.DaemonRoot(), 0o755); err != nil {
		return StartResult{}, err
	}

	logFile, err := os.OpenFile(m.Paths.DaemonLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return StartResult{}, err
	}
	defer logFile.Close()

	cmd := exec.CommandContext(ctx, m.executablePath(), "daemon", "run")
	cmd.Env = m.env()
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := cmd.Start(); err != nil {
		return StartResult{}, err
	}
	_ = cmd.Process.Release()

	state, err = m.waitForReady(ctx)
	if err != nil {
		return StartResult{}, err
	}

	return StartResult{
		Started: true,
		State:   state,
	}, nil
}

func (m DaemonManager) Stop() error {
	state, alive, err := m.readAliveState()
	if err != nil {
		return err
	}
	if !alive {
		if removeErr := os.Remove(m.Paths.DaemonStatePath()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return removeErr
		}
		return nil
	}

	client := DaemonClient{Paths: m.Paths}
	if err := client.Shutdown(context.Background()); err != nil {
		process, findErr := os.FindProcess(state.PID)
		if findErr != nil {
			return findErr
		}
		if signalErr := process.Signal(syscall.SIGTERM); signalErr != nil && !errors.Is(signalErr, os.ErrProcessDone) {
			return signalErr
		}
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		alive, err := processAlive(state.PID)
		if err != nil {
			return err
		}
		if !alive {
			if removeErr := os.Remove(m.Paths.DaemonStatePath()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return removeErr
			}
			_ = os.Remove(m.Paths.DaemonSocketPath())
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timed out waiting for daemon pid %d to stop", state.PID)
}

func (m DaemonManager) Run(ctx context.Context) error {
	if err := os.MkdirAll(m.Paths.DaemonRoot(), 0o755); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	state := DaemonState{
		PID:       os.Getpid(),
		StartedAt: m.now(),
	}
	if err := writeJSONFile(m.Paths.DaemonStatePath(), state); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(m.Paths.DaemonStatePath())
		_ = os.Remove(m.Paths.DaemonSocketPath())
	}()

	server := &DaemonServer{
		Paths:         m.Paths,
		Queue:         m.Scheduler.Queue,
		ReadState:     m.ReadState,
		DiscoverTasks: m.Scheduler.Runner.DiscoverTasks,
		Shutdown:      cancel,
	}

	serverErrs := make(chan error, 1)
	go func() {
		serverErrs <- server.Serve(ctx)
	}()

	ticker := time.NewTicker(m.pollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return <-serverErrs
		case err := <-serverErrs:
			return err
		case <-ticker.C:
			_, err := m.Scheduler.RunNext(ctx)
			if err != nil && !errors.Is(err, errTaskFailed) && !errors.Is(err, errTaskTimedOut) {
				return err
			}
		}
	}
}

func (m DaemonManager) waitForReady(ctx context.Context) (DaemonState, error) {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		client := DaemonClient{Paths: m.Paths}
		state, err := client.Ping(ctx)
		if err == nil {
			return state, nil
		}

		readState, stateErr := m.ReadState()
		if stateErr == nil {
			state = readState
		} else if !errors.Is(stateErr, ErrRecordNotFound) {
			return DaemonState{}, stateErr
		}

		if !errors.Is(stateErr, ErrRecordNotFound) && !errors.Is(err, os.ErrNotExist) {
			time.Sleep(50 * time.Millisecond)
			continue
		}
		if stateErr != nil && !errors.Is(stateErr, ErrRecordNotFound) {
			return DaemonState{}, err
		}
		time.Sleep(50 * time.Millisecond)
	}

	return DaemonState{}, fmt.Errorf("daemon did not write state file")
}

func (m DaemonManager) ReadState() (DaemonState, error) {
	var state DaemonState
	if err := readJSONFile(m.Paths.DaemonStatePath(), &state); err != nil {
		return DaemonState{}, err
	}
	return state, nil
}

func (m DaemonManager) readAliveState() (DaemonState, bool, error) {
	state, err := m.ReadState()
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return DaemonState{}, false, nil
		}
		return DaemonState{}, false, err
	}

	alive, err := processAlive(state.PID)
	if err != nil {
		return DaemonState{}, false, err
	}
	if !alive {
		return state, false, nil
	}

	return state, true, nil
}

func (m DaemonManager) executablePath() string {
	if m.ExecutablePath != "" {
		return m.ExecutablePath
	}

	path, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "localci")
	}
	return path
}

func (m DaemonManager) env() []string {
	if len(m.Env) > 0 {
		return append([]string{}, m.Env...)
	}
	return os.Environ()
}

func (m DaemonManager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now().UTC()
}

func (m DaemonManager) pollInterval() time.Duration {
	if m.PollInterval > 0 {
		return m.PollInterval
	}
	return 250 * time.Millisecond
}

func processAlive(pid int) (bool, error) {
	if pid <= 0 {
		return false, nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false, err
	}
	err = process.Signal(syscall.Signal(0))
	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrProcessDone) {
		return false, nil
	}

	errno, ok := err.(syscall.Errno)
	if ok && errno == syscall.ESRCH {
		return false, nil
	}

	return false, err
}

func DaemonContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	go func() {
		defer signal.Stop(signals)
		select {
		case <-signals:
			cancel()
		case <-ctx.Done():
		}
	}()

	return ctx, cancel
}
