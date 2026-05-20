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
	Method string `json:"method"`
}

type DaemonResponse struct {
	OK         bool         `json:"ok"`
	Error      string       `json:"error,omitempty"`
	State      *DaemonState `json:"state,omitempty"`
	Queue      []QueueEntry `json:"queue,omitempty"`
	ActiveTask *ActiveTask  `json:"active_task,omitempty"`
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
	Paths     Paths
	Queue     QueueStore
	ReadState func() (DaemonState, error)
	Shutdown  func()
	mu        sync.Mutex
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
	default:
		return DaemonResponse{
			OK:    false,
			Error: fmt.Sprintf("unknown daemon method %q", req.Method),
		}
	}
}

func errorResponse(err error) DaemonResponse {
	return DaemonResponse{
		OK:    false,
		Error: err.Error(),
	}
}
