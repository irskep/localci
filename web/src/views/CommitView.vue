<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import TopBar from '@/components/TopBar.vue'
import {
  commitSubject,
  displayAnnotationEntries,
  displayStatusSeverity,
  displayTaskFailure,
  displayTaskStatus,
  formatDuration,
  shortCommit,
} from '@/lib/api'
import { parseRepoRoute, repoPathURL, taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const commit = computed(() => store.currentCommit?.commit)
const tasks = computed(() => commit.value?.tasks ?? [])
const subscribedPage = ref('')
const title = computed(() => {
  const commitLabel = parsed.value.commit ? shortCommit(parsed.value.commit) : 'Commit'
  const repoLabel = store.currentCommit?.repo.repo_label ?? parsed.value.repoPath
  return repoLabel ? `${repoLabel} ${commitLabel}` : commitLabel
})

useDocumentTitle(title)

function subscribe(): void {
  if (parsed.value.kind !== 'commit') return
  subscribedPage.value = parsed.value.apiPath
  store.subscribeCommit(parsed.value.apiPath)
}

onMounted(subscribe)
watch(() => route.path, subscribe)
onUnmounted(() => store.unsubscribePage(subscribedPage.value))
</script>

<template>
  <main class="page">
    <TopBar
      :items="[
        { label: 'Home', to: '/' },
        {
          label: store.currentCommit?.repo.repo_label ?? parsed.repoPath,
          to: repoPathURL(parsed.repoPath),
        },
        { label: parsed.commit ? shortCommit(parsed.commit) : 'Commit' },
      ]"
    />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !commit" class="loading-state">
      <PProgressSpinner />
      <span>Loading commit</span>
    </div>

    <template v-if="commit">
      <section
        v-if="
          commitSubject(commit.annotations) ||
          displayAnnotationEntries(commit.annotations).length > 0
        "
        class="commit-meta"
      >
        <div v-if="commitSubject(commit.annotations)" class="commit-subject">
          {{ commitSubject(commit.annotations) }}
        </div>
        <div v-if="displayAnnotationEntries(commit.annotations).length > 0" class="attribute-list">
          <PTag
            v-for="attribute in displayAnnotationEntries(commit.annotations)"
            :key="attribute.key"
            severity="secondary"
            :value="`${attribute.key}: ${attribute.value}`"
          />
        </div>
      </section>

      <PDataTable :value="tasks" data-key="name" size="small" class="table-surface">
        <template #header>Tasks</template>
        <PColumn header="Task">
          <template #body="{ data }">
            <RouterLink :to="taskURL(parsed.repoPath, commit.commit, data.name)">
              {{ data.short_name }}
            </RouterLink>
          </template>
        </PColumn>
        <PColumn header="Status">
          <template #body="{ data }">
            <PTag :severity="displayStatusSeverity(data)" :value="displayTaskStatus(data)" />
          </template>
        </PColumn>
        <PColumn header="Attempt">
          <template #body="{ data }">
            <span v-if="data.attempt > 0">
              {{ data.attempt }}
              <template v-if="data.attempt_count > data.attempt">
                of {{ data.attempt_count }}</template
              >
            </span>
            <span v-else class="muted">not run</span>
          </template>
        </PColumn>
        <PColumn header="Duration">
          <template #body="{ data }">{{ formatDuration(data.duration_ms) }}</template>
        </PColumn>
        <PColumn header="Failure">
          <template #body="{ data }">
            <span class="muted">{{ displayTaskFailure(data) }}</span>
          </template>
        </PColumn>
      </PDataTable>
    </template>
  </main>
</template>

<style scoped>
.commit-meta {
  display: grid;
  gap: var(--app-space-2);
  min-width: 0;
}

.commit-subject {
  min-width: 0;
  width: fit-content;
  max-width: 100%;
  padding: var(--app-space-2) var(--app-space-3);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-border-radius-sm);
  background: var(--p-content-hover-background);
  color: var(--p-text-muted-color);
  font-family: var(--app-mono-font-family);
  font-size: var(--p-form-field-lg-font-size);
  overflow-wrap: anywhere;
}

.attribute-list {
  display: flex;
  flex-wrap: wrap;
  gap: var(--app-space-2);
  min-width: 0;
}
</style>
