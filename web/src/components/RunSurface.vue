<script setup lang="ts">
import CommitSubject from '@/components/CommitSubject.vue'
import RepoLink from '@/components/RepoLink.vue'
import RunLink from '@/components/RunLink.vue'
import {
  commitSubject,
  displayAnnotationEntries,
  displayTaskStatus,
  displayStatusSeverity,
  taskStatusIcon,
  taskStatusGroups,
} from '@/lib/api'
import type { CommitSummary, TaskStatusGroup, TaskSummary } from '@/lib/api'
import { commitURL, taskURL } from '@/lib/routes'

type PackageTask = {
  task: TaskSummary
  label: string
}

type PackageGroup = {
  name: string
  tasks: PackageTask[]
}

type PackageTaskGroup = {
  label: string
  tasks: PackageTask[]
}

type TemporalInstant = {
  epochMilliseconds: number
  toLocaleString: (locales?: Intl.LocalesArgument, options?: Intl.DateTimeFormatOptions) => string
}

type TemporalGlobal = {
  Instant: {
    from: (value: string) => TemporalInstant
  }
  Now: {
    instant: () => TemporalInstant
  }
}

const TASK_STATUS_GROUPS = [
  'failed',
  'canceled',
  'running',
  'queued',
  'not-run',
  'succeeded',
] as const

withDefaults(
  defineProps<{
    run: CommitSummary
    repoPath: string
    repoLabel?: string
    showRepo?: boolean
    summaryMode?: boolean
  }>(),
  {
    showRepo: true,
    summaryMode: false,
  },
)

const RELATIVE_TIME = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })

function temporal(): TemporalGlobal | undefined {
  return (globalThis as typeof globalThis & { Temporal?: TemporalGlobal }).Temporal
}

function activityInstant(entry: CommitSummary): TemporalInstant | undefined {
  if (!entry.activity_at) return undefined
  const temporalAPI = temporal()
  if (temporalAPI) {
    try {
      return temporalAPI.Instant.from(entry.activity_at)
    } catch {
      return undefined
    }
  }
  const date = new Date(entry.activity_at)
  if (Number.isNaN(date.getTime())) return undefined
  return {
    epochMilliseconds: date.getTime(),
    toLocaleString: (locales, options) => date.toLocaleString(locales, options),
  }
}

function activityRelativeTime(entry: CommitSummary): string {
  const instant = activityInstant(entry)
  if (!instant) return ''

  const now = temporal()?.Now.instant().epochMilliseconds ?? Date.now()
  const deltaMilliseconds = instant.epochMilliseconds - now
  const absoluteDelta = Math.abs(deltaMilliseconds)
  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  const month = 30 * day
  const year = 365 * day

  if (absoluteDelta < 45_000) return 'just now'
  if (absoluteDelta < 45 * minute) {
    return RELATIVE_TIME.format(Math.round(deltaMilliseconds / minute), 'minute')
  }
  if (absoluteDelta < 22 * hour) {
    return RELATIVE_TIME.format(Math.round(deltaMilliseconds / hour), 'hour')
  }
  if (absoluteDelta < 26 * day) {
    return RELATIVE_TIME.format(Math.round(deltaMilliseconds / day), 'day')
  }
  if (absoluteDelta < 11 * month) {
    return RELATIVE_TIME.format(Math.round(deltaMilliseconds / month), 'month')
  }
  return RELATIVE_TIME.format(Math.round(deltaMilliseconds / year), 'year')
}

function activityExactTime(entry: CommitSummary): string {
  return (
    activityInstant(entry)?.toLocaleString(undefined, {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: 'numeric',
      minute: '2-digit',
      second: '2-digit',
      timeZoneName: 'short',
    }) ?? ''
  )
}

function runGroups(entry: CommitSummary): Array<TaskStatusGroup<TaskSummary>> {
  return taskStatusGroups(entry.tasks)
}

