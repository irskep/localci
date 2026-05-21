<script setup lang="ts">
import { onMounted, ref } from 'vue'

import { commitURL, taskURL } from '@/lib/routes'

type RepoSummary = {
  repo_path: string
  repo_name: string
}

type QueueEntry = {
  repo: RepoSummary
  commit: string
  task: string
}

type HomeResponse = {
  repos: RepoSummary[]
  recent_commits: Array<{
    repo: RepoSummary
    commit: string
    summary: string
  }>
  queue: {
    active?: QueueEntry
    pending: QueueEntry[]
  }
}

const data = ref<HomeResponse | null>(null)
const error = ref('')

onMounted(async () => {
  try {
    const response = await fetch('/api')
    if (!response.ok) {
      throw new Error(`request failed with ${response.status}`)
    }
    data.value = (await response.json()) as HomeResponse
  } catch (err) {
    error.value = err instanceof Error ? err.message : String(err)
  }
})
</script>

<template>
  <main>
    <h1>localci</h1>
    <p v-if="error">{{ error }}</p>
    <template v-else-if="data">
      <section>
        <h2>Repo</h2>
        <ul>
          <li v-for="repo in data.repos" :key="repo.repo_path">
            <a :href="`/repo/${repo.repo_path}`">{{ repo.repo_name }}</a>
          </li>
        </ul>
      </section>
      <section>
        <h2>Recent Commit Activity</h2>
        <ul>
          <li v-for="entry in data.recent_commits" :key="`${entry.repo.repo_path}:${entry.commit}`">
            <a :href="commitURL(entry.repo.repo_path, entry.commit)">{{ entry.commit }}</a>
            —
            {{ entry.summary }}
          </li>
        </ul>
      </section>
      <section>
        <h2>Queue</h2>
        <ul>
          <li v-if="data.queue.active">
            active —
            <a
              :href="
                taskURL(
                  data.queue.active.repo.repo_path,
                  data.queue.active.commit,
                  data.queue.active.task,
                )
              "
            >
              {{ data.queue.active.task }}
            </a>
          </li>
          <li
            v-for="entry in data.queue.pending"
            :key="`${entry.repo.repo_path}:${entry.commit}:${entry.task}`"
          >
            pending —
            <a :href="taskURL(entry.repo.repo_path, entry.commit, entry.task)">{{ entry.task }}</a>
          </li>
          <li v-if="!data.queue.active && data.queue.pending.length === 0">idle</li>
        </ul>
      </section>
    </template>
  </main>
</template>
