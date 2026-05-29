<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import TopBar from '@/components/TopBar.vue'
import ArtifactActions from '@/components/ArtifactActions.vue'
import { shortCommit } from '@/lib/api'
import { attemptURL, commitURL, parseRepoRoute, repoPathURL, taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const taskName = computed(() => store.currentArtifact?.task ?? parsed.value.taskName ?? 'Task')
const title = computed(() => {
  const artifact =
    parsed.value.artifactPath ?? store.currentArtifact?.artifact.display_name ?? 'Artifact'
  return `${artifact} - ${taskName.value}`
})

useDocumentTitle(title)

function subscribe(): void {
  if (parsed.value.kind !== 'artifact') return
  store.subscribeArtifact(parsed.value.apiPath)
}

onMounted(subscribe)
watch(() => route.path, subscribe)
onUnmounted(() => store.unsubscribeArtifact())
</script>

<template>
  <main class="page artifact-page">
    <TopBar
      :items="[
        { label: 'Home', to: '/' },
        {
          label: store.currentArtifact?.repo.repo_path ?? parsed.repoPath,
          to: repoPathURL(parsed.repoPath),
        },
        {
          label: parsed.commit ? shortCommit(parsed.commit) : 'Commit',
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

    <PMessage v-if="store.error && !store.currentArtifact" severity="error" :closable="false">{{
      store.error
    }}</PMessage>
    <div v-if="store.loading && !store.currentArtifact" class="loading-state">
      <PProgressSpinner />
      <span>Loading artifact</span>
    </div>

    <div v-if="store.currentArtifact" class="artifact-content">
      <div class="artifact-toolbar">
        <span class="artifact-path mono" :title="store.currentArtifact.artifact.path">{{
          store.currentArtifact.artifact.path
        }}</span>
        <ArtifactActions
          :path="store.currentArtifact.artifact.path"
          :api-path="parsed.apiPath"
          :raw-url="store.currentArtifact.artifact.raw_url"
          :download-url="store.currentArtifact.artifact.download_url"
        />
      </div>
      <pre v-if="store.currentArtifact.artifact.is_text" class="artifact-log-view">{{
        store.currentArtifact.content
      }}</pre>
      <div v-else class="artifact-fallback">
        <i class="pi pi-file" aria-hidden="true"></i>
        <div>
          <p>This artifact is not a text file.</p>
          <p class="muted">Open it in the browser or download it.</p>
        </div>
      </div>
    </div>
  </main>
</template>

<style scoped>
.artifact-content {
  display: grid;
  grid-template-rows: auto auto minmax(0, 1fr);
  gap: var(--app-space-3);
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}

.artifact-toolbar {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  min-width: 0;
}

.artifact-path {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  overflow-wrap: normal;
  color: var(--p-text-muted-color);
}

.artifact-toolbar :global(.p-button) {
  flex: 0 0 auto;
}

.artifact-log-view {
  width: 100%;
  height: 100%;
  min-width: 0;
  min-height: 0;
  max-height: none;
  overflow: auto;
  margin: 0;
  padding: var(--app-space-5);
  border: 1px solid var(--p-content-border-color);
  border-bottom: 0;
  border-radius: var(--p-content-border-radius) var(--p-content-border-radius) 0 0;
  background: var(--p-surface-0);
  color: var(--p-text-color);
  font-size: var(--app-log-font-size);
  line-height: var(--app-log-line-height);
  white-space: pre-wrap;
}

.artifact-fallback {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  min-width: 0;
  padding: var(--app-space-5);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-content-border-radius);
  background: var(--p-surface-0);
}

.artifact-fallback p {
  margin: 0;
}
</style>
