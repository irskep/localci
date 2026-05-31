<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import CommitSubject from '@/components/CommitSubject.vue'
import TopBar from '@/components/TopBar.vue'
import {
  commitSubject,
  displayStatusSeverity,
  displayTaskFailure,
  displayTaskStatus,
  formatDuration,
  shortCommit,
  type RepoTaskHistoryItem,
} from '@/lib/api'
import { commitURL, parseRepoRoute, repoPathURL, taskURL } from '@/lib/routes'
import { useDocumentTitle } from '@/lib/title'
import { useLocalciStore } from '@/stores/localci'

const route = useRoute()
const store = useLocalciStore()
const parsed = computed(() => parseRepoRoute(route.path))
const history = computed(() => store.currentRepoTaskHistory)
const rows = computed(() => history.value?.runs ?? [])
const subscribedPage = ref('')
const title = computed(() => {
  const repoLabel = history.value?.repo.repo_label ?? parsed.value.repoPath
  const taskLabel = history.value?.short_name ?? parsed.value.taskName ?? 'Task'
  return `${repoLabel} ${taskLabel}`
})

useDocumentTitle(title)

function subscribe(): void {
  if (parsed.value.kind !== 'repo-task') return
  subscribedPage.value = parsed.value.apiPath
  store.subscribeRepoTaskHistory(parsed.value.apiPath)
}

function activityTime(row: RepoTaskHistoryItem): string {
  if (!row.activity_at) return ''
  const date = new Date(row.activity_at)
  if (Number.isNaN(date.getTime())) return ''
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
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
          label: history?.repo.repo_label ?? parsed.repoPath,
          to: repoPathURL(parsed.repoPath),
        },
        { label: history?.short_name ?? parsed.taskName ?? 'Task' },
      ]"
    />

    <PMessage v-if="store.error" severity="error" :closable="false">{{ store.error }}</PMessage>
    <div v-if="store.loading && !history" class="loading-state">
      <PProgressSpinner />
      <span>Loading task history</span>
    </div>

    <PDataTable v-if="history" :value="rows" data-key="commit" size="small" class="table-surface">
      <template #header>{{ history.short_name }} history</template>
      <PColumn header="Commit">
        <template #body="{ data }">
          <RouterLink :to="commitURL(history.repo.repo_path, data.commit)" class="mono">
            {{ shortCommit(data.commit) }}
          </RouterLink>
        </template>
      </PColumn>
      <PColumn header="Message">
        <template #body="{ data }">
          <CommitSubject
            v-if="commitSubject(data.annotations)"
            :subject="commitSubject(data.annotations)"
          />
          <span v-else class="muted">No commit subject</span>
        </template>
      </PColumn>
      <PColumn header="Status">
        <template #body="{ data }">
          <RouterLink :to="taskURL(history.repo.repo_path, data.commit, history.task)">
            <PTag
              :severity="displayStatusSeverity(data.task)"
              :value="displayTaskStatus(data.task)"
            />
          </RouterLink>
        </template>
      </PColumn>
      <PColumn header="Attempt">
        <template #body="{ data }">
          <span v-if="data.task.attempt > 0">
            {{ data.task.attempt }}
            <template v-if="data.task.attempt_count > data.task.attempt">
              of {{ data.task.attempt_count }}</template
            >
          </span>
          <span v-else class="muted">not run</span>
        </template>
      </PColumn>
      <PColumn header="Duration">
        <template #body="{ data }">{{ formatDuration(data.task.duration_ms) }}</template>
      </PColumn>
      <PColumn header="Activity">
        <template #body="{ data }">{{ activityTime(data) }}</template>
      </PColumn>
      <PColumn header="Failure">
        <template #body="{ data }">
          <span class="muted">{{ displayTaskFailure(data.task) }}</span>
        </template>
      </PColumn>
      <template #empty>No history recorded for this task.</template>
    </PDataTable>
  </main>
</template>
