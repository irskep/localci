<script setup lang="ts">
import { computed, onMounted } from 'vue'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()
const repos = computed(() => store.home?.repos ?? [])

onMounted(async () => {
  if (!store.home) await store.loadHome()
})
</script>

<template>
  <main class="page">
    <AppBreadcrumbs :items="[{ label: 'Home', to: '/' }, { label: 'Repo' }]" />

    <section class="page-header">
      <span class="eyebrow">Repo</span>
      <h1 class="page-title">Tracked repositories</h1>
    </section>

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.home" class="loading-state">
      <PProgressSpinner style="width: 1.5rem; height: 1.5rem" />
      <span>Loading repos</span>
    </div>
    <PDataTable v-if="repos.length > 0" :value="repos" size="small" class="panel">
      <PColumn header="Name">
        <template #body="{ data }">
          <RouterLink :to="`/repo/${data.repo_path}`">{{ data.repo_name }}</RouterLink>
        </template>
      </PColumn>
      <PColumn field="repo_path" header="Path" />
    </PDataTable>
    <div v-else-if="store.homeLoaded && !store.error" class="panel empty-state">
      No repos have localci history yet.
    </div>
  </main>
</template>
