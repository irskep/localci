package localci

import "testing"

func TestCommitCompleteWaitsForActiveWorkOnly(t *testing.T) {
	t.Parallel()

	if CommitComplete(CommitStatusView{Tasks: []TaskStatusView{{Status: ExecutionStatusQueued}}}) {
		t.Fatalf("queued commit should not be complete")
	}
	if CommitComplete(CommitStatusView{Tasks: []TaskStatusView{{Status: ExecutionStatusRunning}}}) {
		t.Fatalf("running commit should not be complete")
	}
	if !CommitComplete(CommitStatusView{Tasks: []TaskStatusView{{Status: ExecutionStatusNotRun}}}) {
		t.Fatalf("not-run commit should be complete")
	}
	if !CommitComplete(CommitStatusView{Tasks: []TaskStatusView{{Status: ExecutionStatusSucceeded}, {Status: ExecutionStatusFailed}}}) {
		t.Fatalf("finished commit should be complete")
	}
}

func TestSummarizeCommit(t *testing.T) {
	t.Parallel()

	view := CommitStatusView{Tasks: []TaskStatusView{
		{Status: ExecutionStatusSucceeded},
		{Status: ExecutionStatusFailed},
		{Status: ExecutionStatusTimedOut},
		{Status: ExecutionStatusNotRun},
	}}

	if got, want := SummarizeCommit(view), "1 passed, 1 failed, 1 timed out, 1 not run"; got != want {
		t.Fatalf("SummarizeCommit = %q, want %q", got, want)
	}
}
