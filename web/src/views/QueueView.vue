<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { taskURL } from '@/lib/routes'

type RepoSummary = {
  repo_path: string
  repo_name: string
}

type QueueEntry = {
  repo: RepoSummary
  commit: string
  task: string
}

type QueueResponse = {
  active?: QueueEntry
  pending: QueueEntry[]
}

const data = ref<QueueResponse | null>(null)
const error = ref('')

onMounted(async () => {
  try {
    const response = await fetch('/api/queue')
    if (!response.ok) {
      throw new Error(`request failed with ${response.status}`)
    }
    data.value = (await response.json()) as QueueResponse
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
})
</script>

<template>
  <main>
    <p><a href="/">Home</a></p>
    <h1>Queue</h1>
    <p v-if="error">{{ error }}</p>
    <template v-else-if="data">
      <ul>
        <li v-if="data.active">
          active —
          <a :href="taskURL(data.active.repo.repo_path, data.active.commit, data.active.task)">{{
            data.active.task
          }}</a>
        </li>
        <li
          v-for="entry in data.pending"
          :key="`${entry.repo.repo_path}:${entry.commit}:${entry.task}`"
        >
          pending —
          <a :href="taskURL(entry.repo.repo_path, entry.commit, entry.task)">{{ entry.task }}</a>
        </li>
        <li v-if="!data.active && data.pending.length === 0">idle</li>
      </ul>
    </template>
  </main>
</template>
