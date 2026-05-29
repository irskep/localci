import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import type { HomeResponse, TaskSummary } from '@/lib/api'
import { useNotificationStore } from '@/stores/notifications'

class FakeNotification {
  static permission: NotificationPermission = 'default'
  static instances: Array<{ title: string; options?: NotificationOptions }> = []

  static async requestPermission(): Promise<NotificationPermission> {
    return FakeNotification.permission
  }

  constructor(title: string, options?: NotificationOptions) {
    FakeNotification.instances.push({ title, options })
  }
}

class FakeWebSocket {
  static instances: FakeWebSocket[] = []

  readonly listeners = new Map<string, Array<(event: Event) => void>>()
  readonly url: string

  constructor(url: string | URL) {
    this.url = String(url)
    FakeWebSocket.instances.push(this)
  }

  addEventListener(type: string, listener: (event: Event) => void): void {
    this.listeners.set(type, [...(this.listeners.get(type) ?? []), listener])
  }

  close(): void {
    this.dispatch('close', new Event('close'))
  }

  message(data: unknown): void {
    this.dispatch('message', new MessageEvent('message', { data: JSON.stringify(data) }))
  }

  private dispatch(type: string, event: Event): void {
    for (const listener of this.listeners.get(type) ?? []) listener(event)
  }
}

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

function home(tasks: TaskSummary[]): HomeResponse {
  return {
    repos: [{ repo_dir: '/repo', repo_path: 'cli/localci' }],
    recent_commits: [
      {
        repo: { repo_dir: '/repo', repo_path: 'cli/localci' },
        commit: 'abc123',
        tasks,
        activity_at: '2026-05-29T12:00:00Z',
      },
    ],
    queue: { pending: [] },
  }
}

describe('notification store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    FakeNotification.permission = 'granted'
    FakeNotification.instances = []
    FakeWebSocket.instances = []
    window.localStorage.clear()
    vi.stubGlobal('Notification', FakeNotification)
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('does not notify for an initial completed snapshot', () => {
    const store = useNotificationStore()

    store.applyHome(home([task('succeeded')]))

    expect(FakeNotification.instances).toEqual([])
  })

  it('notifies once when a seen-live run completes', () => {
    const store = useNotificationStore()

    store.applyHome(home([task('running')]))
    store.applyHome(home([task('succeeded')]))
    store.applyHome(home([task('succeeded')]))

    expect(FakeNotification.instances).toEqual([
      {
        title: 'LocalCI: passed',
        options: { body: 'cli/localci abc123' },
      },
    ])
  })

  it('monitors home websocket replacements for run completion', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise(() => {})),
    )
    const store = useNotificationStore()

    store.startRunMonitor()
    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(FakeWebSocket.instances[0]?.url).toBe('ws://localhost:3000/api/events')

    FakeWebSocket.instances[0]?.message({
      type: 'snapshot',
      resource: '/api',
      data: home([task('running')]),
    })
    FakeWebSocket.instances[0]?.message({
      type: 'replace',
      resource: '/api',
      data: home([task('succeeded')]),
    })

    expect(FakeNotification.instances).toHaveLength(1)
    expect(FakeNotification.instances[0]?.title).toBe('LocalCI: passed')
  })

  it('does not notify when permission is not granted', () => {
    FakeNotification.permission = 'default'
    const store = useNotificationStore()

    store.applyHome(home([task('queued')]))
    store.applyHome(home([task('succeeded')]))

    expect(FakeNotification.instances).toEqual([])
  })

  it('does not notify when the granted toggle is off', () => {
    const store = useNotificationStore()

    store.setEnabled(false)
    store.applyHome(home([task('queued')]))
    store.applyHome(home([task('failed', 'exit')]))

    expect(FakeNotification.instances).toEqual([])
  })

  it('reports denied permission as disabled', () => {
    FakeNotification.permission = 'denied'
    const store = useNotificationStore()

    expect(store.permission).toBe('denied')
    expect(store.label).toBe('notifications disabled')
    expect(store.canNotify).toBe(false)
  })
})
