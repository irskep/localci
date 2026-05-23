<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import { shortCommit } from '@/lib/api'
import type { QueueEntry } from '@/lib/api'
import { taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()
const pending = computed(() => store.queue?.pending ?? [])
const active = computed(() => store.queue?.active)

useDocumentTitle('Queue')

function taskLabel(entry: QueueEntry): string {
  return `${entry.repo.repo_path} / ${shortCommit(entry.commit)}`
}

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
        <div v-if="active">
          <PTag severity="info" value="running" />
          <RouterLink :to="taskURL(active.repo.repo_path, active.commit, active.task)">
            {{ taskLabel(active) }} / {{ active.task }}
          </RouterLink>
        </div>
        <div v-else-if="store.queueLoaded && !store.error" class="empty-state">No active task.</div>
      </PPanel>

      <PPanel header="Pending">
        <PDataTable v-if="pending.length > 0" :value="pending" size="small">
          <PColumn header="#">
            <template #body="{ index }">{{ index + 1 }}</template>
          </PColumn>
          <PColumn header="Repo">
            <template #body="{ data }">
              <RouterLink :to="`/repo/${data.repo.repo_path}`">{{
                data.repo.repo_path
              }}</RouterLink>
            </template>
          </PColumn>
          <PColumn header="Commit">
            <template #body="{ data }">
              <RouterLink :to="`/repo/${data.repo.repo_path}/commit/${data.commit}`" class="mono">
                {{ shortCommit(data.commit) }}
              </RouterLink>
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
        <div v-else-if="store.queueLoaded && !store.error" class="empty-state">Queue is idle.</div>
      </PPanel>
    </section>
  </main>
</template>
