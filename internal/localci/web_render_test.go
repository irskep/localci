package localci

import (
	"bytes"
	"strings"
	"testing"
)

func TestCommitTemplateEmbedsWebSocketParamsWithoutExtraQuotes(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := commitTemplate.Execute(&out, CommitPageView{
		CommitStatusView: CommitStatusView{
			RepoDir: "/repo",
			Commit:  "abc123",
		},
		TasksHTML: "",
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, `url.searchParams.set("repo", "/repo");`) {
		t.Fatalf("commit template repo param rendered incorrectly: %s", rendered)
	}
	if !strings.Contains(rendered, `url.searchParams.set("commit", "abc123");`) {
		t.Fatalf("commit template commit param rendered incorrectly: %s", rendered)
	}
}

func TestTaskTemplateEmbedsWebSocketParamsWithoutExtraQuotes(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	err := taskTemplate.Execute(&out, TaskPageView{
		RepoDir: "/repo",
		Commit:  "abc123",
		IsLive:  true,
		TaskStatusView: TaskStatusView{
			Name: "localci:test",
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}

	rendered := out.String()
	if !strings.Contains(rendered, `url.searchParams.set("repo", "/repo");`) {
		t.Fatalf("task template repo param rendered incorrectly: %s", rendered)
	}
	if !strings.Contains(rendered, `url.searchParams.set("commit", "abc123");`) {
		t.Fatalf("task template commit param rendered incorrectly: %s", rendered)
	}
	if !strings.Contains(rendered, `const taskName = "localci:test";`) {
		t.Fatalf("task template taskName rendered incorrectly: %s", rendered)
	}
}
