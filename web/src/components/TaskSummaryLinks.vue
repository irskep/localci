<script setup lang="ts">
import { computed } from 'vue'

import type { TaskSummary, TaskStatusView } from '@/lib/api'
import { displayTaskStatus } from '@/lib/api'
import { taskURL } from '@/lib/routes'

const props = defineProps<{
  repoPath: string
  commit: string
  tasks: Array<TaskSummary | TaskStatusView>
}>()

const groups = computed(() => [
  {
    label: 'failed',
    tasks: props.tasks.filter(
      (task) => displayTaskStatus(task) === 'failed' || displayTaskStatus(task) === 'timed-out',
    ),
  },
  {
    label: 'canceled',
    tasks: props.tasks.filter((task) => displayTaskStatus(task) === 'canceled'),
  },
  {
    label: 'running',
    tasks: props.tasks.filter((task) => task.status === 'running'),
  },
  {
    label: 'queued',
    tasks: props.tasks.filter((task) => task.status === 'queued'),
  },
  {
    label: 'not run',
    tasks: props.tasks.filter((task) => task.status === 'not-run'),
  },
  {
    label: 'passed',
    tasks: props.tasks.filter((task) => task.status === 'succeeded'),
  },
])
</script>

<template>
  <span class="task-summary-links">
    <template v-for="group in groups" :key="group.label">
      <span v-if="group.tasks.length > 0" class="task-summary-group">
        <span class="muted">{{ group.label }}:</span>
        <span class="task-summary-items">
          <RouterLink
            v-for="task in group.tasks"
            :key="`${group.label}:${task.name}`"
            :to="taskURL(repoPath, commit, task.name)"
          >
            {{ task.short_name }}
          </RouterLink>
        </span>
      </span>
    </template>
  </span>
</template>

<style scoped>
.task-summary-links {
  display: grid;
  gap: var(--app-space-1);
  margin-top: var(--app-space-1);
}

.task-summary-group {
  display: grid;
  grid-template-columns: max-content minmax(0, 1fr);
  gap: var(--app-space-2);
  align-items: baseline;
  min-width: 0;
}

.task-summary-items {
  display: inline-flex;
  flex-wrap: wrap;
  gap: var(--app-space-2);
}

.task-summary-items a:not(:last-child)::after {
  color: var(--p-text-muted-color);
  content: ',';
}
</style>
