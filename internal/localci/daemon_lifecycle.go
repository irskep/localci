package localci

import (
	"context"
	"sync"
)

// Claim and drain share a lock so a graceful restart cannot race a new task.
type daemonLifecycle struct {
	mu       sync.Mutex
	draining bool
	running  bool
	cancel   context.CancelFunc
	queue    QueueStore
}

func (d *daemonLifecycle) claimNext() (QueueEntry, QueueClaimState, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.draining {
		return QueueEntry{}, QueueClaimNone, nil
	}
	entry, state, err := d.queue.ClaimNext()
	d.running = state == QueueClaimClaimed
	return entry, state, err
}

func (d *daemonLifecycle) drain() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.draining = true
	if !d.running {
		d.cancel()
	}
}

func (d *daemonLifecycle) finished() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = false
	if d.draining {
		d.cancel()
	}
}

func (d *daemonLifecycle) isDraining() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.draining
}
