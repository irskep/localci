import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  APIEvent,
  ArtifactListResponse,
  ArtifactResponse,
  CancelResponse,
  CommitResponse,
  HomeResponse,
  QueueResponse,
  RevealArtifactResponse,
  RepoResponse,
  RepoSummary,
  RepoTaskHistoryResponse,
  RetryResponse,
  TaskResponse,
} from '@/lib/api'
import {
  getJSON,
  parseAPIEvent,
  parseArtifactListResponse,
  parseArtifactResponse,
  parseCancelResponse,
  parseCommitResponse,
  parseHomeResponse,
  parseQueueResponse,
  parseRevealArtifactResponse,
  parseRepoListResponse,
  parseRepoResponse,
  parseRepoTaskHistoryResponse,
  parseRetryResponse,
  parseTaskResponse,
  postJSON,
} from '@/lib/api'
import { taskURL } from '@/lib/routes'
import { useWebsocketStore } from '@/stores/websocket'

type RequestState<T> =
  | { state: 'idle' }
  | { state: 'loading'; key: string; previous: T | null }
  | { state: 'loaded'; key: string; data: T }
  | { state: 'error'; key: string; message: string; previous: T | null }

type EventStream = {
  key: string
  socket: WebSocket | null
  reconnectTimer: number | null
  reconnectAttempts: number
  stopped: boolean
  removeVisibilityListener: (() => void) | null
  cleanup?: () => void
}

