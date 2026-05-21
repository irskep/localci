<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import { artifactURL, attemptURL, commitURL, parseRepoRoute, taskURL } from '@/lib/routes'

type RepoSummary = {
  repo_path: string
  repo_name: string
}

type CommitStatusView = {
  repo_dir: string
  commit: string
  tasks: TaskStatusView[]
}

type TaskStatusView = {
  name: string
  short_name: string
  attempt: number
  attempt_count: number
  status: string
  failure: string
  duration_ms: number
  artifacts: ArtifactView[]
  attempts: Array<{
    attempt: number
    status: string
    failure: string
    duration_ms: number
  }>
}

type ArtifactView = {
  display_name: string
}

const route = useRoute()

const data = ref<unknown>(null)
const error = ref('')
const loading = ref(false)

const parsed = computed(() => parseRepoRoute(route.path))

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    const response = await fetch(parsed.value.apiPath)
    if (!response.ok) {
      throw new Error(`request failed with ${response.status}`)
    }
    data.value = await response.json()
  } catch (err) {
    data.value = null
    error.value = err instanceof Error ? err.message : String(err)
  } finally {
    loading.value = false
  }
}

async function retryTask(): Promise<void> {
  if (parsed.value.kind !== 'task' && parsed.value.kind !== 'attempt') {
    return
  }
  const response = await fetch(
    `/api${taskURL(parsed.value.repoPath, parsed.value.commit!, parsed.value.taskName!)}/retry`,
    {
      method: 'POST',
    },
  )
  if (!response.ok) {
    error.value = `retry failed with ${response.status}`
    return
  }
  await load()
}

onMounted(load)
watch(() => route.path, load)
</script>

<template>
  <main>
    <p><a href="/">Home</a></p>
    <p v-if="loading">Loading…</p>
    <p v-else-if="error">{{ error }}</p>

    <template v-else-if="parsed.kind === 'repo-index'">
      <h1>Repo</h1>
      <ul v-if="Array.isArray(data)">
        <li v-for="repo in data as RepoSummary[]" :key="repo.repo_path">
          <a :href="`/repo/${repo.repo_path}`">{{ repo.repo_name }}</a>
        </li>
      </ul>
    </template>

    <template v-else-if="parsed.kind === 'repo'">
      <h1>{{ (data as { repo: RepoSummary }).repo.repo_name }}</h1>
      <ul>
        <li
          v-for="commit in (data as { commits: CommitStatusView[] }).commits"
          :key="commit.commit"
        >
          <a :href="commitURL(parsed.repoPath, commit.commit)">{{ commit.commit }}</a>
          — {{ commit.tasks.length }} tasks
        </li>
      </ul>
    </template>

    <template v-else-if="parsed.kind === 'commit'">
      <h1>{{ (data as { commit: CommitStatusView }).commit.commit }}</h1>
      <ul>
        <li v-for="task in (data as { commit: CommitStatusView }).commit.tasks" :key="task.name">
          <a :href="taskURL(parsed.repoPath, parsed.commit!, task.name)">{{ task.short_name }}</a>
          — {{ task.status }}
          <template v-if="task.attempt > 0"> · attempt {{ task.attempt }}</template>
          <template v-if="task.duration_ms > 0"> · {{ task.duration_ms }}ms</template>
        </li>
      </ul>
    </template>

    <template v-else-if="parsed.kind === 'task' || parsed.kind === 'attempt'">
      <template
        v-if="data as { task: TaskStatusView; primary_artifact: string; primary_log: string }"
      >
        <h1>{{ (data as { task: TaskStatusView }).task.short_name }}</h1>
        <p>Status: {{ (data as { task: TaskStatusView }).task.status }}</p>
        <p>Attempt: {{ (data as { task: TaskStatusView }).task.attempt }}</p>
        <p v-if="(data as { task: TaskStatusView }).task.failure">
          Failure: {{ (data as { task: TaskStatusView }).task.failure }}
        </p>
        <button type="button" @click="retryTask">Retry task</button>

        <h2>Attempts</h2>
        <ul>
          <li
            v-for="attempt in (data as { task: TaskStatusView }).task.attempts"
            :key="attempt.attempt"
          >
            <a
              :href="attemptURL(parsed.repoPath, parsed.commit!, parsed.taskName!, attempt.attempt)"
            >
              attempt {{ attempt.attempt }}
            </a>
            — {{ attempt.status }}
          </li>
        </ul>

        <h2>Log</h2>
        <p>{{ (data as { primary_artifact: string }).primary_artifact }}</p>
        <pre>{{ (data as { primary_log: string }).primary_log }}</pre>

        <h2>Artifacts</h2>
        <ul>
          <li
            v-for="artifact in (data as { task: TaskStatusView }).task.artifacts"
            :key="artifact.display_name"
          >
            <a
              :href="
                artifactURL(
                  parsed.repoPath,
                  parsed.commit!,
                  parsed.taskName!,
                  (data as { task: TaskStatusView }).task.attempt,
                  artifact.display_name,
                )
              "
            >
              {{ artifact.display_name }}
            </a>
          </li>
        </ul>
      </template>
    </template>

    <template v-else-if="parsed.kind === 'artifact'">
      <h1>{{ parsed.artifactPath }}</h1>
      <pre>{{ (data as { content: string }).content }}</pre>
    </template>
  </main>
</template>
