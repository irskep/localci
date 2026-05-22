package localci

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"sync"
	"time"
)

type DaemonRequest struct {
	Method      string            `json:"method"`
	RepoDir     string            `json:"repo_dir,omitempty"`
	Commit      string            `json:"commit,omitempty"`
	TaskName    string            `json:"task_name,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

type DaemonResponse struct {
	OK         bool               `json:"ok"`
	Error      string             `json:"error,omitempty"`
	State      *DaemonState       `json:"state,omitempty"`
	Queue      []QueueEntry       `json:"queue,omitempty"`
	Enqueued   []QueueEntry       `json:"enqueued,omitempty"`
	Canceled   *QueueCancelResult `json:"canceled,omitempty"`
	ActiveTask *ActiveTask        `json:"active_task,omitempty"`
	StatusView *CommitStatusView  `json:"status_view,omitempty"`
}

type DaemonClient struct {
	Paths Paths
}

func (c DaemonClient) Ping(ctx context.Context) (DaemonState, error) {
	resp, err := c.call(ctx, DaemonRequest{Method: "ping"})
	if err != nil {
		return DaemonState{}, err
	}
	if resp.State == nil {
		return DaemonState{}, fmt.Errorf("daemon ping returned no state")
	}
	return *resp.State, nil
}

func (c DaemonClient) Shutdown(ctx context.Context) error {
	_, err := c.call(ctx, DaemonRequest{Method: "shutdown"})
	return err
}

func (c DaemonClient) Queue(ctx context.Context) ([]QueueEntry, error) {
	resp, err := c.call(ctx, DaemonRequest{Method: "queue"})
	if err != nil {
		return nil, err
	}
	return resp.Queue, nil
}

func (c DaemonClient) ActiveTask(ctx context.Context) (*ActiveTask, error) {
	resp, err := c.call(ctx, DaemonRequest{Method: "active-task"})
	if err != nil {
		return nil, err
	}
	return resp.ActiveTask, nil
}

func (c DaemonClient) Postcommit(ctx context.Context, repoDir string, commit string, annotations map[string]string) ([]QueueEntry, error) {
	resp, err := c.call(ctx, DaemonRequest{
		Method:      "postcommit",
		RepoDir:     repoDir,
		Commit:      commit,
		Annotations: annotations,
	})
	if err != nil {
		return nil, err
	}
	return resp.Enqueued, nil
}

func (c DaemonClient) Status(ctx context.Context, repoDir string, commit string) (CommitStatusView, error) {
	resp, err := c.call(ctx, DaemonRequest{
		Method:  "status",
		RepoDir: repoDir,
		Commit:  commit,
	})
	if err != nil {
		return CommitStatusView{}, err
	}
	if resp.StatusView == nil {
		return CommitStatusView{}, fmt.Errorf("daemon status returned no status view")
	}
	return *resp.StatusView, nil
}

func (c DaemonClient) Retry(ctx context.Context, repoDir string, commit string, taskName string) ([]QueueEntry, error) {
	resp, err := c.call(ctx, DaemonRequest{
		Method:   "retry",
		RepoDir:  repoDir,
		Commit:   commit,
		TaskName: taskName,
	})
	if err != nil {
		return nil, err
	}
	return resp.Enqueued, nil
}

func (c DaemonClient) Cancel(ctx context.Context, repoDir string, commit string, taskName string) (QueueCancelResult, error) {
	resp, err := c.call(ctx, DaemonRequest{
		Method:   "cancel",
		RepoDir:  repoDir,
		Commit:   commit,
		TaskName: taskName,
	})
	if err != nil {
		return QueueCancelResult{}, err
	}
	if resp.Canceled == nil {
		return QueueCancelResult{}, fmt.Errorf("daemon cancel returned no result")
	}
	return *resp.Canceled, nil
}

func (c DaemonClient) call(ctx context.Context, req DaemonRequest) (DaemonResponse, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", c.Paths.DaemonSocketPath())
	if err != nil {
		return DaemonResponse{}, err
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return DaemonResponse{}, err
		}
	}

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return DaemonResponse{}, err
	}

	var resp DaemonResponse
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&resp); err != nil {
		return DaemonResponse{}, err
	}
	if !resp.OK {
		if resp.Error == "" {
			return DaemonResponse{}, fmt.Errorf("daemon request %q failed", req.Method)
		}
		return DaemonResponse{}, errors.New(resp.Error)
	}
	return resp, nil
}

type DaemonServer struct {
	Paths         Paths
	Queue         QueueStore
	ReadState     func() (DaemonState, error)
	DiscoverTasks func(context.Context, string) ([]Task, error)
	Events        *EventNotifier
	Shutdown      func()
	mu            sync.Mutex
}

func (s *DaemonServer) Serve(ctx context.Context) error {
	if err := os.Remove(s.Paths.DaemonSocketPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	listener, err := net.Listen("unix", s.Paths.DaemonSocketPath())
	if err != nil {
		return err
	}
	defer func() {
		_ = listener.Close()
		_ = os.Remove(s.Paths.DaemonSocketPath())
	}()

	go func() {
		<-ctx.Done()
		_ = listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Temporary() {
				continue
			}
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			return err
		}

		if err := s.handleConn(conn); err != nil {
			_ = conn.Close()
			return err
		}
	}
}

func (s *DaemonServer) handleConn(conn net.Conn) error {
	defer conn.Close()

	var req DaemonRequest
	if err := json.NewDecoder(bufio.NewReader(conn)).Decode(&req); err != nil {
		return json.NewEncoder(conn).Encode(DaemonResponse{
			OK:    false,
			Error: fmt.Sprintf("decode request: %v", err),
		})
	}

	resp := s.dispatch(req)
	return json.NewEncoder(conn).Encode(resp)
}

func (s *DaemonServer) dispatch(req DaemonRequest) DaemonResponse {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch req.Method {
	case "ping":
		state, err := s.ReadState()
		if err != nil {
			return errorResponse(err)
		}
		return DaemonResponse{OK: true, State: &state}
	case "queue":
		queue, err := s.Queue.List()
		if err != nil {
			return errorResponse(err)
		}
		return DaemonResponse{OK: true, Queue: queue}
	case "active-task":
		active, err := s.Queue.ReadActive()
		if err != nil {
			if errors.Is(err, ErrRecordNotFound) {
				return DaemonResponse{OK: true}
			}
			return errorResponse(err)
		}
		return DaemonResponse{OK: true, ActiveTask: &active}
	case "shutdown":
		if s.Shutdown != nil {
			s.Shutdown()
		}
		return DaemonResponse{OK: true}
	case "postcommit":
		entries, err := s.enqueuePostcommit(req.RepoDir, req.Commit, req.Annotations)
		if err != nil {
			return errorResponse(err)
		}
		return DaemonResponse{OK: true, Enqueued: entries}
	case "status":
		statusView, err := s.buildStatusView(req.RepoDir, req.Commit)
		if err != nil {
			return errorResponse(err)
		}
		return DaemonResponse{OK: true, StatusView: &statusView}
	case "retry":
		entries, err := s.enqueueRetry(req.RepoDir, req.Commit, req.TaskName)
		if err != nil {
			return errorResponse(err)
		}
		return DaemonResponse{OK: true, Enqueued: entries}
	case "cancel":
		result, err := s.cancelTask(req.RepoDir, req.Commit, req.TaskName)
		if err != nil {
			return errorResponse(err)
		}
		return DaemonResponse{OK: true, Canceled: &result}
	default:
		return DaemonResponse{
			OK:    false,
			Error: fmt.Sprintf("unknown daemon method %q", req.Method),
		}
	}
}

func (s *DaemonServer) enqueueRetry(repoDir string, commit string, taskName string) ([]QueueEntry, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("repo_dir is required")
	}
	if commit == "" {
		return nil, fmt.Errorf("commit is required")
	}
	if taskName == "" {
		return nil, fmt.Errorf("task_name is required")
	}
	active, err := s.Queue.IsTaskActive(repoDir, commit, taskName)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, nil
	}

	entry, err := s.Queue.EnqueueRun(repoDir, commit, []string{taskName})
	if err != nil {
		return nil, err
	}
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}
	return []QueueEntry{entry}, nil
}

func (s *DaemonServer) cancelTask(repoDir string, commit string, taskName string) (QueueCancelResult, error) {
	if repoDir == "" {
		return QueueCancelResult{}, fmt.Errorf("repo_dir is required")
	}
	if commit == "" {
		return QueueCancelResult{}, fmt.Errorf("commit is required")
	}
	if taskName == "" {
		return QueueCancelResult{}, fmt.Errorf("task_name is required")
	}
	result, err := s.Queue.Cancel(repoDir, commit, taskName)
	if err != nil {
		return QueueCancelResult{}, err
	}
	if s.Events != nil && (result.Active || result.Pending > 0) {
		s.Events.EntryChanged(QueueEntry{
			Kind:     QueueEntryKindTask,
			RepoDir:  repoDir,
			RepoID:   normalizeRepoDir(repoDir),
			Commit:   commit,
			TaskName: taskName,
			TaskKey:  sanitizeTaskName(taskName),
		})
	}
	return result, nil
}

func (s *DaemonServer) buildStatusView(repoDir string, commit string) (CommitStatusView, error) {
	if repoDir == "" {
		return CommitStatusView{}, fmt.Errorf("repo_dir is required")
	}
	if commit == "" {
		return CommitStatusView{}, fmt.Errorf("commit is required")
	}
	queue, err := s.Queue.List()
	if err != nil {
		return CommitStatusView{}, err
	}

	active, err := s.Queue.ReadActive()
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		return CommitStatusView{}, err
	}
	var activePtr *ActiveTask
	if err == nil {
		activePtr = &active
	}

	return BuildCommitStatusView(s.Paths, repoDir, commit, nil, queue, activePtr)
}

func (s *DaemonServer) enqueuePostcommit(repoDir string, commit string, annotations map[string]string) ([]QueueEntry, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("repo_dir is required")
	}
	if commit == "" {
		return nil, fmt.Errorf("commit is required")
	}
	if err := s.ensureRunRecord(repoDir, commit, annotations); err != nil {
		return nil, err
	}

	entry, err := s.Queue.EnqueueRun(repoDir, commit, nil)
	if err != nil {
		return nil, err
	}
	if s.Events != nil {
		s.Events.EntryChanged(entry)
	}
	return []QueueEntry{entry}, nil
}

func (s *DaemonServer) ensureRunRecord(repoDir string, commit string, annotations map[string]string) error {
	req := InvokeRequest{
		RepoDir:     repoDir,
		Commit:      commit,
		Annotations: annotations,
	}
	run, err := readRunRecord(s.Paths, req)
	if err != nil {
		if !errors.Is(err, ErrRecordNotFound) {
			return err
		}
		run = newRunRecord(req, time.Now().UTC())
		return writeRunRecord(s.Paths, req, run)
	}
	if len(annotations) == 0 {
		return nil
	}
	if run.Annotations == nil {
		run.Annotations = map[string]string{}
	}
	for key, value := range annotations {
		run.Annotations[key] = value
	}
	return writeRunRecord(s.Paths, req, run)
}

func errorResponse(err error) DaemonResponse {
	return DaemonResponse{
		OK:    false,
		Error: err.Error(),
	}
}