export const useLocalciStore = defineStore('localci', () => {
  const home = ref<HomeResponse | null>(null)
  const repoList = ref<RepoSummary[]>([])
  const queue = ref<QueueResponse | null>(null)
  const currentRepo = ref<RepoResponse | null>(null)
  const currentRepoTaskHistory = ref<RepoTaskHistoryResponse | null>(null)
  const currentCommit = ref<CommitResponse | null>(null)
  const taskRequest = ref<RequestState<TaskResponse>>({ state: 'idle' })
  const artifactList = ref<ArtifactListResponse | null>(null)
  const currentArtifact = ref<ArtifactResponse | null>(null)
  const loading = ref(false)
  const error = ref('')
  const homeLoaded = ref(false)
  const repoListLoaded = ref(false)
  const queueLoaded = ref(false)
  const repoLoaded = ref(false)
  const repoTaskHistoryLoaded = ref(false)
  const commitLoaded = ref(false)
  const taskLoaded = ref(false)
  const artifactLoaded = ref(false)
  let taskStream: EventStream | null = null
  let artifactStream: EventStream | null = null
  let pageStream: EventStream | null = null

  const queueCount = computed(
    () => queue.value?.pending.length ?? home.value?.queue.pending.length ?? 0,
  )
  const activeEntry = computed(() => queue.value?.active ?? home.value?.queue.active)
  const repos = computed(() => (repoListLoaded.value ? repoList.value : (home.value?.repos ?? [])))
  const currentTask = computed(() => {
    switch (taskRequest.value.state) {
      case 'loaded':
        return taskRequest.value.data
      case 'loading':
      case 'error':
        return taskRequest.value.previous
      default:
        return null
    }
  })

  async function load<T>(operation: () => Promise<T>): Promise<T | null> {
    loading.value = true
    error.value = ''
    try {
      return await operation()
    } catch (err) {
      error.value = err instanceof Error ? err.message : String(err)
      return null
    } finally {
      loading.value = false
    }
  }

  async function loadHome(): Promise<void> {
    homeLoaded.value = false
    const result = await load(() => getJSON('/api', parseHomeResponse))
    if (result) {
      home.value = result
      repoList.value = result.repos
      repoListLoaded.value = true
      queue.value = result.queue
    }
    homeLoaded.value = true
  }

  async function loadHomePage(before?: string): Promise<void> {
    homeLoaded.value = false
    const query = before ? `?before=${encodeURIComponent(before)}` : ''
    const result = await load(() => getJSON(`/api${query}`, parseHomeResponse))
    if (result) {
      home.value = result
      repoList.value = result.repos
      repoListLoaded.value = true
      queue.value = result.queue
    }
    homeLoaded.value = true
  }

  function subscribeHome(): void {
    homeLoaded.value = false
    subscribePage('/api', parseHomeResponse, (data) => {
      home.value = data
      repoList.value = data.repos
      repoListLoaded.value = true
      queue.value = data.queue
      homeLoaded.value = true
    })
  }

  async function loadRepos(): Promise<void> {
    if (repoListLoaded.value) return
    const result = await load(() => getJSON('/api/repo', parseRepoListResponse))
    if (result) {
      repoList.value = result
      repoListLoaded.value = true
    }
  }

  async function loadQueue(): Promise<void> {
    queueLoaded.value = false
    queue.value = null
    const result = await load(() => getJSON('/api/queue', parseQueueResponse))
    if (result) queue.value = result
    queueLoaded.value = true
  }

  function subscribeQueue(): void {
    queueLoaded.value = false
    queue.value = null
    subscribePage('/api/queue', parseQueueResponse, (data) => {
      queue.value = data
      queueLoaded.value = true
    })
  }

  async function loadRepo(apiPath: string): Promise<void> {
    repoLoaded.value = false
    currentRepo.value = null
    const result = await load(() => getJSON(apiPath, parseRepoResponse))
    if (result) currentRepo.value = result
    repoLoaded.value = true
  }

  async function loadRepoPage(apiPath: string, before?: string): Promise<void> {
    repoLoaded.value = false
    const separator = apiPath.includes('?') ? '&' : '?'
    const query = before ? `${separator}before=${encodeURIComponent(before)}` : ''
    const result = await load(() => getJSON(`${apiPath}${query}`, parseRepoResponse))
    if (result) currentRepo.value = result
    repoLoaded.value = true
  }

  function subscribeRepo(apiPath: string): void {
    repoLoaded.value = false
    currentRepo.value = null
    subscribePage(apiPath, parseRepoResponse, (data) => {
      currentRepo.value = data
      repoLoaded.value = true
    })
  }

  function subscribeRepoTaskHistory(apiPath: string): void {
    repoTaskHistoryLoaded.value = false
    currentRepoTaskHistory.value = null
    subscribePage(apiPath, parseRepoTaskHistoryResponse, (data) => {
      currentRepoTaskHistory.value = data
      repoTaskHistoryLoaded.value = true
    })
  }

  async function loadRepoTaskHistoryPage(apiPath: string, offset: number): Promise<void> {
    unsubscribePage()
    const key = `${apiPath}?offset=${offset}`
    const stream: EventStream = {
      key: apiPath,
      socket: null,
      reconnectTimer: null,
      reconnectAttempts: 0,
      stopped: true,
      removeVisibilityListener: null,
    }
    // Retain a route identity while browsing a static history page.
    pageStream = stream
    const result = await load(() =>
      getJSON(key, parseRepoTaskHistoryResponse, AbortSignal.timeout(10000)),
    )
    if (result && pageStream === stream) currentRepoTaskHistory.value = result
    repoTaskHistoryLoaded.value = true
  }

  async function loadCommit(apiPath: string): Promise<void> {
    commitLoaded.value = false
    currentCommit.value = null
    const result = await load(() => getJSON(apiPath, parseCommitResponse))
    if (result) currentCommit.value = result
    commitLoaded.value = true
  }

  function subscribeCommit(apiPath: string): void {
    commitLoaded.value = false
    currentCommit.value = null
    subscribePage(apiPath, parseCommitResponse, (data) => {
      currentCommit.value = data
      commitLoaded.value = true
    })
  }

  function subscribePage<T>(
    apiPath: string,
    validateData: (value: unknown) => T,
    apply: (data: T) => void,
  ): void {
    unsubscribePage()
    loading.value = true
    error.value = ''
    pageStream = snapshotStream(
      apiPath,
      validateData,
      (data) => {
        apply(data)
        error.value = ''
        loading.value = false
      },
      () => true,
      (message) => {
        error.value = message
        loading.value = false
      },
    )
  }

  function unsubscribePage(key?: string): void {
    if (key !== undefined && pageStream?.key !== key) return
    closeEventStream(pageStream)
    pageStream = null
  }

  function taskResponseFor(apiPath: string): TaskResponse | null {
    const request = taskRequest.value
    if (request.state === 'loaded' && request.key === apiPath) return request.data
    if ((request.state === 'loading' || request.state === 'error') && request.key === apiPath) {
      return request.previous
    }
    return null
  }

  function taskErrorFor(apiPath: string): string {
    const request = taskRequest.value
    if (request.state === 'error' && request.key === apiPath) return request.message
    return ''
  }

  function taskLoadingFor(apiPath: string): boolean {
    const request = taskRequest.value
    return request.state === 'loading' && request.key === apiPath
  }

  async function loadTask(apiPath: string): Promise<void> {
    taskLoaded.value = false
    const previous = taskResponseFor(apiPath)
    taskRequest.value = { state: 'loading', key: apiPath, previous }
    const pendingRequest = taskRequest.value
    loading.value = true
    error.value = ''
    try {
      const result = await getJSON(apiPath, parseTaskResponse)
      if (taskRequest.value === pendingRequest) {
        taskRequest.value = { state: 'loaded', key: apiPath, data: result }
      }
    } catch (err) {
      if (taskRequest.value !== pendingRequest) return
      const message = err instanceof Error ? err.message : String(err)
      error.value = message
      if (taskRequest.value === pendingRequest) {
        taskRequest.value = { state: 'error', key: apiPath, message, previous }
      }
    } finally {
      if (taskRequest.value === pendingRequest || taskRequest.value.key === apiPath) {
        loading.value = false
        taskLoaded.value = true
      }
    }
  }

  function subscribeTask(apiPath: string): void {
    if (taskStream && taskStream.key === apiPath) return
    unsubscribeTask()
    const previous = taskResponseFor(apiPath)
    taskRequest.value = { state: 'loading', key: apiPath, previous }
    loading.value = true
    error.value = ''
    taskStream = snapshotStream(
      apiPath,
      parseTaskResponse,
      (data) => {
        taskRequest.value = { state: 'loaded', key: apiPath, data }
        taskLoaded.value = true
        loading.value = false
        error.value = ''
      },
      (event) => {
        const current = taskResponseFor(apiPath)
        if (!current) return false
        if (current.primary_artifact !== 'combined.log' || current.primary_log_truncated)
          return true
        if (event.end_offset <= current.primary_log_end) return true
        if (event.offset !== current.primary_log_end) return false
        // Stop the preview at 2 MiB; the download retains the complete log.
        const truncated = event.end_offset > 2 * 1024 * 1024
        taskRequest.value = {
          state: 'loaded',
          key: apiPath,
          data: {
            ...current,
            primary_log: truncated ? current.primary_log : current.primary_log + event.text,
            primary_log_end: truncated ? current.primary_log_end : event.end_offset,
            primary_log_truncated: truncated,
          },
        }
        return true
      },
      (message) => {
        taskRequest.value = {
          state: 'error',
          key: apiPath,
          message,
          previous: taskResponseFor(apiPath),
        }
        loading.value = false
        error.value = message
      },
    )
  }

  function unsubscribeTask(): void {
    closeEventStream(taskStream)
    taskStream = null
  }

  async function loadArtifactList(apiPath: string): Promise<void> {
    const result = await load(() => getJSON(apiPath, parseArtifactListResponse))
    if (result) artifactList.value = result
  }

  async function loadArtifact(apiPath: string): Promise<void> {
    artifactLoaded.value = false
    currentArtifact.value = null
    const result = await load(() => getJSON(apiPath, parseArtifactResponse))
    if (result) currentArtifact.value = result
    artifactLoaded.value = true
  }

  async function revealArtifact(apiPath: string): Promise<RevealArtifactResponse | null> {
    return await load(() =>
      postJSON(`${apiPath.replace(/\/$/, '')}/reveal`, parseRevealArtifactResponse),
    )
  }

  function subscribeArtifact(apiPath: string): void {
    if (artifactStream && artifactStream.key === apiPath) return
    unsubscribeArtifact()
    artifactLoaded.value = false
    currentArtifact.value = null
    loading.value = true
    error.value = ''
    artifactStream = snapshotStream(
      apiPath,
      parseArtifactResponse,
      (data) => {
        currentArtifact.value = data
        artifactLoaded.value = true
        loading.value = false
        error.value = ''
      },
      (event) => {
        const current = currentArtifact.value
        if (!current) return false
        if (!current.artifact.is_text || event.end_offset <= current.content_end) return true
        if (event.offset !== current.content_end) return false
        if (event.end_offset > 2 * 1024 * 1024) {
          currentArtifact.value = {
            ...current,
            artifact: { ...current.artifact, is_text: false },
            content: '',
          }
          return true
        }
        currentArtifact.value = {
          ...current,
          content: current.content + event.text,
          content_end: event.end_offset,
        }
        return true
      },
      (message) => {
        error.value = message
        loading.value = false
      },
    )
  }

  function unsubscribeArtifact(): void {
    closeEventStream(artifactStream)
    artifactStream = null
  }

  async function retryTask(
    repoPath: string,
    commit: string,
    taskName: string,
  ): Promise<RetryResponse | null> {
    const retryPath = `/api${taskURL(repoPath, commit, taskName)}/retry`
    return await load(() => postJSON(retryPath, parseRetryResponse))
  }

  async function cancelTask(
    repoPath: string,
    commit: string,
    taskName: string,
  ): Promise<CancelResponse | null> {
    const cancelPath = `/api${taskURL(repoPath, commit, taskName)}/cancel`
    return await load(() => postJSON(cancelPath, parseCancelResponse))
  }

  return {
    activeEntry,
    artifactList,
    currentArtifact,
    currentCommit,
    currentRepo,
    currentRepoTaskHistory,
    currentTask,
    error,
    artifactLoaded,
    cancelTask,
    commitLoaded,
    home,
    homeLoaded,
    loadArtifact,
    loadArtifactList,
    loadCommit,
    loadRepoTaskHistoryPage,
    loadHome,
    loadHomePage,
    loadQueue,
    loadRepo,
    loadRepoPage,
    loadRepos,
    loadTask,
    loading,
    queue,
    queueLoaded,
    queueCount,
    revealArtifact,
    repoLoaded,
    repoListLoaded,
    repoTaskHistoryLoaded,
    repos,
    retryTask,
    subscribeCommit,
    subscribeHome,
    subscribeQueue,
    subscribeRepo,
    subscribeRepoTaskHistory,
    taskErrorFor,
    taskLoaded,
    taskLoadingFor,
    taskResponseFor,
    subscribeArtifact,
    subscribeTask,
    unsubscribeArtifact,
    unsubscribePage,
    unsubscribeTask,
  }
})

