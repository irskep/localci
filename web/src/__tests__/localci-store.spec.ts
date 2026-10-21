import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useLocalciStore } from '@/stores/localci'

class FakeWebSocket {
  static instances: FakeWebSocket[] = []

  readonly listeners = new Map<string, Array<(event: Event) => void>>()
  readonly url: string

  constructor(url: string) {
    this.url = url
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

describe('localci store', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('refreshes a same-page subscription after route setup resets page state', () => {
    const store = useLocalciStore()
    const snapshot = {
      type: 'snapshot',
      resource: '/api/repo/cli/localci',
      data: {
        repo: { repo_dir: '/repo', repo_path: 'cli/localci', repo_label: 'cli/localci' },
        commits: [],
      },
    }

    store.subscribeRepo('/api/repo/cli/localci')
    FakeWebSocket.instances[0]?.message(snapshot)
    expect(store.repoLoaded).toBe(true)
    expect(store.currentRepo?.repo.repo_path).toBe('cli/localci')

    store.subscribeRepo('/api/repo/cli/localci')
    expect(store.repoLoaded).toBe(false)
    expect(store.currentRepo).toBeNull()
    expect(FakeWebSocket.instances).toHaveLength(2)

    FakeWebSocket.instances[1]?.message(snapshot)
    expect(store.repoLoaded).toBe(true)
    expect(store.currentRepo?.repo.repo_path).toBe('cli/localci')
  })

  it('loads repo subscriptions from HTTP while waiting for the websocket snapshot', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        ok: true,
        json: async () => ({
          repo: { repo_dir: '/repo', repo_path: 'cli/localci', repo_label: 'cli/localci' },
          commits: [],
        }),
      })),
    )
    const store = useLocalciStore()

    store.subscribeRepo('/api/repo/cli/localci')

    await vi.advanceTimersByTimeAsync(1500)
    expect(store.repoLoaded).toBe(true)
    expect(store.currentRepo?.repo.repo_path).toBe('cli/localci')
    expect(store.loading).toBe(false)
    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(fetch).toHaveBeenCalledWith('/api/repo/cli/localci', { signal: expect.any(AbortSignal) })
  })

  it('keeps newer page subscriptions when stale routes unmount', () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise(() => {})),
    )
    const store = useLocalciStore()
    const snapshot = {
      type: 'snapshot',
      resource: '/api/repo/cli/localci',
      data: {
        repo: { repo_dir: '/repo', repo_path: 'cli/localci', repo_label: 'cli/localci' },
        commits: [],
      },
    }

    store.subscribeCommit('/api/repo/cli/localci/commit/abc123')
    store.subscribeRepo('/api/repo/cli/localci')
    store.unsubscribePage('/api/repo/cli/localci/commit/abc123')
    store.unsubscribePage('')
    FakeWebSocket.instances[1]?.message(snapshot)

    expect(store.repoLoaded).toBe(true)
    expect(store.currentRepo?.repo.repo_path).toBe('cli/localci')
  })

  it('clears transient page load errors after the websocket recovers', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => {
        throw new Error('Load failed')
      }),
    )
    const store = useLocalciStore()
    const snapshot = {
      type: 'snapshot',
      resource: '/api/repo/cli/localci',
      data: {
        repo: { repo_dir: '/repo', repo_path: 'cli/localci', repo_label: 'cli/localci' },
        commits: [],
      },
    }

    store.subscribeRepo('/api/repo/cli/localci')
    await vi.advanceTimersByTimeAsync(1500)
    expect(store.error).toBe('Load failed')
    FakeWebSocket.instances[0]?.message(snapshot)

    expect(store.repoLoaded).toBe(true)
    expect(store.currentRepo?.repo.repo_path).toBe('cli/localci')
    expect(store.error).toBe('')
  })
})

