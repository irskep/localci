<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import {
  displayStatusSeverity,
  displayTaskFailure,
  displayTaskStatus,
  formatDuration,
  shortCommit,
} from '@/lib/api'
import { parseRepoRoute, repoPathURL, taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const commit = computed(() => store.currentCommit?.commit)
const tasks = computed(() => commit.value?.tasks ?? [])
const subscribedPage = ref('')
const title = computed(() => {
  const commitLabel = parsed.value.commit ? shortCommit(parsed.value.commit) : 'Commit'
  const repoLabel = store.currentCommit?.repo.repo_path ?? parsed.value.repoPath
  return repoLabel ? `${repoLabel} ${commitLabel}` : commitLabel
})

useDocumentTitle(title)

function subscribe(): void {
  if (parsed.value.kind !== 'commit') return
  subscribedPage.value = parsed.value.apiPath
  store.subscribeCommit(parsed.value.apiPath)
}

onMounted(subscribe)
watch(() => route.path, subscribe)
onUnmounted(() => store.unsubscribePage(subscribedPage.value))
</script>

<template>
  <main class="page">
    <AppBreadcrumbs
      :items="[
        { label: 'Home', to: '/' },
        {
          label: store.currentCommit?.repo.repo_path ?? parsed.repoPath,
          to: repoPathURL(parsed.repoPath),
        },
        { label: parsed.commit ? shortCommit(parsed.commit) : 'Commit' },
      ]"
    />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !commit" class="loading-state">
      <PProgressSpinner />
      <span>Loading commit</span>
    </div>

    <template v-if="commit">
      <PDataTable :value="tasks" data-key="name" size="small" class="table-surface">
        <template #header>Tasks</template>
        <PColumn header="Task">
          <template #body="{ data }">
            <RouterLink :to="taskURL(parsed.repoPath, commit.commit, data.name)">
              {{ data.short_name }}
            </RouterLink>
          </template>
        </PColumn>
        <PColumn header="Status">
          <template #body="{ data }">
            <PTag :severity="displayStatusSeverity(data)" :value="displayTaskStatus(data)" />
          </template>
        </PColumn>
        <PColumn header="Attempt">
          <template #body="{ data }">
            <span v-if="data.attempt > 0">
              {{ data.attempt }}
              <template v-if="data.attempt_count > data.attempt">
                of {{ data.attempt_count }}</template
              >
            </span>
            <span v-else class="muted">not run</span>
          </template>
        </PColumn>
        <PColumn header="Duration">
          <template #body="{ data }">{{ formatDuration(data.duration_ms) }}</template>
        </PColumn>
        <PColumn header="Failure">
          <template #body="{ data }">
            <span class="muted">{{ displayTaskFailure(data) }}</span>
          </template>
        </PColumn>
      </PDataTable>
    </template>
  </main>
</template>