function usePackageGroups(entry: CommitSummary): boolean {
  return entry.tasks.length > 10
}

function packageGroups(entry: CommitSummary): PackageGroup[] {
  const groups = new Map<string, PackageTask[]>()
  for (const task of entry.tasks) {
    const parsed = parsePackageTask(task)
    const tasks = groups.get(parsed.packageName) ?? []
    tasks.push({ task, label: parsed.taskLabel })
    groups.set(parsed.packageName, tasks)
  }
  return Array.from(groups, ([name, tasks]) => ({ name, tasks }))
}

function parsePackageTask(task: TaskSummary): { packageName: string; taskLabel: string } {
  const separator = task.short_name.indexOf(':')
  if (task.short_name.startsWith('//') && separator > 0) {
    const packageName = task.short_name.slice(0, separator)
    return {
      packageName: packageName === '//' ? 'root' : packageName,
      taskLabel: task.short_name.slice(separator + 1),
    }
  }
  return {
    packageName: 'root',
    taskLabel: task.short_name,
  }
}

function groupSeverity(
  group: TaskStatusGroup<TaskSummary>,
): ReturnType<typeof displayStatusSeverity> {
  return displayStatusSeverity(group.tasks[0]!)
}

const PROGRESS_STATUS = [
  {
    status: 'succeeded',
    label: 'passed',
    color: 'var(--p-green-500)',
    complete: true,
  },
  {
    status: 'failed',
    label: 'failed',
    color: 'var(--p-red-500)',
    complete: true,
  },
  {
    status: 'timed-out',
    label: 'timed out',
    color: 'var(--p-red-400)',
    complete: true,
  },
  {
    status: 'canceled',
    label: 'canceled',
    color: 'var(--p-surface-500)',
    complete: true,
  },
  {
    status: 'running',
    label: 'running',
    color: 'var(--p-primary-500)',
    complete: false,
  },
  {
    status: 'queued',
    label: 'queued',
    color: 'var(--p-yellow-500)',
    complete: false,
  },
  {
    status: 'not-run',
    label: 'not run',
    color: 'var(--p-surface-300)',
    complete: false,
  },
] as const

function progressItems(
  entry: CommitSummary,
): Array<{ label: string; value: number; color: string }> {
  return progressItemsForTasks(entry.tasks)
}

function progressItemsForTasks(
  tasks: TaskSummary[],
): Array<{ label: string; value: number; color: string }> {
  return PROGRESS_STATUS.map((progressStatus) => ({
    label: progressStatus.label,
    value: tasks.filter((task) => displayTaskStatus(task) === progressStatus.status).length,
    color: progressStatus.color,
  })).filter((item) => item.value > 0)
}

function hasIncompleteTasks(entry: CommitSummary): boolean {
  return entry.tasks.some((task) => {
    const status = displayTaskStatus(task)
    return status === 'running' || status === 'queued'
  })
}

function runHasLiveTasks(entry: CommitSummary): boolean {
  return hasIncompleteTasks(entry)
}

function completedRunIssueGroups(entry: CommitSummary): PackageGroup[] {
  return packageGroups(entry).filter((group) =>
    group.tasks.some(({ task }) => displayTaskStatus(task) !== 'succeeded'),
  )
}

function visibleIssueGroups(entry: CommitSummary): PackageGroup[] {
  return completedRunIssueGroups(entry).slice(0, 3)
}

function visibleIssueTasks(group: PackageGroup): PackageTask[] {
  return group.tasks.filter(({ task }) => displayTaskStatus(task) !== 'succeeded').slice(0, 3)
}

function issueTaskMoreCount(group: PackageGroup): number {
  return Math.max(
    0,
    group.tasks.filter(({ task }) => displayTaskStatus(task) !== 'succeeded').length - 3,
  )
}

function packageIssueMoreCount(entry: CommitSummary): number {
  return Math.max(0, completedRunIssueGroups(entry).length - 3)
}

