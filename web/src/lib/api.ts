export type RepoSummary = {
  repo_dir: string
  repo_path: string
}

export type QueueEntry = {
  repo: RepoSummary
  commit: string
  task: string
  attempt: number
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
  annotations?: Record<string, string>
  tasks: TaskSummary[]
  activity_at: string
}

export type TaskSummary = {
  name: string
  short_name: string
  attempt: number
  attempt_count: number
  status: string
  duration_ms: number
  failure: string
}

export type RepoResponse = {
  repo: RepoSummary
  commits: CommitSummary[]
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

export type RetryResponse = {
  repo: RepoSummary
  commit: string
  task: string
  attempt: number
  url: string
  enqueued: boolean
}

export type CancelResponse = {
  repo: RepoSummary
  commit: string
  task: string
  active: boolean
  pending: number
  canceled: boolean
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

export type APIEvent<T = unknown> =
  | { type: 'snapshot'; resource: string; data: T }
  | { type: 'replace'; resource: string; data: T }
  | { type: 'append'; resource: string; offset: number; text: string }
  | { type: 'remove'; resource: string }
  | { type: 'error'; resource: string; message: string }

export type CommitStatusView = {
  repo_dir: string
  commit: string
  annotations?: Record<string, string>
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
  path: string
}

type Validator<T> = (value: unknown) => T

export async function getJSON<T>(path: string, validate?: Validator<T>): Promise<T> {
  const response = await fetch(path)
  if (!response.ok) {
    const message = await response.text()
    throw new Error(message || `request failed with ${response.status}`)
  }
  const data = (await response.json()) as unknown
  return validate ? validate(data) : (data as T)
}

export async function postJSON<T>(path: string, validate?: Validator<T>): Promise<T> {
  const response = await fetch(path, { method: 'POST' })
  if (!response.ok) {
    const message = await response.text()
    throw new Error(message || `request failed with ${response.status}`)
  }
  const data = (await response.json()) as unknown
  return validate ? validate(data) : (data as T)
}

export function parseAPIEvent<T>(value: unknown, validateData: Validator<T>): APIEvent<T> {
  const event = asObject(value, 'event')
  const type = asString(event.type, 'event.type')
  const resource = asString(event.resource, 'event.resource')
  switch (type) {
    case 'snapshot':
    case 'replace':
      return { type, resource, data: validateData(event.data) }
    case 'append':
      return {
        type,
        resource,
        offset: asNumber(event.offset, 'event.offset'),
        text: asString(event.text, 'event.text'),
      }
    case 'remove':
      return { type, resource }
    case 'error':
      return { type, resource, message: asString(event.message, 'event.message') }
    default:
      throw new Error(`unsupported event type: ${type}`)
  }
}

export function parseHomeResponse(value: unknown): HomeResponse {
  const data = asObject(value, 'home')
  return {
    repos: asArray(data.repos, 'home.repos').map(parseRepoSummary),
    recent_commits: asArray(data.recent_commits, 'home.recent_commits').map(parseCommitSummary),
    queue: parseQueueResponse(data.queue),
  }
}

export function parseQueueResponse(value: unknown): QueueResponse {
  const data = asObject(value, 'queue')
  return {
    active: data.active === undefined ? undefined : parseQueueEntry(data.active),
    pending: asArray(data.pending, 'queue.pending').map(parseQueueEntry),
  }
}

export function parseRepoResponse(value: unknown): RepoResponse {
  const data = asObject(value, 'repo response')
  return {
    repo: parseRepoSummary(data.repo),
    commits: asArray(data.commits, 'repo.commits').map(parseCommitSummary),
  }
}

export function parseCommitResponse(value: unknown): CommitResponse {
  const data = asObject(value, 'commit response')
  return {
    repo: parseRepoSummary(data.repo),
    commit: parseCommitStatusView(data.commit),
  }
}

export function parseTaskResponse(value: unknown): TaskResponse {
  const data = asObject(value, 'task response')
  return {
    repo: parseRepoSummary(data.repo),
    commit: asString(data.commit, 'task.commit'),
    task: parseTaskStatusView(data.task),
    selected_attempt: asNumber(data.selected_attempt, 'task.selected_attempt'),
    is_live: asBoolean(data.is_live, 'task.is_live'),
    primary_artifact: asString(data.primary_artifact, 'task.primary_artifact'),
    primary_log: asString(data.primary_log, 'task.primary_log'),
  }
}

export function parseRetryResponse(value: unknown): RetryResponse {
  const data = asObject(value, 'retry response')
  return {
    repo: parseRepoSummary(data.repo),
    commit: asString(data.commit, 'retry.commit'),
    task: asString(data.task, 'retry.task'),
    attempt: asNumber(data.attempt, 'retry.attempt'),
    url: asString(data.url, 'retry.url'),
    enqueued: asBoolean(data.enqueued, 'retry.enqueued'),
  }
}

export function parseCancelResponse(value: unknown): CancelResponse {
  const data = asObject(value, 'cancel response')
  return {
    repo: parseRepoSummary(data.repo),
    commit: asString(data.commit, 'cancel.commit'),
    task: asString(data.task, 'cancel.task'),
    active: asBoolean(data.active, 'cancel.active'),
    pending: asNumber(data.pending, 'cancel.pending'),
    canceled: asBoolean(data.canceled, 'cancel.canceled'),
  }
}

export function parseArtifactListResponse(value: unknown): ArtifactListResponse {
  const data = asObject(value, 'artifact list')
  return {
    repo: parseRepoSummary(data.repo),
    commit: asString(data.commit, 'artifact_list.commit'),
    task: asString(data.task, 'artifact_list.task'),
    attempt: asNumber(data.attempt, 'artifact_list.attempt'),
    artifacts: asArray(data.artifacts, 'artifact_list.artifacts').map(parseArtifactView),
  }
}

export function parseArtifactResponse(value: unknown): ArtifactResponse {
  const data = asObject(value, 'artifact response')
  return {
    repo: parseRepoSummary(data.repo),
    commit: asString(data.commit, 'artifact.commit'),
    task: asString(data.task, 'artifact.task'),
    attempt: asNumber(data.attempt, 'artifact.attempt'),
    artifact: parseArtifactView(data.artifact),
    content: asString(data.content, 'artifact.content'),
  }
}

function parseRepoSummary(value: unknown): RepoSummary {
  const data = asObject(value, 'repo')
  return {
    repo_dir: asString(data.repo_dir, 'repo.repo_dir'),
    repo_path: asString(data.repo_path, 'repo.repo_path'),
  }
}

function parseQueueEntry(value: unknown): QueueEntry {
  const data = asObject(value, 'queue entry')
  return {
    repo: parseRepoSummary(data.repo),
    commit: asString(data.commit, 'queue.commit'),
    task: asString(data.task, 'queue.task'),
    attempt: asNumber(data.attempt, 'queue.attempt'),
  }
}

function parseCommitSummary(value: unknown): CommitSummary {
  const data = asObject(value, 'commit summary')
  return {
    repo: parseRepoSummary(data.repo),
    commit: asString(data.commit, 'commit.commit'),
    annotations: parseOptionalStringRecord(data.annotations, 'commit.annotations'),
    tasks: asArray(data.tasks, 'commit.tasks').map(parseTaskSummary),
    activity_at: asString(data.activity_at, 'commit.activity_at'),
  }
}

function parseTaskSummary(value: unknown): TaskSummary {
  const data = asObject(value, 'task summary')
  return {
    name: asString(data.name, 'task.name'),
    short_name: asString(data.short_name, 'task.short_name'),
    attempt: asNumber(data.attempt, 'task.attempt'),
    attempt_count: asNumber(data.attempt_count, 'task.attempt_count'),
    status: asString(data.status, 'task.status'),
    duration_ms: asNumber(data.duration_ms, 'task.duration_ms'),
    failure: asString(data.failure, 'task.failure'),
  }
}

function parseCommitStatusView(value: unknown): CommitStatusView {
  const data = asObject(value, 'commit status')
  return {
    repo_dir: asString(data.repo_dir, 'commit_status.repo_dir'),
    commit: asString(data.commit, 'commit_status.commit'),
    annotations: parseOptionalStringRecord(data.annotations, 'commit_status.annotations'),
    tasks: asArray(data.tasks, 'commit_status.tasks').map(parseTaskStatusView),
  }
}

function parseTaskStatusView(value: unknown): TaskStatusView {
  const data = asObject(value, 'task status')
  return {
    name: asString(data.name, 'task_status.name'),
    short_name: asString(data.short_name, 'task_status.short_name'),
    attempt: asNumber(data.attempt, 'task_status.attempt'),
    attempt_count: asNumber(data.attempt_count, 'task_status.attempt_count'),
    status: asString(data.status, 'task_status.status'),
    failure: asString(data.failure, 'task_status.failure'),
    duration_ms: asNumber(data.duration_ms, 'task_status.duration_ms'),
    artifacts: asArray(data.artifacts, 'task_status.artifacts').map(parseArtifactView),
    attempts: asArray(data.attempts, 'task_status.attempts').map(parseTaskAttemptView),
  }
}

function parseTaskAttemptView(value: unknown): TaskAttemptView {
  const data = asObject(value, 'task attempt')
  return {
    attempt: asNumber(data.attempt, 'task_attempt.attempt'),
    status: asString(data.status, 'task_attempt.status'),
    failure: asString(data.failure, 'task_attempt.failure'),
    duration_ms: asNumber(data.duration_ms, 'task_attempt.duration_ms'),
  }
}

function parseArtifactView(value: unknown): ArtifactView {
  const data = asObject(value, 'artifact')
  return {
    display_name: asString(data.display_name, 'artifact.display_name'),
    path: asString(data.path, 'artifact.path'),
  }
}

function parseOptionalStringRecord(
  value: unknown,
  name: string,
): Record<string, string> | undefined {
  if (value === undefined || value === null) return undefined
  const data = asObject(value, name)
  const result: Record<string, string> = {}
  for (const [key, entry] of Object.entries(data)) {
    result[key] = asString(entry, `${name}.${key}`)
  }
  return result
}

function asObject(value: unknown, name: string): Record<string, unknown> {
  if (typeof value !== 'object' || value === null || Array.isArray(value)) {
    throw new Error(`${name} must be an object`)
  }
  return value as Record<string, unknown>
}

function asArray(value: unknown, name: string): unknown[] {
  if (!Array.isArray(value)) throw new Error(`${name} must be an array`)
  return value
}

function asString(value: unknown, name: string): string {
  if (typeof value !== 'string') throw new Error(`${name} must be a string`)
  return value
}

function asNumber(value: unknown, name: string): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    throw new Error(`${name} must be a finite number`)
  }
  return value
}

