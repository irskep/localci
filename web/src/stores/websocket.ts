import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

type WebsocketState = 'connecting' | 'connected' | 'paused' | 'reconnecting' | 'disconnected'

type WebsocketStreamState = {
  key: string
  state: WebsocketState
  attempts: number
}

export const useWebsocketStore = defineStore('websocket', () => {
  const streams = ref<Record<string, WebsocketStreamState>>({})

  const entries = computed(() =>
    Object.values(streams.value).sort((a, b) => a.key.localeCompare(b.key)),
  )
  const activeCount = computed(() => entries.value.length)
  const reconnectingCount = computed(
    () => entries.value.filter((stream) => stream.state === 'reconnecting').length,
  )
  const pausedCount = computed(
    () => entries.value.filter((stream) => stream.state === 'paused').length,
  )

  const state = computed<WebsocketState | 'idle'>(() => {
    if (activeCount.value === 0) return 'idle'
    if (pausedCount.value > 0) return 'paused'
    if (reconnectingCount.value > 0) return 'reconnecting'
    if (entries.value.some((stream) => stream.state === 'connecting')) return 'connecting'
    if (entries.value.every((stream) => stream.state === 'connected')) return 'connected'
    return 'disconnected'
  })

  const label = computed(() => {
    switch (state.value) {
      case 'idle':
        return 'socket idle'
      case 'connecting':
        return 'socket connecting'
      case 'connected':
        return activeCount.value === 1
          ? 'socket connected'
          : `${activeCount.value} sockets connected`
      case 'paused':
        return 'socket paused'
      case 'reconnecting':
        return 'socket reconnecting'
      default:
        return 'socket disconnected'
    }
  })

  const severity = computed<'success' | 'info' | 'warn' | 'danger' | 'secondary'>(() => {
    switch (state.value) {
      case 'connected':
        return 'success'
      case 'connecting':
        return 'info'
      case 'paused':
        return 'secondary'
      case 'reconnecting':
        return 'warn'
      case 'disconnected':
        return 'danger'
      default:
        return 'secondary'
    }
  })

  function setStream(key: string, streamState: WebsocketState, attempts = 0): void {
    streams.value = {
      ...streams.value,
      [key]: { key, state: streamState, attempts },
    }
  }

  function removeStream(key: string): void {
    const next = { ...streams.value }
    delete next[key]
    streams.value = next
  }

  return {
    activeCount,
    entries,
    label,
    pausedCount,
    reconnectingCount,
    removeStream,
    setStream,
    severity,
    state,
  }
})
