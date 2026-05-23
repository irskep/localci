import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'

import RunSurface from '@/components/RunSurface.vue'
import type { CommitSummary, TaskSummary } from '@/lib/api'

function task(status: string, name = status, failure = ''): TaskSummary {
  return {
    name,
    short_name: name,
    attempt: 1,
    attempt_count: 1,
    status,
    duration_ms: 0,
    failure,
  }
}

function run(tasks: TaskSummary[]): CommitSummary {
  return {
    repo: { repo_dir: '/repo', repo_path: 'cli/localci' },
    commit: 'abc123def456',
    annotations: {},
    tasks,
    activity_at: '2026-05-22T12:00:00Z',
  }
}

function mountRunSurface(tasks: TaskSummary[]) {
  return mount(RunSurface, {
    props: {
      run: run(tasks),
      repoPath: 'cli/localci',
    },
    global: {
      stubs: {
        PMeterGroup: {
          props: ['value', 'max'],
          template: '<div class="meter" :data-max="max">{{ JSON.stringify(value) }}</div>',
        },
        PTag: {
          props: ['value'],
          template: '<span class="tag">{{ value }}</span>',
        },
        RepoLink: {
          props: ['repoPath'],
          template: '<a>{{ repoPath }}</a>',
        },
        RouterLink: {
          props: ['to'],
          template: '<a><slot /></a>',
        },
        RunLink: {
          props: ['commit'],
          template: '<a>{{ commit }}</a>',
        },
      },
    },
  })
}

describe('RunSurface', () => {
  it('does not show progress for completed runs', () => {
    const wrapper = mountRunSurface([task('succeeded'), task('failed', 'lint', 'exit')])

    expect(wrapper.find('.meter').exists()).toBe(false)
  })

  it('shows MeterGroup progress for runs with incomplete tasks', () => {
    const wrapper = mountRunSurface([
      task('succeeded', 'build'),
      task('running', 'test'),
      task('queued', 'lint'),
      task('not-run', 'fmt'),
    ])

    const meter = wrapper.find('.meter')
    expect(meter.exists()).toBe(true)
    expect(meter.attributes('data-max')).toBe('4')
    expect(meter.text()).toContain('"label":"passed","value":1')
    expect(meter.text()).toContain('"label":"running","value":1')
    expect(meter.text()).toContain('"label":"queued","value":1')
    expect(meter.text()).toContain('"label":"not run","value":1')
    expect(wrapper.text()).toContain('1/4 complete')
  })
})
