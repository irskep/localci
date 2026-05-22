import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  APIEvent,
  ArtifactListResponse,
  ArtifactResponse,
  CommitResponse,
  HomeResponse,
  QueueResponse,
  RepoResponse,
  RetryResponse,
  TaskResponse,
} from '@/lib/api'
import {
  getJSON,
  parseAPIEvent,
  parseArtifactListResponse,
  parseArtifactResponse,
  parseCommitResponse,
  parseHomeResponse,
  parseQueueResponse,
  parseRepoResponse,
  parseRetryResponse,
  parseTaskResponse,
  postJSON,
} from '@/lib/api'
import { taskURL } from '@/lib/routes'

type RequestState<T> =
  | { state: 'idle' }
  | { state: 'loading'; key: string; previous: T | null }
  | { state: 'loaded'; key: string; data: T }
  | { state: 'error'; key: string; message: string; previous: T | null }

export const useLocalciStore = defineStore('localci', () => {
  const home = ref<HomeResponse | null>(null)
  const queue = ref<QueueResponse | null>(null)
  const currentRepo = ref<RepoResponse | null>(null)
  const currentCommit = ref<CommitResponse | null>(null)
  const taskRequest = ref<RequestState<TaskResponse>>({ state: 'idle' })
  const artifactList = ref<ArtifactListResponse | null>(null)
  const currentArtifact = ref<ArtifactResponse | null>(null)
  const loading = ref(false)
  const error = ref('')
  const homeLoaded = ref(false)
  const queueLoaded = ref(false)
  const repoLoaded = ref(false)
  const commitLoaded = ref(false)
  const taskLoaded = ref(false)
  const artifactLoaded = ref(false)
  let taskSocket: WebSocket | null = null
  let taskSocketKey = ''
  let artifactSocket: WebSocket | null = null
  let artifactSocketKey = ''
  let pageSocket: WebSocket | null = null
  let pageSocketKey = ''

  const queueCount = computed(
    () => queue.value?.pending.length ?? home.value?.queue.pending.length ?? 0,
  )
  const activeEntry = computed(() => queue.value?.active ?? home.value?.queue.active)
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
      queue.value = result.queue
    }
    homeLoaded.value = true
  }

  function subscribeHome(): void {
    homeLoaded.value = false
    subscribePage('/api', parseHomeResponse, (data) => {
      home.value = data
      queue.value = data.queue
      homeLoaded.value = true
    })
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

  function subscribeRepo(apiPath: string): void {
    repoLoaded.value = false
    currentRepo.value = null
    subscribePage(apiPath, parseRepoResponse, (data) => {
      currentRepo.value = data
      repoLoaded.value = true
    })
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
    if (pageSocket && pageSocketKey === apiPath) return
    unsubscribePage()
    pageSocketKey = apiPath
    loading.value = true
    error.value = ''
    pageSocket = openEventSocket(apiPath)
    pageSocket.addEventListener('message', (message) => {
      if (pageSocketKey !== apiPath) return
      try {
        const event = parseAPIEvent(JSON.parse(message.data as string) as unknown, validateData)
        if (event.type === 'snapshot' || event.type === 'replace') {
          apply(event.data)
          loading.value = false
        }
        if (event.type === 'error') {
          error.value = event.message
          loading.value = false
        }
      } catch (err) {
        error.value = err instanceof Error ? err.message : String(err)
        loading.value = false
      }
    })
    pageSocket.addEventListener('error', () => {
      if (pageSocketKey !== apiPath) return
      error.value = 'Event stream disconnected'
      loading.value = false
    })
  }

  function unsubscribePage(): void {
    if (pageSocket) pageSocket.close()
    pageSocket = null
    pageSocketKey = ''
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
    loading.value = true
    error.value = ''
    try {
      const result = await getJSON(apiPath, parseTaskResponse)
      if (taskRequest.value.state === 'loading' && taskRequest.value.key === apiPath) {
        taskRequest.value = { state: 'loaded', key: apiPath, data: result }
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err)
      error.value = message
      if (taskRequest.value.state === 'loading' && taskRequest.value.key === apiPath) {
        taskRequest.value = { state: 'error', key: apiPath, message, previous }
      }
    } finally {
      if (taskRequest.value.state !== 'loading' || taskRequest.value.key === apiPath) {
        loading.value = false
        taskLoaded.value = true
      }
    }
  }

  function subscribeTask(apiPath: string): void {
    if (taskSocket && taskSocketKey === apiPath) return
    unsubscribeTask()
    taskSocketKey = apiPath
    const previous = taskResponseFor(apiPath)
    taskRequest.value = { state: 'loading', key: apiPath, previous }
    taskSocket = openEventSocket(apiPath)
    taskSocket.addEventListener('message', (message) => {
      if (taskSocketKey !== apiPath) return
      let event: APIEvent<TaskResponse>
      try {
        event = parseAPIEvent(JSON.parse(message.data as string) as unknown, parseTaskResponse)
      } catch (err) {
        error.value = err instanceof Error ? err.message : String(err)
        return
      }
      if (event.type === 'snapshot' || event.type === 'replace') {
        taskRequest.value = { state: 'loaded', key: apiPath, data: event.data }
        taskLoaded.value = true
        return
      }
      if (event.type === 'append') {
        const current = taskResponseFor(apiPath)
        if (!current || current.primary_artifact !== 'combined.log') return
        if (current.primary_log.length === event.offset) {
          taskRequest.value = {
            state: 'loaded',
            key: apiPath,
            data: {
              ...current,
              primary_log: current.primary_log + event.text,
            },
          }
          taskLoaded.value = true
          return
        }
        void loadTask(apiPath)
        return
      }
      if (event.type === 'error') {
        taskRequest.value = {
          state: 'error',
          key: apiPath,
          message: event.message,
          previous: taskResponseFor(apiPath),
        }
        error.value = event.message
      }
    })
    taskSocket.addEventListener('error', () => {
      if (taskSocketKey !== apiPath) return
      const previous = taskResponseFor(apiPath)
      taskRequest.value = {
        state: 'error',
        key: apiPath,
        message: 'Task event stream disconnected',
        previous,
      }
    })
  }

  function unsubscribeTask(): void {
    if (taskSocket) taskSocket.close()
    taskSocket = null
    taskSocketKey = ''
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

  function subscribeArtifact(apiPath: string): void {
    if (artifactSocket && artifactSocketKey === apiPath) return
    unsubscribeArtifact()
    artifactSocketKey = apiPath
    artifactLoaded.value = false
    currentArtifact.value = null
    artifactSocket = openEventSocket(apiPath)
    artifactSocket.addEventListener('message', (message) => {
      if (artifactSocketKey !== apiPath) return
      let event: APIEvent<ArtifactResponse>
      try {
        event = parseAPIEvent(JSON.parse(message.data as string) as unknown, parseArtifactResponse)
      } catch (err) {
        error.value = err instanceof Error ? err.message : String(err)
        return
      }
      if (event.type === 'snapshot' || event.type === 'replace') {
        currentArtifact.value = event.data
        artifactLoaded.value = true
        return
      }
      if (event.type === 'append' && currentArtifact.value) {
        const currentLength = currentArtifact.value.content.length
        if (currentLength === event.offset) {
          currentArtifact.value = {
            ...currentArtifact.value,
            content: currentArtifact.value.content + event.text,
          }
          return
        }
        void loadArtifact(apiPath)
      }
      if (event.type === 'error') {
        error.value = event.message
      }
    })
    artifactSocket.addEventListener('error', () => {
      if (artifactSocketKey === apiPath) error.value = 'Artifact event stream disconnected'
    })
  }

  function unsubscribeArtifact(): void {
    if (artifactSocket) artifactSocket.close()
    artifactSocket = null
    artifactSocketKey = ''
  }

  async function retryTask(
    repoPath: string,
    commit: string,
    taskName: string,
  ): Promise<RetryResponse | null> {
    const retryPath = `/api${taskURL(repoPath, commit, taskName)}/retry`
    return await load(() => postJSON(retryPath, parseRetryResponse))
  }

  return {
    activeEntry,
    artifactList,
    currentArtifact,
    currentCommit,
    currentRepo,
    currentTask,
    error,
    artifactLoaded,
    commitLoaded,
    home,
    homeLoaded,
    loadArtifact,
    loadArtifactList,
    loadCommit,
    loadHome,
    loadQueue,
    loadRepo,
    loadTask,
    loading,
    queue,
    queueLoaded,
    queueCount,
    repoLoaded,
    retryTask,
    subscribeCommit,
    subscribeHome,
    subscribeQueue,
    subscribeRepo,
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