function openEventSocket(apiPath: string): WebSocket {
  const url = new URL(`${apiPath}/events`, window.location.href)
  url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return new WebSocket(url)
}

function openReconnectingEventStream(
  apiPath: string,
  handlers: {
    onMessage: (message: MessageEvent) => void
    onDisconnect?: () => void
    onReconnect?: () => void
  },
): EventStream {
  const stream: EventStream = {
    key: apiPath,
    socket: null,
    reconnectTimer: null,
    reconnectAttempts: 0,
    stopped: false,
    removeVisibilityListener: null,
  }
  const websocket = useWebsocketStore()

  const connect = (): void => {
    if (stream.stopped || document.visibilityState === 'hidden') return
    websocket.setStream(apiPath, 'connecting', stream.reconnectAttempts)
    let socket: WebSocket
    try {
      socket = openEventSocket(apiPath)
    } catch {
      scheduleReconnect(stream, connect, handlers.onDisconnect)
      return
    }
    stream.socket = socket

    socket.addEventListener('open', () => {
      if (stream.stopped || stream.socket !== socket) return
      stream.reconnectAttempts = 0
      websocket.setStream(apiPath, 'connected')
      handlers.onReconnect?.()
    })
    socket.addEventListener('message', (message) => {
      if (stream.stopped || stream.socket !== socket) return
      handlers.onMessage(message)
    })
    socket.addEventListener('close', () => {
      if (stream.stopped || stream.socket !== socket) return
      scheduleReconnect(stream, connect, handlers.onDisconnect)
    })
    socket.addEventListener('error', () => {
      if (stream.stopped || stream.socket !== socket) return
      socket.close()
    })
  }

  const handleVisibilityChange = (): void => {
    if (stream.stopped) return
    if (document.visibilityState === 'hidden') {
      if (stream.reconnectTimer !== null) {
        window.clearTimeout(stream.reconnectTimer)
        stream.reconnectTimer = null
      }
      const socket = stream.socket
      stream.socket = null
      socket?.close()
      websocket.setStream(apiPath, 'paused', stream.reconnectAttempts)
      handlers.onDisconnect?.()
      return
    }
    connect()
  }

  document.addEventListener('visibilitychange', handleVisibilityChange)
  stream.removeVisibilityListener = () => {
    document.removeEventListener('visibilitychange', handleVisibilityChange)
  }

  connect()
  return stream
}