function asBoolean(value: unknown, name: string): boolean {
  if (typeof value !== 'boolean') throw new Error(`${name} must be a boolean`)
  return value
}

export type TaskStatusLike = {
  status: string
  failure?: string
}

export function displayTaskStatus(task: TaskStatusLike): string {
  if (task.status === 'failed' && task.failure === 'canceled') return 'canceled'
  return task.status
}

export function displayTaskFailure(task: TaskStatusLike): string {
  if (displayTaskStatus(task) === 'canceled') return ''
  return task.failure ?? ''
}

export function summarizeCommit(commit: { tasks: TaskStatusLike[] }): string {
  const total = commit.tasks.length
  const failed = commit.tasks.filter(
    (task) => displayTaskStatus(task) === 'failed' || displayTaskStatus(task) === 'timed-out',
  ).length
  const canceled = commit.tasks.filter((task) => displayTaskStatus(task) === 'canceled').length
  const running = commit.tasks.filter((task) => task.status === 'running').length
  const queued = commit.tasks.filter((task) => task.status === 'queued').length
  const notRun = commit.tasks.filter((task) => task.status === 'not-run').length
  const passed = commit.tasks.filter((task) => task.status === 'succeeded').length

  const parts: string[] = []
  if (failed > 0) parts.push(`${failed} failed`)
  if (canceled > 0) parts.push(`${canceled} canceled`)
  if (running > 0) parts.push(`${running} running`)
  if (queued > 0) parts.push(`${queued} queued`)
  if (notRun > 0) parts.push(`${notRun} not run`)
  parts.push(`${passed}/${total} passed`)
  return parts.join(', ')
}

