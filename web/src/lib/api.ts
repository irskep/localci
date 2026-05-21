export type RepoSummary = {
  repo_dir: string
  repo_path: string
  repo_name: string
}

export type QueueEntry = {
  repo: RepoSummary
  commit: string
  task: string
}

export type QueueResponse = {
  active?: QueueEntry
  pending: QueueEntry[]
}

export type HomeResponse = {
  repos: RepoSummary[]
  recent_commits: CommitSummary[]
  queue: QueueResponse
}

export type CommitSummary = {
  repo: RepoSummary
  commit: string
  summary: string
  task_count: number
  activity_at: string
}

export type RepoResponse = {
  repo: RepoSummary
  commits: CommitStatusView[]
}

export type CommitResponse = {
  repo: RepoSummary
  commit: CommitStatusView
}

export type TaskResponse = {
  repo: RepoSummary
  commit: string
  task: TaskStatusView
  selected_attempt: number
  is_live: boolean
  primary_artifact: string
  primary_log: string
}

export type ArtifactListResponse = {
  repo: RepoSummary
  commit: string
  task: string
  attempt: number
  artifacts: ArtifactView[]
}

export type ArtifactResponse = {
  repo: RepoSummary
  commit: string
  task: string
  attempt: number
  artifact: ArtifactView
  content: string
}

export type CommitStatusView = {
  repo_dir: string
  commit: string
  tasks: TaskStatusView[]
}

export type TaskStatusView = {
  name: string
  short_name: string
  attempt: number
  attempt_count: number
  status: string
  failure: string
  duration_ms: number
  artifacts: ArtifactView[]
  attempts: TaskAttemptView[]
}

export type TaskAttemptView = {
  attempt: number
  status: string
  failure: string
  duration_ms: number
}

export type ArtifactView = {
  display_name: string
}

export async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path)
  if (!response.ok) {
    const message = await response.text()
    throw new Error(message || `request failed with ${response.status}`)
  }
  return (await response.json()) as T
}

export async function postJSON<T>(path: string): Promise<T> {
  const response = await fetch(path, { method: 'POST' })
  if (!response.ok) {
    const message = await response.text()
    throw new Error(message || `request failed with ${response.status}`)
  }
  return (await response.json()) as T
}

export function summarizeCommit(commit: CommitStatusView): string {
  const total = commit.tasks.length
  const failed = commit.tasks.filter((task) => task.status === 'failed').length
  const running = commit.tasks.filter((task) => task.status === 'running').length
  const queued = commit.tasks.filter((task) => task.status === 'queued').length
  const passed = commit.tasks.filter((task) => task.status === 'succeeded').length

  if (running > 0) return `${running} running, ${passed}/${total} passed`
  if (queued > 0) return `${queued} queued, ${passed}/${total} passed`
  if (failed > 0) return `${failed} failed, ${passed}/${total} passed`
  return `${passed}/${total} passed`
}

export function statusSeverity(
  status: string,
): 'success' | 'info' | 'warn' | 'danger' | 'secondary' {
  switch (status) {
    case 'succeeded':
      return 'success'
    case 'running':
      return 'info'
    case 'queued':
      return 'warn'
    case 'failed':
    case 'timed-out':
      return 'danger'
    default:
      return 'secondary'
  }
}

export function formatDuration(durationMs: number): string {
  if (durationMs <= 0) return ''
  if (durationMs < 1000) return `${durationMs}ms`
  return `${(durationMs / 1000).toFixed(1)}s`
}

export function shortCommit(commit: string): string {
  return commit.length > 12 ? commit.slice(0, 12) : commit
}
