import type { CommitSummary, TaskStatusLike } from '@/lib/api'
import { displayTaskStatus, shortCommit } from '@/lib/api'

export type RunNotificationStatus = 'passed' | 'failed' | 'canceled' | 'not run'

export type RunNotification = {
  key: string
  title: string
  body: string
  status: RunNotificationStatus
}

export function runHasLiveTasks(run: { tasks: TaskStatusLike[] }): boolean {
  return run.tasks.some((task) => {
    const status = displayTaskStatus(task)
    return status === 'queued' || status === 'running'
  })
}

export function runIsComplete(run: { tasks: TaskStatusLike[] }): boolean {
  return !runHasLiveTasks(run)
}

export function runNotificationStatus(run: { tasks: TaskStatusLike[] }): RunNotificationStatus {
  if (
    run.tasks.some((task) => {
      const status = displayTaskStatus(task)
      return status === 'failed' || status === 'timed-out'
    })
  ) {
    return 'failed'
  }
  if (run.tasks.some((task) => displayTaskStatus(task) === 'canceled')) return 'canceled'
  if (run.tasks.some((task) => displayTaskStatus(task) === 'succeeded')) return 'passed'
  return 'not run'
}

export function notificationForRun(run: CommitSummary): RunNotification {
  const status = runNotificationStatus(run)
  return {
    key: `${run.repo.repo_path}:${run.commit}`,
    title: `LocalCI: ${status}`,
    body: `${run.repo.repo_label} ${shortCommit(run.commit)}`,
    status,
  }
}
