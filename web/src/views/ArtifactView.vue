<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import { attemptURL, commitURL, parseRepoRoute, repoPathURL, taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const revealing = ref(false)
const revealNotice = ref('')
const canShowInFinder = computed(() => navigator.platform.toLowerCase().includes('mac'))
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

async function showInFinder(): Promise<void> {
  if (parsed.value.kind !== 'artifact' || revealing.value) return
  revealing.value = true
  revealNotice.value = ''
  const result = await store.revealArtifact(parsed.value.apiPath)
  if (result?.ok) revealNotice.value = 'Shown in Finder'
  revealing.value = false
}

onMounted(subscribe)
watch(() => route.path, subscribe)
onUnmounted(() => store.unsubscribeArtifact())
</script>

<template>
  <main class="page artifact-page">
    <AppBreadcrumbs
      :items="[
        { label: 'Home', to: '/' },
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

    <PMessage v-if="store.error && !store.currentArtifact" severity="error" :closable="false">{{
      store.error
    }}</PMessage>
    <div v-if="store.loading && !store.currentArtifact" class="loading-state">
      <PProgressSpinner />
      <span>Loading artifact</span>
    </div>

    <template v-if="store.currentArtifact">
      <div class="artifact-toolbar">
        <span class="artifact-path mono">{{ store.currentArtifact.artifact.path }}</span>
        <PButton
          v-if="canShowInFinder"
          label="Show in Finder"
          icon="pi pi-folder-open"
          size="small"
          :loading="revealing"
          @click="showInFinder"
        />
      </div>
      <PMessage v-if="revealNotice" severity="success" :closable="false" class="artifact-notice">{{
        revealNotice
      }}</PMessage>
      <pre class="artifact-log-view">{{ store.currentArtifact.content }}</pre>
    </template>
  </main>
</template>

<style scoped>
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
  color: var(--p-text-muted-color);
}

.artifact-notice {
  margin: 0;
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
</style>
