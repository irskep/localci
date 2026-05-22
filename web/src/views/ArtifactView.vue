<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import { attemptURL, commitURL, parseRepoRoute, repoPathURL, taskURL } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const taskName = computed(() => store.currentArtifact?.task ?? parsed.value.taskName ?? 'Task')

async function load(): Promise<void> {
  if (parsed.value.kind !== 'artifact') return
  await store.loadArtifact(parsed.value.apiPath)
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
        {
          label: store.currentArtifact?.repo.repo_path ?? parsed.repoPath,
          to: repoPathURL(parsed.repoPath),
        },
        {
          label: parsed.commit ? parsed.commit.slice(0, 12) : 'Commit',
          to: parsed.commit ? commitURL(parsed.repoPath, parsed.commit) : undefined,
        },
        {
          label: taskName,
          to:
            parsed.commit && parsed.taskName
              ? taskURL(parsed.repoPath, parsed.commit, parsed.taskName)
              : undefined,
        },
        {
          label: parsed.attempt ? `attempt ${parsed.attempt}` : 'Attempt',
          to:
            parsed.commit && parsed.taskName && parsed.attempt
              ? attemptURL(parsed.repoPath, parsed.commit, parsed.taskName, parsed.attempt)
              : undefined,
        },
        { label: parsed.artifactPath ?? 'Artifact' },
      ]"
    />

    <section class="page-header">
      <span class="eyebrow">Artifact</span>
      <h1 class="page-title">{{ parsed.artifactPath }}</h1>
      <p class="page-subtitle">
        {{ store.currentArtifact?.repo.repo_path }} / {{ parsed.taskName }} / attempt
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
