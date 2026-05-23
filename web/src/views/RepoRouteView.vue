<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import TaskSummaryLinks from '@/components/TaskSummaryLinks.vue'
import { annotationEntries, shortCommit, summarizeCommit } from '@/lib/api'
import { commitURL, parseRepoRoute } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const commits = computed(() => store.currentRepo?.commits ?? [])
const title = computed(() => store.currentRepo?.repo.repo_path ?? parsed.value.repoPath)

useDocumentTitle(title)

function subscribe(): void {
  if (parsed.value.kind !== 'repo') return
  store.subscribeRepo(parsed.value.apiPath)
}

onMounted(subscribe)
watch(() => route.path, subscribe)
onUnmounted(() => store.unsubscribePage())
</script>

<template>
  <main class="page">
    <AppBreadcrumbs
      :items="[
        { label: 'Home', to: '/' },
        { label: store.currentRepo?.repo.repo_path ?? parsed.repoPath },
      ]"
    />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.currentRepo" class="loading-state">
      <PProgressSpinner />
      <span>Loading repo</span>
    </div>

    <PDataTable v-if="commits.length > 0" :value="commits" data-key="commit" size="small">
      <PColumn header="Commit">
        <template #body="{ data }">
          <RouterLink :to="commitURL(parsed.repoPath, data.commit)" class="mono">
            {{ shortCommit(data.commit) }}
          </RouterLink>
        </template>
      </PColumn>
      <PColumn header="Summary">
        <template #body="{ data }">
          <div>{{ summarizeCommit(data) }}</div>
          <TaskSummaryLinks
            :repo-path="parsed.repoPath"
            :commit="data.commit"
            :tasks="data.tasks"
          />
        </template>
      </PColumn>
      <PColumn header="Attributes">
        <template #body="{ data }">
          <span class="attribute-list">
            <PTag
              v-for="attribute in annotationEntries(data.annotations)"
              :key="attribute.key"
              severity="secondary"
              :value="`${attribute.key}: ${attribute.value}`"
            />
          </span>
        </template>
      </PColumn>
      <PColumn header="Tasks">
        <template #body="{ data }">{{ data.tasks.length }}</template>
      </PColumn>
    </PDataTable>
    <div v-else-if="store.repoLoaded && !store.error" class="empty-state">
      No commits recorded for this repo.
    </div>
  </main>
</template>
