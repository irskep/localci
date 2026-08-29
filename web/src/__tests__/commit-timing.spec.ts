import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it, vi } from 'vitest'

import CommitTiming from '@/components/CommitTiming.vue'

afterEach(() => {
  vi.useRealTimers()
})

describe('CommitTiming', () => {
  it('ticks duration so far and stops its clock when unmounted', async () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-29T12:00:10Z'))
    const wrapper = mount(CommitTiming, {
      props: { startedAt: '2026-08-29T12:00:00Z' },
    })

    expect(wrapper.text()).toContain('In progress')
    expect(wrapper.text()).toContain('Duration so far')
    expect(wrapper.text()).toContain('10s')
    expect(wrapper.find('time').text()).toBe('just now')
    expect(wrapper.find('time').attributes('title')).toBeTruthy()
    expect(vi.getTimerCount()).toBe(1)

    await vi.advanceTimersByTimeAsync(2000)
    expect(wrapper.text()).toContain('12s')

    wrapper.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('shows relative completed times with exact native tooltips', () => {
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-08-29T12:00:10Z'))
    const wrapper = mount(CommitTiming, {
      props: {
        startedAt: '2026-08-29T12:00:00Z',
        finishedAt: '2026-08-29T12:00:05Z',
      },
    })

    expect(wrapper.text()).toContain('Duration')
    expect(wrapper.text()).not.toContain('Duration so far')
    expect(wrapper.text()).toContain('5s')
    expect(wrapper.text()).not.toContain('In progress')
    expect(wrapper.findAll('time').map((element) => element.text())).toEqual([
      'just now',
      'just now',
    ])
    expect(wrapper.findAll('time').every((element) => Boolean(element.attributes('title')))).toBe(
      true,
    )
    expect(vi.getTimerCount()).toBe(1)

    wrapper.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })
})
