<script setup lang="ts">
import { computed, onMounted, onUnmounted, watch } from 'vue'
import { useRoute } from 'vue-router'

import AppBreadcrumbs from '@/components/AppBreadcrumbs.vue'
import RunLink from '@/components/RunLink.vue'
import {
  annotationEntries,
  displayStatusSeverity,
  taskStatusIcon,
  taskStatusGroups,
} from '@/lib/api'
import type { CommitSummary, TaskStatusGroup, TaskSummary } from '@/lib/api'
import { parseRepoRoute, taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const commits = computed(() => store.currentRepo?.commits ?? [])
const title = computed(() => store.currentRepo?.repo.repo_path ?? parsed.value.repoPath)

useDocumentTitle(title)

function subscribe(): void {
  if (parsed.value.kind !== 'repo') return
  store.subscribeRepo(parsed.value.apiPath)
}

function activityTime(entry: CommitSummary): string {
  if (!entry.activity_at) return ''
  return new Date(entry.activity_at).toLocaleString()
}

function runGroups(entry: CommitSummary): Array<TaskStatusGroup<TaskSummary>> {
  return taskStatusGroups(entry.tasks)
}

function groupSeverity(
  group: TaskStatusGroup<TaskSummary>,
): ReturnType<typeof displayStatusSeverity> {
  return displayStatusSeverity(group.tasks[0]!)
}

onMounted(subscribe)
watch(() => route.path, subscribe)
onUnmounted(() => store.unsubscribePage())
</script>

<template>
  <main class="page">
    <AppBreadcrumbs
      :items="[
        { label: 'Home', to: '/' },
        { label: store.currentRepo?.repo.repo_path ?? parsed.repoPath },
      ]"
    />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.currentRepo" class="loading-state">
      <PProgressSpinner />
      <span>Loading repo</span>
    </div>

    <PDataTable
      v-if="commits.length > 0"
      :value="commits"
      data-key="commit"
      size="small"
      :show-headers="false"
      class="table-surface run-table"
    >
      <template #header>Runs</template>
      <PColumn>
        <template #body="{ data }">
          <div class="run-row">
            <div class="run-meta">
              <span>{{ store.currentRepo?.repo.repo_path ?? parsed.repoPath }}</span>
              <RunLink :repo-path="parsed.repoPath" :commit="data.commit" />
              <span class="attribute-list">
                <PTag
                  v-for="attribute in annotationEntries(data.annotations)"
                  :key="attribute.key"
                  severity="secondary"
                  :value="`${attribute.key}: ${attribute.value}`"
                />
              </span>
              <span class="muted">{{ activityTime(data) }}</span>
            </div>
            <div class="run-status-list">
              <div v-for="group in runGroups(data)" :key="group.label" class="run-status-row">
                <PTag :severity="groupSeverity(group)" :value="group.label" />
                <span class="run-task-list">
                  <RouterLink
                    v-for="task in group.tasks"
                    :key="task.name"
                    :to="taskURL(parsed.repoPath, data.commit, task.name)"
                  >
                    <i :class="taskStatusIcon(task)" aria-hidden="true"></i>
                    {{ task.short_name }}
                  </RouterLink>
                </span>
              </div>
            </div>
          </div>
        </template>
      </PColumn>
    </PDataTable>
    <div v-else-if="store.repoLoaded && !store.error" class="empty-state">
      No commits recorded for this repo.
    </div>
  </main>
</template>

<style scoped>
.run-row,
.run-status-list {
  display: grid;
  gap: var(--app-space-3);
  min-width: 0;
}

.run-meta,
.run-status-row,
.run-task-list {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--app-space-3);
  min-width: 0;
}

.run-meta {
  justify-content: space-between;
}

.run-task-list a {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-1);
}
</style>