function scheduleReconnect(
  stream: EventStream,
  connect: () => void,
  onDisconnect?: () => void,
): void {
  if (stream.stopped || stream.reconnectTimer !== null || document.visibilityState === 'hidden') {
    return
  }
  useWebsocketStore().setStream(stream.key, 'reconnecting', stream.reconnectAttempts + 1)
  onDisconnect?.()
  const delay = Math.min(5000, 250 * 2 ** stream.reconnectAttempts)
  stream.reconnectAttempts += 1
  stream.reconnectTimer = window.setTimeout(() => {
    stream.reconnectTimer = null
    connect()
  }, delay)
}

function closeEventStream(stream: EventStream | null): void {
  if (!stream) return
  stream.stopped = true
  stream.cleanup?.()
  if (stream.reconnectTimer !== null) window.clearTimeout(stream.reconnectTimer)
  stream.removeVisibilityListener?.()
  stream.socket?.close()
  useWebsocketStore().removeStream(stream.key)
}

// The socket is the normal snapshot source. HTTP is a delayed fallback, with
// one in-flight request and a deadline. New socket data wins over HTTP replies.
export function snapshotStream<T>(
  apiPath: string,
  validate: (value: unknown) => T,
  apply: (data: T) => void,
  append: (event: Extract<APIEvent<T>, { type: 'append' }>) => boolean,
  fail: (message: string) => void,
): EventStream {
  let revision = 0
  let received = false
  let pending: AbortController | null = null
  let fallbackTimer: ReturnType<typeof setTimeout> | undefined
  let deadline: ReturnType<typeof setTimeout> | undefined
  const stream = openReconnectingEventStream(apiPath, {
    onMessage(message) {
      try {
        const event = parseAPIEvent(JSON.parse(message.data as string) as unknown, validate)
        if (event.type === 'snapshot' || event.type === 'replace') {
          revision++
          received = true
          clearTimeout(fallbackTimer)
          clearTimeout(deadline)
          pending?.abort()
          apply(event.data)
        } else if (event.type === 'append') {
          if (!received || !append(event)) {
            // One reconnect replaces any number of overlapping recovery GETs.
            stream.socket?.close()
          } else {
            revision++
          }
        } else if (event.type === 'error') {
          fail(event.message)
        }
      } catch (err) {
        fail(err instanceof Error ? err.message : String(err))
      }
    },
    onDisconnect() {
      fail('Reconnecting to daemon')
    },
    onReconnect() {
      armFallback()
    },
  })
  function armFallback(): void {
    clearTimeout(fallbackTimer)
    fallbackTimer = setTimeout(() => {
      void fallback()
    }, 1500)
  }
  async function fallback(): Promise<void> {
    if (stream.stopped || pending) return
    const version = revision
    const controller = new AbortController()
    pending = controller
    deadline = setTimeout(() => {
      controller.abort()
      if (!stream.stopped && revision === version) {
        fail('Loading timed out. Reconnecting to daemon…')
        stream.socket?.close()
      }
    }, 10000)
    try {
      const data = await getJSON(apiPath, validate, controller.signal)
      if (!stream.stopped && revision === version && !controller.signal.aborted) {
        received = true
        apply(data)
      }
    } catch (err) {
      if (!stream.stopped && revision === version && !controller.signal.aborted)
        fail(err instanceof Error ? err.message : String(err))
    } finally {
      clearTimeout(deadline)
      if (pending === controller) pending = null
    }
  }
  stream.cleanup = () => {
    clearTimeout(fallbackTimer)
    clearTimeout(deadline)
    pending?.abort()
  }
  armFallback()
  return stream
}
