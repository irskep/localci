<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import TopBar from '@/components/TopBar.vue'
import RepoLink from '@/components/RepoLink.vue'
import RunLink from '@/components/RunLink.vue'
import RunList from '@/components/RunList.vue'
import WebsocketStatus from '@/components/WebsocketStatus.vue'
import { taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()
const route = useRoute()

const recentRows = computed(() => store.home?.recent_commits ?? [])
const repoRows = computed(() => store.home?.repos ?? [])
const queueRows = computed(() => store.home?.queue.pending ?? [])
const active = computed(() => store.home?.queue.active)
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

    <template v-if="store.home">
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
          <PPanel header="Active Now">
            <div class="active-panel-content">
              <WebsocketStatus />
              <div v-if="active" class="inline-link-list">
                <RepoLink :repo-path="active.repo.repo_path" />
                <RunLink :repo-path="active.repo.repo_path" :commit="active.commit" />
                <RouterLink :to="taskURL(active.repo.repo_path, active.commit, active.task)">
                  {{ active.task }}
                </RouterLink>
              </div>
              <div v-else class="empty-state">No task is running.</div>
            </div>
          </PPanel>

          <PPanel header="Queue">
            <template #icons>
              <RouterLink to="/queue">See more</RouterLink>
            </template>
            <ul v-if="queueRows.length > 0" class="artifact-list">
              <li
                v-for="entry in queueRows.slice(0, 6)"
                :key="`${entry.repo.repo_path}:${entry.commit}:${entry.task}`"
              >
                <i class="pi pi-clock run-task-icon run-task-icon-queued" aria-hidden="true"></i>
                <span class="inline-link-list">
                  <RepoLink :repo-path="entry.repo.repo_path" />
                  <RouterLink :to="taskURL(entry.repo.repo_path, entry.commit, entry.task)">
                    {{ entry.task }}
                  </RouterLink>
                </span>
              </li>
            </ul>
            <div v-else class="empty-state">Queue is idle.</div>
          </PPanel>

          <PPanel header="Repo">
            <ul class="artifact-list">
              <li v-for="repo in repoRows" :key="repo.repo_path">
                <RepoLink :repo-path="repo.repo_path" />
              </li>
            </ul>
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

.artifact-list {
  display: grid;
  gap: var(--app-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}

.artifact-list li {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  min-width: 0;
}

.artifact-list a {
  min-width: 0;
  padding: var(--app-space-3) 0;
  overflow-wrap: anywhere;
}

.active-panel-content {
  display: grid;
  gap: var(--app-space-3);
  min-width: 0;
}

.inline-link-list {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--app-space-3);
  min-width: 0;
}

@media (max-width: 860px) {
  .section-grid {
    grid-template-columns: 1fr;
  }
}
</style>
