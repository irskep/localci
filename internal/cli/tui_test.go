package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"localci/internal/localci"
)

func TestTUIRouteForTargetBuildsAPIPaths(t *testing.T) {
	t.Parallel()

	route, err := tuiRouteForTarget(commitTarget{
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
	want := "/api/repo/Users/steve/dev/cli/localci/commit/abc123/task/%2F%2Fweb:localci:test"
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
		Repos: []tuiRepoSummary{{RepoDir: "/repo", RepoPath: "repo", RepoLabel: "team/repo"}},
		Queue: tuiQueueResponse{Active: &tuiQueueEntry{
			Repo:    tuiRepoSummary{RepoDir: "/repo", RepoPath: "repo", RepoLabel: "team/repo"},
			Commit:  "abc123",
			Task:    "localci:test",
			Attempt: 2,
		}},
		RecentCommits: []tuiCommitSummary{{
			Repo:       tuiRepoSummary{RepoDir: "/repo", RepoPath: "repo", RepoLabel: "team/repo"},
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

func TestTUIResponsesAcceptPaginationFields(t *testing.T) {
	t.Parallel()

	decodeStrict := func(data string, out any) error {
		dec := json.NewDecoder(bytes.NewReader([]byte(data)))
		dec.DisallowUnknownFields()
		return dec.Decode(out)
	}

	var home tuiHomeResponse
	if err := decodeStrict(`{"repos":[],"recent_commits":[],"queue":{"pending":[]},"next_before":"older","newer_before":"newer"}`, &home); err != nil {
		t.Fatalf("decode home response: %v", err)
	}
	if home.NextBefore != "older" || home.NewerBefore != "newer" {
		t.Fatalf("home pagination fields = %q, %q; want older, newer", home.NextBefore, home.NewerBefore)
	}

	var repo tuiRepoResponse
	if err := decodeStrict(`{"repo":{"repo_dir":"/repo","repo_path":"repo","repo_label":"repo"},"commits":[],"next_before":"older","newer_before":"newer"}`, &repo); err != nil {
		t.Fatalf("decode repo response: %v", err)
	}
	if repo.NextBefore != "older" || repo.NewerBefore != "newer" {
		t.Fatalf("repo pagination fields = %q, %q; want older, newer", repo.NextBefore, repo.NewerBefore)
	}
}

func TestTUIArtifactTabsFitWidth(t *testing.T) {
	t.Parallel()

	model := newTUIModel(nil, tuiRoute{view: tuiViewTask, apiPath: "/api", title: "Task"})
	model.cursor = 3
	model.task = &tuiTaskResponse{
		Task: localci.TaskStatusView{
			Artifacts: []localci.ArtifactView{
				{DisplayName: "combined.log"},
				{DisplayName: "static-site/index.html"},
				{DisplayName: "static-site/mark.svg"},
				{DisplayName: "static-site/style.css"},
			},
		},
	}

	const width = 80
	rendered := model.renderArtifactTabs(tuiTheme{}, width)
	if height := lipgloss.Height(rendered); height > 3 {
		t.Fatalf("artifact tabs height = %d, want at most 3:\n%s", height, rendered)
	}
	for _, line := range splitLines(rendered) {
		if got := lipgloss.Width(line); got > width {
			t.Fatalf("artifact tab line width = %d, want <= %d:\n%s", got, width, rendered)
		}
	}
}

func TestTUIArtifactTabsUseBoundedWindow(t *testing.T) {
	t.Parallel()

	start, end := artifactTabRange(30, 16, 100)
	if end-start > maxVisibleArtifactTabs {
		t.Fatalf("visible tab count = %d, want <= %d", end-start, maxVisibleArtifactTabs)
	}
	if start > 16 || end <= 16 {
		t.Fatalf("range = [%d, %d), want cursor 16 visible", start, end)
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
