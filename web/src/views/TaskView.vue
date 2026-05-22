<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import { artifactURL, attemptURL, parseRepoRoute } from '@/lib/routes'
import { formatDuration, statusSeverity } from '@/lib/api'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const task = computed(() => store.currentTask?.task)

async function load(): Promise<void> {
  if (parsed.value.kind !== 'task' && parsed.value.kind !== 'attempt') return
  await store.loadTask(parsed.value.apiPath)
}

async function retry(): Promise<void> {
  if (!parsed.value.commit || !parsed.value.taskName) return
  await store.retryTask(parsed.value.repoPath, parsed.value.commit, parsed.value.taskName)
}

onMounted(load)
watch(() => route.path, load)
</script>

<template>
  <main class="page task-page">
    <section class="page-header">
      <span class="eyebrow">Task</span>
      <h1 class="page-title">{{ task?.short_name ?? parsed.taskName }}</h1>
      <p class="page-subtitle">
        {{ store.currentTask?.repo.repo_name }}
        <template v-if="parsed.commit"> / {{ parsed.commit }}</template>
      </p>
    </section>

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !task" class="loading-state">
      <PProgressSpinner style="width: 1.5rem; height: 1.5rem" />
      <span>Loading task</span>
    </div>

    <template v-if="task && parsed.commit && parsed.taskName">
      <section class="task-layout">
        <aside class="task-sidebar">
          <div class="panel">
            <div class="panel-header">
              <h2 class="panel-title">Latest</h2>
              <PButton label="Retry" size="small" icon="pi pi-refresh" @click="retry" />
            </div>
            <div class="panel-body">
              <p>
                <PTag :severity="statusSeverity(task.status)" :value="task.status" />
              </p>
              <p>Attempt {{ task.attempt }} of {{ task.attempt_count }}</p>
              <p v-if="task.duration_ms > 0">{{ formatDuration(task.duration_ms) }}</p>
              <p v-if="task.failure">{{ task.failure }}</p>
            </div>
          </div>

          <div class="panel">
            <div class="panel-header">
              <h2 class="panel-title">Attempts</h2>
            </div>
            <ul class="attempt-list">
              <li v-for="attempt in task.attempts" :key="attempt.attempt">
                <RouterLink
                  :to="attemptURL(parsed.repoPath, parsed.commit, parsed.taskName, attempt.attempt)"
                >
                  <span>attempt {{ attempt.attempt }}</span>
                  <span>
                    <PTag :severity="statusSeverity(attempt.status)" :value="attempt.status" />
                    {{ formatDuration(attempt.duration_ms) }}
                  </span>
                </RouterLink>
              </li>
            </ul>
          </div>

          <div class="panel">
            <div class="panel-header">
              <h2 class="panel-title">Artifacts</h2>
            </div>
            <ul v-if="task.artifacts.length > 0" class="artifact-list">
              <li v-for="artifact in task.artifacts" :key="artifact.display_name">
                <i class="pi pi-file" aria-hidden="true"></i>
                <RouterLink
                  :to="
                    artifactURL(
                      parsed.repoPath,
                      parsed.commit,
                      parsed.taskName,
                      task.attempt,
                      artifact.display_name,
                    )
                  "
                >
                  {{ artifact.display_name }}
                </RouterLink>
              </li>
            </ul>
            <div v-else class="empty-state">No artifacts for this attempt.</div>
          </div>
        </aside>

        <div class="task-log-panel panel">
          <div class="panel-header">
            <h2 class="panel-title">Primary Log</h2>
            <div class="task-log-meta">
              <PTag :severity="statusSeverity(task.status)" :value="task.status" />
              <span class="muted mono">{{
                store.currentTask?.primary_artifact || 'combined.log'
              }}</span>
            </div>
          </div>
          <pre class="task-log-view">{{
            store.currentTask?.primary_log || 'No primary log content.'
          }}</pre>
        </div>
      </section>
    </template>
  </main>
</template>
