<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import RunList from '@/components/RunList.vue'
import { parseRepoRoute } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const commits = computed(() => store.currentRepo?.commits ?? [])
const title = computed(() => store.currentRepo?.repo.repo_path ?? parsed.value.repoPath)
const currentBefore = computed(() => queryString(route.query.before))
const newerPage = computed(() => {
  if (!currentBefore.value || parsed.value.kind !== 'repo') return undefined
  const before = store.currentRepo?.newer_before
  return { path: route.path, query: before ? { before } : {} }
})
const olderPage = computed(() => {
  if (parsed.value.kind !== 'repo') return undefined
  const before = store.currentRepo?.next_before
  return before ? { path: route.path, query: { before } } : undefined
})
const loadingPage = ref(false)

useDocumentTitle(title)

watch(() => [route.path, route.query.before], loadCurrentPage, { immediate: true })
onUnmounted(() => store.unsubscribePage())

function queryString(value: unknown): string | undefined {
  if (typeof value === 'string' && value !== '') return value
  if (Array.isArray(value) && typeof value[0] === 'string' && value[0] !== '') return value[0]
  return undefined
}

async function loadCurrentPage(): Promise<void> {
  if (parsed.value.kind !== 'repo') return
  loadingPage.value = true
  try {
    if (currentBefore.value) {
      store.unsubscribePage()
      await store.loadRepoPage(parsed.value.apiPath, currentBefore.value)
    } else {
      store.subscribeRepo(parsed.value.apiPath)
    }
  } finally {
    loadingPage.value = false
  }
}
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

    <RunList
      v-if="commits.length > 0"
      :runs="commits"
      :repo-path="parsed.repoPath"
      :show-repo="false"
      :newer-to="newerPage"
      :older-to="olderPage"
      :loading-page="loadingPage"
    />
    <div v-else-if="store.repoLoaded && !store.error" class="empty-state">
      No commits recorded for this repo.
    </div>
  </main>
</template>
