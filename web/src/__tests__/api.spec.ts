import { describe, expect, it } from 'vitest'

import {
  displayStatusSeverity,
  displayTaskFailure,
  displayTaskStatus,
  parseAPIEvent,
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
