<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import TopBar from '@/components/TopBar.vue'
import ArtifactActions from '@/components/ArtifactActions.vue'
import {
  artifactURL,
  attemptURL,
  commitURL,
  parseRepoRoute,
  repoPathURL,
  taskURL,
} from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import type { ArtifactView } from '@/lib/api'
import { displayStatusSeverity, displayTaskStatus, formatDuration, shortCommit } from '@/lib/api'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const router = useRouter()
const store = useLocalciStore()
const canceling = ref(false)
const parsed = computed(() => parseRepoRoute(route.path))
const taskResponse = computed(() => store.taskResponseFor(parsed.value.apiPath))
const task = computed(() => taskResponse.value?.task)
const taskError = computed(() => store.taskErrorFor(parsed.value.apiPath))
const taskName = computed(() => task.value?.name ?? parsed.value.taskName ?? 'Task')
const canCancel = computed(
  () => task.value?.status === 'running' || task.value?.status === 'queued',
)
const title = computed(() => {
  const commitLabel = parsed.value.commit ? shortCommit(parsed.value.commit) : ''
  return commitLabel ? `${taskName.value} ${commitLabel}` : taskName.value
})

useDocumentTitle(title)

function subscribe(): void {
  if (parsed.value.kind !== 'task' && parsed.value.kind !== 'attempt') return
  store.subscribeTask(parsed.value.apiPath)
}

async function retry(): Promise<void> {
  if (!parsed.value.commit || !parsed.value.taskName) return
  const result = await store.retryTask(
    parsed.value.repoPath,
    parsed.value.commit,
    parsed.value.taskName,
  )
  if (!result?.url) return
  if (route.path !== result.url) await router.push(result.url)
}

async function cancel(): Promise<void> {
  if (!parsed.value.commit || !parsed.value.taskName || !canCancel.value || canceling.value) return
  canceling.value = true
  const result = await store.cancelTask(
    parsed.value.repoPath,
    parsed.value.commit,
    parsed.value.taskName,
  )
  if (!result?.canceled) canceling.value = false
}

function artifactPath(displayName: string): string {
  if (!parsed.value.commit || !parsed.value.taskName || !task.value) return ''
  return artifactURL(
    parsed.value.repoPath,
    parsed.value.commit,
    parsed.value.taskName,
    task.value.attempt,
    displayName,
  )
}

function artifactApiPath(displayName: string): string {
  return `/api${artifactPath(displayName)}`
}

function isWebArtifact(displayName: string): boolean {
  return /\.(html?|svg|png|jpe?g|gif|webp|css|js|json|pdf)$/i.test(displayName)
}

function artifactPrimaryIcon(artifact: ArtifactView): string {
  if (artifact.raw_url && isWebArtifact(artifact.display_name)) return 'pi pi-globe'
  if (artifact.is_text) return 'pi pi-align-left'
  return 'pi pi-download'
}

function artifactPrimaryHref(artifact: ArtifactView): string {
  if (artifact.raw_url && isWebArtifact(artifact.display_name)) return artifact.raw_url
  if (!artifact.is_text && artifact.download_url) return artifact.download_url
  return artifactPath(artifact.display_name)
}

function artifactPrimaryLabel(artifact: ArtifactView): string {
  if (artifact.raw_url && isWebArtifact(artifact.display_name)) return 'Open webpage artifact'
  if (artifact.is_text) return 'Open live log artifact'
  return 'Download artifact'
}

onMounted(subscribe)
watch(
  () => route.path,
  () => {
    canceling.value = false
    subscribe()
  },
)
watch(canCancel, (cancelable) => {
  if (!cancelable) canceling.value = false
})
onUnmounted(() => store.unsubscribeTask())
</script>

