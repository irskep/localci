<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import TopBar from '@/components/TopBar.vue'
import RepoLink from '@/components/RepoLink.vue'
import RunList from '@/components/RunList.vue'
import SetupEmptyState from '@/components/SetupEmptyState.vue'
import WebsocketStatus from '@/components/WebsocketStatus.vue'
import { shortCommit, type QueueEntry } from '@/lib/api'
import { taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

type QueueDisplayEntry = QueueEntry & {
  state: 'running' | 'queued'
}

const store = useLocalciStore()
const route = useRoute()

const recentRows = computed(() => store.home?.recent_commits ?? [])
const repoRows = computed(() => store.home?.repos ?? [])
const queueRows = computed(() => store.home?.queue.pending ?? [])
const active = computed(() => store.home?.queue.active)
const queueDisplayRows = computed<QueueDisplayEntry[]>(() => [
  ...(active.value ? [{ ...active.value, state: 'running' as const }] : []),
  ...queueRows.value.map((entry) => ({ ...entry, state: 'queued' as const })),
])
const isFreshEmpty = computed(
  () =>
    !!store.home &&
    recentRows.value.length === 0 &&
    repoRows.value.length === 0 &&
    queueDisplayRows.value.length === 0,
)
const currentBefore = computed(() => queryString(route.query.before))
const newerPage = computed(() => {
  if (!currentBefore.value) return undefined
  const before = store.home?.newer_before
  return { path: route.path, query: before ? { before } : {} }
})
const olderPage = computed(() => {
  const before = store.home?.next_before
  return before ? { path: route.path, query: { before } } : undefined
})
const loadingPage = ref(false)
const subscribedPage = ref('')

useDocumentTitle('Overview')

watch(() => route.query.before, loadCurrentPage, { immediate: true })
onUnmounted(() => store.unsubscribePage(subscribedPage.value))

function queryString(value: unknown): string | undefined {
  if (typeof value === 'string' && value !== '') return value
  if (Array.isArray(value) && typeof value[0] === 'string' && value[0] !== '') return value[0]
  return undefined
}

async function loadCurrentPage(): Promise<void> {
  loadingPage.value = true
  try {
    if (currentBefore.value) {
      store.unsubscribePage(subscribedPage.value)
      subscribedPage.value = ''
      await store.loadHomePage(currentBefore.value)
    } else {
      subscribedPage.value = '/api'
      store.subscribeHome()
    }
  } finally {
    loadingPage.value = false
  }
}
</script>

<template>
  <main class="page">
    <TopBar :items="[{ label: 'Home' }]" />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.home" class="loading-state">
      <PProgressSpinner />
      <span>Loading localci state</span>
    </div>

    <div v-if="isFreshEmpty" class="fresh-empty-wrap">
      <SetupEmptyState />
    </div>

    <template v-else-if="store.home">
      <section class="section-grid">
        <div class="stack">
          <RunList
            :runs="recentRows"
            :newer-to="newerPage"
            :older-to="olderPage"
            :loading-page="loadingPage"
          />
        </div>

        <aside class="stack">
          <PPanel header="Queue">
            <div class="queue-panel-content">
              <WebsocketStatus />
              <TransitionGroup
                v-if="queueDisplayRows.length > 0"
                tag="ul"
                name="queue-row"
                class="queue-list"
                appear
              >
                <li
                  v-for="entry in queueDisplayRows.slice(0, 6)"
                  :key="`${entry.repo.repo_path}:${entry.commit}:${entry.task}:${entry.attempt}`"
                  class="queue-entry"
                >
                  <i
                    :class="
                      entry.state === 'running'
                        ? 'pi pi-spin pi-spinner run-task-icon run-task-icon-running'
                        : 'pi pi-clock run-task-icon run-task-icon-queued'
                    "
                    aria-hidden="true"
                  ></i>
                  <div class="queue-entry-body">
                    <RouterLink
                      class="queue-task"
                      :to="taskURL(entry.repo.repo_path, entry.commit, entry.task)"
                    >
                      {{ entry.task }}
                    </RouterLink>
                    <div class="queue-meta">
                      <RepoLink
                        :repo-path="entry.repo.repo_path"
                        :repo-label="entry.repo.repo_label"
                      />
                      <span>{{ shortCommit(entry.commit) }}</span>
                      <span v-if="entry.attempt > 1">attempt {{ entry.attempt }}</span>
                    </div>
                  </div>
                </li>
              </TransitionGroup>
              <div v-else class="empty-state">Queue is idle.</div>
            </div>
          </PPanel>
        </aside>
      </section>
    </template>
  </main>
</template>

<style scoped>
.section-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: var(--app-space-5);
  align-items: start;
  min-width: 0;
}

.fresh-empty-wrap {
  display: grid;
  place-items: center;
  min-height: calc(100vh - var(--app-page-block-padding) * 4);
}

.queue-list {
  position: relative;
  display: grid;
  gap: var(--app-space-3);
  margin: 0;
  padding: 0;
  list-style: none;
}

.queue-entry {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  align-items: start;
  gap: var(--app-space-3);
  min-width: 0;
}

.queue-entry > .run-task-icon {
  margin-top: 0.2em;
}

.queue-entry-body {
  display: grid;
  gap: var(--app-space-1);
  min-width: 0;
}

.queue-task {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.queue-meta {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--app-space-2);
  min-width: 0;
  color: var(--p-text-muted-color);
  font-size: var(--p-form-field-sm-font-size);
}

.queue-panel-content {
  display: grid;
  gap: var(--app-space-3);
  min-width: 0;
}

.queue-row-enter-active,
.queue-row-leave-active,
.queue-row-move {
  transition:
    opacity 160ms ease,
    transform 160ms ease;
}

.queue-row-enter-from,
.queue-row-leave-to {
  opacity: 0;
  transform: translateY(calc(var(--app-space-2) * -1));
}

.queue-row-leave-active {
  position: absolute;
  width: 100%;
}

@media (max-width: 860px) {
  .section-grid {
    grid-template-columns: 1fr;
  }
}
</style>
