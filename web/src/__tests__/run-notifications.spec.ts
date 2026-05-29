import { describe, expect, it } from 'vitest'

import type { CommitSummary, TaskSummary } from '@/lib/api'
import {
  notificationForRun,
  runHasLiveTasks,
  runIsComplete,
  runNotificationStatus,
} from '@/lib/run-notifications'

function task(status: string, failure = ''): TaskSummary {
  return {
    name: status,
    short_name: status,
    attempt: 1,
    attempt_count: 1,
    status,
    duration_ms: 0,
    failure,
  }
}

function run(tasks: TaskSummary[]): CommitSummary {
  return {
    repo: { repo_dir: '/repo', repo_path: 'cli/localci', repo_label: 'cli/localci' },
    commit: '3a974feb300e734293a95d7bea0809c11293f2c9',
    tasks,
    activity_at: '2026-05-29T12:00:00Z',
  }
}

describe('run notification helpers', () => {
  it('treats queued and running runs as incomplete', () => {
    expect(runHasLiveTasks(run([task('queued')]))).toBe(true)
    expect(runHasLiveTasks(run([task('running')]))).toBe(true)
    expect(runIsComplete(run([task('running')]))).toBe(false)
  })

  it('treats not-run tasks as complete', () => {
    expect(runIsComplete(run([task('not-run')]))).toBe(true)
    expect(runNotificationStatus(run([task('not-run')]))).toBe('not run')
  })

  it('uses the shortest completed status label', () => {
    expect(runNotificationStatus(run([task('succeeded')]))).toBe('passed')
    expect(runNotificationStatus(run([task('failed', 'exit')]))).toBe('failed')
    expect(runNotificationStatus(run([task('timed-out')]))).toBe('failed')
    expect(runNotificationStatus(run([task('failed', 'canceled')]))).toBe('canceled')
  })

  it('formats notification title and body', () => {
    expect(notificationForRun(run([task('succeeded')]))).toMatchObject({
      key: 'cli/localci:3a974feb300e734293a95d7bea0809c11293f2c9',
      title: 'LocalCI: passed',
      body: 'cli/localci 3a974feb300e',
    })
  })
})
