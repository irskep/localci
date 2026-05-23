package cli

import (
	"strings"
	"testing"
	"time"

	"localci/internal/localci"
)

func TestTUIRouteForTargetBuildsAPIPaths(t *testing.T) {
	t.Parallel()

	route, err := tuiRouteForTarget("/Users/steve/dev", commitTarget{
		RepoDir: "/Users/steve/dev/cli/localci",
		Commit:  "abc123",
		Task:    "//web:localci:test",
	})
	if err != nil {
		t.Fatalf("tuiRouteForTarget returned error: %v", err)
	}
	if route.view != tuiViewTask {
		t.Fatalf("view = %v, want task", route.view)
	}
	want := "/api/repo/cli/localci/commit/abc123/task/%2F%2Fweb:localci:test"
	if route.apiPath != want {
		t.Fatalf("apiPath = %q, want %q", route.apiPath, want)
	}
}

func TestTUIClientURLForPreservesEscapedTaskSlashes(t *testing.T) {
	t.Parallel()

	client, err := newTUIClient("http://127.0.0.1:61924")
	if err != nil {
		t.Fatalf("newTUIClient returned error: %v", err)
	}
	got := client.urlFor("/api/repo/cli/localci/commit/abc123/task/%2F%2Fweb:localci:test").String()
	want := "http://127.0.0.1:61924/api/repo/cli/localci/commit/abc123/task/%2F%2Fweb:localci:test"
	if got != want {
		t.Fatalf("url = %q, want %q", got, want)
	}
}

func TestTUIRenderHomeIncludesQueueAndRuns(t *testing.T) {
	t.Parallel()

	model := newTUIModel(nil, tuiRoute{view: tuiViewHome, apiPath: "/api", title: "Home"})
	model.width = 100
	model.height = 30
	model.home = &tuiHomeResponse{
		Repos: []tuiRepoSummary{{RepoDir: "/repo", RepoPath: "team/repo"}},
		Queue: tuiQueueResponse{Active: &tuiQueueEntry{
			Repo:    tuiRepoSummary{RepoDir: "/repo", RepoPath: "team/repo"},
			Commit:  "abc123",
			Task:    "localci:test",
			Attempt: 2,
		}},
		RecentCommits: []tuiCommitSummary{{
			Repo:       tuiRepoSummary{RepoDir: "/repo", RepoPath: "team/repo"},
			Commit:     "abc123456789",
			ActivityAt: time.Now(),
			Tasks: []tuiTaskSummary{{
				Name:      "localci:test",
				ShortName: "test",
				Status:    localci.ExecutionStatusSucceeded,
				Attempt:   1,
			}},
		}},
	}

	rendered := model.View()
	for _, want := range []string{"active", "team/repo", "abc123456789", "test", "1 ok"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered home missing %q:\n%s", want, rendered)
		}
	}
}

func TestTaskStatusGroups(t *testing.T) {
	t.Parallel()

	got := taskStatusGroups([]tuiTaskSummary{
		{ShortName: "test", Status: localci.ExecutionStatusSucceeded},
		{ShortName: "lint", Status: localci.ExecutionStatusFailed},
	})
	if !strings.Contains(got, "failed: lint") || !strings.Contains(got, "ok: test") {
		t.Fatalf("taskStatusGroups = %q", got)
	}
}