function pluralize(count: number, noun: string): string {
  return count === 1 ? noun : `${noun}s`
}

function completedTaskCount(entry: CommitSummary): number {
  return completedTaskCountForTasks(entry.tasks)
}

function completedTaskCountForTasks(tasks: TaskSummary[]): number {
  return tasks.filter((task) => {
    const status = displayTaskStatus(task)
    return status !== 'running' && status !== 'queued' && status !== 'not-run'
  }).length
}

function packageSeverity(group: PackageGroup): ReturnType<typeof displayStatusSeverity> {
  const task = group.tasks.find(
    (item) => displayTaskStatus(item.task) === packageStatus(group),
  )?.task
  return task ? displayStatusSeverity(task) : 'secondary'
}

function taskGroupsForPackage(group: PackageGroup): PackageTaskGroup[] {
  return TASK_STATUS_GROUPS.map((status) => ({
    label: status === 'not-run' ? 'not run' : status === 'succeeded' ? 'passed' : status,
    tasks: group.tasks.filter(({ task }) => {
      const displayStatus = displayTaskStatus(task)
      if (status === 'failed') return displayStatus === 'failed' || displayStatus === 'timed-out'
      return displayStatus === status
    }),
  })).filter((taskGroup) => taskGroup.tasks.length > 0)
}

function packageProgressItems(
  group: PackageGroup,
): Array<{ label: string; value: number; color: string }> {
  return progressItemsForTasks(group.tasks.map(({ task }) => task))
}

function taskGroupSeverity(group: PackageTaskGroup): ReturnType<typeof displayStatusSeverity> {
  return displayStatusSeverity(group.tasks[0]!.task)
}

function taskGroupIcon(group: PackageTaskGroup): string {
  return taskStatusIcon(group.tasks[0]!.task)
}

function packageSummary(group: PackageGroup): string {
  const status = packageStatus(group)
  if (status === 'timed-out') return 'failed'
  if (status === 'not-run') return 'not run'
  return status
}

function packageStatus(group: PackageGroup): string {
  const priority = ['failed', 'timed-out', 'running', 'queued', 'not-run', 'canceled', 'succeeded']
  return (
    priority.find((candidate) =>
      group.tasks.some(({ task }) => displayTaskStatus(task) === candidate),
    ) ?? 'succeeded'
  )
}

function packageNeedsLongForm(group: PackageGroup): boolean {
  return packageStatus(group) !== 'succeeded'
}

function packageHasIncompleteTasks(group: PackageGroup): boolean {
  return group.tasks.some(({ task }) => {
    const status = displayTaskStatus(task)
    return status === 'running' || status === 'queued'
  })
}

function packageProgressSummary(group: PackageGroup): string {
  return TASK_STATUS_GROUPS.flatMap((status) => packageStatusSummary(group, status)).join(', ')
}

function packageStatusSummary(
  group: PackageGroup,
  status: (typeof TASK_STATUS_GROUPS)[number],
): string[] {
  const tasks = group.tasks.filter(({ task }) => {
    const displayStatus = displayTaskStatus(task)
    if (status === 'failed') return displayStatus === 'failed' || displayStatus === 'timed-out'
    return displayStatus === status
  })
  if (tasks.length === 0) return []

  const label = taskSummaryStatusLabel(status)
  if (status === 'succeeded') return [`${tasks.length} ${label}`]

  const taskNames = tasks
    .slice(0, 3)
    .map(({ label: taskLabel }) => taskLabel)
    .join(', ')
  const more = tasks.length > 3 ? `, +${tasks.length - 3} more` : ''
  return [`${tasks.length} ${label} (${taskNames}${more})`]
}

function taskSummaryStatusLabel(status: string): string {
  if (status === 'succeeded') return 'passed'
  if (status === 'timed-out') return 'failed'
  if (status === 'not-run') return 'not run'
  return status
}
</script>

