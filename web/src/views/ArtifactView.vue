<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import { parseRepoRoute } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))

async function load(): Promise<void> {
  if (parsed.value.kind !== 'artifact') return
  await store.loadArtifact(parsed.value.apiPath)
}

onMounted(load)
watch(() => route.path, load)
</script>

<template>
  <main class="page">
    <section class="page-header">
      <span class="eyebrow">Artifact</span>
      <h1 class="page-title">{{ parsed.artifactPath }}</h1>
      <p class="page-subtitle">
        {{ store.currentArtifact?.repo.repo_name }} / {{ parsed.taskName }} / attempt
        {{ parsed.attempt }}
      </p>
    </section>

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.currentArtifact" class="loading-state">
      <PProgressSpinner style="width: 1.5rem; height: 1.5rem" />
      <span>Loading artifact</span>
    </div>

    <pre v-if="store.currentArtifact" class="log-view">{{ store.currentArtifact.content }}</pre>
  </main>
</template>
