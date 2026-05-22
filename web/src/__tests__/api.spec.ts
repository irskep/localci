import { describe, expect, it } from 'vitest'

import { parseAPIEvent, parseTaskResponse } from '@/lib/api'

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
      repo: { repo_dir: '/repo', repo_path: 'repo' },
      commit: 'abc123',
      task: {
        name: 'localci:test',
        short_name: 'test',
        attempt: 1,
        attempt_count: 1,
        status: 'succeeded',
        failure: '',
        duration_ms: 10,
        artifacts: [{ display_name: 'combined.log', path: '/tmp/combined.log' }],
        attempts: [{ attempt: 1, status: 'succeeded', failure: '', duration_ms: 10 }],
      },
      selected_attempt: 1,
      is_live: false,
      primary_artifact: 'combined.log',
      primary_log: 'ok',
    })

    expect(response.task.artifacts).toEqual([{ display_name: 'combined.log' }])
  })
})
