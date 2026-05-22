<script setup lang="ts">
import { computed, onMounted } from 'vue'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import { shortCommit } from '@/lib/api'
import type { QueueEntry } from '@/lib/api'
import { taskURL } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()
const pending = computed(() => store.queue?.pending ?? [])
const active = computed(() => store.queue?.active)

function taskLabel(entry: QueueEntry): string {
  return `${entry.repo.repo_path} / ${shortCommit(entry.commit)}`
}

onMounted(() => {
  void store.loadQueue()
})
</script>

<template>
  <main class="page">
    <AppBreadcrumbs :items="[{ label: 'Home', to: '/' }, { label: 'Queue' }]" />

    <section class="page-header">
      <span class="eyebrow">Queue</span>
      <h1 class="page-title">Scheduler state</h1>
      <p class="page-subtitle">
        The queue is task-level: one active task and ordered pending entries.
      </p>
    </section>

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.queue" class="loading-state">
      <PProgressSpinner style="width: 1.5rem; height: 1.5rem" />
      <span>Loading queue</span>
    </div>

    <section v-if="store.queue" class="stack">
      <div class="panel">
        <div class="panel-header">
          <h2 class="panel-title">Active</h2>
        </div>
        <div v-if="active" class="panel-body">
          <PTag severity="info" value="running" />
          <RouterLink :to="taskURL(active.repo.repo_path, active.commit, active.task)">
            {{ taskLabel(active) }} / {{ active.task }}
          </RouterLink>
        </div>
        <div v-else-if="store.queueLoaded && !store.error" class="empty-state">No active task.</div>
      </div>

      <div class="panel">
        <div class="panel-header">
          <h2 class="panel-title">Pending</h2>
        </div>
        <PDataTable v-if="pending.length > 0" :value="pending" size="small">
          <PColumn header="#" style="width: 5rem">
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
      </div>
    </section>
  </main>
</template>
