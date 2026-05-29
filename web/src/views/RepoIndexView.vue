<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'

import TopBar from '@/components/TopBar.vue'
import RepoLink from '@/components/RepoLink.vue'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()
const repos = computed(() => store.home?.repos ?? [])
const subscribedPage = ref('')

useDocumentTitle('Repos')

onMounted(() => {
  subscribedPage.value = '/api'
  store.subscribeHome()
})
onUnmounted(() => store.unsubscribePage(subscribedPage.value))
</script>

<template>
  <main class="page">
    <TopBar :items="[{ label: 'Home', to: '/' }, { label: 'Repo' }]" />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.home" class="loading-state">
      <PProgressSpinner />
      <span>Loading repos</span>
    </div>
    <PPanel v-if="repos.length > 0" header="Repos">
      <PDataTable :value="repos" size="small" class="table-surface">
        <PColumn header="Name">
          <template #body="{ data }">
            <RepoLink :repo-path="data.repo_path" />
          </template>
        </PColumn>
        <PColumn field="repo_path" header="Path" />
      </PDataTable>
    </PPanel>
    <div v-else-if="store.homeLoaded && !store.error" class="empty-state">
      No repos have localci history yet.
    </div>
  </main>
</template>