<template>
  <article class="run-surface">
    <div class="run-meta">
      <div class="run-meta-primary">
        <RunLink :repo-path="repoPath" :commit="run.commit" />
        <time
          v-if="run.activity_at"
          class="muted run-meta-time"
          :datetime="run.activity_at"
          :title="activityExactTime(run)"
          :aria-label="activityExactTime(run)"
        >
          {{ activityRelativeTime(run) }}
        </time>
        <CommitSubject
          v-if="commitSubject(run.annotations)"
          :subject="commitSubject(run.annotations)"
        />
      </div>
      <div class="run-meta-secondary">
        <RepoLink v-if="showRepo" :repo-path="repoPath" :repo-label="repoLabel" />
        <span v-else>{{ repoLabel ?? repoPath }}</span>
        <span class="attribute-list">
          <PTag
            v-for="attribute in displayAnnotationEntries(run.annotations)"
            :key="attribute.key"
            severity="secondary"
            :value="`${attribute.key}: ${attribute.value}`"
          />
        </span>
      </div>
    </div>
    <details
      v-if="summaryMode && !runHasLiveTasks(run) && usePackageGroups(run)"
      class="run-summary-disclosure"
    >
      <summary class="run-summary run-summary-expandable">
        <i class="pi pi-chevron-right run-package-toggle" aria-hidden="true"></i>
        <span>
          <template v-if="completedRunIssueGroups(run).length === 0">
            <i class="pi pi-check run-task-icon run-task-icon-success" aria-hidden="true"></i>
            <RouterLink :to="commitURL(repoPath, run.commit)">all passed</RouterLink>
          </template>
          <template v-else>
            <span>
              {{ completedRunIssueGroups(run).length }}
              {{ pluralize(completedRunIssueGroups(run).length, 'package') }} with issues:
            </span>
            <template
              v-for="(packageGroup, packageIndex) in visibleIssueGroups(run)"
              :key="packageGroup.name"
            >
              <span>{{ packageIndex === 0 ? ' ' : '' }}{{ packageGroup.name }} (</span>
              <template
                v-for="(item, taskIndex) in visibleIssueTasks(packageGroup)"
                :key="item.task.name"
              >
                <span v-if="taskIndex > 0">, </span>
                <RouterLink :to="taskURL(repoPath, run.commit, item.task.name)">
                  {{ item.label }}
                </RouterLink>
              </template>
              <span v-if="issueTaskMoreCount(packageGroup) > 0">
                , +{{ issueTaskMoreCount(packageGroup) }} more</span
              >
              <span>)</span>
              <span v-if="packageIndex < visibleIssueGroups(run).length - 1">, </span>
            </template>
            <span v-if="packageIssueMoreCount(run) > 0"
              >, +{{ packageIssueMoreCount(run) }} more</span
            >
          </template>
        </span>
      </summary>
      <div class="run-package-list run-summary-package-list">
        <template v-for="packageGroup in packageGroups(run)" :key="packageGroup.name">
          <details v-if="packageNeedsLongForm(packageGroup)" class="run-package">
            <summary class="run-package-summary run-package-summary-expandable">
              <i class="pi pi-chevron-right run-package-toggle" aria-hidden="true"></i>
              <span class="run-package-title">
                <span>{{ packageGroup.name }}</span>
                <PTag
                  :severity="packageSeverity(packageGroup)"
                  :value="packageSummary(packageGroup)"
                />
              </span>
              <span class="run-package-detail">
                {{ packageProgressSummary(packageGroup) }}
              </span>
              <span v-if="packageHasIncompleteTasks(packageGroup)" class="run-package-progress">
                <PMeterGroup
                  :value="packageProgressItems(packageGroup)"
                  :max="packageGroup.tasks.length"
                  :aria-label="`${packageGroup.name} progress`"
                >
                  <template #label></template>
                </PMeterGroup>
              </span>
            </summary>
            <div class="run-package-tasks">
              <div
                v-for="group in taskGroupsForPackage(packageGroup)"
                :key="group.label"
                class="run-status-row"
              >
                <PTag :severity="taskGroupSeverity(group)" :value="group.label" />
                <span class="run-task-list">
                  <RouterLink
                    v-for="item in group.tasks"
                    :key="item.task.name"
                    :to="taskURL(repoPath, run.commit, item.task.name)"
                  >
                    <i :class="taskGroupIcon(group)" aria-hidden="true"></i>
                    {{ item.label }}
                  </RouterLink>
                </span>
              </div>
            </div>
          </details>
          <div v-else class="run-package run-package-summary run-package-summary-static">
            <span class="run-package-title">
              <span>{{ packageGroup.name }}</span>
              <PTag
                :severity="packageSeverity(packageGroup)"
                :value="packageSummary(packageGroup)"
              />
            </span>
            <span class="run-package-task-summary">
              <template v-for="(item, taskIndex) in packageGroup.tasks" :key="item.task.name">
                <span v-if="taskIndex > 0">, </span>
                <RouterLink :to="taskURL(repoPath, run.commit, item.task.name)">
                  {{ item.label }}
                </RouterLink>
              </template>
            </span>
          </div>
        </template>
      </div>
    </details>
    <div v-else-if="hasIncompleteTasks(run)" class="run-progress">
      <PMeterGroup :value="progressItems(run)" :max="run.tasks.length" aria-label="Run progress">
        <template #label></template>
      </PMeterGroup>
      <span class="muted">{{ completedTaskCount(run) }}/{{ run.tasks.length }} complete</span>
    </div>
    <div
      v-if="
        !(summaryMode && !runHasLiveTasks(run) && usePackageGroups(run)) && usePackageGroups(run)
      "
      class="run-package-list"
    >
      <template v-for="packageGroup in packageGroups(run)" :key="packageGroup.name">
        <details v-if="packageNeedsLongForm(packageGroup)" class="run-package">
          <summary class="run-package-summary run-package-summary-expandable">
            <i class="pi pi-chevron-right run-package-toggle" aria-hidden="true"></i>
            <span class="run-package-title">
              <span>{{ packageGroup.name }}</span>
              <PTag
                :severity="packageSeverity(packageGroup)"
                :value="packageSummary(packageGroup)"
              />
            </span>
            <span class="run-package-detail">
              {{ packageProgressSummary(packageGroup) }}
            </span>
            <span v-if="packageHasIncompleteTasks(packageGroup)" class="run-package-progress">
              <PMeterGroup
                :value="packageProgressItems(packageGroup)"
                :max="packageGroup.tasks.length"
                :aria-label="`${packageGroup.name} progress`"
              >
                <template #label></template>
              </PMeterGroup>
            </span>
          </summary>
          <div class="run-package-tasks">
            <div
              v-for="group in taskGroupsForPackage(packageGroup)"
              :key="group.label"
              class="run-status-row"
            >
              <PTag :severity="taskGroupSeverity(group)" :value="group.label" />
              <span class="run-task-list">
                <RouterLink
                  v-for="item in group.tasks"
                  :key="item.task.name"
                  :to="taskURL(repoPath, run.commit, item.task.name)"
                >
                  <i :class="taskGroupIcon(group)" aria-hidden="true"></i>
                  {{ item.label }}
                </RouterLink>
              </span>
            </div>
          </div>
        </details>
        <div v-else class="run-package run-package-summary run-package-summary-static">
          <span class="run-package-title">
            <span>{{ packageGroup.name }}</span>
            <PTag :severity="packageSeverity(packageGroup)" :value="packageSummary(packageGroup)" />
          </span>
          <span class="run-package-task-summary">
            <template v-for="(item, taskIndex) in packageGroup.tasks" :key="item.task.name">
              <span v-if="taskIndex > 0">, </span>
              <RouterLink :to="taskURL(repoPath, run.commit, item.task.name)">
                {{ item.label }}
              </RouterLink>
            </template>
          </span>
        </div>
      </template>
    </div>
    <div
      v-else-if="!(summaryMode && !runHasLiveTasks(run) && usePackageGroups(run))"
      class="run-status-list"
    >
      <div v-for="group in runGroups(run)" :key="group.label" class="run-status-row">
        <PTag
          v-if="group.label !== 'passed'"
          :severity="groupSeverity(group)"
          :value="group.label"
        />
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

