package localci

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestQueueStoreEnqueueListAndRemove(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := QueueStore{
		Paths: Paths{Root: root},
	}

	base := time.Date(2026, 5, 20, 22, 0, 0, 0, time.UTC)
	times := []time.Time{
		base.Add(2 * time.Second),
		base,
		base.Add(time.Second),
	}
	index := 0
	store.Now = func() time.Time {
		now := times[index]
		index++
		return now
	}

	entryA, err := store.Enqueue("/repo-a", "c1", "localci:test")
	if err != nil {
		t.Fatalf("Enqueue(entryA) returned error: %v", err)
	}
	entryB, err := store.Enqueue("/repo-b", "c2", "localci:build")
	if err != nil {
		t.Fatalf("Enqueue(entryB) returned error: %v", err)
	}
	entryC, err := store.Enqueue("/repo-a", "c3", "localci:fmt")
	if err != nil {
		t.Fatalf("Enqueue(entryC) returned error: %v", err)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("len(entries) = %d, want 3", len(entries))
	}
	if entryA.Attempt != 1 || entryB.Attempt != 1 || entryC.Attempt != 1 {
		t.Fatalf("unexpected attempts: %#v %#v %#v", entryA, entryB, entryC)
	}
	if entries[0].Commit != entryB.Commit || entries[1].Commit != entryC.Commit || entries[2].Commit != entryA.Commit {
		t.Fatalf("unexpected queue order: %#v", entries)
	}

	if err := store.Remove(entryC); err != nil {
		t.Fatalf("Remove(entryC) returned error: %v", err)
	}

	entries, err = store.List()
	if err != nil {
		t.Fatalf("List after remove returned error: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("len(entries) after remove = %d, want 2", len(entries))
	}
}

func TestQueueStoreActiveMarker(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := QueueStore{
		Paths: Paths{Root: root},
		Now: func() time.Time {
			return time.Date(2026, 5, 20, 22, 30, 0, 0, time.UTC)
		},
	}

	entry := QueueEntry{
		RepoDir:    "/repo",
		RepoID:     normalizeRepoDir("/repo"),
		Commit:     "abc123",
		TaskName:   "localci:test",
		TaskKey:    sanitizeTaskName("localci:test"),
		EnqueuedAt: time.Date(2026, 5, 20, 22, 0, 0, 0, time.UTC),
	}

	active, err := store.MarkActive(entry)
	if err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}
	if active.StartedAt.IsZero() {
		t.Fatalf("StartedAt should be set")
	}

	readBack, err := store.ReadActive()
	if err != nil {
		t.Fatalf("ReadActive returned error: %v", err)
	}
	if readBack.TaskName != entry.TaskName || readBack.Commit != entry.Commit {
		t.Fatalf("unexpected active task: %#v", readBack)
	}

	isActive, err := store.IsTaskActive("/repo", "abc123", "localci:test")
	if err != nil {
		t.Fatalf("IsTaskActive returned error: %v", err)
	}
	if !isActive {
		t.Fatalf("IsTaskActive returned false, want true")
	}

	if err := store.ClearActive(); err != nil {
		t.Fatalf("ClearActive returned error: %v", err)
	}

	isActive, err = store.IsTaskActive("/repo", "abc123", "localci:test")
	if err != nil {
		t.Fatalf("IsTaskActive after clear returned error: %v", err)
	}
	if isActive {
		t.Fatalf("IsTaskActive after clear returned true, want false")
	}
}

func TestQueueStoreClaimNextMarksActiveAndRemovesPendingAtomically(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	store := QueueStore{
		Paths: Paths{Root: root},
		Now: func() time.Time {
			return time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC)
		},
	}

	entry, err := store.Enqueue("/repo", "abc123", "localci:test")
	if err != nil {
		t.Fatalf("Enqueue returned error: %v", err)
	}

	claimed, didClaim, err := store.ClaimNext()
	if err != nil {
		t.Fatalf("ClaimNext returned error: %v", err)
	}
	if !didClaim {
		t.Fatalf("ClaimNext didClaim = false, want true")
	}
	if claimed.TaskName != entry.TaskName || claimed.Attempt != entry.Attempt {
		t.Fatalf("claimed entry = %#v, want %#v", claimed, entry)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("len(entries) = %d, want 0", len(entries))
	}

	active, err := store.ReadActive()
	if err != nil {
		t.Fatalf("ReadActive returned error: %v", err)
	}
	if active.TaskName != entry.TaskName {
		t.Fatalf("active task = %#v, want %#v", active, entry)
	}

	claimedAgain, didClaim, err := store.ClaimNext()
	if err != nil {
		t.Fatalf("ClaimNext with active returned error: %v", err)
	}
	if didClaim {
		t.Fatalf("ClaimNext with active didClaim = true, want false")
	}
	if claimedAgain.TaskName != entry.TaskName {
		t.Fatalf("claimed active = %#v, want active task", claimedAgain)
	}
}

func TestQueueStoreCancelRemovesPendingAndMarksActive(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	paths := Paths{Root: root}
	store := QueueStore{
		Paths: paths,
		Now: func() time.Time {
			return time.Date(2026, 5, 21, 13, 0, 0, 0, time.UTC)
		},
	}

	pending, err := store.Enqueue("/repo", "abc123", "localci:test")
	if err != nil {
		t.Fatalf("Enqueue pending returned error: %v", err)
	}
	other, err := store.Enqueue("/repo", "abc123", "localci:build")
	if err != nil {
		t.Fatalf("Enqueue other returned error: %v", err)
	}
	activeEntry := QueueEntry{
		RepoDir:  "/repo",
		RepoID:   normalizeRepoDir("/repo"),
		Commit:   "abc123",
		TaskName: "localci:test",
		TaskKey:  sanitizeTaskName("localci:test"),
		Attempt:  pending.Attempt + 1,
	}
	if _, err := store.MarkActive(activeEntry); err != nil {
		t.Fatalf("MarkActive returned error: %v", err)
	}

	result, err := store.Cancel("/repo", "abc123", "localci:test")
	if err != nil {
		t.Fatalf("Cancel returned error: %v", err)
	}
	if !result.Active || result.Pending != 1 {
		t.Fatalf("Cancel result = %#v, want active=true pending=1", result)
	}

	entries, err := store.List()
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].TaskName != other.TaskName {
		t.Fatalf("queue entries after cancel = %#v, want only other task", entries)
	}

	markerPath := filepath.Join(paths.TaskAttemptDir("/repo", "abc123", "localci:test", activeEntry.Attempt), cancelMarkerName)
	if _, err := os.Stat(markerPath); err != nil {
		t.Fatalf("cancel marker missing at %s: %v", markerPath, err)
	}
}
