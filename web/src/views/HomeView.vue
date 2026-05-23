<script setup lang="ts">
import { computed, onMounted, onUnmounted } from 'vue'

import {
  annotationEntries,
  displayStatusSeverity,
  shortCommit,
  taskStatusIcon,
  taskStatusGroups,
} from '@/lib/api'
import type { CommitSummary, QueueEntry, TaskStatusGroup, TaskSummary } from '@/lib/api'
import { commitURL, taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const store = useLocalciStore()

const recentRows = computed(() => store.home?.recent_commits ?? [])
const repoRows = computed(() => store.home?.repos ?? [])
const queueRows = computed(() => store.home?.queue.pending ?? [])
const active = computed(() => store.home?.queue.active)

useDocumentTitle('Overview')

function queueLabel(entry: QueueEntry): string {
  return `${entry.repo.repo_path} / ${shortCommit(entry.commit)} / ${entry.task}`
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

onMounted(() => {
  store.subscribeHome()
})
onUnmounted(() => store.unsubscribePage())
</script>

<template>
  <main class="page">
    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !store.home" class="loading-state">
      <PProgressSpinner />
      <span>Loading localci state</span>
    </div>

    <template v-if="store.home">
      <section class="section-grid">
        <div class="stack">
          <PDataTable :value="recentRows" data-key="commit" size="small" :show-headers="false">
            <PColumn>
              <template #body="{ data }">
                <div class="run-row">
                  <div class="run-meta">
                    <RouterLink :to="`/repo/${data.repo.repo_path}`">
                      {{ data.repo.repo_path }}
                    </RouterLink>
                    <RouterLink :to="commitURL(data.repo.repo_path, data.commit)" class="mono">
                      {{ shortCommit(data.commit) }}
                    </RouterLink>
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
                          :to="taskURL(data.repo.repo_path, data.commit, task.name)"
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
        </div>

        <aside class="stack">
          <PPanel header="Active Now">
            <div v-if="active">
              <RouterLink :to="taskURL(active.repo.repo_path, active.commit, active.task)">
                {{ queueLabel(active) }}
              </RouterLink>
            </div>
            <div v-else class="empty-state">No task is running.</div>
          </PPanel>

          <PPanel header="Queue">
            <template #icons>
              <RouterLink to="/queue">
                <PButton label="Open" text size="small" icon="pi pi-arrow-right" icon-pos="right" />
              </RouterLink>
            </template>
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
          </PPanel>

          <PPanel header="Repo">
            <ul class="artifact-list">
              <li v-for="repo in repoRows" :key="repo.repo_path">
                <i class="pi pi-folder" aria-hidden="true"></i>
                <RouterLink :to="`/repo/${repo.repo_path}`">{{ repo.repo_path }}</RouterLink>
              </li>
            </ul>
          </PPanel>
        </aside>
      </section>
    </template>
  </main>
</template>

<style scoped>
.section-grid {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 360px;
  gap: var(--app-space-5);
  align-items: start;
  min-width: 0;
}

.artifact-list {
  display: grid;
  gap: var(--app-space-2);
  margin: 0;
  padding: 0;
  list-style: none;
}

.artifact-list li {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  min-width: 0;
}

.artifact-list a {
  min-width: 0;
  padding: var(--app-space-3) 0;
  overflow-wrap: anywhere;
}

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

@media (max-width: 860px) {
  .section-grid {
    grid-template-columns: 1fr;
  }
}
</style>
