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
)

type DaemonRequest struct {
	Method   string `json:"method"`
	RepoDir  string `json:"repo_dir,omitempty"`
	Commit   string `json:"commit,omitempty"`
	TaskName string `json:"task_name,omitempty"`
}

type DaemonResponse struct {
	OK         bool              `json:"ok"`
	Error      string            `json:"error,omitempty"`
	State      *DaemonState      `json:"state,omitempty"`
	Queue      []QueueEntry      `json:"queue,omitempty"`
	Enqueued   []QueueEntry      `json:"enqueued,omitempty"`
	ActiveTask *ActiveTask       `json:"active_task,omitempty"`
	StatusView *CommitStatusView `json:"status_view,omitempty"`
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

func (c DaemonClient) Postcommit(ctx context.Context, repoDir string, commit string) ([]QueueEntry, error) {
	resp, err := c.call(ctx, DaemonRequest{
		Method:  "postcommit",
		RepoDir: repoDir,
		Commit:  commit,
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

func (c DaemonClient) call(ctx context.Context, req DaemonRequest) (DaemonResponse, error) {
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(ctx, "unix", c.Paths.DaemonSocketPath())
	if err != nil {
		return DaemonResponse{}, err
	}
	defer conn.Close()

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
		entries, err := s.enqueuePostcommit(req.RepoDir, req.Commit)
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
	if s.DiscoverTasks == nil {
		return nil, fmt.Errorf("daemon task discovery is not configured")
	}

	tasks, err := s.DiscoverTasks(context.Background(), repoDir)
	if err != nil {
		return nil, err
	}
	found := false
	for _, task := range tasks {
		if task.Name == taskName {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("task %q not found", taskName)
	}

	active, err := s.Queue.IsTaskActive(repoDir, commit, taskName)
	if err != nil {
		return nil, err
	}
	if active {
		return nil, nil
	}

	entry, err := s.Queue.Enqueue(repoDir, commit, taskName)
	if err != nil {
		return nil, err
	}
	return []QueueEntry{entry}, nil
}

func (s *DaemonServer) buildStatusView(repoDir string, commit string) (CommitStatusView, error) {
	if repoDir == "" {
		return CommitStatusView{}, fmt.Errorf("repo_dir is required")
	}
	if commit == "" {
		return CommitStatusView{}, fmt.Errorf("commit is required")
	}
	if s.DiscoverTasks == nil {
		return CommitStatusView{}, fmt.Errorf("daemon task discovery is not configured")
	}

	tasks, err := s.DiscoverTasks(context.Background(), repoDir)
	if err != nil {
		return CommitStatusView{}, err
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

	return BuildCommitStatusView(s.Paths, repoDir, commit, tasks, queue, activePtr)
}

func (s *DaemonServer) enqueuePostcommit(repoDir string, commit string) ([]QueueEntry, error) {
	if repoDir == "" {
		return nil, fmt.Errorf("repo_dir is required")
	}
	if commit == "" {
		return nil, fmt.Errorf("commit is required")
	}
	if s.DiscoverTasks == nil {
		return nil, fmt.Errorf("daemon task discovery is not configured")
	}

	tasks, err := s.DiscoverTasks(context.Background(), repoDir)
	if err != nil {
		return nil, err
	}

	enqueued := make([]QueueEntry, 0, len(tasks))
	for _, task := range tasks {
		active, err := s.Queue.IsTaskActive(repoDir, commit, task.Name)
		if err != nil {
			return nil, err
		}
		if active {
			continue
		}

		entry, err := s.Queue.Enqueue(repoDir, commit, task.Name)
		if err != nil {
			return nil, err
		}
		enqueued = append(enqueued, entry)
	}

	return enqueued, nil
}

func errorResponse(err error) DaemonResponse {
	return DaemonResponse{
		OK:    false,
		Error: err.Error(),
	}
}
