<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'

import { formatElapsedDuration, formatRelativeTimestamp, formatTimestamp } from '@/lib/api'

const props = defineProps<{
  startedAt: string
  finishedAt?: string
}>()

const nowMilliseconds = ref(Date.now())
let timer: ReturnType<typeof setInterval> | undefined

const duration = computed(() =>
  formatElapsedDuration(props.startedAt, props.finishedAt, nowMilliseconds.value),
)

function stopClock(): void {
  if (timer === undefined) return
  clearInterval(timer)
  timer = undefined
}

function syncClock(): void {
  stopClock()
  nowMilliseconds.value = Date.now()
  timer = setInterval(() => {
    nowMilliseconds.value = Date.now()
  }, 1000)
}

watch(() => props.finishedAt, syncClock)
onMounted(syncClock)
onUnmounted(stopClock)
</script>

<template>
  <dl class="commit-timing">
    <div class="commit-timing-item">
      <dt>Start</dt>
      <dd>
        <time :datetime="startedAt" :title="formatTimestamp(startedAt)">
          {{ formatRelativeTimestamp(startedAt, nowMilliseconds) || '—' }}
        </time>
      </dd>
    </div>
    <div class="commit-timing-item">
      <dt>End</dt>
      <dd v-if="finishedAt">
        <time :datetime="finishedAt" :title="formatTimestamp(finishedAt)">
          {{ formatRelativeTimestamp(finishedAt, nowMilliseconds) || '—' }}
        </time>
      </dd>
      <dd v-else>In progress</dd>
    </div>
    <div class="commit-timing-item">
      <dt>{{ finishedAt ? 'Duration' : 'Duration so far' }}</dt>
      <dd>{{ duration }}</dd>
    </div>
  </dl>
</template>

<style scoped>
.commit-timing {
  display: flex;
  flex-wrap: wrap;
  gap: var(--app-space-3) var(--app-space-6);
  margin: 0;
}

.commit-timing-item {
  display: grid;
  gap: var(--app-space-1);
}

dt {
  color: var(--p-text-muted-color);
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
}

dd {
  margin: 0;
  font-family: var(--app-font-family-mono);
}
</style>
