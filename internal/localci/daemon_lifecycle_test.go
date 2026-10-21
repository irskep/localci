package localci

import (
	"context"
	"testing"
)

func TestGracefulDrainFinishesCurrentWorkAndPreservesQueue(t *testing.T) {
	queue := QueueStore{Paths: Paths{Root: t.TempDir()}}
	for _, commit := range []string{"first", "second"} {
		if _, err := queue.EnqueueRun("/repo", commit, nil); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lifecycle := daemonLifecycle{queue: queue, cancel: cancel}
	_, state, err := lifecycle.claimNext()
	if err != nil || state != QueueClaimClaimed {
		t.Fatalf("claim: %s %v", state, err)
	}
	lifecycle.drain()
	if ctx.Err() != nil {
		t.Fatal("draining interrupted current work")
	}
	if !lifecycle.isDraining() {
		t.Fatal("draining state not reported")
	}
	_, state, err = lifecycle.claimNext()
	if err != nil || state != QueueClaimNone {
		t.Fatalf("claimed while draining: %s %v", state, err)
	}
	if err := queue.ClearActive(); err != nil {
		t.Fatal(err)
	}
	lifecycle.finished()
	if ctx.Err() == nil {
		t.Fatal("did not stop after work finished")
	}
	entries, err := queue.List()
	if err != nil || len(entries) != 1 {
		t.Fatalf("pending queue: %+v %v", entries, err)
	}
}

func TestGracefulDrainStopsIdleDaemon(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	lifecycle := daemonLifecycle{cancel: cancel}
	lifecycle.drain()
	if ctx.Err() == nil {
		t.Fatal("idle daemon did not stop")
	}
}