export type TaskStatusGroup<T extends TaskStatusLike> = {
  label: string
  tasks: T[]
}

const TASK_STATUS_GROUPS = [
  'failed',
  'canceled',
  'running',
  'queued',
  'not-run',
  'succeeded',
] as const

export function taskStatusGroups<T extends TaskStatusLike>(tasks: T[]): Array<TaskStatusGroup<T>> {
  return TASK_STATUS_GROUPS.map((status) => ({
    label: status === 'not-run' ? 'not run' : status === 'succeeded' ? 'passed' : status,
    tasks: tasks.filter((task) => {
      const displayStatus = displayTaskStatus(task)
      if (status === 'failed') return displayStatus === 'failed' || displayStatus === 'timed-out'
      return displayStatus === status
    }),
  })).filter((group) => group.tasks.length > 0)
}

export function taskStatusIcon(task: TaskStatusLike): string {
  switch (displayTaskStatus(task)) {
    case 'succeeded':
      return 'pi pi-check run-task-icon run-task-icon-success'
    case 'failed':
    case 'timed-out':
      return 'pi pi-times run-task-icon run-task-icon-failed'
    case 'canceled':
      return 'pi pi-stop run-task-icon run-task-icon-canceled'
    case 'running':
      return 'pi pi-spin pi-spinner run-task-icon run-task-icon-running'
    case 'queued':
      return 'pi pi-clock run-task-icon run-task-icon-queued'
    default:
      return 'pi pi-circle run-task-icon run-task-icon-muted'
  }
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

export function displayStatusSeverity(
  task: TaskStatusLike,
): 'success' | 'info' | 'warn' | 'danger' | 'secondary' {
  return statusSeverity(displayTaskStatus(task))
}

export function formatDuration(durationMs: number): string {
  if (durationMs <= 0) return ''
  if (durationMs < 1000) return `${durationMs}ms`
  return `${(durationMs / 1000).toFixed(1)}s`
}

export function shortCommit(commit: string): string {
  return commit.length > 12 ? commit.slice(0, 12) : commit
}

export function formatAnnotations(annotations?: Record<string, string>): string {
  return annotationEntries(annotations)
    .map(({ key, value }) => `${key}: ${value}`)
    .join(', ')
}

export function annotationEntries(
  annotations?: Record<string, string>,
): Array<{ key: string; value: string }> {
  if (!annotations) return []
  return Object.entries(annotations)
    .sort(([left], [right]) => left.localeCompare(right))
    .map(([key, value]) => ({ key, value }))
}