<template>
  <main class="page task-page">
    <TopBar
      :items="[
        { label: 'Home', to: '/' },
        {
          label: taskResponse?.repo.repo_path ?? parsed.repoPath,
          to: repoPathURL(parsed.repoPath),
        },
        {
          label: parsed.commit ? shortCommit(parsed.commit) : 'Commit',
          to: parsed.commit ? commitURL(parsed.repoPath, parsed.commit) : undefined,
        },
        {
          label: taskName,
          to:
            parsed.kind === 'attempt' && parsed.commit && parsed.taskName
              ? taskURL(parsed.repoPath, parsed.commit, parsed.taskName)
              : undefined,
        },
        ...(parsed.kind === 'attempt' && parsed.attempt
          ? [{ label: `attempt ${parsed.attempt}` }]
          : []),
      ]"
    />

    <PMessage v-if="taskError && !task" severity="error" :closable="false">{{
      taskError
    }}</PMessage>
    <div v-if="store.taskLoadingFor(parsed.apiPath) && !task" class="loading-state">
      <PProgressSpinner />
      <span>Loading task</span>
    </div>

    <template v-if="task && parsed.commit && parsed.taskName">
      <section class="task-layout">
        <aside class="task-sidebar">
          <PPanel header="Attempts">
            <template #icons>
              <div class="panel-actions">
                <PButton
                  :label="canceling ? 'Canceling...' : 'Cancel'"
                  size="small"
                  severity="danger"
                  outlined
                  icon="pi pi-stop-circle"
                  :loading="canceling"
                  :disabled="!canCancel || canceling"
                  @click="cancel"
                />
                <PButton label="Retry" size="small" icon="pi pi-refresh" @click="retry" />
              </div>
            </template>
            <ul class="attempt-list">
              <li v-for="attempt in task.attempts" :key="attempt.attempt">
                <RouterLink
                  :to="attemptURL(parsed.repoPath, parsed.commit, parsed.taskName, attempt.attempt)"
                >
                  <span>attempt {{ attempt.attempt }}</span>
                  <span>
                    <PTag
                      :severity="displayStatusSeverity(attempt)"
                      :value="displayTaskStatus(attempt)"
                    />
                    {{ formatDuration(attempt.duration_ms) }}
                  </span>
                </RouterLink>
              </li>
            </ul>
          </PPanel>

          <PPanel header="Artifacts">
            <ul v-if="task.artifacts.length > 0" class="artifact-list">
              <li v-for="artifact in task.artifacts" :key="artifact.display_name">
                <i :class="artifactPrimaryIcon(artifact)" aria-hidden="true"></i>
                <a
                  :href="artifactPrimaryHref(artifact)"
                  :aria-label="`${artifactPrimaryLabel(artifact)}: ${artifact.display_name}`"
                >
                  {{ artifact.display_name }}
                </a>
                <ArtifactActions
                  :path="artifact.path"
                  :api-path="artifactApiPath(artifact.display_name)"
                  :raw-url="artifact.raw_url"
                  :download-url="artifact.download_url"
                  mode="menu"
                />
              </li>
            </ul>
            <div v-else class="empty-state">No artifacts for this attempt.</div>
          </PPanel>
        </aside>

        <PPanel header="Primary Log" class="task-log-panel">
          <template #icons>
            <div class="task-log-meta">
              <PTag v-if="taskError" severity="warn" :value="taskError" />
              <PTag :severity="displayStatusSeverity(task)" :value="displayTaskStatus(task)" />
              <span class="muted mono">{{ taskResponse?.primary_artifact || 'combined.log' }}</span>
            </div>
          </template>
          <pre class="task-log-view">{{
            taskResponse?.primary_log || 'No primary log content.'
          }}</pre>
        </PPanel>
      </section>
    </template>
  </main>
</template>

<style scoped>
.task-layout {
  display: grid;
  grid-template-columns: 320px minmax(0, 1fr);
  gap: var(--app-space-5);
  align-items: stretch;
  align-self: stretch;
  min-width: 0;
  min-height: 0;
  height: 100%;
  overflow: hidden;
}

.task-sidebar {
  display: grid;
  align-content: start;
  gap: var(--app-space-5);
  min-width: 0;
  min-height: 0;
  overflow: auto;
}

.task-log-panel {
  display: grid;
  grid-template-rows: auto minmax(0, 1fr);
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

:global(.task-log-panel .p-panel-content-container),
:global(.task-log-panel .p-panel-content-wrapper),
:global(.task-log-panel .p-panel-content) {
  min-height: 0;
}

:global(.task-log-panel .p-panel-content-container),
:global(.task-log-panel .p-panel-content-wrapper) {
  display: grid;
}

:global(.task-log-panel .p-panel-content) {
  display: grid;
  padding: 0;
}

.task-log-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--app-space-3);
  min-width: 0;
  overflow-wrap: anywhere;
}

.panel-actions {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  justify-content: flex-end;
  gap: var(--p-navigation-item-gap);
}

.attempt-list,
.artifact-list {
  display: grid;
  margin: 0;
  padding: 0;
  list-style: none;
}

.attempt-list {
  gap: var(--app-space-3);
}

.attempt-list a {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--app-space-5);
  padding: var(--app-item-padding);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-content-border-radius);
}

.artifact-list {
  gap: var(--app-space-3);
}

.artifact-list li {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  min-width: 0;
}

.artifact-list a {
  flex: 1 1 auto;
  min-width: 0;
  padding: var(--app-space-3) 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.task-log-view {
  overflow: auto;
  width: 100%;
  min-width: 0;
  min-height: 0;
  margin: 0;
  padding: var(--app-space-5);
  border-top: 1px solid var(--p-content-border-color);
  background: var(--p-surface-0);
  color: var(--p-text-color);
  font-size: var(--app-log-font-size);
  line-height: var(--app-log-line-height);
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}

@media (max-width: 860px) {
  .task-layout {
    grid-template-columns: 1fr;
  }
}
</style>
