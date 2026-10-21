import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

import type { HomeResponse } from '@/lib/api'
import { parseHomeResponse } from '@/lib/api'
import { snapshotStream } from '@/stores/localci'
import { notificationForRun, runHasLiveTasks, runIsComplete } from '@/lib/run-notifications'

type PermissionState = NotificationPermission | 'unsupported'

const enabledStorageKey = 'localci.notifications.enabled'

export const useNotificationStore = defineStore('notifications', () => {
  const permission = ref<PermissionState>(notificationPermission())
  const enabled = ref(loadEnabled())
  const monitorStarted = ref(false)
  const error = ref('')
  const seenLiveRuns = new Set<string>()
  const notifiedRuns = new Set<string>()
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
    snapshotStream(
      '/api',
      parseHomeResponse,
      (data) => {
        error.value = ''
        applyHome(data)
      },
      () => true,
      (message) => {
        error.value = message
      },
    )
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
