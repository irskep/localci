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
    setActivePinia(createPinia())
    FakeWebSocket.instances = []
    vi.stubGlobal('WebSocket', FakeWebSocket)
  })

  afterEach(() => {
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

    await vi.waitFor(() => expect(store.repoLoaded).toBe(true))
    expect(store.currentRepo?.repo.repo_path).toBe('cli/localci')
    expect(store.loading).toBe(false)
    expect(FakeWebSocket.instances).toHaveLength(1)
    expect(fetch).toHaveBeenCalledWith('/api/repo/cli/localci')
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
    await vi.waitFor(() => expect(store.error).toBe('Load failed'))
    FakeWebSocket.instances[0]?.message(snapshot)

    expect(store.repoLoaded).toBe(true)
    expect(store.currentRepo?.repo.repo_path).toBe('cli/localci')
    expect(store.error).toBe('')
  })
})
