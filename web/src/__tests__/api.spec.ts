import { describe, expect, it } from 'vitest'

import {
  displayStatusSeverity,
  displayTaskFailure,
  displayTaskStatus,
  formatDuration,
  formatElapsedDuration,
  formatRelativeTimestamp,
  formatTimestamp,
  parseAPIEvent,
  parseCommitResponse,
  parseTaskResponse,
  shortCommit,
  summarizeCommit,
} from '@/lib/api'

describe('api validation', () => {
  it('parses valid append events', () => {
    expect(
      parseAPIEvent(
        {
          type: 'append',
          resource: '/api/repo/example',
          offset: 12,
          text: 'hello',
        },
        parseTaskResponse,
      ),
    ).toMatchObject({
      type: 'append',
      offset: 12,
      text: 'hello',
    })
  })

  it('rejects malformed events', () => {
    expect(() =>
      parseAPIEvent(
        {
          type: 'append',
          resource: '/api/repo/example',
          offset: '12',
          text: 'hello',
        },
        parseTaskResponse,
      ),
    ).toThrow(/event.offset/)
  })

  it('accepts task responses without trusting artifact paths', () => {
    const response = parseTaskResponse({
      repo: { repo_dir: '/repo', repo_path: 'repo', repo_label: 'repo' },
      commit: 'abc123',
      task: {
        name: 'localci:test',
        short_name: 'test',
        attempt: 1,
        attempt_count: 1,
        status: 'succeeded',
        failure: '',
        duration_ms: 10,
        artifacts: [
          {
            display_name: 'site/index.html',
            path: '/tmp/site/index.html',
            marked_name: 'docs html',
            action: 'open',
            is_text: true,
            raw_url: '/artifacts/repo/repo/commit/abc/task/localci:test/attempt/1/site/index.html',
            download_url:
              '/artifacts/repo/repo/commit/abc/task/localci:test/attempt/1/site/index.html?download=1',
          },
        ],
        marked_artifacts: [{ name: 'docs html', path: 'site/index.html', action: 'open' }],
        attempts: [{ attempt: 1, status: 'succeeded', failure: '', duration_ms: 10 }],
      },
      selected_attempt: 1,
      is_live: false,
      primary_artifact: 'combined.log',
      primary_log: 'ok',
    })

    expect(response.task.artifacts).toEqual([
      {
        display_name: 'site/index.html',
        path: '/tmp/site/index.html',
        marked_name: 'docs html',
        action: 'open',
        is_text: true,
        raw_url: '/artifacts/repo/repo/commit/abc/task/localci:test/attempt/1/site/index.html',
        download_url:
          '/artifacts/repo/repo/commit/abc/task/localci:test/attempt/1/site/index.html?download=1',
      },
    ])
    expect(response.task.marked_artifacts).toEqual([
      { name: 'docs html', path: 'site/index.html', action: 'open' },
    ])
  })

  it('parses optional commit timing metadata', () => {
    const response = parseCommitResponse({
      repo: { repo_dir: '/repo', repo_path: 'repo', repo_label: 'repo' },
      commit: {
        repo_dir: '/repo',
        commit: 'abc123',
        tasks: [],
        started_at: '2026-08-29T12:00:00Z',
        finished_at: '2026-08-29T12:00:05Z',
      },
    })

    expect(response.commit.started_at).toBe('2026-08-29T12:00:00Z')
    expect(response.commit.finished_at).toBe('2026-08-29T12:00:05Z')
  })

  it('formats completed and live elapsed durations', () => {
    expect(formatElapsedDuration('2026-08-29T12:00:00Z', '2026-08-29T12:00:05Z')).toBe('5s')
    expect(
      formatElapsedDuration('2026-08-29T12:00:00Z', undefined, Date.parse('2026-08-29T12:00:12Z')),
    ).toBe('12s')
    expect(formatElapsedDuration('invalid')).toBe('')
    expect(
      formatRelativeTimestamp('2026-08-29T12:00:00Z', Date.parse('2026-08-29T12:01:00Z')),
    ).toBe('1 minute ago')
    expect(formatTimestamp('invalid')).toBe('')
    expect(formatTimestamp('2026-08-29T12:00:00')).toBe('2026-08-29 12:00:00 pm')
    expect(formatTimestamp('2026-08-29T03:04:05')).toBe('2026-08-29 3:04:05 am')
  })

  it.each([
    [0, ''],
    [1, '<1s'],
    [999, '<1s'],
    [1000, '1s'],
    [59_999, '59s'],
    [60_000, '1m'],
    [121_000, '2m1s'],
    [3_600_000, '1h'],
    [3_721_000, '1h2m'],
    [86_400_000, '1d'],
    [93_600_000, '1d2h'],
  ])('formats %dms as %s', (durationMs, expected) => {
    expect(formatDuration(durationMs)).toBe(expected)
  })

  it('displays canceled task failures as canceled', () => {
    const task = { status: 'failed', failure: 'canceled' }

    expect(displayTaskStatus(task)).toBe('canceled')
    expect(displayTaskFailure(task)).toBe('')
    expect(displayStatusSeverity(task)).toBe('secondary')
  })

  it('keeps non-canceled failures as failed', () => {
    const task = { status: 'failed', failure: 'exit' }

    expect(displayTaskStatus(task)).toBe('failed')
    expect(displayTaskFailure(task)).toBe('exit')
    expect(displayStatusSeverity(task)).toBe('danger')
  })

  it('summarizes canceled tasks separately from failed tasks', () => {
    expect(
      summarizeCommit({
        tasks: [
          { status: 'succeeded', failure: '' },
          { status: 'failed', failure: 'canceled' },
          { status: 'failed', failure: 'exit' },
        ],
      }),
    ).toBe('1 failed, 1 canceled, 1/3 passed')
  })

  it('shortens no-clone commits without counting the marker', () => {
    expect(shortCommit('3a974feb300e734293a95d7bea0809c11293f2c9')).toBe('3a974feb300e')
    expect(shortCommit('3a974feb300e734293a95d7bea0809c11293f2c9*')).toBe('3a974feb300e*')
    expect(shortCommit('abc123*')).toBe('abc123*')
  })
})
