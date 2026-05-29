import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import type { APIEvent, HomeResponse } from '@/lib/api'
import { getJSON, parseAPIEvent, parseHomeResponse } from '@/lib/api'
import { notificationForRun, runHasLiveTasks, runIsComplete } from '@/lib/run-notifications'

type PermissionState = NotificationPermission | 'unsupported'

const enabledStorageKey = 'localci.notifications.enabled'
const testNotificationIntervalMs = 10_000

export const useNotificationStore = defineStore('notifications', () => {
  const permission = ref<PermissionState>(notificationPermission())
  const enabled = ref(loadEnabled())
  const monitorStarted = ref(false)
  const error = ref('')
  const seenLiveRuns = new Set<string>()
  const notifiedRuns = new Set<string>()
  let socket: WebSocket | null = null
  let reconnectTimer: number | null = null
  let testNotificationTimer: number | null = null
  let reconnectAttempts = 0
  let seeded = false

  const supported = computed(() => permission.value !== 'unsupported')
  const canNotify = computed(() => permission.value === 'granted' && enabled.value)
  const label = computed(() => {
    if (permission.value === 'unsupported') return 'notifications unavailable'
    if (permission.value === 'default') return 'click to allow notifications'
    if (permission.value === 'denied') return 'notifications disabled'
    return enabled.value ? 'notifications on' : 'notifications off'
  })
  const icon = computed(() => {
    if (permission.value === 'granted' && enabled.value) return 'pi pi-bell'
    if (permission.value === 'granted') return 'pi pi-bell-slash'
    return 'pi pi-bell'
  })
  const severity = computed(() => {
    if (permission.value === 'granted' && enabled.value) return 'success'
    if (permission.value === 'denied' || permission.value === 'unsupported') return 'secondary'
    return 'info'
  })

  function refreshPermission(): void {
    permission.value = notificationPermission()
  }

  async function activate(): Promise<void> {
    refreshPermission()
    if (permission.value === 'unsupported') return
    if (permission.value !== 'granted') {
      permission.value = await Notification.requestPermission()
      if (permission.value === 'granted') setEnabled(true)
      return
    }
    setEnabled(!enabled.value)
  }

  function setEnabled(value: boolean): void {
    enabled.value = value
    try {
      window.localStorage.setItem(enabledStorageKey, value ? 'true' : 'false')
    } catch {
      // Ignore private-mode or disabled storage failures; the in-memory toggle still works.
    }
  }

  function startRunMonitor(): void {
    if (monitorStarted.value) return
    monitorStarted.value = true
    refreshPermission()
    startTestNotifications()
    connect()
    void getJSON('/api', parseHomeResponse)
      .then((data) => {
        applyHome(data)
      })
      .catch((err) => {
        error.value = err instanceof Error ? err.message : String(err)
      })
  }

  function applyHome(data: HomeResponse): void {
    const isInitial = !seeded
    seeded = true
    for (const run of data.recent_commits) {
      const notification = notificationForRun(run)
      if (runHasLiveTasks(run)) {
        seenLiveRuns.add(notification.key)
        continue
      }
      if (
        !isInitial &&
        seenLiveRuns.has(notification.key) &&
        !notifiedRuns.has(notification.key) &&
        runIsComplete(run)
      ) {
        sendNotification(notification.title, notification.body)
        notifiedRuns.add(notification.key)
      }
    }
  }

  function connect(): void {
    let nextSocket: WebSocket
    try {
      nextSocket = openHomeEventSocket()
    } catch {
      scheduleReconnect()
      return
    }
    socket = nextSocket
    nextSocket.addEventListener('open', () => {
      if (socket !== nextSocket) return
      reconnectAttempts = 0
      error.value = ''
    })
    nextSocket.addEventListener('message', (message) => {
      if (socket !== nextSocket) return
      let event: APIEvent<HomeResponse>
      try {
        event = parseAPIEvent(JSON.parse(message.data as string) as unknown, parseHomeResponse)
      } catch (err) {
        error.value = err instanceof Error ? err.message : String(err)
        return
      }
      if (event.type === 'snapshot' || event.type === 'replace') {
        applyHome(event.data)
      }
      if (event.type === 'error') error.value = event.message
    })
    nextSocket.addEventListener('close', () => {
      if (socket !== nextSocket) return
      scheduleReconnect()
    })
    nextSocket.addEventListener('error', () => {
      if (socket !== nextSocket) return
      nextSocket.close()
    })
  }

  function scheduleReconnect(): void {
    if (reconnectTimer !== null) return
    const delay = Math.min(5000, 250 * 2 ** reconnectAttempts)
    reconnectAttempts += 1
    reconnectTimer = window.setTimeout(() => {
      reconnectTimer = null
      connect()
    }, delay)
  }

  function startTestNotifications(): void {
    if (testNotificationTimer !== null) return
    testNotificationTimer = window.setInterval(() => {
      sendNotification('LocalCI: test', 'notification test')
    }, testNotificationIntervalMs)
  }

  function sendNotification(title: string, body: string): void {
    refreshPermission()
    if (!canNotify.value) return
    try {
      new Notification(title, { body })
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
    }
  }

  return {
    activate,
    applyHome,
    canNotify,
    enabled,
    error,
    icon,
    label,
    monitorStarted,
    permission,
    refreshPermission,
    severity,
    setEnabled,
    startRunMonitor,
    supported,
  }
})

function notificationPermission(): PermissionState {
  if (!('Notification' in window)) return 'unsupported'
  return Notification.permission
}

function loadEnabled(): boolean {
  try {
    return window.localStorage.getItem(enabledStorageKey) !== 'false'
  } catch {
    return true
  }
}

function openHomeEventSocket(): WebSocket {
  const url = new URL('/api/events', window.location.href)
  url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return new WebSocket(url)
}
