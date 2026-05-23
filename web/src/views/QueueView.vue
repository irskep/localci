<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import RepoLink from '@/components/RepoLink.vue'
import RunLink from '@/components/RunLink.vue'
import { taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()
const pending = computed(() => store.queue?.pending ?? [])
const active = computed(() => store.queue?.active)

useDocumentTitle('Queue')

onMounted(() => {
  store.subscribeQueue()
})
onUnmounted(() => store.unsubscribePage())
</script>

<template>
  <main class="page">
    <AppBreadcrumbs :items="[{ label: 'Home', to: '/' }, { label: 'Queue' }]" />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.queue" class="loading-state">
      <PProgressSpinner />
      <span>Loading queue</span>
    </div>

    <section v-if="store.queue" class="stack">
      <PPanel header="Active">
        <div v-if="active" class="inline-link-list">
          <PTag severity="info" value="running" />
          <RepoLink :repo-path="active.repo.repo_path" />
          <RunLink :repo-path="active.repo.repo_path" :commit="active.commit" />
          <RouterLink :to="taskURL(active.repo.repo_path, active.commit, active.task)">
            {{ active.task }}
          </RouterLink>
        </div>
        <div v-else-if="store.queueLoaded && !store.error" class="empty-state">No active task.</div>
      </PPanel>

      <PDataTable v-if="pending.length > 0" :value="pending" size="small" class="table-surface">
        <template #header>Pending</template>
        <PColumn header="#">
          <template #body="{ index }">{{ index + 1 }}</template>
        </PColumn>
        <PColumn header="Repo">
          <template #body="{ data }">
            <RepoLink :repo-path="data.repo.repo_path" />
          </template>
        </PColumn>
        <PColumn header="Commit">
          <template #body="{ data }">
            <RunLink :repo-path="data.repo.repo_path" :commit="data.commit" />
          </template>
        </PColumn>
        <PColumn header="Task">
          <template #body="{ data }">
            <RouterLink :to="taskURL(data.repo.repo_path, data.commit, data.task)">
              {{ data.task }}
            </RouterLink>
          </template>
        </PColumn>
      </PDataTable>
      <PPanel v-else-if="store.queueLoaded && !store.error" header="Pending">
        <div class="empty-state">Queue is idle.</div>
      </PPanel>
    </section>
  </main>
</template>

<style scoped>
.inline-link-list {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--app-space-3);
  min-width: 0;
}
</style>
