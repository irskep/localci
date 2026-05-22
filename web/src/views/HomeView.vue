<script setup lang="ts">
import { computed, onMounted } from 'vue'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import { annotationEntries, shortCommit } from '@/lib/api'
import type { CommitSummary, QueueEntry } from '@/lib/api'
import { commitURL, taskURL } from '@/lib/routes'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()

const recentRows = computed(() => store.home?.recent_commits ?? [])
const repoRows = computed(() => store.home?.repos ?? [])
const queueRows = computed(() => store.home?.queue.pending ?? [])
const active = computed(() => store.home?.queue.active)

function queueLabel(entry: QueueEntry): string {
  return `${entry.repo.repo_path} / ${shortCommit(entry.commit)} / ${entry.task}`
}

function activityTime(entry: CommitSummary): string {
  if (!entry.activity_at) return ''
  return new Date(entry.activity_at).toLocaleString()
}

onMounted(() => {
  void store.loadHome()
})
</script>

<template>
  <main class="page">
    <AppBreadcrumbs :items="[{ label: 'Home' }]" />

    <section class="page-header">
      <span class="eyebrow">Overview</span>
      <h1 class="page-title">Runs across every repo</h1>
      <p class="page-subtitle">
        Current daemon activity, recent commits, and the literal task queue.
      </p>
    </section>

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.home" class="loading-state">
      <PProgressSpinner style="width: 1.5rem; height: 1.5rem" />
      <span>Loading localci state</span>
    </div>

    <template v-if="store.home">
      <section class="section-grid">
        <div class="stack">
          <div class="panel">
            <div class="panel-header">
              <h2 class="panel-title">Recent Commit Activity</h2>
            </div>
            <PDataTable :value="recentRows" data-key="commit" size="small">
              <PColumn header="Repo">
                <template #body="{ data }">
                  <RouterLink :to="`/repo/${data.repo.repo_path}`">{{
                    data.repo.repo_path
                  }}</RouterLink>
                </template>
              </PColumn>
              <PColumn header="Commit">
                <template #body="{ data }">
                  <RouterLink :to="commitURL(data.repo.repo_path, data.commit)" class="mono">
                    {{ shortCommit(data.commit) }}
                  </RouterLink>
                </template>
              </PColumn>
              <PColumn field="summary" header="Result" />
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
              <PColumn header="Updated">
                <template #body="{ data }">{{ activityTime(data) }}</template>
              </PColumn>
            </PDataTable>
          </div>
        </div>

        <aside class="stack">
          <div class="panel">
            <div class="panel-header">
              <h2 class="panel-title">Active Now</h2>
            </div>
            <div v-if="active" class="panel-body">
              <RouterLink :to="taskURL(active.repo.repo_path, active.commit, active.task)">
                {{ queueLabel(active) }}
              </RouterLink>
            </div>
            <div v-else class="empty-state">No task is running.</div>
          </div>

          <div class="panel">
            <div class="panel-header">
              <h2 class="panel-title">Queue</h2>
              <RouterLink to="/queue">
                <PButton label="Open" text size="small" icon="pi pi-arrow-right" icon-pos="right" />
              </RouterLink>
            </div>
            <ul v-if="queueRows.length > 0" class="artifact-list">
              <li
                v-for="entry in queueRows.slice(0, 6)"
                :key="`${entry.repo.repo_path}:${entry.commit}:${entry.task}`"
              >
                <PTag severity="warn" value="pending" />
                <RouterLink :to="taskURL(entry.repo.repo_path, entry.commit, entry.task)">
                  {{ queueLabel(entry) }}
                </RouterLink>
              </li>
            </ul>
            <div v-else class="empty-state">Queue is idle.</div>
          </div>

          <div class="panel">
            <div class="panel-header">
              <h2 class="panel-title">Repo</h2>
            </div>
            <ul class="artifact-list">
              <li v-for="repo in repoRows" :key="repo.repo_path">
                <i class="pi pi-folder" aria-hidden="true"></i>
                <RouterLink :to="`/repo/${repo.repo_path}`">{{ repo.repo_path }}</RouterLink>
              </li>
            </ul>
          </div>
        </aside>
      </section>
    </template>
  </main>
</template>
