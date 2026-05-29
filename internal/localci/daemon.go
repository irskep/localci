package localci

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

type DaemonState struct {
	PID         int       `json:"pid"`
	StartedAt   time.Time `json:"started_at"`
	HTTPAddress string    `json:"http_address,omitempty"`
	HTTPBaseURL string    `json:"http_base_url,omitempty"`
}

type DaemonManager struct {
	Paths          Paths
	ExecutablePath string
	Env            []string
	Now            func() time.Time
	Scheduler      Scheduler
	PollInterval   time.Duration
	HTTPAddress    string
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
	if err := m.ensureHTTPAddressAvailable(ctx); err != nil {
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

func (m DaemonManager) Restart(ctx context.Context) (StartResult, error) {
	if err := m.Stop(); err != nil {
		return StartResult{}, err
	}

	return m.Start(ctx)
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
	sentSignal := false
	if err := client.Shutdown(context.Background()); err != nil {
		if signalErr := signalProcess(state.PID, syscall.SIGTERM); signalErr != nil {
			return signalErr
		}
		sentSignal = true
	}

	if stopped, err := waitForProcessExit(state.PID, 5*time.Second); err != nil {
		return err
	} else if stopped {
		return m.clearState()
	}

	if !sentSignal {
		if err := signalProcess(state.PID, syscall.SIGTERM); err != nil {
			return err
		}
	}
	if stopped, err := waitForProcessExit(state.PID, 2*time.Second); err != nil {
		return err
	} else if stopped {
		return m.clearState()
	}

	if err := signalProcess(state.PID, syscall.SIGKILL); err != nil {
		return err
	}
	if stopped, err := waitForProcessExit(state.PID, 2*time.Second); err != nil {
		return err
	} else if stopped {
		return m.clearState()
	}

	return fmt.Errorf("timed out waiting for daemon pid %d to stop", state.PID)
}

func (m DaemonManager) clearState() error {
	if removeErr := os.Remove(m.Paths.DaemonStatePath()); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
		return removeErr
	}
	_ = os.Remove(m.Paths.DaemonSocketPath())
	return nil
}

func waitForProcessExit(pid int, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		alive, err := processAlive(pid)
		if err != nil {
			return false, err
		}
		if !alive {
			return true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false, nil
}

func signalProcess(pid int, signal syscall.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Signal(signal); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

func (m DaemonManager) Run(ctx context.Context) error {
	if err := os.MkdirAll(m.Paths.DaemonRoot(), 0o755); err != nil {
		return err
	}
	if err := m.recoverInterruptedWork(); err != nil {
		return err
	}
	if err := (CloneManager{Paths: m.Paths, Now: m.Now}).CleanupUnreferenced(m.Scheduler.Queue); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	state := DaemonState{
		PID:       os.Getpid(),
		StartedAt: m.now(),
	}

	httpListener, err := net.Listen("tcp", m.httpAddress())
	if err != nil {
		return err
	}
	defer httpListener.Close()

	state.HTTPAddress = httpListener.Addr().String()
	state.HTTPBaseURL = "http://" + state.HTTPAddress
	if err := writeJSONFile(m.Paths.DaemonStatePath(), state); err != nil {
		return err
	}
	defer func() {
		_ = os.Remove(m.Paths.DaemonStatePath())
		_ = os.Remove(m.Paths.DaemonSocketPath())
	}()

	eventHub := NewEventHub()
	eventNotifier := &EventNotifier{Hub: eventHub}
	scheduler := m.Scheduler
	scheduler.Events = eventNotifier
	scheduler.Runner.Events = eventNotifier

	server := &DaemonServer{
		Paths:         m.Paths,
		Queue:         scheduler.Queue,
		ReadState:     m.ReadState,
		DiscoverTasks: scheduler.Runner.DiscoverTasks,
		Events:        eventNotifier,
		Shutdown:      cancel,
	}

	serverErrs := make(chan error, 1)
	go func() {
		serverErrs <- server.Serve(ctx)
	}()

	webServer := WebServer{
		Paths:         m.Paths,
		Queue:         scheduler.Queue,
		DiscoverTasks: scheduler.Runner.DiscoverTasks,
		AssetDir:      os.Getenv("LOCALCI_WEB_DIR"),
		Events:        eventNotifier,
		EventHub:      eventHub,
		Shutdown:      cancel,
	}
	webErrs := make(chan error, 1)
	go func() {
		webErrs <- webServer.Serve(ctx, httpListener)
	}()

	ticker := time.NewTicker(m.pollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			if err := <-serverErrs; err != nil {
				return err
			}
			return <-webErrs
		case err := <-serverErrs:
			return err
		case err := <-webErrs:
			return err
		case <-ticker.C:
			_, err := scheduler.RunNext(ctx)
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
			if err := waitForHTTPReady(ctx, state.HTTPBaseURL); err == nil {
				return state, nil
			}
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

func waitForHTTPReady(ctx context.Context, baseURL string) error {
	if baseURL == "" {
		return fmt.Errorf("daemon did not publish an HTTP base URL")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/healthz", nil)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout: 200 * time.Millisecond,
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected daemon HTTP status %d", resp.StatusCode)
	}

	return nil
}

func (m DaemonManager) ensureHTTPAddressAvailable(ctx context.Context) error {
	address := m.httpAddress()
	listener, err := net.Listen("tcp", address)
	if err == nil {
		_ = listener.Close()
		return nil
	}
	if strings.HasSuffix(address, ":0") {
		return nil
	}

	if shutdownErr := shutdownHTTPDaemon(ctx, "http://"+address); shutdownErr != nil {
		return fmt.Errorf("daemon HTTP address %s is already in use and did not accept shutdown: %w", address, err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		listener, err = net.Listen("tcp", address)
		if err == nil {
			_ = listener.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timed out waiting for daemon HTTP address %s to become available", address)
}

func shutdownHTTPDaemon(ctx context.Context, baseURL string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/api/daemon/shutdown", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected shutdown status %d", resp.StatusCode)
	}
	return nil
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
	ready, err := m.daemonReady(state)
	if err != nil {
		return DaemonState{}, false, err
	}
	if !ready {
		return state, false, nil
	}

	return state, true, nil
}

func (m DaemonManager) daemonReady(state DaemonState) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	pingState, err := DaemonClient{Paths: m.Paths}.Ping(ctx)
	if err != nil {
		return false, nil
	}
	if pingState.PID != state.PID {
		return false, nil
	}
	if err := waitForHTTPReady(ctx, pingState.HTTPBaseURL); err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			return false, nil
		}
		return false, nil
	}
	return true, nil
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

func (m DaemonManager) httpAddress() string {
	if m.HTTPAddress != "" {
		return m.HTTPAddress
	}

	return "127.0.0.1:61924"
}

func (m DaemonManager) recoverInterruptedWork() error {
	active, err := m.Scheduler.Queue.ReadActive()
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return nil
		}
		return err
	}
	defer func() {
		_ = m.Scheduler.Queue.ClearActive()
		_ = (CloneManager{Paths: m.Paths, Now: m.Now}).Cleanup(active.RepoDir, active.Commit)
	}()

	if active.Kind == QueueEntryKindRun {
		_, err := m.Scheduler.Queue.EnqueueRunWithOptions(active.RepoDir, active.Commit, active.RequestedTasks, active.NoClone)
		return err
	}

	shouldRetry, err := recoverActiveTaskRecord(m.Paths, active, m.now())
	if err != nil {
		return err
	}
	if !shouldRetry {
		return nil
	}

	_, err = m.Scheduler.Queue.EnqueueRunWithOptions(active.RepoDir, active.Commit, []string{active.TaskName}, active.NoClone)
	return err
}

func recoverActiveTaskRecord(paths Paths, active ActiveTask, now time.Time) (bool, error) {
	reader := StatusReader{Paths: paths}
	status, err := reader.ReadCommit(active.RepoDir, active.Commit)
	if err != nil {
		if errors.Is(err, ErrRecordNotFound) {
			return true, nil
		}
		return false, err
	}

	record, ok := findTaskRecord(status.Tasks, active.TaskName)
	if !ok {
		return true, nil
	}
	if record.Status != TaskStatusRunning {
		return false, nil
	}

	record.Status = TaskStatusFailed
	record.Failure = "daemon-restart"
	record.Message = "task interrupted by daemon restart"
	record.FinishedAt = now
	record.DurationMilliseconds = durationMilliseconds(record.StartedAt, record.FinishedAt)
	if err := writeTaskRecord(record); err != nil {
		return false, err
	}

	_, err = upsertTaskRecord(paths, InvokeRequest{
		RepoDir: active.RepoDir,
		Commit:  active.Commit,
	}, record, now)
	if err != nil {
		return false, err
	}

	return true, nil
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

func findTaskRecord(tasks []TaskRecord, name string) (TaskRecord, bool) {
	for _, task := range tasks {
		if task.Name == name {
			return task, true
		}
	}
	return TaskRecord{}, false
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
