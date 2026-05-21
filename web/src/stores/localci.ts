import { defineStore } from 'pinia'
import { computed, ref } from 'vue'

import type {
  ArtifactListResponse,
  ArtifactResponse,
  CommitResponse,
  HomeResponse,
  QueueResponse,
  RepoResponse,
  TaskResponse,
} from '@/lib/api'
import { getJSON, postJSON } from '@/lib/api'
import { taskURL } from '@/lib/routes'

export const useLocalciStore = defineStore('localci', () => {
  const home = ref<HomeResponse | null>(null)
  const queue = ref<QueueResponse | null>(null)
  const currentRepo = ref<RepoResponse | null>(null)
  const currentCommit = ref<CommitResponse | null>(null)
  const currentTask = ref<TaskResponse | null>(null)
  const artifactList = ref<ArtifactListResponse | null>(null)
  const currentArtifact = ref<ArtifactResponse | null>(null)
  const loading = ref(false)
  const error = ref('')

  const queueCount = computed(
    () => queue.value?.pending.length ?? home.value?.queue.pending.length ?? 0,
  )
  const activeEntry = computed(() => queue.value?.active ?? home.value?.queue.active)

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
    const result = await load(() => getJSON<HomeResponse>('/api'))
    if (result) {
      home.value = result
      queue.value = result.queue
    }
  }

  async function loadQueue(): Promise<void> {
    const result = await load(() => getJSON<QueueResponse>('/api/queue'))
    if (result) queue.value = result
  }

  async function loadRepo(apiPath: string): Promise<void> {
    const result = await load(() => getJSON<RepoResponse>(apiPath))
    if (result) currentRepo.value = result
  }

  async function loadCommit(apiPath: string): Promise<void> {
    const result = await load(() => getJSON<CommitResponse>(apiPath))
    if (result) currentCommit.value = result
  }

  async function loadTask(apiPath: string): Promise<void> {
    const result = await load(() => getJSON<TaskResponse>(apiPath))
    if (result) currentTask.value = result
  }

  async function loadArtifactList(apiPath: string): Promise<void> {
    const result = await load(() => getJSON<ArtifactListResponse>(apiPath))
    if (result) artifactList.value = result
  }

  async function loadArtifact(apiPath: string): Promise<void> {
    const result = await load(() => getJSON<ArtifactResponse>(apiPath))
    if (result) currentArtifact.value = result
  }

  async function retryTask(repoPath: string, commit: string, taskName: string): Promise<void> {
    const retryPath = `/api${taskURL(repoPath, commit, taskName)}/retry`
    const result = await load(() => postJSON<{ enqueued: boolean }>(retryPath))
    if (result) await loadTask(`/api${taskURL(repoPath, commit, taskName)}`)
  }

  return {
    activeEntry,
    artifactList,
    currentArtifact,
    currentCommit,
    currentRepo,
    currentTask,
    error,
    home,
    loadArtifact,
    loadArtifactList,
    loadCommit,
    loadHome,
    loadQueue,
    loadRepo,
    loadTask,
    loading,
    queue,
    queueCount,
    retryTask,
  }
})
