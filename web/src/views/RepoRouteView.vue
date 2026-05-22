<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import { annotationEntries, shortCommit, summarizeCommit } from '@/lib/api'
import { commitURL, parseRepoRoute } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const commits = computed(() => store.currentRepo?.commits ?? [])

async function load(): Promise<void> {
  if (parsed.value.kind !== 'repo') return
  await store.loadRepo(parsed.value.apiPath)
}

onMounted(load)
watch(() => route.path, load)
</script>

<template>
  <main class="page">
    <AppBreadcrumbs
      :items="[
        { label: 'Home', to: '/' },
        { label: 'Repo', to: '/repo' },
        { label: store.currentRepo?.repo.repo_name ?? parsed.repoPath },
      ]"
    />

    <section class="page-header">
      <span class="eyebrow">Repo</span>
      <h1 class="page-title">{{ store.currentRepo?.repo.repo_name ?? parsed.repoPath }}</h1>
      <p class="page-subtitle mono">{{ parsed.repoPath }}</p>
    </section>

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.currentRepo" class="loading-state">
      <PProgressSpinner style="width: 1.5rem; height: 1.5rem" />
      <span>Loading repo</span>
    </div>

    <div class="panel">
      <div class="panel-header">
        <h2 class="panel-title">Commit Activity</h2>
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
          <template #body="{ data }">{{ summarizeCommit(data) }}</template>
        </PColumn>
        <PColumn header="Attributes">
          <template #body="{ data }">
            <span class="attribute-list">
              <span
                v-for="attribute in annotationEntries(data.annotations)"
                :key="attribute.key"
                class="attribute-pill"
              >
                {{ attribute.key }}: {{ attribute.value }}
              </span>
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
    </div>
  </main>
</template>
