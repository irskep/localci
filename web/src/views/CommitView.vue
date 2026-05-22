<script setup lang="ts">
import { computed, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import {
  annotationEntries,
  formatAnnotations,
  formatDuration,
  shortCommit,
  statusSeverity,
  summarizeCommit,
} from '@/lib/api'
import { parseRepoRoute, repoPathURL, taskURL } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const commit = computed(() => store.currentCommit?.commit)
const tasks = computed(() => commit.value?.tasks ?? [])

async function load(): Promise<void> {
  if (parsed.value.kind !== 'commit') return
  await store.loadCommit(parsed.value.apiPath)
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
          label: store.currentCommit?.repo.repo_name ?? parsed.repoPath,
          to: repoPathURL(parsed.repoPath),
        },
        { label: parsed.commit ? shortCommit(parsed.commit) : 'Commit' },
      ]"
    />

    <section class="page-header">
      <span class="eyebrow">Commit</span>
      <h1 class="page-title mono">{{ parsed.commit ? shortCommit(parsed.commit) : '' }}</h1>
      <p class="page-subtitle">
        {{ store.currentCommit?.repo.repo_name }}
        <template v-if="commit"> / {{ summarizeCommit(commit) }}</template>
        <template v-if="formatAnnotations(commit?.annotations)">
          /
          <span class="attribute-list">
            <span
              v-for="attribute in annotationEntries(commit?.annotations)"
              :key="attribute.key"
              class="attribute-pill"
            >
              {{ attribute.key }}: {{ attribute.value }}
            </span>
          </span>
        </template>
      </p>
    </section>

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !commit" class="loading-state">
      <PProgressSpinner style="width: 1.5rem; height: 1.5rem" />
      <span>Loading commit</span>
    </div>

    <template v-if="commit">
      <section class="panel">
        <div class="panel-header">
          <h2 class="panel-title">Tasks</h2>
        </div>
        <PDataTable :value="tasks" data-key="name" size="small">
          <PColumn header="Task">
            <template #body="{ data }">
              <RouterLink :to="taskURL(parsed.repoPath, commit.commit, data.name)">
                {{ data.short_name }}
              </RouterLink>
            </template>
          </PColumn>
          <PColumn header="Status">
            <template #body="{ data }">
              <PTag :severity="statusSeverity(data.status)" :value="data.status" />
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
              <span class="muted">{{ data.failure }}</span>
            </template>
          </PColumn>
        </PDataTable>
      </section>
    </template>
  </main>
</template>
