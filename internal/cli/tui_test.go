package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
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
			Repo:   tuiRepoSummary{RepoDir: "/repo", RepoPath: "repo", RepoLabel: "team/repo"},
			Commit: "abc123456789",
			Annotations: map[string]string{
				localci.AnnotationCommitSubject: "Fix artifact tab overflow",
			},
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
	for _, want := range []string{"active", "team/repo", "abc123456789", "Fix artifact tab overflow", "test", "1 ok"} {
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

	var taskHistory tuiRepoTaskHistoryResponse
	if err := decodeStrict(`{"repo":{"repo_dir":"/repo","repo_path":"repo","repo_label":"repo"},"task":"localci:test","short_name":"test","runs":[]}`, &taskHistory); err != nil {
		t.Fatalf("decode task history response: %v", err)
	}
	if taskHistory.Task != "localci:test" || taskHistory.ShortName != "test" {
		t.Fatalf("task history metadata = %#v", taskHistory)
	}
}

func TestTUITaskHistoryRouteAndRender(t *testing.T) {
	t.Parallel()

	repo := tuiRepoSummary{RepoDir: "/repo", RepoPath: "repo", RepoLabel: "repo"}
	route := tuiRepoTaskHistoryRoute(repo, "localci:test")
	if route.view != tuiViewRepoTaskHistory || route.apiPath != "/api/repo/repo/task/localci:test" {
		t.Fatalf("task history route = %#v", route)
	}

	model := newTUIModel(nil, route)
	model.width = 100
	model.height = 20
	model.taskHistory = &tuiRepoTaskHistoryResponse{
		Repo:      repo,
		Task:      "localci:test",
		ShortName: "test",
		Runs: []tuiRepoTaskHistoryItem{{
			Commit: "abc123456789",
			Annotations: map[string]string{
				localci.AnnotationCommitSubject: "Fix task history",
			},
			Task: tuiTaskSummary{
				Name:                 "localci:test",
				ShortName:            "test",
				Status:               localci.ExecutionStatusFailed,
				Attempt:              2,
				AttemptCount:         3,
				DurationMilliseconds: 54,
				Failure:              "exit",
			},
			ActivityAt: time.Now(),
		}},
	}

	rendered := model.View()
	for _, want := range []string{"test history", "abc123456789", "Fix task history", "failed", "attempt 2/3", "exit"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("task history view missing %q:\n%s", want, rendered)
		}
	}
	next, ok := model.openSelected()
	if !ok || next.view != tuiViewTask || next.commit != "abc123456789" || next.task != "localci:test" {
		t.Fatalf("open selected = %#v, %v", next, ok)
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

func TestTUICommitTaskArtifactSummaryIsBounded(t *testing.T) {
	t.Parallel()

	artifacts := []localci.ArtifactView{{
		DisplayName: "site/index.html",
		MarkedName:  "docs html",
		Action:      localci.ArtifactActionOpen,
	}}
	for i := range 20 {
		artifacts = append(artifacts, localci.ArtifactView{
			DisplayName: fmt.Sprintf("site/page-%02d/index.html", i),
		})
	}

	rows := commitTaskArtifactSummary(artifacts, 6, 80)
	rendered := strings.Join(rows, "\n")
	for _, want := range []string{"artifacts: 21", "docs html -> site/index.html (open)", "more (press a)"} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("artifact summary missing %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "site/page-19/index.html") {
		t.Fatalf("artifact summary should not dump every artifact:\n%s", rendered)
	}
}

func TestTUICommitTaskArtifactSummaryTruncatesWidePaths(t *testing.T) {
	t.Parallel()

	rows := commitTaskArtifactSummary([]localci.ArtifactView{{
		DisplayName: "site/cli/localci_completion_powershell/index.html",
	}}, 4, 24)

	if len(rows) < 3 {
		t.Fatalf("rows = %#v, want artifact row", rows)
	}
	if got := lipgloss.Width(rows[2]); got > 24 {
		t.Fatalf("artifact row width = %d, want <= 24: %q", got, rows[2])
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
