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

function mountRunSurface(
  tasks: TaskSummary[],
  props: Partial<InstanceType<typeof RunSurface>['$props']> = {},
) {
  return mount(RunSurface, {
    props: {
      run: run(tasks),
      repoPath: 'cli/localci',
      ...props,
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
          template: '<a :data-to="to"><slot /></a>',
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

  it('groups high-volume runs by package path', () => {
    const tasks = [
      task('succeeded', '//:localci:setup'),
      task('succeeded', '//apps/app:localci:build'),
      task('succeeded', '//apps/app:localci:lint'),
      task('succeeded', '//apps/app:localci:test'),
      task('succeeded', '//apps/app:localci:typecheck'),
      task('succeeded', '//packages/design-system:localci:test:ui'),
      task('succeeded', '//packages/design-system:localci:typecheck'),
      task('succeeded', '//packages/shared:localci:lint'),
      task('succeeded', '//packages/shared:localci:test'),
      task('succeeded', '//packages/shared:localci:typecheck'),
      task('succeeded', '//packages/testing:localci:test'),
    ].map((item) => ({
      ...item,
      short_name: item.name.replace(':localci:', ':'),
    }))

    const wrapper = mountRunSurface(tasks)

    expect(wrapper.findAll('.run-package')).toHaveLength(5)
    expect(wrapper.findAll('.run-package[open]')).toHaveLength(0)
    expect(wrapper.findAll('.run-package-progress')).toHaveLength(0)
    expect(wrapper.text()).toContain('//apps/app')
    expect(wrapper.text()).toContain('//packages/design-system')
    expect(wrapper.text()).toContain('test:ui')
  })

  it('summarizes high-volume package rows that need attention', () => {
    const tasks = [
      task('succeeded', '//:localci:setup'),
      task('succeeded', '//apps/app:localci:build'),
      task('failed', '//apps/app:localci:lint', 'exit'),
      task('succeeded', '//apps/app:localci:test'),
      task('succeeded', '//apps/app:localci:typecheck'),
      task('running', '//packages/design-system:localci:test:ui'),
      task('queued', '//packages/design-system:localci:typecheck'),
      task('succeeded', '//packages/shared:localci:lint'),
      task('succeeded', '//packages/shared:localci:test'),
      task('succeeded', '//packages/shared:localci:typecheck'),
      task('succeeded', '//packages/testing:localci:test'),
    ].map((item) => ({
      ...item,
      short_name: item.name.replace(':localci:', ':'),
    }))

    const wrapper = mountRunSurface(tasks)

    expect(wrapper.findAll('.run-package[open]')).toHaveLength(0)
    expect(wrapper.findAll('.run-package-progress')).toHaveLength(1)
    expect(wrapper.text()).toContain('failed')
    expect(wrapper.text()).toContain('running')
    expect(wrapper.text()).toContain('1 failed (lint), 3 passed')
    expect(wrapper.text()).toContain('1 running (test:ui), 1 queued (typecheck)')
  })

  it('keeps short completed runs detailed in summary mode', () => {
    const wrapper = mountRunSurface([task('succeeded', 'build'), task('succeeded', 'test')], {
      summaryMode: true,
    })

    expect(wrapper.text()).not.toContain('all passed')
    expect(wrapper.find('.run-status-list').exists()).toBe(true)
    expect(wrapper.findAll('a[data-to]')).toHaveLength(2)
  })

  it('collapses high-volume successful runs in summary mode', () => {
    const tasks = Array.from({ length: 11 }, (_, index) => {
      const item = task('succeeded', `//packages/pkg-${index}:localci:test`)
      return { ...item, short_name: item.name.replace(':localci:', ':') }
    })

    const wrapper = mountRunSurface(tasks, { summaryMode: true })

    expect(wrapper.text()).toContain('all passed')
    expect(wrapper.find('.run-task-icon-success').exists()).toBe(true)
    expect(wrapper.find('.run-status-list').exists()).toBe(false)
    expect(wrapper.find('.run-summary-disclosure').exists()).toBe(true)
    expect(wrapper.find('.run-package-list').exists()).toBe(true)
    expect(wrapper.find('a[data-to="/repo/cli/localci/commit/abc123def456"]').exists()).toBe(true)
  })

  it('summarizes completed failures in summary mode', () => {
    const tasks = [
      task('failed', '//:localci:noisy-fail', 'exit'),
      task('succeeded', '//:localci:build'),
      task('failed', '//web:localci:lint', 'exit'),
      task('failed', '//docs:localci:build', 'exit'),
      task('failed', '//packages/core:localci:test', 'exit'),
      task('succeeded', '//packages/a:localci:test'),
      task('succeeded', '//packages/b:localci:test'),
      task('succeeded', '//packages/c:localci:test'),
      task('succeeded', '//packages/d:localci:test'),
      task('succeeded', '//packages/e:localci:test'),
      task('succeeded', '//packages/f:localci:test'),
    ].map((item) => ({
      ...item,
      short_name: item.name.replace(':localci:', ':'),
    }))

    const wrapper = mountRunSurface(tasks, { summaryMode: true })
    const text = wrapper.text().replace(/\s+/g, ' ')

    expect(text).toContain(
      '4 packages with issues: root (noisy-fail), //web (lint), //docs (build), +1 more',
    )
    expect(wrapper.find('.run-status-list').exists()).toBe(false)
    expect(wrapper.find('.run-summary-disclosure').exists()).toBe(true)
    expect(wrapper.find('.run-package-list').exists()).toBe(true)
    expect(wrapper.find('.run-summary').findAll('a[data-to]')).toHaveLength(3)
    expect(wrapper.find('a[data-to*="noisy-fail"]').exists()).toBe(true)
  })
})
