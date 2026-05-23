<script setup lang="ts">
import RepoLink from '@/components/RepoLink.vue'
import RunLink from '@/components/RunLink.vue'
import {
  annotationEntries,
  displayStatusSeverity,
  taskStatusIcon,
  taskStatusGroups,
} from '@/lib/api'
import type { CommitSummary, TaskStatusGroup, TaskSummary } from '@/lib/api'
import { taskURL } from '@/lib/routes'

const props = withDefaults(
  defineProps<{
    run: CommitSummary
    repoPath: string
    showRepo?: boolean
  }>(),
  {
    showRepo: true,
  },
)

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
</script>

<template>
  <article class="run-surface">
    <div class="run-meta">
      <RepoLink v-if="showRepo" :repo-path="repoPath" />
      <span v-else>{{ repoPath }}</span>
      <RunLink :repo-path="repoPath" :commit="run.commit" />
      <span class="attribute-list">
        <PTag
          v-for="attribute in annotationEntries(run.annotations)"
          :key="attribute.key"
          severity="secondary"
          :value="`${attribute.key}: ${attribute.value}`"
        />
      </span>
      <span class="muted">{{ activityTime(run) }}</span>
    </div>
    <div class="run-status-list">
      <div v-for="group in runGroups(run)" :key="group.label" class="run-status-row">
        <PTag :severity="groupSeverity(group)" :value="group.label" />
        <span class="run-task-list">
          <RouterLink
            v-for="task in group.tasks"
            :key="task.name"
            :to="taskURL(repoPath, run.commit, task.name)"
          >
            <i :class="taskStatusIcon(task)" aria-hidden="true"></i>
            {{ task.short_name }}
          </RouterLink>
        </span>
      </div>
    </div>
  </article>
</template>

<style scoped>
.run-surface {
  display: grid;
  gap: var(--app-space-4);
  min-width: 0;
  padding: var(--app-space-5);
  border: 1px solid var(--p-content-border-color);
  border-radius: var(--p-content-border-radius);
  background: var(--p-content-background);
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

.run-status-list {
  display: grid;
  gap: var(--app-space-3);
  min-width: 0;
}

.run-task-list a {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-1);
}
</style>