function taskSnapshot(text = '') {
  return {
    repo: { repo_dir: '/repo', repo_path: 'repo', repo_label: 'repo' },
    commit: 'abc',
    task: {
      name: 'localci:test',
      short_name: 'test',
      attempt: 1,
      attempt_count: 1,
      status: 'running',
      failure: '',
      duration_ms: 0,
      artifacts: [],
      attempts: [],
    },
    selected_attempt: 1,
    is_live: true,
    primary_artifact: 'combined.log',
    primary_log: text,
    primary_log_end: new TextEncoder().encode(text).length,
    primary_log_truncated: false,
  }
}

describe('task stream recovery', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setActivePinia(createPinia())
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
    vi.stubGlobal('fetch', vi.fn())
  })
  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('appends Unicode using byte positions and ignores snapshot overlap', async () => {
    const store = useLocalciStore()
    store.subscribeTask('/api/task')
    const socket = FakeWebSocket.instances[0]!
    socket.message({ type: 'snapshot', resource: '/api/task', data: taskSnapshot('🙂') })
    socket.message({ type: 'append', resource: '/api/task', offset: 0, end_offset: 4, text: '🙂' })
    socket.message({ type: 'append', resource: '/api/task', offset: 4, end_offset: 6, text: 'é' })
    expect(store.currentTask?.primary_log).toBe('🙂é')
    expect(store.currentTask?.primary_log_end).toBe(6)
    await vi.advanceTimersByTimeAsync(1500)
    expect(fetch).not.toHaveBeenCalled()
    store.unsubscribeTask()
  })

  it('reconnects once on a gap instead of issuing recovery requests per append', async () => {
    const store = useLocalciStore()
    store.subscribeTask('/api/task')
    const socket = FakeWebSocket.instances[0]!
    socket.message({ type: 'snapshot', resource: '/api/task', data: taskSnapshot() })
    for (let n = 10; n < 30; n++)
      socket.message({
        type: 'append',
        resource: '/api/task',
        offset: n,
        end_offset: n + 1,
        text: 'x',
      })
    await vi.advanceTimersByTimeAsync(250)
    expect(FakeWebSocket.instances).toHaveLength(2)
    expect(fetch).not.toHaveBeenCalled()
    store.unsubscribeTask()
  })

  it('ignores a slow HTTP fallback after streamed data arrives', async () => {
    let resolve!: (value: unknown) => void
    vi.stubGlobal(
      'fetch',
      vi.fn(
        () =>
          new Promise((r) => {
            resolve = r
          }),
      ),
    )
    const store = useLocalciStore()
    store.subscribeTask('/api/task')
    await vi.advanceTimersByTimeAsync(1500)
    FakeWebSocket.instances[0]!.message({
      type: 'snapshot',
      resource: '/api/task',
      data: taskSnapshot('new'),
    })
    resolve({ ok: true, json: async () => taskSnapshot('old') })
    await vi.advanceTimersByTimeAsync(0)
    expect(store.currentTask?.primary_log).toBe('new')
    store.unsubscribeTask()
  })

  it('shows malformed snapshots and load deadlines on the task page', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(() => new Promise(() => {})),
    )
    const store = useLocalciStore()
    store.subscribeTask('/api/task')
    FakeWebSocket.instances[0]!.message({ type: 'snapshot', resource: '/api/task', data: {} })
    expect(store.taskErrorFor('/api/task')).not.toBe('')
    expect(store.taskLoadingFor('/api/task')).toBe(false)
    await vi.advanceTimersByTimeAsync(11500)
    expect(store.taskErrorFor('/api/task')).toMatch(/timed out|Reconnecting/)
    store.unsubscribeTask()
  })

  it('does not apply an HTTP fallback from an abandoned route', async () => {
    let resolve!: (value: unknown) => void
    vi.stubGlobal(
      'fetch',
      vi.fn(
        () =>
          new Promise((r) => {
            resolve = r
          }),
      ),
    )
    const store = useLocalciStore()
    store.subscribeTask('/api/old')
    await vi.advanceTimersByTimeAsync(1500)
    store.subscribeTask('/api/new')
    resolve({ ok: true, json: async () => taskSnapshot('old') })
    await vi.advanceTimersByTimeAsync(0)
    expect(store.taskResponseFor('/api/new')).toBeNull()
    store.unsubscribeTask()
  })
})
