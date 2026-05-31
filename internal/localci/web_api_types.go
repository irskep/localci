package localci

import "time"

type apiRepoSummary struct {
	RepoDir   string `json:"repo_dir"`
	RepoPath  string `json:"repo_path"`
	RepoLabel string `json:"repo_label"`
}

type apiCommitSummary struct {
	Repo        apiRepoSummary    `json:"repo"`
	Commit      string            `json:"commit"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Tasks       []apiTaskSummary  `json:"tasks"`
	ActivityAt  time.Time         `json:"activity_at"`
}

type apiTaskSummary struct {
	Name                 string          `json:"name"`
	ShortName            string          `json:"short_name"`
	Attempt              int             `json:"attempt"`
	AttemptCount         int             `json:"attempt_count"`
	Status               ExecutionStatus `json:"status"`
	DurationMilliseconds int64           `json:"duration_ms"`
	Failure              string          `json:"failure"`
	Artifacts            []ArtifactView  `json:"artifacts,omitempty"`
}

type apiQueueEntry struct {
	Repo      apiRepoSummary `json:"repo"`
	Commit    string         `json:"commit"`
	Task      string         `json:"task"`
	Attempt   int            `json:"attempt"`
	Artifacts []ArtifactView `json:"artifacts,omitempty"`
}

type apiQueueResponse struct {
	Active  *apiQueueEntry  `json:"active,omitempty"`
	Pending []apiQueueEntry `json:"pending"`
}

type apiHomeResponse struct {
	Repos         []apiRepoSummary   `json:"repos"`
	RecentCommits []apiCommitSummary `json:"recent_commits"`
	Queue         apiQueueResponse   `json:"queue"`
	NextBefore    string             `json:"next_before,omitempty"`
	NewerBefore   string             `json:"newer_before,omitempty"`
}

type apiRepoResponse struct {
	Repo        apiRepoSummary     `json:"repo"`
	Commits     []apiCommitSummary `json:"commits"`
	NextBefore  string             `json:"next_before,omitempty"`
	NewerBefore string             `json:"newer_before,omitempty"`
}

type apiRepoTaskHistoryResponse struct {
	Repo      apiRepoSummary           `json:"repo"`
	Task      string                   `json:"task"`
	ShortName string                   `json:"short_name"`
	Runs      []apiRepoTaskHistoryItem `json:"runs"`
}

type apiRepoTaskHistoryItem struct {
	Commit      string            `json:"commit"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Task        apiTaskSummary    `json:"task"`
	ActivityAt  time.Time         `json:"activity_at"`
}

type apiCommitResponse struct {
	Repo   apiRepoSummary   `json:"repo"`
	Commit CommitStatusView `json:"commit"`
}

type apiTaskResponse struct {
	Repo            apiRepoSummary `json:"repo"`
	Commit          string         `json:"commit"`
	Task            TaskStatusView `json:"task"`
	SelectedAttempt int            `json:"selected_attempt"`
	IsLive          bool           `json:"is_live"`
	PrimaryArtifact string         `json:"primary_artifact"`
	PrimaryLog      string         `json:"primary_log"`
}

type apiArtifactListResponse struct {
	Repo      apiRepoSummary `json:"repo"`
	Commit    string         `json:"commit"`
	Task      string         `json:"task"`
	Attempt   int            `json:"attempt"`
	Artifacts []ArtifactView `json:"artifacts"`
}

type apiArtifactResponse struct {
	Repo     apiRepoSummary `json:"repo"`
	Commit   string         `json:"commit"`
	Task     string         `json:"task"`
	Attempt  int            `json:"attempt"`
	Artifact ArtifactView   `json:"artifact"`
	Content  string         `json:"content"`
}

type apiRevealArtifactResponse struct {
	Path string `json:"path"`
	OK   bool   `json:"ok"`
}

type apiRetryResponse struct {
	Repo     apiRepoSummary `json:"repo"`
	Commit   string         `json:"commit"`
	Task     string         `json:"task"`
	Attempt  int            `json:"attempt"`
	URL      string         `json:"url"`
	Enqueued bool           `json:"enqueued"`
}

type apiCancelResponse struct {
	Repo     apiRepoSummary `json:"repo"`
	Commit   string         `json:"commit"`
	Task     string         `json:"task"`
	Active   bool           `json:"active"`
	Pending  int            `json:"pending"`
	Canceled bool           `json:"canceled"`
}
