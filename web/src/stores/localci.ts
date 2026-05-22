import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  ArtifactListResponse,
  ArtifactResponse,
  CommitResponse,
  HomeResponse,
  QueueResponse,
  RepoResponse,
  RetryResponse,
  TaskResponse,
} from '@/lib/api'
import { getJSON, postJSON } from '@/lib/api'
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
    const result = await load(() => getJSON<HomeResponse>('/api'))
    if (result) {
      home.value = result
      queue.value = result.queue
    }
    homeLoaded.value = true
  }

  async function loadQueue(): Promise<void> {
    queueLoaded.value = false
    queue.value = null
    const result = await load(() => getJSON<QueueResponse>('/api/queue'))
    if (result) queue.value = result
    queueLoaded.value = true
  }

  async function loadRepo(apiPath: string): Promise<void> {
    repoLoaded.value = false
    currentRepo.value = null
    const result = await load(() => getJSON<RepoResponse>(apiPath))
    if (result) currentRepo.value = result
    repoLoaded.value = true
  }

  async function loadCommit(apiPath: string): Promise<void> {
    commitLoaded.value = false
    currentCommit.value = null
    const result = await load(() => getJSON<CommitResponse>(apiPath))
    if (result) currentCommit.value = result
    commitLoaded.value = true
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
      const result = await getJSON<TaskResponse>(apiPath)
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

  async function loadArtifactList(apiPath: string): Promise<void> {
    const result = await load(() => getJSON<ArtifactListResponse>(apiPath))
    if (result) artifactList.value = result
  }

  async function loadArtifact(apiPath: string): Promise<void> {
    artifactLoaded.value = false
    currentArtifact.value = null
    const result = await load(() => getJSON<ArtifactResponse>(apiPath))
    if (result) currentArtifact.value = result
    artifactLoaded.value = true
  }

  async function retryTask(
    repoPath: string,
    commit: string,
    taskName: string,
  ): Promise<RetryResponse | null> {
    const retryPath = `/api${taskURL(repoPath, commit, taskName)}/retry`
    return await load(() => postJSON<RetryResponse>(retryPath))
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
    taskErrorFor,
    taskLoaded,
    taskLoadingFor,
    taskResponseFor,
  }
})
