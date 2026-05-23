import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'

import { useWebsocketStore } from '@/stores/websocket'

describe('websocket store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('summarizes active websocket state', () => {
    const store = useWebsocketStore()

    expect(store.state).toBe('idle')
    expect(store.label).toBe('socket idle')

    store.setStream('/api', 'connected')
    expect(store.state).toBe('connected')
    expect(store.label).toBe('socket connected')

    store.setStream('/api/task', 'connected')
    expect(store.label).toBe('2 sockets connected')

    store.setStream('/api/task', 'paused')
    expect(store.state).toBe('paused')
    expect(store.label).toBe('socket paused')

    store.removeStream('/api/task')
    expect(store.state).toBe('connected')
  })
})
