package localci

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDaemonManagerRunWritesAndClearsState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	manager := DaemonManager{
		Paths: Paths{Root: root},
		Now: func() time.Time {
			return time.Date(2026, 5, 21, 0, 0, 0, 0, time.UTC)
		},
		PollInterval: 10 * time.Millisecond,
		Scheduler: Scheduler{
			Queue:  QueueStore{Paths: Paths{Root: root}},
			Runner: Runner{Paths: Paths{Root: root}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- manager.Run(ctx)
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		_, err := manager.ReadState()
		if err == nil {
			cancel()
			break
		}
		if !errors.Is(err, ErrRecordNotFound) {
			t.Fatalf("ReadState returned error: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	if err := <-done; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	_, err := manager.ReadState()
	if !errors.Is(err, ErrRecordNotFound) {
		t.Fatalf("ReadState after shutdown error = %v, want ErrRecordNotFound", err)
	}
}

func TestProcessAliveInvalidPID(t *testing.T) {
	t.Parallel()

	alive, err := processAlive(-1)
	if err != nil {
		t.Fatalf("processAlive returned error: %v", err)
	}
	if alive {
		t.Fatalf("processAlive returned true for invalid pid")
	}
}
