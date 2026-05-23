<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
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

    <RunList
      v-if="commits.length > 0"
      :runs="commits"
      :repo-path="parsed.repoPath"
      :show-repo="false"
    />
    <div v-else-if="store.repoLoaded && !store.error" class="empty-state">
      No commits recorded for this repo.
    </div>
  </main>
</template>