.run-status-row,
.run-task-list {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: var(--app-space-3);
  min-width: 0;
}

.run-meta {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  align-items: start;
  gap: var(--app-space-4);
  min-width: 0;
}

.run-meta-primary,
.run-meta-secondary {
  display: grid;
  gap: var(--app-space-1);
  min-width: 0;
}

.run-meta-secondary {
  justify-items: end;
  text-align: right;
}

.run-meta-time {
  font-size: var(--p-form-field-sm-font-size);
}

.attribute-list {
  display: flex;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: var(--app-space-2);
  min-width: 0;
}

.run-status-list {
  display: grid;
  gap: var(--app-space-3);
  min-width: 0;
}

.run-package-list {
  display: grid;
  min-width: 0;
}

.run-package {
  min-width: 0;
  border-top: 1px solid var(--p-content-border-color);
}

.run-package:first-child {
  border-top: 0;
}

.run-package-summary {
  display: grid;
  grid-template-columns: auto minmax(0, max-content) minmax(0, 1fr) minmax(180px, 28%);
  align-items: center;
  gap: var(--app-space-3);
  min-width: 0;
  padding: var(--app-space-3) var(--app-space-4);
  list-style: none;
}

.run-package-summary-expandable {
  cursor: pointer;
}

.run-package-summary-static {
  grid-template-columns: minmax(0, max-content) minmax(0, 1fr);
}

.run-package-summary::-webkit-details-marker {
  display: none;
}

.run-package-toggle {
  color: var(--p-text-muted-color);
  font-size: var(--p-icon-size);
  transition: transform 120ms ease;
}

.run-package[open] > .run-package-summary .run-package-toggle {
  transform: rotate(90deg);
}

.run-package-title {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-3);
  min-width: 0;
}

.run-package-detail,
.run-package-task-summary {
  min-width: 0;
  color: var(--p-text-muted-color);
  overflow-wrap: anywhere;
}

.run-package-progress {
  min-width: 0;
}

.run-package-tasks {
  display: grid;
  gap: var(--app-space-3);
  padding: 0 var(--app-space-4) var(--app-space-4)
    calc(var(--app-space-4) + var(--p-icon-size) + var(--app-space-3));
}

.run-progress {
  display: grid;
  gap: var(--app-space-2);
  min-width: 0;
}

.run-summary {
  display: flex;
  align-items: center;
  gap: var(--app-space-3);
  color: var(--p-text-muted-color);
  overflow-wrap: anywhere;
}

.run-summary-disclosure {
  min-width: 0;
}

.run-summary-expandable {
  cursor: pointer;
  list-style: none;
}

.run-summary-expandable::-webkit-details-marker {
  display: none;
}

.run-summary-disclosure[open] > .run-summary .run-package-toggle {
  transform: rotate(90deg);
}

.run-summary-package-list {
  margin-top: var(--app-space-3);
}

.run-summary .run-task-icon {
  margin-right: var(--app-space-1);
}

.run-task-list a {
  display: inline-flex;
  align-items: center;
  gap: var(--app-space-1);
}

@media (max-width: 720px) {
  .run-meta {
    grid-template-columns: minmax(0, 1fr);
  }

  .run-meta-secondary {
    justify-items: start;
    text-align: left;
  }

  .attribute-list {
    justify-content: flex-start;
  }

  .run-package-summary {
    grid-template-columns: auto minmax(0, 1fr);
  }

  .run-package-detail,
  .run-package-task-summary,
  .run-package-progress {
    grid-column: 2;
  }
}
</style>
